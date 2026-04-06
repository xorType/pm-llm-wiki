package processor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pm-wiki/pm-wiki/internal/converter"
	"github.com/pm-wiki/pm-wiki/internal/ollama"
	"github.com/pm-wiki/pm-wiki/internal/wiki"
)

// pageBlockRe matches  <<<PAGE: path>>> ... content ... <<<END>>>
var pageBlockRe = regexp.MustCompile(`(?s)<<<PAGE:\s*([^>]+)>>>(.*?)<<<END>>>`)

// slugRe strips characters that are unsafe in filenames.
var slugRe = regexp.MustCompile(`[^\w\-./]`)

// Processor orchestrates the full ingest pipeline for a single raw file.
type Processor struct {
	RawRoot  string // absolute path to raw/
	WikiRoot string // absolute path to wiki/
	Schema   string // contents of PM-WIKI.md
	Ollama   *ollama.Client
	Wiki     *wiki.Wiki
}

// New creates a Processor. schemaPath is the path to PM-WIKI.md.
func New(rawRoot, wikiRoot, schemaPath string, ollamaClient *ollama.Client) (*Processor, error) {
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("reading schema %s: %w", schemaPath, err)
	}
	return &Processor{
		RawRoot:  rawRoot,
		WikiRoot: wikiRoot,
		Schema:   string(schema),
		Ollama:   ollamaClient,
		Wiki:     wiki.New(wikiRoot),
	}, nil
}

// Handle is the watcher Handler — called for every new file in raw/.
func (p *Processor) Handle(rawPath string) error {
	start := time.Now()

	// Determine the client name from the sub-folder directly under raw/.
	client, err := p.clientFromPath(rawPath)
	if err != nil {
		return err
	}

	log.Printf("[processor] client=%s  file=%s", client, filepath.Base(rawPath))

	// Bootstrap the client wiki if this is the first file.
	if err := p.Wiki.EnsureClientBootstrap(client); err != nil {
		return fmt.Errorf("bootstrap %s: %w", client, err)
	}

	// Step 1: convert the raw file to text.
	text, err := converter.ToText(rawPath)
	if err != nil {
		return fmt.Errorf("convert %s: %w", rawPath, err)
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("empty content after converting %s", rawPath)
	}

	// Step 2: load relevant existing wiki pages as context for the LLM.
	existingContext := p.loadExistingContext(client)

	// Step 3: build the prompt.
	prompt := p.buildPrompt(client, rawPath, text, existingContext)

	// Step 4: call Ollama.
	log.Printf("[processor] calling Ollama (model: %s)...", p.Ollama.Model)
	response, err := p.Ollama.Generate(prompt)
	if err != nil {
		return fmt.Errorf("ollama generate: %w", err)
	}

	// Step 5: parse page blocks from response and write them.
	pagesWritten, err := p.writePageBlocks(response)
	if err != nil {
		return fmt.Errorf("writing pages: %w", err)
	}

	if len(pagesWritten) == 0 {
		log.Printf("[processor] WARNING: Ollama returned no page blocks for %s", rawPath)
		// Fallback: write the raw response as a summary.
		slug := fileSlug(rawPath)
		fallbackPath := fmt.Sprintf("summaries/%s.md", slug)
		content := fmt.Sprintf("---\ntype: summary\nsource: %s\ningested: %s\nproject: %s\n---\n\n# %s\n\n%s\n",
			rawPath, time.Now().Format("2006-01-02 15:04"), client, slug, response)
		if err := p.Wiki.WritePage(client, fallbackPath, content); err != nil {
			return err
		}
		pagesWritten = []string{fallbackPath}
	}

	// Step 6: update index and log.
	if err := p.Wiki.UpdateIndex(client); err != nil {
		log.Printf("[processor] index update error: %v", err)
	}

	elapsed := time.Since(start).Round(time.Second)
	logEntry := fmt.Sprintf("ingest | %s | pages: %s | elapsed: %s",
		filepath.Base(rawPath), strings.Join(pagesWritten, ", "), elapsed)
	if err := p.Wiki.AppendLog(client, logEntry); err != nil {
		log.Printf("[processor] log error: %v", err)
	}

	log.Printf("[processor] done — %d pages written in %s", len(pagesWritten), elapsed)
	return nil
}

