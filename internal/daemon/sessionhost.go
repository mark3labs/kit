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
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/creack/pty"
)

// The session supervisor: one process per hosted session.
//
// This is the process that makes a session outlive its daemon. It owns
// the PTY master and the kit child on the other end of it, and it offers
// a socket that any daemon may dial to drive them. Stopping a daemon
// closes that socket and nothing else. The next daemon finds the socket,
// dials it, and is looking at the same session.
//
// The arrangement is forced by the kernel, not chosen: a PTY master
// cannot be re-opened for an existing slave, so whichever process holds
// the master is the only one that can ever reach that child. Keeping the
// master in the daemon made the daemon's lifetime the session's lifetime.
// Moving it into a process whose only job is to hold it removes the
// coupling entirely — and that process is small enough that "it crashed"
// stops being a realistic way to lose work.
//
// What the supervisor deliberately does NOT do: talk to clients, know
// about pairing, allocate ids, or listen on the network. All of that
// stays in the daemon. The supervisor is a terminal on a socket.

// RunSessionHost is the supervisor's entry point, run as
// `kit daemon session-host --config <path>`.
//
// It returns when the session's child has exited, which is the only thing
// that ends a session. Losing the daemon does not: that is the point.
func RunSessionHost(ctx context.Context, configPath string) error {
	cfg, err := readSessionHostConfig(configPath)
	if err != nil {
		return err
	}

	// A supervisor must not inherit the daemon's signal disposition. It
	// is deliberately not in the daemon's process group, but a terminal
	// hangup or an interrupt aimed at a shell that once touched this tree
	// would still be a way to lose a session; ignoring both leaves
	// SIGTERM, which is what terminateProcess uses, as the one way to end
	// it deliberately.
	signal.Ignore(syscall.SIGHUP, syscall.SIGINT)

	ln, err := listenSessionHost(cfg.Socket)
	if err != nil {
		return err
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(cfg.Socket)
	}()

	h := &sessionHost{
		cfg:     cfg,
		name:    cfg.Name,
		started: time.Now(),
		modes:   newTermModes(),
		scroll:  newScrollback(scrollbackLimit),
		done:    make(chan struct{}),
	}
	if err := h.start(); err != nil {
		return err
	}

	// The child is the session. When it exits there is nothing left to
	// supervise, so the listener is closed and everything unwinds.
	go func() {
		h.wait()
		_ = ln.Close()
	}()

	go h.pump()

	for {
		conn, aerr := ln.Accept()
		if aerr != nil {
			break // listener closed: the child has gone, or we are stopping
		}
		if perr := checkPeer(conn); perr != nil {
			// Only the user's own daemon may drive a session. The socket
			// is already 0600 in a 0700 directory; this is the same
			// defence in depth the daemon's own socket has.
			log.Warn("session host: rejected a peer", "error", perr)
			_ = conn.Close()
			continue
		}
		h.serve(ctx, conn)
	}

	h.stopChild()
	return nil
}

// readSessionHostConfig loads and then removes the hand-off file.
//
// Removed immediately because it holds the caller's environment, which
// routinely includes API keys. It has served its purpose the moment it is
// parsed, and a session that runs for days should not leave it on disk.
func readSessionHostConfig(path string) (SessionHostConfig, error) {
	var cfg SessionHostConfig
	if path == "" {
		return cfg, fmt.Errorf("daemon: session host needs --config")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("daemon: session host config: %w", err)
	}
	_ = os.Remove(path)
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("daemon: bad session host config: %w", err)
	}
	if cfg.ID == 0 || cfg.Socket == "" {
		return cfg, fmt.Errorf("daemon: session host config names no session")
	}
	return cfg, nil
}

