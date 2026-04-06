package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounce = 2 * time.Second

// Handler is called with the absolute path of each new file detected.
type Handler func(path string) error

// Watch monitors root and all sub-directories recursively for newly created
// files and calls h for each one. It blocks until done is closed.
func Watch(root string, h Handler, done <-chan struct{}) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	// Add all existing subdirectories.
	if err := addDirs(w, root); err != nil {
		return err
	}

	log.Printf("[watcher] watching %s", root)

	// pending holds debounce timers keyed by file path.
	pending := make(map[string]*time.Timer)
	var mu sync.Mutex

	for {
		select {
		case <-done:
			return nil

		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !event.Has(fsnotify.Create) {
				continue
			}

			fi, err := os.Stat(event.Name)
			if err != nil {
				continue
			}
			// If a new directory appeared, watch it too.
			if fi.IsDir() {
				_ = w.Add(event.Name)
				continue
			}
			if skipFile(event.Name) {
				continue
			}

			// Debounce: reset the timer every time a create fires for the same
			// path so we wait until the copy/download is fully settled.
			mu.Lock()
			if t, ok := pending[event.Name]; ok {
				t.Stop()
			}
			path := event.Name
			pending[path] = time.AfterFunc(debounce, func() {
				mu.Lock()
				delete(pending, path)
				mu.Unlock()

				log.Printf("[watcher] processing: %s", path)
				if err := h(path); err != nil {
					log.Printf("[watcher] handler error for %s: %v", path, err)
				}
			})
			mu.Unlock()

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			log.Printf("[watcher] error: %v", err)
		}
	}
}

// addDirs recursively adds all directories under root to the watcher.
func addDirs(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if d.IsDir() {
			return w.Add(path)
		}
		return nil
	})
}

// skipFile returns true for hidden files, temp files, and partial downloads.
func skipFile(name string) bool {
	base := filepath.Base(name)
	if strings.HasPrefix(base, ".") || strings.HasPrefix(base, "~$") {
		return true
	}
	for _, sfx := range []string{".tmp", ".crdownload", ".part", ".swp", ".lock"} {
		if strings.HasSuffix(strings.ToLower(base), sfx) {
			return true
		}
	}
	return false
}
