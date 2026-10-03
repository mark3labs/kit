//go:build !windows

package session

import (
	"os"
	"syscall"
)

// lockFileExclusive takes an advisory exclusive lock on the whole file
// without blocking. Returns an error when another process holds the lock.
func lockFileExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// unlockFile releases the advisory lock.
func unlockFile(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// isLockBusy reports whether err means "another process holds the lock".
// Every other error is treated as "cannot lock here" and must not fail the
// open.
func isLockBusy(err error) bool {
	return err == syscall.EWOULDBLOCK || err == syscall.EAGAIN
}
