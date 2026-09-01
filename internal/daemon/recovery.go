package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// Crash recovery for session children.
//
// A session's PTY master lives in the daemon process. When the daemon
// exits, that fd goes with it and there is no way to re-open a master for
// an existing slave, so a bare child can never be adopted by the next
// daemon: it is unreachable by construction. Surviving a daemon restart
// would need a per-session supervisor process that owns the master and
// hands it over (the tmux/mosh design) — see docs.
//
// What we can guarantee is that a child never outlives the daemon that
// owns it. Whether the kernel's SIGHUP happens to kill it depends on
// process-group state at the moment the master closes, which makes
// survival a race rather than a rule. Two independent mechanisms close it:
//
//   - Pdeathsig (Linux): the kernel signals the child the instant its
//     parent dies, whatever killed the parent. Nothing to run, nothing to
//     miss.
//   - This registry (all platforms): the daemon records each child's pid,
//     and the next daemon start sweeps any survivor. This is the portable
//     fallback, and it also covers the window before Pdeathsig is armed.

const sessionsFileName = "sessions.json"

// sessionRecord is the on-disk description of one session child. It holds
// what a sweep needs to identify the process safely, plus the fields a
// future supervisor-based implementation would use to reattach.
type sessionRecord struct {
	ID      uint64    `json:"id"`
	PID     int       `json:"pid"`
	Cwd     string    `json:"cwd,omitempty"`
	Name    string    `json:"name,omitempty"`
	Started time.Time `json:"started"`
	// Run is the nonce of the daemon run that spawned this child. A record
	// from another run is a crash survivor.
	Run string `json:"run"`
}

func sessionsFilePath() (string, error) {
	dir, err := daemonRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sessionsFileName), nil
}

// readSessionRecords loads the registry. A missing or corrupt file yields
// no records: the registry is a recovery hint, never a source of truth,
// so it must not be able to stop a daemon from starting.
func readSessionRecords() []sessionRecord {
	path, err := sessionsFilePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var records []sessionRecord
	if err := json.Unmarshal(data, &records); err != nil {
		log.Warn("daemon: ignoring an unreadable session registry", "error", err)
		return nil
	}
	return records
}

