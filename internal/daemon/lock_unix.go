//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

// lockFileExclusive takes an advisory exclusive lock on the file without
// blocking. Returns an error when another process holds the lock.
func lockFileExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// unlockFile releases the advisory lock.
func unlockFile(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
