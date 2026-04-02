package extensions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
)

// Watcher monitors extension directories for file changes and triggers
// a reload callback when .go files are created, modified, or removed.
// It uses fsnotify for kernel-level file notifications (inotify on Linux,
// kqueue on macOS) with debouncing to coalesce rapid editor writes.
type Watcher struct {
	watcher  *fsnotify.Watcher
	onReload func()
	debounce time.Duration
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.Mutex
}

// NewWatcher creates a file watcher that monitors the given directories
// for .go file changes. When a change is detected (after debouncing),
// onReload is called. The watcher must be started with Start() and
// stopped with Close().
func NewWatcher(dirs []string, onReload func()) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating file watcher: %w", err)
	}

	for _, dir := range dirs {
		// Watch the directory itself.
		if err := fsw.Add(dir); err != nil {
			log.Debug("watcher: skipping directory", "dir", dir, "err", err)
			continue
		}

		// Also watch immediate subdirectories (for */main.go pattern).
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				subdir := filepath.Join(dir, entry.Name())
				if err := fsw.Add(subdir); err != nil {
					log.Debug("watcher: skipping subdirectory", "dir", subdir, "err", err)
				}
			}
		}
	}

	return &Watcher{
		watcher:  fsw,
		onReload: onReload,
		debounce: 300 * time.Millisecond,
		done:     make(chan struct{}),
	}, nil
}

// Start begins watching for file changes. It blocks until the context
// is cancelled or Close() is called. Typically called in a goroutine.
func (w *Watcher) Start(ctx context.Context) {
	w.mu.Lock()
	ctx, w.cancel = context.WithCancel(ctx)
	w.mu.Unlock()

	defer close(w.done)

	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only care about .go files.
			if !strings.HasSuffix(event.Name, ".go") {
				continue
			}

			// React to write, create, remove, rename events.
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			log.Debug("watcher: file changed", "file", event.Name, "op", event.Op)

			// Debounce: reset timer on each event.
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(w.debounce)
			timerC = timer.C

		case <-timerC:
			timerC = nil
			timer = nil
			log.Debug("watcher: reloading extensions")
			w.onReload()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Warn("watcher: error", "err", err)
		}
	}
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	w.mu.Lock()
	cancel := w.cancel
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	// Wait for the event loop to finish.
	<-w.done
	return w.watcher.Close()
}

// WatchedDirs returns the directories to watch for extension changes.
// This includes the global extensions directory and the project-local
// .kit/extensions/ directory (if they exist). Explicit -e paths that
// point to directories are also included; explicit file paths cause
// their parent directory to be watched instead.
func WatchedDirs(extraPaths []string) []string {
	var dirs []string
	seen := make(map[string]bool)

	add := func(dir string) {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return
		}
		if seen[abs] {
			return
		}

		// Verify the directory exists.
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			return
		}

		seen[abs] = true
		dirs = append(dirs, abs)
	}

	// Global extensions dir.
	add(globalExtensionsDir())

	// Project-local extensions dir.
	add(filepath.Join(".kit", "extensions"))

	// Explicit paths that are directories.
	for _, p := range extraPaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			add(p)
		} else {
			// For explicit files, watch the parent directory.
			add(filepath.Dir(p))
		}
	}

	return dirs
}
