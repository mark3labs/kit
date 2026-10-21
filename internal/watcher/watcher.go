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

	// roots are the configured directories, rescanned for symlinked entries.
	roots []string
	// watchedDirs are directories watched for all their matching files.
	watchedDirs map[string]bool
	// linkTargets are the resolved targets of symlinked entries in roots, and
	// linkParents the directories watched only so that edits to, or a
	// replacement of, those targets are seen. Other events in linkParents
	// are ignored. Only the Start goroutine touches these after New.
	linkTargets map[string]bool
	linkParents map[string]bool
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

	debounce := opts.Debounce
	if debounce == 0 {
		debounce = 300 * time.Millisecond
	}

	w := &ContentWatcher{
		watcher:     fsw,
		onReload:    opts.OnReload,
		extensions:  opts.Extensions,
		label:       opts.Label,
		debounce:    debounce,
		done:        make(chan struct{}),
		roots:       opts.Dirs,
		watchedDirs: make(map[string]bool),
		linkTargets: make(map[string]bool),
		linkParents: make(map[string]bool),
	}

	for _, dir := range opts.Dirs {
		if err := fsw.Add(dir); err != nil {
			continue
		}
		w.watchedDirs[dir] = true

		// Also watch immediate subdirectories (for skill/SKILL.md pattern).
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				subdir := filepath.Join(dir, entry.Name())
				if fsw.Add(subdir) == nil {
					w.watchedDirs[subdir] = true
				}
			}
		}
	}
	w.watchSymlinkTargets()

	return w, nil
}

// watchSymlinkTargets resolves the symlinked entries in the root directories
// and watches their targets. fsnotify does not follow links, so an edit to a
// target outside the watched tree would otherwise go unnoticed. A linked
// directory is watched directly; for both linked directories and linked
// files the target's parent is watched too, so a replaced target (removed
// and created again, as a deployment does) is picked up. Adding an existing
// watch is a no-op, so this is safe to call again after any change.
func (w *ContentWatcher) watchSymlinkTargets() {
	for _, dir := range w.roots {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink == 0 {
				continue
			}
			target, err := filepath.EvalSymlinks(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			info, err := os.Stat(target)
			if err != nil {
				continue
			}
			if info.IsDir() {
				if w.watcher.Add(target) == nil {
					w.watchedDirs[target] = true
				}
			} else if !w.matchesExtension(entry.Name()) {
				continue
			}
			parent := filepath.Dir(target)
			if w.watcher.Add(parent) == nil {
				w.linkTargets[target] = true
				w.linkParents[parent] = true
			}
		}
	}
}

// isUnrelatedLinkEvent reports whether name is in a directory watched only
// for symlink targets and is not one of those targets.
func (w *ContentWatcher) isUnrelatedLinkEvent(name string) bool {
	dir := filepath.Dir(name)
	return w.linkParents[dir] && !w.watchedDirs[dir] && !w.linkTargets[name]
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

			if w.isUnrelatedLinkEvent(event.Name) {
				continue
			}

			// A symlink or its target may have been added, removed or
			// replaced; refresh the target watches.
			if event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				w.watchSymlinkTargets()
			}

			// When a new subdirectory is created, start watching it so
			// that files added inside (e.g. new-skill/SKILL.md) trigger
			// reload events. Also schedule a reload in case the directory
			// was created with matching files already inside.
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if addErr := w.watcher.Add(event.Name); addErr == nil {
						w.watchedDirs[event.Name] = true
						// Check if the new directory already contains matching files.
						if w.dirContainsMatchingFiles(event.Name) {
							if timer != nil {
								timer.Stop()
							}
							timer = time.NewTimer(w.debounce)
							timerC = timer.C
						}
					}
					continue
				}
			}

			// Only care about files matching our extensions. A symlinked
			// file's target may be named without one.
			if !w.matchesExtension(event.Name) && !w.linkTargets[event.Name] {
				continue
			}

			// React to write, create, remove, rename events.
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			// Debounce: reset timer on each event.
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(w.debounce)
			timerC = timer.C

		case <-timerC:
			timerC = nil
			timer = nil
			w.onReload()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			_ = err
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

// dirContainsMatchingFiles returns true if the directory contains at least
// one file matching the watched extensions. Used to detect cases where a
// directory is created with files already inside (e.g. cp -r).
func (w *ContentWatcher) dirContainsMatchingFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && w.matchesExtension(entry.Name()) {
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