// listenSessionHost binds a supervisor's socket.
//
// A live socket at this path is another supervisor for the same session
// id, which must never be disturbed: unlinking it would leave that
// session running and permanently unreachable, the exact failure this
// design exists to remove.
func listenSessionHost(path string) (net.Listener, error) {
	if err := mkdirPrivate(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("daemon: session host dir: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		if socketIsLive(path) {
			return nil, fmt.Errorf("daemon: session %s is already hosted", filepath.Base(path))
		}
		if rerr := os.Remove(path); rerr != nil {
			return nil, fmt.Errorf("daemon: remove stale session socket: %w", rerr)
		}
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("daemon: session host listen: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("daemon: secure session socket: %w", err)
	}
	return ln, nil
}

// sessionHost is one supervised session.
type sessionHost struct {
	cfg     SessionHostConfig
	started time.Time

	cmd  *exec.Cmd
	ptmx *os.File

	// modes and scroll are what a client that attaches later is owed: the
	// terminal state the child set once and will not repeat, and enough
	// recent output to put a screen back without waiting for the child to
	// repaint. Both live HERE rather than in the daemon, because the
	// daemon is the part that goes away.
	modes  *termModes
	scroll *scrollback

	mu   sync.Mutex
	name string
	// sink and conn are the daemon currently driving this session, if
	// any. Both are kept because closing a sink only stops WRITES: the
	// socket underneath has a reader on it, and a daemon that has been
	// replaced must be cut off entirely, not merely muted.
	sink *frameSink
	conn net.Conn

	doneOnce sync.Once
	done     chan struct{}
}

// start opens the PTY and launches the session's kit child.
func (h *sessionHost) start() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("daemon: resolve kit binary: %w", err)
	}
	dir, args := specCommand(h.cfg.Spec)

	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Env = childEnv(specBase(os.Environ(), h.cfg.Spec), h.cfg.Terminal, h.cfg.Env)
	// The child must not outlive its supervisor: this process is the only
	// holder of its PTY master, so a child that survived it would be
	// exactly the unreachable session the old design produced.
	cmd.SysProcAttr = applyChildDeathSignal(cmd.SysProcAttr)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		return fmt.Errorf("daemon: session host start child: %w", err)
	}
	h.cmd, h.ptmx = cmd, ptmx
	return nil
}

// pump reads the child's output for the session's whole life.
//
// It runs whether or not a daemon is connected. A session with no daemon
// is still working — the child is mid-turn, writing output nobody is
// watching — and a pump that stopped when the socket dropped would block
// the child on a full PTY buffer until someone reattached.
func (h *sessionHost) pump() {
	buf := make([]byte, chunkSize)
	for {
		n, err := h.ptmx.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			h.modes.Feed(chunk)
			h.scroll.Write(chunk)
			h.send(Frame{Type: FrameData, Payload: chunk})
		}
		if err != nil {
			h.finish()
			return // EIO when the child exits, or the PTY closed
		}
	}
}

// send writes one frame to the connected daemon, if there is one. There
// usually is; when there is not, the frame is dropped and the session
// carries on, because the scrollback is what a later daemon reads instead.
//
// The sink serialises its own writes, so no lock is held across the
// write: holding h.mu there would block a daemon hand-over behind a
// client that has stopped reading.
func (h *sessionHost) send(f Frame) {
	h.mu.Lock()
	sink := h.sink
	h.mu.Unlock()
	if sink == nil {
		return
	}
	if f.Type != FrameData {
		_ = sink.write(f)
		return
	}
	// Split oversized output the same way the daemon does, so one large
	// read cannot exceed the frame length field.
	for b := f.Payload; len(b) > 0; {
		n := min(chunkSize, len(b))
		if err := sink.write(Frame{Type: FrameData, Payload: b[:n]}); err != nil {
			return
		}
		b = b[n:]
	}
}

// serve drives one daemon connection until it drops.
//
// Connections are served ONE at a time, by design: exactly one daemon
// runs per runtime directory, so a second connection means the first
// daemon has gone and this one is taking over. Replacing the sink rather
// than fanning out keeps the session with a single driver and makes the
// hand-over instantaneous.
func (h *sessionHost) serve(ctx context.Context, conn net.Conn) {
	sink := newFrameSink(conn)

	// The hello goes out BEFORE the sink is published.
	//
	// dialSessionHost requires the hello to be the first frame and refuses
	// the connection otherwise, and a refused connection means an adopted
	// session is unreachable — the exact failure this design exists to
	// remove. pump() writes through h.sink the instant it becomes visible,
	// so a child producing output in the window between publishing and
	// greeting would put DATA on the wire ahead of the hello.
	h.mu.Lock()
	name, childPID := h.name, h.childPID()
	h.mu.Unlock()

	info := sessionHostHello(h.cfg, childPID, h.started, name, h.reportedCwd())
	payload, err := EncodeSessionHostInfo(info)
	if err == nil {
		err = sink.write(Frame{Type: FrameHello, Payload: payload})
	}
	if err != nil {
		// A connection we could not introduce ourselves on is no use to
		// anyone, and must not displace a daemon that is driving this
		// session perfectly well.
		log.Warn("session host: could not greet a daemon", "session_id", h.cfg.ID, "error", err)
		sink.close()
		_ = conn.Close()
		return
	}

	h.mu.Lock()
	prevSink, prevConn := h.sink, h.conn
	h.sink, h.conn = sink, conn
	h.mu.Unlock()
	if prevSink != nil {
		prevSink.close()
	}
	if prevConn != nil {
		// The SOCKET, not just the sink. A closed sink stops writes and
		// nothing else: the previous daemon's readDaemon goroutine is
		// still parked on this connection and would go on feeding the
		// session input, resizes, renames — and FrameBye, which ends it —
		// after another daemon has taken over. Closing it here is what
		// makes "one driver at a time" true rather than merely intended.
		_ = prevConn.Close()
	}

	go func() {
		defer func() {
			_ = conn.Close()
			sink.close()
			h.mu.Lock()
			if h.sink == sink {
				h.sink = nil // the daemon went away; the session has not
			}
			if h.conn == conn {
				h.conn = nil
			}
			h.mu.Unlock()
		}()
		h.readDaemon(ctx, conn)
	}()
}

