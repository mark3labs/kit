//go:build !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// The daemon's half of the session-host link: starting supervisors,
// driving them, and — the reason they exist — adopting the ones a
// previous daemon left behind.

// hostIO drives a session that lives in a supervisor process.
//
// Read and Write look exactly like a PTY master to the session table
// above, which is the whole trick: the daemon's frame loop, fan-out and
// resize logic are unchanged, and only the object underneath knows that
// the terminal is on the far side of a socket.
type hostIO struct {
	id   uint64
	info SessionHostInfo

	conn net.Conn
	sink *frameSink

	// pending holds the tail of a DATA frame that did not fit in the
	// caller's buffer. A Read must never drop the remainder: it is the
	// child's output.
	pending []byte

	endOnce sync.Once
	ended   chan struct{}

	closeOnce sync.Once
}

func newHostIO(id uint64, conn net.Conn, info SessionHostInfo) *hostIO {
	return &hostIO{
		id:    id,
		info:  info,
		conn:  conn,
		sink:  newFrameSink(conn),
		ended: make(chan struct{}),
	}
}

func (h *hostIO) Hosted() bool { return true }
func (h *hostIO) PID() int     { return h.info.PID }

// Read returns the next chunk of the session child's output.
//
// Control frames from the supervisor are consumed here rather than
// surfaced: a BYE means the child has exited, which the caller learns as
// the io.EOF that ends its read loop — the same signal a closed PTY gives.
func (h *hostIO) Read(p []byte) (int, error) {
	if len(h.pending) > 0 {
		n := copy(p, h.pending)
		h.pending = h.pending[n:]
		return n, nil
	}
	for {
		frame, err := ReadFrame(h.conn)
		if err != nil {
			h.markEnded()
			return 0, err
		}
		switch frame.Type {
		case FrameData:
			if len(frame.Payload) == 0 {
				continue
			}
			n := copy(p, frame.Payload)
			if n < len(frame.Payload) {
				h.pending = append(h.pending[:0], frame.Payload[n:]...)
			}
			return n, nil
		case FrameBye:
			h.markEnded()
			return 0, io.EOF
		case FrameHello:
			// A supervisor that re-introduces itself; the fields we care
			// about were settled when the connection was made.
		}
	}
}

// Write sends terminal input to the session child.
func (h *hostIO) Write(p []byte) (int, error) {
	for b := p; len(b) > 0; {
		n := min(chunkSize, len(b))
		if err := h.sink.write(Frame{Type: FrameData, Payload: b[:n]}); err != nil {
			return 0, err
		}
		b = b[n:]
	}
	return len(p), nil
}

// Close detaches this daemon from the session WITHOUT ending it. The
// supervisor and its child carry on, and the next daemon dials the same
// socket.
func (h *hostIO) Close() error {
	h.closeOnce.Do(func() {
		h.sink.close()
		_ = h.conn.Close()
	})
	return nil
}

// Resize forwards the size the attached clients agree on.
func (h *hostIO) Resize(ws winSize) error {
	if ws.cols == 0 || ws.rows == 0 {
		return nil
	}
	return h.sink.write(Frame{Type: FrameResize, Payload: EncodeResize(ws.cols, ws.rows)})
}

// Redraw asks the supervisor to restore a client that has just taken over
// the screen: terminal modes, recent output, then a repaint. The size
// travels with the request so the nudge happens on the supervisor's side,
// where the two size changes are local instead of two network round trips.
func (h *hostIO) Redraw(ws winSize) {
	_ = h.sink.write(Frame{Type: FrameSessionRedraw, Payload: EncodeResize(ws.cols, ws.rows)})
}

// Rename records the display name with the session rather than with this
// daemon, so it survives a restart along with the session itself.
func (h *hostIO) Rename(name string) {
	_ = h.sink.write(Frame{Type: FrameSessionRename, Payload: []byte(name)})
}

// Terminate ends the session for good. This is the one message that stops
// a supervisor, so it is sent only where the old code killed a child.
func (h *hostIO) Terminate() {
	_ = h.sink.write(Frame{Type: FrameBye})
	// Give the supervisor a moment to stop its child cleanly, then drop
	// the socket. Off this goroutine: Terminate is called inline on the
	// frame loop when a write fails, and the wait would stall a client.
	go func() {
		select {
		case <-h.ended:
		case <-time.After(childGrace + time.Second):
		}
		_ = h.Close()
	}()
}

