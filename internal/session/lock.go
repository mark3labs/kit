package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Session files are shared mutable state: a JSONL transcript can be opened
// by several code paths (a live TUI, a viewer, a headless run), and two
// processes appending to the same file interleave their entries and corrupt
// the tree. Every TreeManager backed by a file therefore holds an advisory
// exclusive lock on that file for its lifetime, so at most one PROCESS can
// have a session open at a time.
//
// Within one process, locking is reentrant through a reference count keyed
// by path. Kit legitimately opens one session while another is still open
// (resume switches sessions by opening the new one first), and a process can
// never distinguish its own lock from a second copy of itself anyway: flock
// conflicts are decided per open file description, so the second open of the
// same path in one process would report a false conflict without the
// refcount. Concurrent in-process TreeManagers on the same file remain as
// unsafe as they always were; the refcount exists so that sequential and
// overlapping opens do not fail.

// lockTable holds the per-process reference counts for session file locks.
var lockTable = struct {
	sync.Mutex
	entries map[string]*lockEntry
}{entries: make(map[string]*lockEntry)}

// lockEntry is one held lock: the file handle it is held on and the number
// of TreeManagers in this process that share it.
type lockEntry struct {
	file *os.File
	refs int
}

// acquireSessionLock takes the exclusive lock for an existing file, or
// returns an error when another process holds it. The returned release func
// drops one reference and is safe to call more than once (extras are no-ops).
func acquireSessionLock(path string) (release func(), err error) {
	clean := filepath.Clean(path)

	// One table for the whole lookup-open-lock-register sequence. Both lock
	// calls are non-blocking, so holding the mutex across them risks no
	// deadlock, and it closes the race where two in-process openers each
	// miss the table and the second's own handle then conflicts with the
	// first's (flock and LockFileEx conflicts are per handle, not per
	// process) — which would report a false "another process" error.
	lockTable.Lock()
	defer lockTable.Unlock()

	if e, ok := lockTable.entries[clean]; ok {
		e.refs++
		return releaseOnce(clean), nil
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := lockFileExclusive(f); err != nil {
		_ = f.Close()
		if isLockBusy(err) {
			return nil, fmt.Errorf("session file is already open in another process: %s", path)
		}
		// A filesystem that cannot lock (some network mounts) must not stop
		// kit from running. Proceed unlocked, and remember nothing: the next
		// open will try again.
		return func() {}, nil
	}

	lockTable.entries[clean] = &lockEntry{file: f, refs: 1}
	return releaseOnce(clean), nil
}

// releaseSessionLock drops one reference and unlocks when the last one goes.
func releaseSessionLock(clean string) {
	lockTable.Lock()
	defer lockTable.Unlock()
	e, ok := lockTable.entries[clean]
	if !ok {
		return
	}
	e.refs--
	if e.refs > 0 {
		return
	}
	delete(lockTable.entries, clean)
	unlockFile(e.file)
	_ = e.file.Close()
}

// replaceSessionLock moves the lock from the handle it is held on to the
// file now at path, keeping the reference count. SetParentLink needs this:
// its header rewrite renames a fresh file over the session path, so the
// lock's inode stops backing the path and every manager sharing the entry
// must end up guarded by a lock on the new one instead.
//
// The caller must already hold a reference for clean. The old handle stays
// locked until the new one is secured, so the window without any lock on
// the path is one syscall wide.
func replaceSessionLock(clean string) error {
	lockTable.Lock()
	defer lockTable.Unlock()
	e, ok := lockTable.entries[clean]
	if !ok {
		return fmt.Errorf("session file lock was already released")
	}

	f, err := os.OpenFile(clean, os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("failed to reopen session file for its lock: %w", err)
	}
	if err := lockFileExclusive(f); err != nil {
		_ = f.Close()
		if isLockBusy(err) {
			return fmt.Errorf("session file is already open in another process: %s", clean)
		}
		return fmt.Errorf("failed to lock session file: %w", err)
	}

	old := e.file
	e.file = f
	unlockFile(old)
	_ = old.Close()
	return nil
}

// releaseOnce wraps a single release so double Close calls are harmless.
func releaseOnce(clean string) func() {
	var once bool
	return func() {
		if once {
			return
		}
		once = true
		releaseSessionLock(clean)
	}
}
