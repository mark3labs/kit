package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Single-instance and status plumbing for the daemon.
//
// The daemon holds an exclusive flock on daemon.lock for its lifetime:
// kernel-owned, so a crashed or SIGKILLed daemon releases it automatically
// and there is no stale-lock problem to clean up. A human-readable state
// snapshot (daemon.json) sits next to it and is rewritten atomically
// whenever something worth reporting changes.

const (
	lockFileName  = "daemon.lock"
	stateFileName = "daemon.json"
)

// daemonState is the snapshot `kit daemon status` reports.
type daemonState struct {
	PID            int       `json:"pid"`
	Endpoint       string    `json:"endpoint,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	SessionsActive int       `json:"sessions_active"`
}

func daemonRuntimeDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "kit", "daemon")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("daemon: runtime dir: %w", err)
	}
	return dir, nil
}

// daemonLock owns the exclusive flock that marks a daemon as running.
type daemonLock struct {
	f *os.File
}

// acquireDaemonLock enforces one daemon per user. When another daemon holds
// the lock, the returned error carries the running instance's details.
func acquireDaemonLock() (*daemonLock, error) {
	dir, err := daemonRuntimeDir()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("daemon: lock: %w", err)
	}
	if err := lockFileExclusive(f); err != nil {
		_ = f.Close()
		if st := readStateFile(dir); st != nil {
			return nil, fmt.Errorf(
				"daemon: another instance is already running (pid %d, started %s) — see 'kit daemon status'",
				st.PID, st.StartedAt.Format("2006-01-02 15:04"))
		}
		return nil, errors.New("daemon: another instance is already running — see 'kit daemon status'")
	}
	return &daemonLock{f: f}, nil
}

// release drops the lock. The process exit path also releases it implicitly
// when the fd closes.
func (l *daemonLock) release() {
	unlockFile(l.f)
	_ = l.f.Close()
}

// runtime is the daemon's live state holder: it owns the lock and persists
// the snapshot on every change.
type daemonRuntime struct {
	lock *daemonLock

	mu    sync.Mutex
	state daemonState
	tun   *Tunnel    // the live sidecar; nil while it is down
	sink  *frameSink // the live sidecar's frame sink; nil while it is down
}

// setSink records the frame sink of the current sidecar tunnel.
func (rt *daemonRuntime) setSink(s *frameSink) {
	rt.mu.Lock()
	rt.sink = s
	rt.mu.Unlock()
}

// setTunnel records the current sidecar tunnel. Called by Serve whenever
// the tunnel (re)starts; logical sessions survive these restarts and their
// output goes through whatever tunnel is current.
func (rt *daemonRuntime) setTunnel(t *Tunnel) {
	rt.mu.Lock()
	rt.tun = t
	rt.mu.Unlock()
}

// tunnel returns the current sidecar tunnel, or nil while it is down.
func (rt *daemonRuntime) tunnel() *Tunnel {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.tun
}

func newDaemonRuntime(lock *daemonLock) *daemonRuntime {
	return &daemonRuntime{
		lock: lock,
		state: daemonState{
			PID:       os.Getpid(),
			StartedAt: time.Now(),
		},
	}
}

// setEndpoint records the endpoint id once the tunnel is online.
func (r *daemonRuntime) setEndpoint(endpoint string) {
	r.mu.Lock()
	r.state.Endpoint = endpoint
	r.mu.Unlock()
	_ = r.persist()
}

// setSessions records the active session count.
func (r *daemonRuntime) setSessions(n int) {
	r.mu.Lock()
	r.state.SessionsActive = n
	r.mu.Unlock()
	_ = r.persist()
}

func (r *daemonRuntime) persist() error {
	r.mu.Lock()
	snapshot := r.state
	r.mu.Unlock()
	return r.lock.persist(snapshot)
}

// persist writes the state atomically (temp file + rename). The lock file
// keeps its inode, so the held flock is unaffected by the rename.
func (l *daemonLock) persist(s daemonState) error {
	dir, err := daemonRuntimeDir()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return fmt.Errorf("daemon: state temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("daemon: state write: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("daemon: state chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(dir, stateFileName)); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("daemon: state rename: %w", err)
	}
	return nil
}

// clearState removes a no-longer-valid state file (graceful shutdown).
func clearState() {
	if dir, err := daemonRuntimeDir(); err == nil {
		_ = os.Remove(filepath.Join(dir, stateFileName))
	}
}

func readStateFile(dir string) *daemonState {
	b, err := os.ReadFile(filepath.Join(dir, stateFileName))
	if err != nil {
		return nil
	}
	var s daemonState
	if json.Unmarshal(b, &s) != nil {
		return nil
	}
	return &s
}

// Status describes a kit daemon on this machine (per user).
type Status struct {
	// Running is true when a live process holds the daemon lock.
	Running bool
	// State is the daemon's last persisted snapshot, nil when none exists.
	// For a stopped daemon this is stale data from a previous run.
	State *daemonState
}

// ReadStatus reports whether a daemon currently holds the lock. Probing is
// a non-blocking exclusive flock: acquiring it means no daemon is alive.
func ReadStatus() Status {
	dir, err := daemonRuntimeDir()
	if err != nil {
		return Status{}
	}
	f, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return Status{}
	}
	defer func() { _ = f.Close() }() // best effort: read-only probe
	if err := lockFileExclusive(f); err != nil {
		return Status{Running: true, State: readStateFile(dir)}
	}
	unlockFile(f)
	return Status{Running: false, State: readStateFile(dir)}
}