// Wait blocks until the session has ended.
func (h *hostIO) Wait() { <-h.ended }

func (h *hostIO) markEnded() { h.endOnce.Do(func() { close(h.ended) }) }

// dialSessionHost connects to a supervisor and reads its hello.
//
// The hello is mandatory and bounded: it is how the daemon learns what is
// behind the socket, and a supervisor that does not answer is not one we
// can drive. Nothing is killed on failure — see adoptHostedSessions.
func dialSessionHost(ctx context.Context, path string) (*hostIO, SessionHostInfo, error) {
	dctx, cancel := context.WithTimeout(ctx, sessionHostDialTimeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(dctx, "unix", path)
	if err != nil {
		return nil, SessionHostInfo{}, err
	}
	// The deadline covers the hello only; it is cleared before the
	// connection is handed on, because a session is idle for hours at a
	// time and a read deadline would end it.
	_ = conn.SetReadDeadline(time.Now().Add(sessionHostDialTimeout))
	frame, err := ReadFrame(conn)
	if err != nil {
		_ = conn.Close()
		return nil, SessionHostInfo{}, fmt.Errorf("daemon: session host did not introduce itself: %w", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	if frame.Type != FrameHello {
		_ = conn.Close()
		return nil, SessionHostInfo{}, fmt.Errorf("daemon: session host sent %#x, not a hello", byte(frame.Type))
	}
	info, err := DecodeSessionHostInfo(frame.Payload)
	if err != nil {
		_ = conn.Close()
		return nil, SessionHostInfo{}, err
	}
	if cerr := info.Compatible(); cerr != nil {
		_ = conn.Close()
		return nil, SessionHostInfo{}, cerr
	}
	return newHostIO(info.ID, conn, info), info, nil
}

// sessionHostDialTimeout bounds connecting to a supervisor and reading
// its hello. Both are local and immediate; the timeout exists so one
// wedged supervisor cannot stall a daemon's whole start-up.
const sessionHostDialTimeout = 3 * time.Second

// spawnSessionHost starts a supervisor for a new session and connects to
// it.
//
// The supervisor is fully detached: its own session and process group, no
// controlling terminal, and explicitly NO parent-death signal. Every
// other child the daemon starts gets one; this one must not have it,
// because outliving the daemon is its entire purpose.
func (t *sessionTable) spawnSessionHost(ctx context.Context, id uint64, info TerminalInfo, spec *SessionSpec) (*hostIO, error) {
	sock, err := sessionSocketPath(id)
	if err != nil {
		return nil, err
	}
	owner, _ := daemonRuntimeDir()
	cfg := SessionHostConfig{
		ID:       id,
		Socket:   sock,
		Spec:     spec,
		Terminal: info,
		Owner:    owner,
		Env:      t.sessionEnv(id, owner),
	}
	cfgPath, err := writeSessionHostConfig(cfg)
	if err != nil {
		return nil, err
	}

	exe, err := os.Executable()
	if err != nil {
		_ = os.Remove(cfgPath)
		return nil, fmt.Errorf("daemon: resolve kit binary: %w", err)
	}
	cmd := exec.Command(exe, "daemon", "session-host", "--config", cfgPath)
	cmd.Dir = homeDir()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = detachedProcAttr()
	if err := cmd.Start(); err != nil {
		_ = os.Remove(cfgPath)
		return nil, fmt.Errorf("daemon: start session host: %w", err)
	}
	// Do not wait for it: it is meant to outlive us. Reaping is left to
	// init once this daemon exits.
	go func() { _ = cmd.Wait() }()

	io, _, err := waitForSessionHost(ctx, sock)
	if err != nil {
		_ = os.Remove(cfgPath)
		// The supervisor may be coming up anyway, just too slowly to be
		// waited for. Left alone it would bind this session's socket with
		// nothing driving it, and the NEXT daemon would adopt it as a
		// session the user never saw start — while this daemon has
		// already fallen back and given the id to a session of its own.
		// Ending it here keeps one id to one session.
		if cmd.Process != nil {
			go terminateProcess(cmd.Process.Pid)
		}
		return nil, err
	}
	return io, nil
}

// waitForSessionHost polls a supervisor's socket until it answers.
func waitForSessionHost(ctx context.Context, path string) (*hostIO, SessionHostInfo, error) {
	deadline := time.Now().Add(sessionHostStartTimeout)
	var last error
	for {
		io, info, err := dialSessionHost(ctx, path)
		if err == nil {
			return io, info, nil
		}
		last = err
		if time.Now().After(deadline) {
			return nil, SessionHostInfo{}, fmt.Errorf(
				"daemon: session host did not come up within %s: %w", sessionHostStartTimeout, last)
		}
		select {
		case <-ctx.Done():
			return nil, SessionHostInfo{}, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// sessionHostStartTimeout bounds waiting for a new supervisor to bind its
// socket. It covers a cold binary on a loaded machine; beyond it,
// something is wrong and the caller falls back to an in-daemon session.
const sessionHostStartTimeout = 10 * time.Second

// writeSessionHostConfig hands a supervisor its configuration through a
// private file.
//
// 0600 in the daemon's own runtime directory, because it carries the
// caller's environment and that routinely includes API keys. The
// supervisor removes it as soon as it has read it.
func writeSessionHostConfig(cfg SessionHostConfig) (string, error) {
	dir, err := sessionsDir()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, fmt.Sprintf(".cfg-%d-*", cfg.ID))
	if err != nil {
		return "", fmt.Errorf("daemon: session host config: %w", err)
	}
	name := f.Name()
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// adoptHostedSessions puts every session a previous daemon left running
// back into this daemon's table.
//
// This is the payoff for everything else in this file: a daemon that has
// just started scans for supervisor sockets, dials each one, and learns
// from its hello exactly what it was told to forget. The user's sessions
// come back with their ids, names, working directories and ages intact,
// and the clients still waiting on them reattach.
//
// Failures are per session and never fatal. A socket that does not answer
// is stale, and is unlinked; a supervisor speaking another protocol
// version is left strictly alone — not adopted and NOT killed, because
// the work inside it is real and an unreadable supervisor is no reason to
// destroy it.
func (t *sessionTable) adoptHostedSessions(ctx context.Context) []uint64 {
	dir, err := sessionsDir()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	owner, _ := daemonRuntimeDir()

	var adopted []uint64
	for _, e := range entries {
		id, ok := sessionIDFromSocket(e.Name())
		if !ok {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if !socketIsLive(path) {
			_ = os.Remove(path) // a supervisor that died without cleaning up
			continue
		}
		io, info, derr := dialSessionHost(ctx, path)
		if derr != nil {
			log.Warn("daemon: could not adopt a session", "session_id", id, "error", derr)
			continue
		}
		// A supervisor started by a DIFFERENT daemon (a test daemon under
		// its own XDG_CACHE_HOME beside a packaged one) is not ours to
		// drive. It cannot normally be here — the directory is per
		// runtime directory — so this is a check against a shared or
		// moved directory, not an expected case.
		if owner != "" && info.Owner != "" && info.Owner != owner {
			log.Warn("daemon: ignoring a session owned by another daemon",
				"session_id", id, "owner", info.Owner)
			_ = io.Close()
			continue
		}

		sess := &remoteSession{
			id:      id,
			io:      io,
			started: info.Started,
			name:    info.Name,
			clients: make(map[uint32]winSize),
			modes:   newTermModes(),
		}
		if sess.started.IsZero() {
			sess.started = time.Now()
		}
		t.mu.Lock()
		t.sessions[id] = sess
		if id > t.nextID {
			t.nextID = id
		}
		t.mu.Unlock()
		t.watchSession(sess)
		adopted = append(adopted, id)
		log.Info("daemon: adopted a session from a previous run",
			"session_id", id, "cwd", info.Cwd, "child_pid", info.ChildPID)
	}
	slices.Sort(adopted)
	if len(adopted) > 0 {
		t.reportSessions()
		t.syncSessionRegistry()
	}
	return adopted
}

// sessionIDFromSocket parses "<id>.sock".
func sessionIDFromSocket(name string) (uint64, bool) {
	base, ok := strings.CutSuffix(name, ".sock")
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseUint(base, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return id, true
}

// hostedSessionsSupported reports whether this build can put sessions in
// supervisor processes. It is the source of FeatureReattach.
func hostedSessionsSupported() bool { return true }
