package daemon

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/kit/internal/clipboard"
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

// pasteKey is Ctrl-V. In a remote session the client intercepts a bare
// Ctrl-V: it reads THIS machine's clipboard and streams any image to the
// daemon as FrameClipboard chunks (the host TUI would otherwise read the
// host's clipboard, which is the wrong one). When the clipboard holds no
// image the keystroke is forwarded verbatim.
const pasteKey = 0x16

// terminalResetSeq restores terminal modes the remote TUI may have enabled
// and we may not have seen disabled: alt screen off, cursor on, mouse and
// bracketed paste off, kitty keyboard protocol popped. Emitted by the
// client on teardown because the remote side may die mid-frame (SIGKILL,
// network loss) without ever sending its own restore sequences.
const terminalResetSeq = "\x1b[?1049l\x1b[?25h" +
	"\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l" +
	"\x1b[?2004l" +
	"\x1b[<u\x1b[<u\x1b[<u"

// PairOptions controls `kit remote --pair`. Code is required; all other
// fields are optional.
type PairOptions struct {
	// Name pre-selects the saved host name (skips the interactive prompt).
	Name string
	// Code is the one-time pairing code shown by 'kit daemon pair'.
	Code string
}

// RunPair performs one-time pairing against a host's pairing window: it
// proves knowledge of the one-time code, hands the host this machine's
// signing public key, and stores the host under a friendly name for
// codeless reconnection with RunHost.
//
// All user-facing messages go to stderr — nothing touches stdout except
// the eventual remote session.
func RunPair(ctx context.Context, opts PairOptions) error {
	rawCode := opts.Code
	if rawCode == "" {
		return fmt.Errorf("daemon: --pair needs the code shown by 'kit daemon pair' on the host")
	}
	code, err := NormalizeCode(rawCode)
	if err != nil {
		return err
	}
	seed, err := SeedFromCode(code)
	if err != nil {
		return err
	}
	clientSeed, err := LoadClientIdentity()
	if err != nil {
		return err
	}
	kp := NewClientKeyPair(clientSeed)

	fmt.Fprintln(os.Stderr, "Pairing with host…")
	tun, err := StartTunnel(ctx, TunnelOptions{
		Mode: "dial-pair",
		Args: []string{
			"--client-pub-hex", kp.PubHex,
			"--timeout", "150", // covers the human decision on the host
		},
		Env: []string{"KIT_TUNNEL_PAIR_SEED=" + fmt.Sprintf("%x", seed)},
	})
	if err != nil {
		return err
	}
	defer tun.Close()

	st, err := tun.WaitAnyStatus(ctx, 160*time.Second, "PAIRED", "DENIED")
	if err != nil {
		return fmt.Errorf("pairing failed: %w (last: %s)", err, tun.LastStatuses())
	}
	if strings.HasPrefix(st, "DENIED") {
		reason := strings.TrimSpace(strings.TrimPrefix(st, "DENIED reason="))
		switch {
		case strings.Contains(reason, "rejected on the host"):
			return fmt.Errorf("the pairing request was rejected on the host")
		case strings.Contains(reason, "bad pairing tag"):
			return fmt.Errorf("the host rejected the pairing code")
		default:
			return fmt.Errorf("pairing denied: %s", reason)
		}
	}
	hostID, ok := strings.CutPrefix(st, "PAIRED host_endpoint_id=")
	if !ok || len(hostID) != 64 {
		return fmt.Errorf("daemon: pairing completed without a host endpoint id")
	}

	name := opts.Name
	if name == "" {
		name = promptHostName(ctx)
	}
	if err := SaveHost(name, hostID); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Paired with host %q (fp %s).\n", name, fingerprintShort(Fingerprint(mustHexDecode(hostID))))
	fmt.Fprintf(os.Stderr, "Connect with: kit remote --host %s\n", name)
	return nil
}

// promptHostName asks for a friendly name on the terminal, defaulting to
// the local hostname. Returns the default when ctx is cancelled while
// waiting for input.
func promptHostName(ctx context.Context) string {
	host, _ := os.Hostname()
	host = strings.SplitN(host, ".", 2)[0]
	if !term.IsTerminal(os.Stdin.Fd()) {
		return host
	}
	fmt.Fprintf(os.Stderr, "Save this host as [%s]: ", host)
	line := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		line <- strings.TrimSpace(text)
	}()
	var answer string
	select {
	case answer = <-line:
	case <-ctx.Done():
		return host
	}
	if answer == "" {
		return host
	}
	return answer
}

// RunHost attaches the local terminal to a paired daemon by name:
// `kit remote --host zora`. Authentication is by client signing key; no
// pairing code is involved. The remote daemon shows its directory picker
// inside this terminal, then the session TUI takes over.
func RunHost(ctx context.Context, name string) error {
	entry, err := GetHost(name)
	if err != nil {
		return err
	}
	clientSeed, err := LoadClientIdentity()
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "Connecting to daemon…")
	tun, err := StartTunnel(ctx, TunnelOptions{
		Mode: "dial-host",
		Args: []string{
			"--endpoint-id", entry.EndpointID,
			"--timeout", "35",
		},
		Env: []string{"KIT_TUNNEL_CLIENT_SEED=" + fmt.Sprintf("%x", clientSeed)},
	})
	if err != nil {
		return err
	}
	defer tun.Close()

	if _, err := tun.WaitStatus(ctx, "VERIFIED", 40*time.Second); err != nil {
		last := tun.LastStatuses()
		switch {
		case strings.Contains(last, "client not paired"):
			return fmt.Errorf("the host no longer knows this machine — pair again with 'kit remote --pair <code>'")
		case strings.Contains(last, "No addressing information available"):
			return fmt.Errorf("could not resolve the daemon's endpoint (is 'kit daemon' running on the host?)")
		case strings.Contains(last, "timed out"):
			return fmt.Errorf("could not reach the daemon (network or relay issue)")
		}
		return fmt.Errorf("daemon: %w", err)
	}
	_ = TouchHost(name)

	stdinFD := os.Stdin.Fd()
	stdoutFD := os.Stdout.Fd()
	if !term.IsTerminal(stdinFD) || !term.IsTerminal(stdoutFD) {
		return fmt.Errorf("remote mode needs an interactive terminal")
	}
	oldState, err := term.MakeRaw(stdinFD)
	if err != nil {
		return fmt.Errorf("daemon: raw mode: %w", err)
	}
	defer func() {
		_, _ = os.Stdout.WriteString(terminalResetSeq)
		_ = term.Restore(stdinFD, oldState)
	}()

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
				if n == 1 && buf[0] == pasteKey {
					// Image paste: read the local clipboard and stream any
					// image to the daemon. No image — forward the keystroke
					// so the host keeps its normal Ctrl-V behavior.
					if img, err := clipboard.ReadImage(); err == nil && len(img.Data) > 0 {
						writeMu.Lock()
						for _, payload := range EncodeClipboardChunks(img.MediaType, img.Data) {
							if werr := WriteFrame(tun.Stdin(), FrameClipboard, 0, payload); werr != nil {
								writeMu.Unlock()
								return
							}
						}
						writeMu.Unlock()
						fmt.Fprintln(os.Stderr, "Image sent from local clipboard.")
						continue
					}
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
	_, _ = os.Stdout.WriteString(terminalResetSeq)
	_ = term.Restore(stdinFD, oldState)
	if detached.Load() {
		fmt.Fprintln(os.Stderr, "\nDetached from remote session.")
	} else {
		fmt.Fprintln(os.Stderr, "\nRemote session ended.")
	}
	return nil
}
