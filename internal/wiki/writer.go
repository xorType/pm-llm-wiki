package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Wiki manages the wiki directory tree.
type Wiki struct {
	Root string // absolute path to wiki/
}

// New creates a Wiki manager rooted at root.
func New(root string) *Wiki {
	return &Wiki{Root: root}
}

// ClientDir returns the per-client subdirectory, creating it if needed.
func (w *Wiki) ClientDir(client string) string {
	dir := filepath.Join(w.Root, client)
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// WritePage writes content to wiki/{client}/{relPath}, creating any intermediate
// directories. relPath is a slash-separated path relative to the client dir
// (e.g. "meetings/2024-01-15-kickoff.md").
func (w *Wiki) WritePage(client, relPath, content string) error {
	abs := filepath.Join(w.Root, client, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, []byte(content), 0o644)
}

// ReadPage returns the content of a wiki page, or "" if it does not exist.
func (w *Wiki) ReadPage(client, relPath string) string {
	b, _ := os.ReadFile(filepath.Join(w.Root, client, filepath.FromSlash(relPath)))
	return string(b)
}

// PageExists reports whether a wiki page exists.
func (w *Wiki) PageExists(client, relPath string) bool {
	_, err := os.Stat(filepath.Join(w.Root, client, filepath.FromSlash(relPath)))
	return err == nil
}

// AppendLog appends a timestamped entry to wiki/{client}/log.md.
func (w *Wiki) AppendLog(client, entry string) error {
	dir := w.ClientDir(client)
	path := filepath.Join(dir, "log.md")

	// Bootstrap the file if it doesn't exist yet.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		header := "# Ingest Log\n\nAppend-only record of ingests, queries, and maintenance passes.\n\n"
		if err := os.WriteFile(path, []byte(header), 0o644); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "\n## [%s] %s\n\n", time.Now().Format("2006-01-02 15:04"), entry)
	return err
}

// UpdateIndex rebuilds wiki/{client}/index.md with a catalog of all pages.
func (w *Wiki) UpdateIndex(client string) error {
	dir := w.ClientDir(client)

	var lines []string
	lines = append(lines,
		fmt.Sprintf("# %s — Project Wiki Index\n", client),
		fmt.Sprintf("_Last updated: %s_\n", time.Now().Format("2006-01-02 15:04")),
		"\n---\n",
	)

	// Collect pages grouped by sub-directory.
	groups := map[string][]string{}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if rel == "index.md" || rel == "log.md" {
			return nil
		}
		group := filepath.ToSlash(filepath.Dir(rel))
		if group == "." {
			group = "core"
		}
		groups[group] = append(groups[group], rel)
		return nil
	})

	// Emit core pages first, then sub-directories.
	for _, group := range []string{"core", "meetings", "entities", "summaries"} {
		pages, ok := groups[group]
		if !ok {
			continue
		}
		lines = append(lines, fmt.Sprintf("\n## %s\n", strings.Title(group)))
		for _, p := range pages {
			name := strings.TrimSuffix(filepath.Base(p), ".md")
			lines = append(lines, fmt.Sprintf("- [%s](%s)", name, p))
		}
		delete(groups, group)
	}
	// Remaining groups.
	for group, pages := range groups {
		lines = append(lines, fmt.Sprintf("\n## %s\n", strings.Title(group)))
		for _, p := range pages {
			name := strings.TrimSuffix(filepath.Base(p), ".md")
			lines = append(lines, fmt.Sprintf("- [%s](%s)", name, p))
		}
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filepath.Join(dir, "index.md"), []byte(content), 0o644)
}

// EnsureClientBootstrap creates the skeleton pages for a new client if they
// don't already exist.
func (w *Wiki) EnsureClientBootstrap(client string) error {
	// Create the client directory before trying to write any files into it.
	w.ClientDir(client)

	stubs := map[string]string{
		"sow.md":          fmt.Sprintf("---\ntype: sow\nproject: %s\n---\n# Statement of Work\n\n_Not yet ingested. Drop a SOW document into raw/%s/_\n", client, client),
		"timeline.md":     fmt.Sprintf("---\ntype: timeline\nproject: %s\n---\n# Timeline & Milestones\n\n| Milestone | Owner | Due Date | Status |\n|---|---|---|---|\n", client),
		"decisions.md":    fmt.Sprintf("---\ntype: decisions\nproject: %s\n---\n# Decisions Log\n\n", client),
		"risks.md":        fmt.Sprintf("---\ntype: risks\nproject: %s\n---\n# Risk Register\n\n| # | Risk | Likelihood | Impact | Mitigation | Owner |\n|---|---|---|---|---|---|\n", client),
		"deliverables.md": fmt.Sprintf("---\ntype: deliverables\nproject: %s\n---\n# Deliverables Tracker\n\n| Deliverable | Owner | Due Date | Status | Notes |\n|---|---|---|---|---|\n", client),
	}

	for name, content := range stubs {
		path := filepath.Join(w.Root, client, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("bootstrapping %s: %w", name, err)
			}
		}
	}

	// Ensure sub-directories exist.
	for _, sub := range []string{"meetings", "entities", "summaries"} {
		_ = os.MkdirAll(filepath.Join(w.Root, client, sub), 0o755)
	}

	return w.UpdateIndex(client)
}
