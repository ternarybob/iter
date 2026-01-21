package index

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors file system changes and triggers reindexing.
type Watcher struct {
	indexer    *Indexer
	watcher    *fsnotify.Watcher
	debounceMs int

	running bool
	stopCh  chan struct{}
	mu      sync.RWMutex

	// Debouncing state
	pending   map[string]time.Time
	pendingMu sync.Mutex
}

// NewWatcher creates a new file system watcher.
func NewWatcher(indexer *Indexer) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	return &Watcher{
		indexer:    indexer,
		watcher:    fsWatcher,
		debounceMs: indexer.cfg.DebounceMs,
		stopCh:     make(chan struct{}),
		pending:    make(map[string]time.Time),
	}, nil
}

// Start begins watching for file changes.
func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.mu.Unlock()

	// Add directories to watch
	if err := w.addDirectories(); err != nil {
		return fmt.Errorf("add directories: %w", err)
	}

	// Start event processing goroutine
	go w.processEvents()

	// Start debounce processor
	go w.processDebounced()

	return nil
}

// Stop stops the file watcher.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	w.running = false
	close(w.stopCh)

	return w.watcher.Close()
}

// IsRunning returns whether the watcher is active.
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// addDirectories recursively adds directories to watch.
func (w *Watcher) addDirectories() error {
	cfg := w.indexer.GetConfig()

	return filepath.Walk(cfg.RepoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		// Skip excluded directories
		rel, _ := filepath.Rel(cfg.RepoRoot, path)
		if w.shouldSkipDir(rel) {
			return filepath.SkipDir
		}

		// Add directory to watcher
		if err := w.watcher.Add(path); err != nil {
			// Log but don't fail - some directories might not be accessible
			fmt.Fprintf(os.Stderr, "warning: cannot watch %s: %v\n", path, err)
		}

		return nil
	})
}

// shouldSkipDir checks if a directory should be skipped.
func (w *Watcher) shouldSkipDir(relPath string) bool {
	skipDirs := []string{"vendor", ".git", "node_modules", ".iter"}

	for _, dir := range skipDirs {
		if relPath == dir || strings.HasPrefix(relPath, dir+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// processEvents handles file system events.
func (w *Watcher) processEvents() {
	for {
		select {
		case <-w.stopCh:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only process Go files
			if !strings.HasSuffix(event.Name, ".go") {
				continue
			}

			// Only process write/create events
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Add to pending with current timestamp
			w.pendingMu.Lock()
			w.pending[event.Name] = time.Now()
			w.pendingMu.Unlock()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
		}
	}
}

// processDebounced processes pending file changes after debounce delay.
func (w *Watcher) processDebounced() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return

		case <-ticker.C:
			w.processPendingFiles()
		}
	}
}

// processPendingFiles indexes files that have been stable long enough.
func (w *Watcher) processPendingFiles() {
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	now := time.Now()
	debounce := time.Duration(w.debounceMs) * time.Millisecond

	for path, ts := range w.pending {
		// Check if file has been stable long enough
		if now.Sub(ts) < debounce {
			continue
		}

		// Remove from pending
		delete(w.pending, path)

		// Check if file still exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		// Index the file
		if err := w.indexer.IndexFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "error indexing %s: %v\n", path, err)
		}
	}
}

// WatchGitHead watches .git/HEAD for branch changes.
func (w *Watcher) WatchGitHead() error {
	cfg := w.indexer.GetConfig()
	gitHeadPath := filepath.Join(cfg.RepoRoot, ".git", "HEAD")

	// Check if .git/HEAD exists
	if _, err := os.Stat(gitHeadPath); os.IsNotExist(err) {
		return nil // Not a git repository
	}

	return w.watcher.Add(filepath.Dir(gitHeadPath))
}
