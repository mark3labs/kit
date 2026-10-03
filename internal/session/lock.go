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
	entries   map[string]*lockEntry
	rewriting map[string]bool
}{entries: make(map[string]*lockEntry), rewriting: make(map[string]bool)}

// lockEntry is one held lock: the file handle it is held on, the number of
// TreeManagers in this process that share it, and the append mutex those
// managers serialize their file writes on.
type lockEntry struct {
	file *os.File
	refs int
	// appendMu serializes every append that writes the shared transcript
	// file: seek-to-end, buffered writes, flush, sync, and — critically —
	// any rollback truncate, which would otherwise cut away an entry
	// another manager committed in the meantime. It is a leaf lock: hold it
	// across file syscalls only, never call back into another manager's
	// append methods from inside it.
	appendMu sync.Mutex
}

// acquireSessionLock takes the exclusive lock for an existing file, or
// returns an error when another process holds it. The returned release func
// drops one reference and is safe to call more than once (extras are no-ops).
func acquireSessionLock(path string) (release func(), err error) {
	return acquireSessionLockForRewrite(path, false)
}

func acquireSessionLockForRewrite(path string, rewriting bool) (func(), error) {
	clean := filepath.Clean(path)

	// One table for the whole lookup-open-lock-register sequence. Both lock
	// calls are non-blocking, so holding the mutex across them risks no
	// deadlock, and it closes the race where two in-process openers each
	// miss the table and the second's own handle then conflicts with the
	// first's (flock and LockFileEx conflicts are per handle, not per
	// process) — which would report a false "another process" error.
	lockTable.Lock()
	defer lockTable.Unlock()

	if lockTable.rewriting[clean] && !rewriting {
		return nil, fmt.Errorf("session header rewrite is in progress: %s", path)
	}
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

// appendPathMu returns the per-file append mutex shared by every manager of
// the same session file, or nil when this session has no table entry (an
// in-memory session, or a file that could not be locked). Callers that get
// nil skip the serialization, which keeps the previous behavior.
func (tm *TreeManager) appendPathMu() *sync.Mutex {
	if tm.cleanPath == "" {
		return nil
	}
	lockTable.Lock()
	defer lockTable.Unlock()
	if e, ok := lockTable.entries[tm.cleanPath]; ok {
		return &e.appendMu
	}
	return nil
}

// reserveRewrite rejects shared ownership and blocks new in-process opens
// until the rewrite has restored its file lock.
func (tm *TreeManager) reserveRewrite() (func(), error) {
	lockTable.Lock()
	defer lockTable.Unlock()
	e, ok := lockTable.entries[tm.cleanPath]
	if !ok || e.refs != 1 || lockTable.rewriting[tm.cleanPath] {
		return nil, fmt.Errorf("header rewrite requires exclusive session ownership")
	}
	lockTable.rewriting[tm.cleanPath] = true
	return func() {
		lockTable.Lock()
		delete(lockTable.rewriting, tm.cleanPath)
		lockTable.Unlock()
	}, nil
}
