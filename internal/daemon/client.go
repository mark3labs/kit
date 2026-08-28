package daemon

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/x/term"
)

// detachKey is Ctrl-] (the classic telnet escape). Pressed alone, it
// detaches the local terminal from the remote session instead of
// forwarding the keystroke.
const detachKey = 0x1d

// RunRemote attaches the local terminal to a daemon session identified by
// a pairing code: `kit --remote A1B2C3D4`. The remote daemon shows its
// directory picker inside this terminal, then the session TUI takes over.
//
// All user-facing messages go to stderr — stdout is the remote session's
// rendering surface and must stay pristine.
func RunRemote(ctx context.Context, rawCode string) error {
	code, err := NormalizeCode(rawCode)
	if err != nil {
		return err
	}
	seed, err := SeedFromCode(code)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "Connecting to daemon…")
	tun, err := StartTunnel(ctx, "dial", fmt.Sprintf("%x", seed))
	if err != nil {
		return err
	}
	defer tun.Close()

	if _, err := tun.WaitStatus(ctx, "VERIFIED", 35*time.Second); err != nil {
		last := tun.LastStatuses()
		switch {
		case strings.Contains(last, "rejected the pairing code"):
			return fmt.Errorf("daemon rejected the pairing code")
		case strings.Contains(last, "No addressing information available"):
			return fmt.Errorf("no daemon is live for this pairing code (it may have expired or already been used)")
		case strings.Contains(last, "timed out"):
			return fmt.Errorf("could not reach the daemon (network or relay issue)")
		}
		return fmt.Errorf("daemon: %w", err)
	}

	stdinFD := os.Stdin.Fd()
	stdoutFD := os.Stdout.Fd()
	if !term.IsTerminal(stdinFD) || !term.IsTerminal(stdoutFD) {
		return fmt.Errorf("remote mode needs an interactive terminal")
	}
	oldState, err := term.MakeRaw(stdinFD)
	if err != nil {
		return fmt.Errorf("daemon: raw mode: %w", err)
	}
	defer func() { _ = term.Restore(stdinFD, oldState) }()

	done := make(chan struct{})
	var once sync.Once
	var writeMu sync.Mutex // tunnel stdin is written by pumps and resizes
	var detached atomic.Bool
	finish := func() { once.Do(func() { close(done) }) }

	// Local keystrokes -> remote. A lone Ctrl-] detaches.
	go func() {
		defer finish()
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if n == 1 && buf[0] == detachKey {
					writeMu.Lock()
					_ = WriteFrame(tun.Stdin(), FrameBye, 0, nil)
					writeMu.Unlock()
					detached.Store(true)
					return
				}
				writeMu.Lock()
				werr := WriteDataFrames(tun.Stdin(), 0, buf[:n])
				writeMu.Unlock()
				if werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Remote output -> local terminal.
	go func() {
		defer finish()
		for {
			frame, err := ReadFrame(tun.Stdout())
			if err != nil {
				return
			}
			switch frame.Type {
			case FrameData:
				if _, werr := os.Stdout.Write(frame.Payload); werr != nil {
					return
				}
			case FrameBye:
				return
			}
		}
	}()

	// Window size: send the current size now, then on every SIGWINCH.
	stopResize := watchResize(stdoutFD, func(cols, rows int) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = WriteFrame(tun.Stdin(), FrameResize, 0, EncodeResize(cols, rows))
	})
	defer stopResize()
	if cols, rows, err := term.GetSize(stdoutFD); err == nil {
		_ = WriteFrame(tun.Stdin(), FrameResize, 0, EncodeResize(cols, rows))
	}

	select {
	case <-done:
	case <-ctx.Done():
	}

	_ = WriteFrame(tun.Stdin(), FrameBye, 0, nil)
	tun.Close()
	_ = term.Restore(stdinFD, oldState)
	if detached.Load() {
		fmt.Fprintln(os.Stderr, "\nDetached from remote session.")
	} else {
		fmt.Fprintln(os.Stderr, "\nRemote session ended.")
	}
	return nil
}