// clientFromPath infers the client name from the directory name immediately
// under raw/. e.g. raw/AcmeCorp/sow.pdf → "AcmeCorp"
func (p *Processor) clientFromPath(rawPath string) (string, error) {
	rel, err := filepath.Rel(p.RawRoot, rawPath)
	if err != nil {
		return "", fmt.Errorf("path %s is not under raw root %s", rawPath, p.RawRoot)
	}
	parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
	if len(parts) < 2 || parts[0] == "" {
		return "", fmt.Errorf("drop files into raw/{ClientName}/ — got %s", rel)
	}
	return parts[0], nil
}

// loadExistingContext reads the core wiki pages for a client to give the LLM
// awareness of what's already known.
func (p *Processor) loadExistingContext(client string) string {
	corePages := []string{"index.md", "sow.md", "timeline.md", "deliverables.md", "decisions.md", "risks.md"}
	var sb strings.Builder
	for _, name := range corePages {
		content := p.Wiki.ReadPage(client, name)
		if content == "" {
			continue
		}
		fmt.Fprintf(&sb, "\n\n--- EXISTING WIKI PAGE: %s ---\n%s", name, content)
	}
	return sb.String()
}

// buildPrompt assembles the full LLM prompt from the schema, existing context,
// and the new source document.
func (p *Processor) buildPrompt(client, rawPath, docText, existingCtx string) string {
	filename := filepath.Base(rawPath)
	now := time.Now().Format("2006-01-02 15:04")

	return fmt.Sprintf(`%s

---

## INGEST TASK

**Client:** %s
**File:** %s
**Date:** %s

You are ingesting the following document into the PM-Wiki for client %q.

### EXISTING WIKI STATE
%s

### NEW SOURCE DOCUMENT
%s

---

## YOUR TASK

1. Read the source document carefully.
2. Classify its type (transcript / sow / schedule / deliverable / financial / general).
3. Extract all PM-relevant information per the schema above.
4. Write the required wiki pages using the exact response format:

   <<<PAGE: wiki/%s/path/to/page.md>>>
   [full markdown content]
   <<<END>>>

IMPORTANT:
- Emit a summaries/ page for this document.
- Emit ALL pages that need creating or updating (include full content, not diffs).
- Use today's date %s in frontmatter.
- Do not emit any text outside the <<<PAGE>>> blocks.
- Never fabricate information. Only use what is in the source document.
- If something is unclear, mark it [UNCLEAR].
`,
		p.Schema,
		client, filename, now,
		client,
		existingCtx,
		docText,
		client,
		now,
	)
}

// writePageBlocks parses the LLM response for <<<PAGE:...>>>...<<<END>>> blocks
// and writes each one to disk. Returns the list of relative paths written.
func (p *Processor) writePageBlocks(response string) ([]string, error) {
	matches := pageBlockRe.FindAllStringSubmatch(response, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	var written []string
	for _, m := range matches {
		rawPath := strings.TrimSpace(m[1]) // e.g. wiki/AcmeCorp/meetings/2024-01-15-kickoff.md
		content := strings.TrimSpace(m[2])

		// Parse "wiki/{client}/{relPath}" from the page path.
		// Strip leading "wiki/" prefix.
		trimmed := strings.TrimPrefix(rawPath, "wiki/")
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) != 2 {
			log.Printf("[processor] skipping malformed page path: %s", rawPath)
			continue
		}
		client, relPath := parts[0], parts[1]

		// Sanitize the relative path to prevent path traversal.
		relPath = sanitizePath(relPath)
		if relPath == "" {
			log.Printf("[processor] skipping empty/unsafe page path from %s", rawPath)
			continue
		}

		if err := p.Wiki.WritePage(client, relPath, content+"\n"); err != nil {
			return written, fmt.Errorf("writing %s: %w", relPath, err)
		}
		log.Printf("[processor] wrote %s/%s", client, relPath)
		written = append(written, relPath)
	}
	return written, nil
}

// sanitizePath cleans a relative wiki page path and rejects any traversal attempts.
func sanitizePath(rel string) string {
	// Normalize to forward slashes, clean, reject traversal.
	clean := filepath.ToSlash(filepath.Clean(rel))
	if strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") {
		return ""
	}
	return clean
}

// fileSlug turns a file path into a URL-safe slug for use in wiki page names.
func fileSlug(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	slug := strings.ToLower(base)
	slug = slugRe.ReplaceAllString(slug, "-")
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}
