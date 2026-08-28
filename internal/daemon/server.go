package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
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

// Serve runs the daemon accept loop until ctx is cancelled: generate a
// pairing code, wait for a verified remote connection, host one session in
// a PTY, then rotate the code and wait again.
//
// Each pairing attempt gets a fresh tunnel process. Because the endpoint
// identity is derived from the code, a wrong guess costs the attacker a
// full endpoint rebind on our side — an implicit rate limit on top of the
// code's entropy.
func Serve(ctx context.Context, opts ServeOptions) error {
	if _, err := FindTunnelBinary(); err != nil {
		return err // fail fast with a clear message instead of per attempt
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
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

		tun, err := waitForPairing(ctx, code)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if tun != nil {
			runSession(ctx, tun)
			fmt.Println()
		}
	}
}

// waitForPairing prints the banner, starts a tunnel in serve mode, and
// blocks until the handshake verdict. It returns the tunnel still holding
// the verified connection (ownership transfers to the caller), or nil when
// the attempt was denied (the loop rotates the code).
func waitForPairing(ctx context.Context, code string) (*Tunnel, error) {
	seed, err := SeedFromCode(code)
	if err != nil {
		return nil, err
	}
	printBanner(code)

	tun, err := StartTunnel(ctx, "serve", fmt.Sprintf("%x", seed))
	if err != nil {
		return nil, err
	}

	status, err := tun.WaitStatus(ctx, "READY", 30*time.Second)
	if err != nil {
		tun.Close()
		return nil, fmt.Errorf("daemon: tunnel failed to start: %w", err)
	}
	if nodeID, ok := strings.CutPrefix(status, "READY node_id="); ok {
		fmt.Printf("  Endpoint %s\n", nodeID)
	}
	fmt.Println("  Waiting for a connection… (Ctrl+C to stop)")

	// The tunnel emits PAIRING as soon as a peer dials, then either
	// VERIFIED or DENIED. No deadline: the operator decides how long the
	// code stays live; SIGINT (ctx) is the exit path.
	_, _ = tun.WaitAnyStatus(ctx, 0, "PAIRING")
	verdict, err := tun.WaitAnyStatus(ctx, 0, "VERIFIED", "DENIED")
	if err != nil {
		tun.Close()
		return nil, fmt.Errorf("daemon: tunnel ended during pairing: %w", err)
	}
	if strings.HasPrefix(verdict, "DENIED") {
		tun.Close()
		fmt.Printf("  Pairing attempt denied (%s) — rotating code.\n",
			strings.TrimPrefix(verdict, "DENIED "))
		return nil, nil
	}

	fmt.Println("  Connected — session started.")
	return tun, nil
}

// runSession hosts one remote session over an already-verified tunnel:
// spawn `kit --pick-dir` in a PTY, relay frames between the tunnel and the
// PTY, and clean up on any exit path (child quits, client detaches,
// network drops, SIGINT).
func runSession(ctx context.Context, tun *Tunnel) {
	defer tun.Close()

	// Catch the client's initial RESIZE, which arrives right after the
	// handshake, so the child starts with the peer's real window size.
	pendingResize := drainInitialResize(tun)

	child, ptmx, err := spawnPickDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: spawn session: %v\n", err)
		return
	}
	if pendingResize != nil {
		_ = pty.Setsize(ptmx, pendingResize)
	}
	defer func() { _ = ptmx.Close() }()

	done := make(chan struct{})
	var once sync.Once
	var writeMu sync.Mutex // tunnel stdin is written by the pty pump and resizes
	finish := func() { once.Do(func() { close(done) }) }

	// Remote -> PTY: DATA frames feed the child; RESIZE applies winsize;
	// BYE (client detached) ends the session.
	go func() {
		defer finish()
		for {
			t, payload, err := ReadFrame(tun.Stdout())
			if err != nil {
				return
			}
			switch t {
			case FrameData:
				if _, err := ptmx.Write(payload); err != nil {
					return
				}
			case FrameResize:
				cols, rows, derr := DecodeResize(payload)
				if derr != nil {
					continue
				}
				_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
			case FrameBye:
				return
			}
		}
	}()

	// PTY -> remote: raw child output as DATA frames.
	go func() {
		defer finish()
		buf := make([]byte, chunkSize)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				writeMu.Lock()
				werr := WriteDataFrames(tun.Stdin(), buf[:n])
				writeMu.Unlock()
				if werr != nil {
					return
				}
			}
			if err != nil {
				return // EIO when the child exits, or PTY closed
			}
		}
	}()

	select {
	case <-done:
	case <-tun.Exited(): // tunnel died (peer gone, network timeout)
	case <-ctx.Done():
	}

	// Tear down: tell the remote we are done, stop the child.
	_ = WriteFrame(tun.Stdin(), FrameBye, nil)
	_ = ptmx.Close()
	_ = child.Process.Kill()
	_, _ = child.Process.Wait()

	fmt.Println("  Session ended.")
}

// drainInitialResize reads frames for a short window after the handshake
// and returns the first RESIZE seen. DATA frames in this window are
// dropped — the client sends nothing but its size until the picker renders.
func drainInitialResize(tun *Tunnel) *pty.Winsize {
	var result *pty.Winsize
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			t, payload, err := ReadFrame(tun.Stdout())
			if err != nil {
				return
			}
			if t == FrameResize {
				cols, rows, derr := DecodeResize(payload)
				if derr == nil {
					result = &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}
				}
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}
	return result
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
	fmt.Println()
}
