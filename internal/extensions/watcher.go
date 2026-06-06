package extensions

import (
	"os"
	"path/filepath"

	"github.com/mark3labs/kit/internal/watcher"
)

// Watcher monitors extension directories for .go file changes and triggers
// a reload callback when changes are detected. It is implemented in terms
// of the general-purpose internal/watcher.ContentWatcher.
//
// Type-aliasing here lets existing call sites (cmd/root.go and the
// watcher_test.go suite) keep using `extensions.NewWatcher` / `*Watcher`
// without knowing about the underlying implementation.
type Watcher = watcher.ContentWatcher

// NewWatcher creates a file watcher that monitors the given directories
// for .go file changes. When a change is detected (after debouncing),
// onReload is called. The watcher must be started with Start() and
// stopped with Close().
func NewWatcher(dirs []string, onReload func()) (*Watcher, error) {
	return watcher.New(watcher.Options{
		Dirs:       dirs,
		Extensions: []string{".go"},
		OnReload:   onReload,
		Label:      "extensions",
	})
}

// WatchedDirs returns the directories to watch for extension changes.
// This includes the global extensions directory and the project-local
// .kit/extensions/ directory (if they exist). Explicit -e paths that
// point to directories are also included; explicit file paths cause
// their parent directory to be watched instead.
func WatchedDirs(extraPaths []string) []string {
	standard := []string{
		globalExtensionsDir(),
		filepath.Join(".kit", "extensions"),
	}

	// Filter explicit paths into directories (passed through) and files
	// (parent dir watched) for CollectDirs to dedupe.
	var extras []string
	for _, p := range extraPaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			extras = append(extras, p)
		} else {
			extras = append(extras, filepath.Dir(p))
		}
	}

	return watcher.CollectDirs(standard, extras)
}
