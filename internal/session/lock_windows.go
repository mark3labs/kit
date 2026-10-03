//go:build windows

package session

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockRegionOffsetHigh places the locked byte far beyond any file's end.
// Windows byte-range locks are mandatory, not advisory: a handle that takes
// the lock blocks every OTHER handle — including ones in the same process —
// from reading or writing inside the region. The lock therefore must not
// overlap the file's data at all, and locking one byte at offset
// 0x7FFFFFFF00000000 (well under the 2^63-1 limit, unimaginably above any
// session file) keeps readers of the header and appenders at EOF working.
// Locks beyond EOF are legal on Windows, mirroring flock on Unix.
const lockRegionOffsetHigh = 0x7FFFFFFF

// lockFileExclusive takes an exclusive lock on that out-of-band byte without
// blocking (LockFileEx fails immediately when another handle holds the
// region). Conflicts stay per handle, so two processes opening the same
// session cannot both hold it, while ordinary reads and appends are free.
func lockFileExclusive(f *os.File) error {
	overlapped := &windows.Overlapped{OffsetHigh: lockRegionOffsetHigh}
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, overlapped,
	)
}

// unlockFile releases the region locked by lockFileExclusive. The offset
// must match the locked one exactly.
func unlockFile(f *os.File) {
	overlapped := &windows.Overlapped{OffsetHigh: lockRegionOffsetHigh}
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlapped)
}

// isLockBusy reports whether err means "another handle holds the lock".
func isLockBusy(err error) bool {
	var errno windows.Errno
	return errors.As(err, &errno) && errno == windows.ERROR_LOCK_VIOLATION
}
