// Package watcher provides a general-purpose file watcher that monitors
// directories for changes to files matching specified extensions. It uses
// fsnotify for kernel-level notifications with debouncing to coalesce
// rapid editor writes.
package watcher

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

// ContentWatcher monitors directories for file changes matching a set of
// extensions and triggers a reload callback when changes are detected.
// It uses fsnotify for kernel-level file notifications (inotify on Linux,
// kqueue on macOS) with debouncing to coalesce rapid editor writes.
type ContentWatcher struct {
	watcher    *fsnotify.Watcher
	onReload   func()
	extensions []string // e.g. [".md", ".txt"]
	label      string   // for logging (e.g. "prompts", "skills")
	debounce   time.Duration
	cancel     context.CancelFunc
	done       chan struct{}
	mu         sync.Mutex
}

// Options configures a ContentWatcher.
type Options struct {
	// Dirs are the directories to watch.
	Dirs []string
	// Extensions are the file extensions to watch for (e.g. ".md", ".txt").
	// Include the leading dot.
	Extensions []string
	// OnReload is called when a matching file changes (after debouncing).
	OnReload func()
	// Label is a human-readable name for logging (e.g. "prompts", "skills").
	Label string
	// Debounce is the debounce duration. Defaults to 300ms if zero.
	Debounce time.Duration
}

// New creates a ContentWatcher that monitors the given directories for
// file changes matching the specified extensions. When a change is detected
// (after debouncing), onReload is called. The watcher must be started with
// Start() and stopped with Close().
func New(opts Options) (*ContentWatcher, error) {
	if len(opts.Dirs) == 0 {
		return nil, fmt.Errorf("no directories to watch")
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating file watcher: %w", err)
	}

	for _, dir := range opts.Dirs {
		if err := fsw.Add(dir); err != nil {
			log.Debug("watcher: skipping directory", "label", opts.Label, "dir", dir, "err", err)
			continue
		}

		// Also watch immediate subdirectories (for skill/SKILL.md pattern).
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				subdir := filepath.Join(dir, entry.Name())
				if err := fsw.Add(subdir); err != nil {
					log.Debug("watcher: skipping subdirectory", "label", opts.Label, "dir", subdir, "err", err)
				}
			}
		}
	}

	debounce := opts.Debounce
	if debounce == 0 {
		debounce = 300 * time.Millisecond
	}

	return &ContentWatcher{
		watcher:    fsw,
		onReload:   opts.OnReload,
		extensions: opts.Extensions,
		label:      opts.Label,
		debounce:   debounce,
		done:       make(chan struct{}),
	}, nil
}

// Start begins watching for file changes. It blocks until the context
// is cancelled or Close() is called. Typically called in a goroutine.
func (w *ContentWatcher) Start(ctx context.Context) {
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

			// Only care about files matching our extensions.
			if !w.matchesExtension(event.Name) {
				continue
			}

			// React to write, create, remove, rename events.
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			log.Debug("watcher: file changed", "label", w.label, "file", event.Name, "op", event.Op)

			// Debounce: reset timer on each event.
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(w.debounce)
			timerC = timer.C

		case <-timerC:
			timerC = nil
			timer = nil
			log.Debug("watcher: reloading", "label", w.label)
			w.onReload()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Warn("watcher: error", "label", w.label, "err", err)
		}
	}
}

// Close stops the watcher and releases resources.
func (w *ContentWatcher) Close() error {
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

// matchesExtension returns true if the file name ends with one of the
// watched extensions.
func (w *ContentWatcher) matchesExtension(name string) bool {
	for _, ext := range w.extensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// CollectDirs returns the directories to watch for a given set of standard
// directories and extra paths. Directories are deduplicated by absolute path
// and verified to exist. For explicit file paths, the parent directory is
// watched instead.
func CollectDirs(standardDirs []string, extraPaths []string) []string {
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

	for _, d := range standardDirs {
		add(d)
	}

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