// readDaemon consumes the driving daemon's frames.
func (h *sessionHost) readDaemon(ctx context.Context, r io.Reader) {
	for {
		frame, err := ReadFrame(r)
		if err != nil {
			return
		}
		switch frame.Type {
		case FrameData:
			if _, werr := h.ptmx.Write(frame.Payload); werr != nil {
				h.finish()
				return
			}
		case FrameResize:
			if cols, rows, derr := DecodeResize(frame.Payload); derr == nil && cols > 0 && rows > 0 {
				_ = pty.Setsize(h.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
			}
		case FrameSessionRedraw:
			cols, rows, derr := DecodeResize(frame.Payload)
			if derr != nil {
				cols, rows = 0, 0
			}
			h.redraw(cols, rows)
		case FrameSessionRename:
			h.rename(string(frame.Payload))
		case FrameBye:
			// The daemon is ending this session deliberately (the user
			// killed it, or the daemon is tearing down a session it owns
			// outright). This is the ONE frame that stops a supervisor.
			h.stopChild()
			return
		}
		if ctx.Err() != nil {
			return
		}
	}
}

// redraw restores a client that has just taken over the screen.
//
// Three things in order, and the order matters. The terminal modes first,
// so everything after is drawn into a terminal in the state the child
// believes it is drawing on. Then the scrollback, which puts a screen
// back immediately — this is what a client sees after a daemon restart,
// where nothing else could have repainted it. Then the size nudge, so the
// child itself repaints over the top with the authoritative frame.
func (h *sessionHost) redraw(cols, rows int) {
	if replay := h.modes.Replay(); len(replay) > 0 {
		h.send(Frame{Type: FrameData, Payload: replay})
	}
	if recent := h.scroll.Bytes(); len(recent) > 0 {
		h.send(Frame{Type: FrameData, Payload: recent})
	}
	if cols < 2 || rows < 2 {
		return
	}
	go func() {
		_ = pty.Setsize(h.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows - 1)})
		time.Sleep(redrawNudgeGap)
		_ = pty.Setsize(h.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	}()
}

// rename records a display name. It lives here so it survives the daemon
// that set it: a name the user gave a session is part of the session.
func (h *sessionHost) rename(name string) {
	h.mu.Lock()
	h.name = name
	h.mu.Unlock()
}

// reportedCwd reads the directory the child settled on, empty while the
// directory picker is still up.
func (h *sessionHost) reportedCwd() string {
	return readReportedCwd(h.cfg.Env[sessionCwdEnv])
}

func (h *sessionHost) childPID() int {
	if h.cmd == nil || h.cmd.Process == nil {
		return 0
	}
	return h.cmd.Process.Pid
}

// wait blocks until the child has exited.
func (h *sessionHost) wait() {
	if h.cmd != nil && h.cmd.Process != nil {
		_, _ = h.cmd.Process.Wait()
	}
	h.finish()
	<-h.done
}

// finish marks the session over and tells the driving daemon once.
func (h *sessionHost) finish() {
	h.doneOnce.Do(func() {
		h.send(Frame{Type: FrameBye})
		close(h.done)
	})
}

// stopChild ends the session's child, politely and then not.
func (h *sessionHost) stopChild() {
	if pid := h.childPID(); pid > 0 {
		terminateProcess(pid)
	}
	if h.ptmx != nil {
		_ = h.ptmx.Close()
	}
	h.finish()
}
