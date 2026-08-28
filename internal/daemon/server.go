package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"sync"
	"time"

	"github.com/creack/pty"
)

// ServeOptions controls the daemon loop. Zero values are valid.
type ServeOptions struct {
	// Code forces a specific pairing code instead of a random one.
	// Intended for tests.
	Code string
}

// Serve runs the daemon until ctx is cancelled: derive the endpoint from a
// pairing code, then host remote sessions over it. The code stays valid for
// the daemon's lifetime; each verified client gets its own session (its own
// `kit --pick-dir` child in its own PTY) and sessions end independently.
func Serve(ctx context.Context, opts ServeOptions) error {
	if _, err := FindTunnelBinary(); err != nil {
		return err // fail fast with a clear message instead of per attempt
	}

	code := opts.Code
	if code == "" {
		var err error
		code, err = GenerateCode()
		if err != nil {
			return err
		}
	} else if _, err := NormalizeCode(code); err != nil {
		return err
	}
	seed, err := SeedFromCode(code)
	if err != nil {
		return err
	}
	seedHex := fmt.Sprintf("%x", seed)

	printBanner(code)

	// If the tunnel process dies unexpectedly (crash, kill), restart it
	// with the same seed: the endpoint id is derived from the code, so the
	// same code finds us again. Live sessions do not survive the restart.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		tun, err := StartTunnel(ctx, "serve", seedHex)
		if err != nil {
			return err
		}
		if _, err := tun.WaitStatus(ctx, "READY", 30*time.Second); err != nil {
			tun.Close()
			return fmt.Errorf("daemon: tunnel failed to start: %w", err)
		}

		err = runSessions(ctx, tun)

		tun.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return fmt.Errorf("daemon: tunnel ended: %w", err)
		}
		fmt.Println("  Listener restarted — same pairing code, waiting…")
	}
}

// remoteSession is one connected client and its kit child process.
type remoteSession struct {
	id   uint32
	cmd  *exec.Cmd
	ptmx *os.File
}

// sessionTable tracks live sessions. The tunnel's stdout frames are read by
// a single goroutine, so map access is confined to it plus teardown paths
// guarded by mu.
type sessionTable struct {
	tunnel   *Tunnel
	mu       sync.Mutex
	sessions map[uint32]*remoteSession
	writeMu  sync.Mutex // tunnel stdin is shared by all session pumps
}

// writeTo sends one frame to the tunnel stdin. Errors are the caller's to
// ignore or handle; a closed tunnel means everything is going away anyway.
func (t *sessionTable) writeTo(frame Frame) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return WriteFrame(t.tunnel.Stdin(), frame.Type, frame.Session, frame.Payload)
}

func runSessions(ctx context.Context, tun *Tunnel) error {
	table := &sessionTable{tunnel: tun, sessions: make(map[uint32]*remoteSession)}
	defer table.teardownAll()

	// Child exits are noticed by the per-session PTY reader; when a client
	// detaches (BYE) or the tunnel drops the session (SESSION_CLOSED), the
	// reader loop tears that one session down.
	for {
		frame, err := ReadFrame(tun.Stdout())
		if err != nil {
			return nil // tunnel stream/process ended
		}
		switch frame.Type {
		case FrameSessionOpen:
			table.openSession(frame.Session)
		case FrameSessionClosed, FrameBye:
			table.closeSession(frame.Session)
		case FrameData:
			if s := table.get(frame.Session); s != nil {
				if _, err := s.ptmx.Write(frame.Payload); err != nil {
					table.closeSession(frame.Session)
				}
			}
		case FrameResize:
			if s := table.get(frame.Session); s != nil {
				if cols, rows, derr := DecodeResize(frame.Payload); derr == nil {
					_ = pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
				}
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// openSession spawns a fresh `kit --pick-dir` child for a newly verified
// client. A failure to spawn is reported to that client as a BYE; the
// daemon and other sessions continue.
func (table *sessionTable) openSession(id uint32) {
	s := &remoteSession{id: id}
	child, ptmx, err := spawnPickDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: session %d: spawn: %v\n", id, err)
		_ = table.writeTo(Frame{Type: FrameBye, Session: id})
		return
	}
	s.cmd, s.ptmx = child, ptmx

	table.mu.Lock()
	table.sessions[id] = s
	table.mu.Unlock()
	fmt.Printf("  Session %d started.\n", id)

	// PTY -> remote: raw child output as DATA frames tagged with the id.
	go func(s *remoteSession) {
		buf := make([]byte, chunkSize)
		for {
			n, err := s.ptmx.Read(buf)
			if n > 0 {
				// Write errors mean the tunnel is gone; the restart loop
				// takes over from here.
				_ = table.writeTo(Frame{Type: FrameData, Session: s.id, Payload: buf[:n]})
			}
			if err != nil {
				return // EIO when the child exits, or PTY closed
			}
		}
	}(s)

	// Child lifecycle: when the child quits (user exited the remote TUI),
	// end exactly this session — other clients are unaffected.
	go func(s *remoteSession) {
		_, _ = s.cmd.Process.Wait()
		table.closeSession(s.id)
	}(s)
}

// applyResize was removed: the child exists before the session is
// registered and stdout frames are processed in order, so a RESIZE always
// finds a live PTY. Resizes that arrive during the handshake are buffered
// by the pipe and applied right after registration.

// closeSession tears down one session: tell the client we are done, stop
// the child, and free the table slot. Idempotent.
func (t *sessionTable) closeSession(id uint32) {
	t.mu.Lock()
	s, ok := t.sessions[id]
	delete(t.sessions, id)
	t.mu.Unlock()
	if !ok {
		return
	}

	_ = t.writeTo(Frame{Type: FrameBye, Session: id})

	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	fmt.Printf("  Session %d ended.\n", id)
}

func (t *sessionTable) teardownAll() {
	t.mu.Lock()
	ids := make([]uint32, 0, len(t.sessions))
	for id := range t.sessions {
		ids = append(ids, id)
	}
	t.mu.Unlock()
	for _, id := range ids {
		t.closeSession(id)
	}
}

func (t *sessionTable) get(id uint32) *remoteSession {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sessions[id]
}

// spawnPickDir starts a kit child with the hidden --pick-dir flag in the
// daemon user's home directory, so the remote peer picks the session's
// working directory from the modal rendered inside the PTY.
func spawnPickDir() (*exec.Cmd, *os.File, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve kit binary: %w", err)
	}

	cmd := exec.Command(exe, "--pick-dir")
	cmd.Dir = homeDir()
	env := append(os.Environ(), "KIT_REMOTE_SESSION=1")
	if os.Getenv("TERM") == "" {
		env = append(env, "TERM=xterm-256color")
	}
	cmd.Env = env

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		return nil, nil, fmt.Errorf("start child: %w", err)
	}
	return cmd, ptmx, nil
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	return "/"
}

func printBanner(code string) {
	fmt.Println()
	fmt.Println("  kit daemon")
	fmt.Println()
	fmt.Printf("  Pairing code: %s\n", FormatCode(code))
	fmt.Println("  Enter this code on the remote machine with: kit --remote " + code)
	fmt.Println("  The code stays valid while the daemon runs; multiple sessions allowed.")
	fmt.Println()
}
