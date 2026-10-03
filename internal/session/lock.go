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

	lockTable.Lock()
	if e, ok := lockTable.entries[clean]; ok {
		e.refs++
		lockTable.Unlock()
		return releaseOnce(clean), nil
	}
	lockTable.Unlock()

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

	lockTable.Lock()
	// A second opener may have raced us into the table while the flock was
	// in flight. Fold onto their entry and drop the duplicate handle; only
	// the first handle ever needs the lock.
	if e, ok := lockTable.entries[clean]; ok {
		e.refs++
		lockTable.Unlock()
		unlockFile(f)
		_ = f.Close()
		return releaseOnce(clean), nil
	}
	lockTable.entries[clean] = &lockEntry{file: f, refs: 1}
	lockTable.Unlock()

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
