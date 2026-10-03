package session

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// syncSessionDir persists a created or replaced directory entry. Windows
// does not support syncing an ordinary directory handle.
func syncSessionDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}

// createSessionDirs persists each newly created directory in its parent.
func createSessionDirs(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return fmt.Errorf("missing filesystem root: %s", path)
	}
	if err := createSessionDirs(parent); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	return syncSessionDir(parent)
}
