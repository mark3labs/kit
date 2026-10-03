package session

import (
	"os"
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
