package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The daemon <-> session-host link.
//
// A hosted session runs in a supervisor process of its own, and the
// daemon reaches it over a Unix socket. The link reuses the frame format
// from protocol.go so there is one wire encoding in the codebase rather
// than two, but it is a SEPARATE conversation: it carries one session, it
// has no wire ids to multiplex (the session field is always zero), and it
// is never exposed to a client.
//
// Only a handful of frames travel on it:
//
//	daemon -> host   DATA      terminal input for the child
//	                 RESIZE    the size the attached clients agree on
//	                 REDRAW    repaint, and replay what a new client missed
//	                 RENAME    set the display name
//	                 BYE       end the session for good
//	host   -> daemon HELLO     who I am and what I am running (once, first)
//	                 DATA      the child's output
//	                 BYE       the child has exited
//
// The HELLO is what makes adoption possible: a daemon that has just
// started knows nothing about the session behind a socket, and this is
// where it learns the id, the working directory, the name and the age it
// has to put back into its table.

// sessionsDirName holds one socket per hosted session, inside the
// daemon's runtime directory. A daemon adopts exactly the sessions it
// finds here, which is why it must be per runtime directory and not
// shared: two daemons with different runtime directories must never adopt
// each other's sessions.
const sessionsDirName = "sessions"

// sessionsDir returns the directory holding hosted-session sockets,
// creating it if needed.
func sessionsDir() (string, error) {
	dir, err := daemonRuntimeDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, sessionsDirName)
	if err := mkdirPrivate(dir); err != nil {
		return "", fmt.Errorf("daemon: sessions dir: %w", err)
	}
	return dir, nil
}

// sessionSocketPath is the socket a given session's supervisor listens on.
//
// Named by logical id alone. The id is stable across daemon restarts
// (see nextSessionID), so this path is the one durable handle a new
// daemon has on a session started by a previous one.
func sessionSocketPath(id uint64) (string, error) {
	dir, err := sessionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%d.sock", id)), nil
}

// SessionHostConfig is everything a supervisor needs to start and serve
// one session. The daemon writes it to a private file and passes the
// path; the supervisor reads it once and removes it.
//
// A file rather than arguments or environment: it carries the caller's
// whole environment, which is far larger than an argument list may safely
// be, and it keeps that environment out of the process table where any
// other process on the machine could read it.
type SessionHostConfig struct {
	// ID is the logical session id, matching the socket name.
	ID uint64 `json:"id"`
	// Socket is the path to listen on.
	Socket string `json:"socket"`
	// Spec describes the command line to run, or nil for the daemon's
	// default (the directory picker in the user's home).
	Spec *SessionSpec `json:"spec,omitempty"`
	// Terminal describes the terminal of the client that asked for this
	// session, so the child renders for it rather than for the daemon.
	Terminal TerminalInfo `json:"terminal"`
	// Env holds the per-session variables the daemon owns: the clipboard
	// file, the cwd report file, and the ownership marker.
	Env map[string]string `json:"env,omitempty"`
	// Name is the session's display name, empty until it is renamed.
	Name string `json:"name,omitempty"`
	// Owner is the daemon runtime directory that started this session. A
	// supervisor records it so a sweep can prove the process is its own
	// before signalling it.
	Owner string `json:"owner,omitempty"`
}

// SessionHostInfo is the supervisor's HELLO: what a daemon needs to put a
// session it has never seen back into its table.
type SessionHostInfo struct {
	// Protocol, Version and Features describe the supervisor exactly as a
	// daemon's own hello describes it. A supervisor is a long-lived
	// process too, and a daemon upgraded underneath it meets the same
	// skew a client meets, so it is settled the same way.
	Protocol string  `json:"protocol"`
	Version  uint16  `json:"version"`
	Features Feature `json:"features"`
	Build    string  `json:"build,omitempty"`

	// ID is the logical session id.
	ID uint64 `json:"id"`
	// PID is the supervisor's own process id.
	PID int `json:"pid"`
	// ChildPID is the kit process inside the session.
	ChildPID int `json:"child_pid"`
	// Started is when the session began, not when the supervisor was
	// last dialled: a session's age must not reset because a daemon did.
	Started time.Time `json:"started"`
	// Name is the display name, empty when it has none.
	Name string `json:"name,omitempty"`
	// Cwd is the session's working directory, empty while the directory
	// picker is still on screen.
	Cwd string `json:"cwd,omitempty"`
	// Owner is the runtime directory of the daemon that started this
	// session; a daemon refuses to adopt a session owned by another.
	Owner string `json:"owner,omitempty"`
}

// EncodeSessionHostInfo renders a supervisor HELLO payload.
func EncodeSessionHostInfo(info SessionHostInfo) ([]byte, error) { return json.Marshal(info) }

// DecodeSessionHostInfo parses a supervisor HELLO payload.
func DecodeSessionHostInfo(payload []byte) (SessionHostInfo, error) {
	var info SessionHostInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		return SessionHostInfo{}, fmt.Errorf("daemon: bad session host hello: %w", err)
	}
	return info, nil
}

// Compatible reports whether a daemon can drive this supervisor.
//
// Same rule as everywhere else: the protocol version decides, features
// never do. A supervisor from another protocol version is left strictly
// alone — not adopted, and above all not killed, because the session
// inside it is somebody's work and an unreadable supervisor is no reason
// to destroy it.
func (i SessionHostInfo) Compatible() error {
	if i.Protocol != "" && i.Protocol != ProtocolName {
		return fmt.Errorf("daemon: session host speaks %q, not %q", i.Protocol, ProtocolName)
	}
	if i.Version != ProtocolVersion {
		return fmt.Errorf(
			"daemon: session host speaks protocol %d, this daemon speaks %d",
			i.Version, ProtocolVersion)
	}
	return nil
}

// sessionHostHello builds a supervisor's own hello.
func sessionHostHello(cfg SessionHostConfig, childPID int, started time.Time, name, cwd string) SessionHostInfo {
	return SessionHostInfo{
		Protocol: ProtocolName,
		Version:  ProtocolVersion,
		Features: ProtocolFeatures,
		Build:    BuildVersion(),
		ID:       cfg.ID,
		PID:      os.Getpid(),
		ChildPID: childPID,
		Started:  started,
		Name:     name,
		Cwd:      cwd,
		Owner:    cfg.Owner,
	}
}

// readReportedCwd reads a session's cwd report file. A missing file means
// the child has not chosen a directory yet, which is the honest answer
// while the directory picker is still on screen.
func readReportedCwd(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// mkdirPrivate creates a directory only its owner can enter.
func mkdirPrivate(dir string) error { return os.MkdirAll(dir, 0o700) }

// scrollbackLimit bounds the output a supervisor remembers for the next
// client to attach.
//
// It exists so a client that attaches after a daemon restart sees the
// screen at once instead of a blank terminal waiting for the child's next
// repaint. One screenful is not enough — a repaint is drawn in pieces and
// the interesting part is usually the last few of them — and an unbounded
// buffer would grow with every character a long-running session ever
// printed. 256 KiB covers a large terminal several times over and costs a
// fixed amount per session.
const scrollbackLimit = 256 * 1024
