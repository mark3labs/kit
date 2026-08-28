//go:build windows

package daemon

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFileExclusive takes an advisory exclusive lock on the first byte of
// the file without blocking (LockFileEx fails immediately when another
// process holds the region), mirroring flock(LOCK_EX|LOCK_NB) on Unix.
func lockFileExclusive(f *os.File) error {
	overlapped := new(windows.Overlapped)
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, overlapped,
	)
}

// unlockFile releases the region locked by lockFileExclusive.
func unlockFile(f *os.File) {
	overlapped := new(windows.Overlapped)
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlapped)
}
