package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// Crash recovery and session adoption.
//
// A session's PTY master has to live somewhere, and whichever process
// holds it is the only one that can ever reach the child: a master cannot
// be re-opened for an existing slave. That single fact decides everything
// here.
//
// Sessions are therefore hosted by supervisor processes of their own (see
// sessionhost.go), and a daemon that starts ADOPTS them by dialling their
// sockets. Stopping, restarting or upgrading a daemon costs a socket and
// nothing more.
//
// Two things still need this file:
//
//   - A session a daemon had to host itself, because no supervisor could
//     be started. It dies with that daemon, and a survivor of one is
//     unreachable by construction, so the next daemon sweeps it. Pdeathsig
//     (Linux) normally gets there first; this registry is the portable
//     fallback and covers the window before Pdeathsig is armed.
//   - Session ids, which must never be reused. An adopted session keeps
//     the id its clients, its scratch files and its socket already carry,
//     so a new session must be numbered above every id that has ever
//     existed rather than above the ones that happen to be live.

const sessionsFileName = "sessions.json"

// sessionRecord is the on-disk description of one session. It holds what
// a sweep needs to identify a process safely, what adoption needs to
// recognise a session it can reach, and what id allocation needs to avoid
// handing out a number twice.
type sessionRecord struct {
	ID uint64 `json:"id"`
	// PID is the supervisor for a hosted session, or the kit child itself
	// for one the daemon hosted directly. Only the latter is ever
	// signalled by a sweep.
	PID     int       `json:"pid"`
	Cwd     string    `json:"cwd,omitempty"`
	Name    string    `json:"name,omitempty"`
	Started time.Time `json:"started"`
	// Run is the nonce of the daemon run that wrote this record. A record
	// from another run describes a session that daemon left behind.
	Run string `json:"run"`
	// Hosted marks a session that lives in a supervisor process, so it
	// survives the daemon and is adopted rather than swept.
	Hosted bool `json:"hosted,omitempty"`
	// Socket is the supervisor's socket path, for hosted sessions.
	Socket string `json:"socket,omitempty"`
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
	if !isSessionChildCmdline(cmdline) {
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

// pickDirFlagName asks a session child to open the directory picker
// before it starts. It is what a session spawned without a SessionSpec
// gets, and what every session child got before specs existed.
const pickDirFlagName = "--pick-dir"

// isSessionChildCmdline reports whether a command line is a kit session
// child's.
//
// Two markers are accepted. --daemon-session is on every child this
// daemon spawns, whatever else the spec asked for, and is the reason a
// child running the user's own arguments is still recognisable. The older
// --pick-dir is accepted as well so a daemon upgraded in place still
// sweeps the children its predecessor left behind; on its own it is no
// longer sufficient proof, which is why ownership is settled by the
// environment marker in isSessionChild rather than here.
func isSessionChildCmdline(cmdline string) bool {
	return strings.Contains(cmdline, SessionFlag) ||
		strings.Contains(cmdline, pickDirFlagName)
}

// sweepOrphanSessions ends any session left behind by a previous run of
// THIS daemon that cannot be adopted, and clears the registry. Called
// once at start-up, AFTER adoption.
//
// adopted names the sessions this daemon has already taken over; they are
// alive, reachable and must never be touched. Everything else in the
// registry from another run is a session whose daemon held its PTY master
// directly: that master is gone, so the child is unreachable by any
// future daemon and is ended rather than left running invisibly.
//
// The single-instance lock is per runtime directory, not per user, so
// another daemon with its own state directory may be running right now
// with sessions of its own. Ownership is therefore proved per process
// (see isSessionChild) rather than assumed from the lock.
func sweepOrphanSessions(run string, adopted []uint64) {
	records := readSessionRecords()
	if len(records) == 0 {
		removeSessionRegistry()
		return
	}
	owner, err := daemonRuntimeDir()
	if err != nil {
		return // cannot establish ownership, so sweep nothing
	}
	live := make(map[uint64]bool, len(adopted))
	for _, id := range adopted {
		live[id] = true
	}
	swept := 0
	for _, rec := range records {
		switch {
		case rec.Run == run:
			continue // our own record, written by this run
		case live[rec.ID]:
			continue // adopted: alive and reachable
		case rec.Hosted:
			// A hosted session that adoption did not pick up. Its
			// supervisor did not answer, which usually means it is gone
			// already; it is NOT killed on that evidence, because the one
			// other explanation is a supervisor from another protocol
			// version, and destroying a user's work over a version skew
			// is the worst outcome available here.
			continue
		case !isSessionChild(rec.PID, owner):
			continue // already gone, or not provably ours to end
		}
		log.Warn("daemon: ending an unreachable session left by a previous run",
			"session_id", rec.ID, "pid", rec.PID, "cwd", rec.Cwd)
		terminateProcess(rec.PID)
		swept++
	}
	if swept > 0 {
		log.Info("daemon: swept unreachable sessions from a previous run", "count", swept)
	}
}

// seedSessionIDs sets the id allocator above every id that has ever been
// handed out, so a new session can never reuse one.
//
// Reuse used to be harmless: ids were per daemon run and nothing outlived
// a run. Now a session's id is written into its child's environment (the
// clipboard and cwd files), into its supervisor's socket name, and into
// whatever a client wrote down to reattach with. Handing the same number
// to a second session would point two of them at one set of files and let
// `kit attach 3` reach the wrong work.
//
// The registry is the memory here: it names every session the previous
// daemon knew about, adopted or not.
func (t *sessionTable) seedSessionIDs(adopted []uint64) {
	highest := uint64(0)
	for _, id := range adopted {
		if id > highest {
			highest = id
		}
	}
	for _, rec := range readSessionRecords() {
		if rec.ID > highest {
			highest = rec.ID
		}
	}
	t.mu.Lock()
	if highest > t.nextID {
		t.nextID = highest
	}
	t.mu.Unlock()
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
// leaves a registry that names every live session.
func (t *sessionTable) syncSessionRegistry() {
	t.mu.Lock()
	records := make([]sessionRecord, 0, len(t.sessions))
	for id, sess := range t.sessions {
		if sess.io == nil {
			continue // spawned but not running yet
		}
		rec := sessionRecord{
			ID:      id,
			PID:     sess.io.PID(),
			Cwd:     t.sessionCwd(sess),
			Name:    sess.displayName(),
			Started: sess.started,
			Run:     t.run,
			Hosted:  sess.io.Hosted(),
		}
		if rec.Hosted {
			if sock, err := sessionSocketPath(id); err == nil {
				rec.Socket = sock
			}
		}
		records = append(records, rec)
	}
	t.mu.Unlock()

	if err := writeSessionRecords(records); err != nil {
		// The registry is a recovery aid; failing to write it must not
		// disturb a working daemon.
		log.Warn("daemon: could not update the session registry", "error", err)
	}
}

// sweepStaleTempFiles removes per-session scratch files whose session no
// longer exists.
//
// The files are named by session id, which is what lets an ADOPTED
// session keep using the exact path its child was handed at start-up.
// That is also why the sweep has to be told which sessions are live
// rather than deriving it from a daemon run nonce: a file belonging to an
// adopted session was written by a previous run and must survive.
//
// The sweep is confined to this daemon's runtime directory. Sweeping a
// shared directory would delete the live clipboard and cwd files of a
// concurrently running daemon, whose sessions would then report no working
// directory and silently drop image pastes.
func sweepStaleTempFiles(live []uint64) {
	dir, err := daemonRuntimeDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	keep := make(map[uint64]bool, len(live))
	for _, id := range live {
		keep[id] = true
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, tempFilePrefix) {
			continue
		}
		if id, ok := tempFileSession(name); ok && keep[id] {
			continue // belongs to a session that is still running
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}

// tempFileSession reads the session id off a scratch file name
// ("kit-session-clip-7", "kit-session-cwd-7").
//
// A name that does not parse is reported as belonging to no session,
// which makes it sweepable — that covers the run-nonce names written by
// daemons from before ids were stable, which have no owner any more.
func tempFileSession(name string) (uint64, bool) {
	idx := strings.LastIndexByte(name, '-')
	if idx < 0 || idx+1 >= len(name) {
		return 0, false
	}
	id, err := strconv.ParseUint(name[idx+1:], 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// tempFilePrefix is shared by every per-session scratch file so a sweep can
// find them all.
const tempFilePrefix = "kit-session-"

// newRunNonce returns a short random identifier for one daemon run.
func newRunNonce() string {
	return fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano()%1e6)
}
