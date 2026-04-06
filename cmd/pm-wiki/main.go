package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pm-wiki/pm-wiki/internal/ollama"
	"github.com/pm-wiki/pm-wiki/internal/processor"
	"github.com/pm-wiki/pm-wiki/internal/watcher"
)

func main() {
	// ── Flags ──────────────────────────────────────────────────────────────────
	rawDir := flag.String("raw", "raw", "Directory to watch for new source files")
	wikiDir := flag.String("wiki", "wiki", "Directory where the wiki is maintained")
	schema := flag.String("schema", "PM-WIKI.md", "Path to the PM-WIKI schema file")
	ollamaURL := flag.String("ollama-url", ollama.DefaultBaseURL, "Ollama API base URL")
	model := flag.String("model", ollama.DefaultModel, "Ollama model name")
	flag.Parse()

	// Resolve to absolute paths so the watcher and processor agree on roots.
	rawAbs := mustAbs(*rawDir)
	wikiAbs := mustAbs(*wikiDir)
	schemaAbs := mustAbs(*schema)

	// ── Directory bootstrap ────────────────────────────────────────────────────
	for _, dir := range []string{rawAbs, wikiAbs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("creating directory %s: %v", dir, err)
		}
	}

	// ── Ollama health check ────────────────────────────────────────────────────
	ollamaClient := ollama.New(*ollamaURL, *model)
	log.Printf("pm-wiki starting")
	log.Printf("  model      : %s", *model)
	log.Printf("  ollama URL : %s", *ollamaURL)
	log.Printf("  raw dir    : %s", rawAbs)
	log.Printf("  wiki dir   : %s", wikiAbs)
	log.Printf("  schema     : %s", schemaAbs)

	log.Print("checking Ollama connection...")
	if err := ollamaClient.Ping(); err != nil {
		log.Fatalf("Ollama check failed: %v", err)
	}
	log.Printf("Ollama OK — model %q ready", *model)

	// ── Processor ─────────────────────────────────────────────────────────────
	proc, err := processor.New(rawAbs, wikiAbs, schemaAbs, ollamaClient)
	if err != nil {
		log.Fatalf("processor init: %v", err)
	}

	// ── Signal handling ────────────────────────────────────────────────────────
	done := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Printf("received %s — shutting down", sig)
		close(done)
	}()

	// ── Watch ─────────────────────────────────────────────────────────────────
	log.Printf("watching %s for new files — drop a file into raw/{ClientName}/ to begin", rawAbs)
	if err := watcher.Watch(rawAbs, proc.Handle, done); err != nil {
		log.Fatalf("watcher error: %v", err)
	}
	log.Print("pm-wiki stopped")
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("resolving path %q: %v", path, err)
	}
	return abs
}