// writeSessionRecords replaces the registry atomically.
func writeSessionRecords(records []sessionRecord) error {
	path, err := sessionsFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// removeSessionRegistry deletes the registry on a clean shutdown, so the
// next start has nothing to sweep.
func removeSessionRegistry() {
	if path, err := sessionsFilePath(); err == nil {
		_ = os.Remove(path)
	}
}

// sessionOwnerEnv marks a session child with the runtime directory of the
// daemon that spawned it. The sweep uses it to prove ownership.
//
// The runtime directory is the daemon's whole identity: it holds the
// single-instance lock, so two daemons with the same value cannot run at
// once and one is necessarily a previous run of the other. Two daemons
// with DIFFERENT values (a test daemon under its own XDG_CACHE_HOME
// alongside a packaged one, say) run concurrently and must never touch
// each other's sessions.
const sessionOwnerEnv = "KIT_DAEMON_HOME"

// isSessionChild reports whether pid is alive AND is a session child of a
// daemon sharing our runtime directory.
//
// Every check here is a veto, because the cost of the two mistakes is not
// symmetric: leaking a session wastes memory until the next sweep, while
// killing the wrong process destroys someone's work. A pid is signalled
// only when it is positively identified as ours.
//
//   - Pids are recycled, so a registry that outlived a reboot can name a
//     pid now held by something else entirely.
//   - A kit session child looks identical whichever daemon spawned it, so
//     the command line alone cannot distinguish our sessions from a
//     concurrently running daemon's.
//
// The environment marker settles both: it is inherited from the spawning
// daemon and names its runtime directory exactly.
func isSessionChild(pid int, owner string) bool {
	if pid <= 0 || pid == os.Getpid() {
		return false
	}
	if !processExists(pid) {
		return false
	}
	cmdline, err := processCmdline(pid)
	if err != nil {
		return false // cannot identify it, so do not touch it
	}
	if !strings.Contains(cmdline, pickDirFlagName) {
		return false
	}
	owned, err := processOwnedBy(pid, owner)
	if err != nil {
		// Unverifiable: another user's process, or a platform without the
		// means to check. Leave it alone.
		return false
	}
	return owned
}

// pickDirFlagName is the flag every session child is spawned with.
const pickDirFlagName = "--pick-dir"

// sweepOrphanSessions kills any session child left behind by a previous
// run of THIS daemon and clears the registry. Called once at start-up.
//
// The single-instance lock is per runtime directory, not per user, so
// another daemon with its own state directory may be running right now
// with sessions of its own. Ownership is therefore proved per process
// (see isSessionChild) rather than assumed from the lock.
func sweepOrphanSessions(run string) {
	records := readSessionRecords()
	if len(records) == 0 {
		removeSessionRegistry()
		return
	}
	owner, err := daemonRuntimeDir()
	if err != nil {
		return // cannot establish ownership, so sweep nothing
	}
	swept := 0
	for _, rec := range records {
		if rec.Run == run {
			continue // our own record, written by this run
		}
		if !isSessionChild(rec.PID, owner) {
			continue // already gone, or not provably ours to kill
		}
		log.Warn("daemon: killing a session left by a previous run",
			"session_id", rec.ID, "pid", rec.PID, "cwd", rec.Cwd)
		terminateProcess(rec.PID)
		swept++
	}
	if swept > 0 {
		log.Info("daemon: swept sessions from a previous run", "count", swept)
	}
	removeSessionRegistry()
}

// terminateProcess asks a process to exit, then insists.
//
// SIGTERM first so kit can flush its conversation store and restore the
// terminal; SIGKILL only if it is still there after the grace period. A
// session killed mid-write would otherwise leave a truncated JSONL.
func terminateProcess(pid int) {
	if err := signalTerm(pid); err != nil {
		return
	}
	deadline := time.Now().Add(childGrace)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return // exited on its own terms
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = signalKill(pid)
}

// childGrace is how long a session child gets to exit after SIGTERM before
// it is killed outright.
const childGrace = 3 * time.Second

// syncSessionRegistry rewrites the registry from the live session table.
// Called whenever a session is created or retired, so a crash at any point
// leaves a registry that names every live child.
func (t *sessionTable) syncSessionRegistry() {
	t.mu.Lock()
	records := make([]sessionRecord, 0, len(t.sessions))
	for id, sess := range t.sessions {
		if sess.cmd == nil || sess.cmd.Process == nil {
			continue // spawned but not running yet
		}
		records = append(records, sessionRecord{
			ID:      id,
			PID:     sess.cmd.Process.Pid,
			Cwd:     t.sessionCwd(sess),
			Name:    sess.displayName(),
			Started: sess.started,
			Run:     t.run,
		})
	}
	t.mu.Unlock()

	if err := writeSessionRecords(records); err != nil {
		// The registry is a recovery aid; failing to write it must not
		// disturb a working daemon.
		log.Warn("daemon: could not update the session registry", "error", err)
	}
}

// sweepStaleTempFiles removes per-session scratch files left by previous
// runs.
//
// The files are named per daemon run, so a new daemon cannot inherit an
// old session's clipboard image through a reused logical id — but the old
// files would still accumulate in the temp directory after a crash.
func sweepStaleTempFiles(run string) {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, tempFilePrefix) {
			continue
		}
		if strings.Contains(name, run) {
			continue // belongs to this run
		}
		_ = os.Remove(filepath.Join(os.TempDir(), name))
	}
}

// tempFilePrefix is shared by every per-session scratch file so a sweep can
// find them all.
const tempFilePrefix = "kit-session-"

// newRunNonce returns a short random identifier for one daemon run.
func newRunNonce() string {
	return fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano()%1e6)
}
