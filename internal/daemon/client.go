package daemon

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/kit/internal/clipboard"
	"github.com/mark3labs/kit/internal/ui"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/x/term"
)

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
// `kit remote --host homelab`. Authentication is by client signing key; no
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

	// Session choice happens before anything takes over the terminal: the
	// daemon reports its live sessions and, when any exist, the user picks
	// attach vs new. Pairing is the authorization — any paired client may
	// attach to any session.
	ctrlCh := make(chan Frame, 8)
	endedCh := make(chan struct{})
	var endedOnce sync.Once
	sessionEnded := func() { endedOnce.Do(func() { close(endedCh) }) }

	done := make(chan struct{})
	var once sync.Once
	var writeMu sync.Mutex // tunnel stdin is written by pumps and resizes
	var detached atomic.Bool
	var attached atomic.Bool
	finish := func() { once.Do(func() { close(done) }) }

	// Remote output -> local terminal. Until a session is attached,
	// control frames feed the choice flow and DATA is dropped.
	go func() {
		defer finish()
		for {
			frame, err := ReadFrame(tun.Stdout())
			if err != nil {
				return
			}
			switch frame.Type {
			case FrameData:
				if attached.Load() {
					if _, werr := os.Stdout.Write(frame.Payload); werr != nil {
						return
					}
				}
			case FrameSessionListReply, FrameSessionAttachAck:
				select {
				case ctrlCh <- frame:
				default:
				}
			case FrameBye, FrameSessionClosed:
				sessionEnded()
				return
			}
		}
	}()

	chosen, err := chooseSession(tun, ctrlCh, endedCh)
	if err != nil {
		return err
	}
	select {
	case <-endedCh:
		return fmt.Errorf("remote session ended")
	default:
	}

	oldState, err := term.MakeRaw(stdinFD)
	if err != nil {
		return fmt.Errorf("daemon: raw mode: %w", err)
	}
	defer func() {
		_, _ = os.Stdout.WriteString(terminalResetSeq)
		_ = term.Restore(stdinFD, oldState)
	}()

	// Bind this connection: attach to the chosen session, or logical 0 for
	// a fresh one (the daemon spawns the child; its directory picker
	// renders inside this terminal).
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, chosen)
	if err := WriteFrame(tun.Stdin(), FrameSessionAttach, 0, payload); err != nil {
		return err
	}
	select {
	case f := <-ctrlCh:
		if f.Type != FrameSessionAttachAck || len(f.Payload) < 9 || f.Payload[8] != 1 {
			return fmt.Errorf("the host refused the attach")
		}
		if f.Type == FrameSessionAttachAck {
			fmt.Fprintf(os.Stderr, "DEBUG: ack assigned=%d ok=%d\n", binary.BigEndian.Uint64(f.Payload[:8]), f.Payload[8])
		}
	case <-time.After(10 * time.Second):
		return fmt.Errorf("daemon did not answer the attach")
	case <-endedCh:
		return fmt.Errorf("remote session ended")
	}
	attached.Store(true)

	// Stdin reader: os.Stdin.Read blocks, so it feeds a channel and the
	// pump below can also react to the Esc idle-flush timer.
	readCh := make(chan []byte, 4)
	readErr := make(chan error, 1)
	go func() {
		defer close(readCh)
		rbuf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(rbuf)
			if n > 0 {
				out := make([]byte, n)
				copy(out, rbuf[:n])
				readCh <- out
			}
			if err != nil {
				readErr <- err
				return
			}
		}
	}()

	go func() {
		defer finish()
		scanner := &keyScanner{}
		suppressRel := false // swallow the ctrl+v release after a successful image interception
		var leaderBuf []byte // pending Ctrl-X chord prefix (nil = none)
		var idleTimer *time.Timer
		var idleC <-chan time.Time
		armIdle := func() {
			if scanner.PendingEscape() {
				d := max(escIdleFlush-time.Since(scanner.escAt), 0)
				if idleTimer == nil {
					idleTimer = time.NewTimer(d)
				} else {
					idleTimer.Stop()
					idleTimer.Reset(d)
				}
				idleC = idleTimer.C
			} else if idleTimer != nil {
				idleTimer.Stop()
				idleC = nil
			}
		}
		forward := func(data []byte) bool {
			writeMu.Lock()
			defer writeMu.Unlock()
			return WriteDataFrames(tun.Stdin(), 0, data) == nil
		}
		handle := func(ev keyEvent) bool { // false = write error, give up
			if ev.Paste {
				// Image paste: read the local clipboard and stream any
				// image to the daemon. No image — forward the keystroke
				// so the host keeps its normal Ctrl-V behavior.
				img, imgErr := clipboard.ReadImage()
				if imgErr == nil && len(img.Data) > 0 {
					writeMu.Lock()
					sent := true
					for _, p := range EncodeClipboardChunks(img.MediaType, img.Data) {
						if werr := WriteFrame(tun.Stdin(), FrameClipboard, 0, p); werr != nil {
							sent = false
							break
						}
					}
					writeMu.Unlock()
					if !sent {
						return false
					}
					suppressRel = true // the matching release is ours
					fmt.Fprintln(os.Stderr, "Image sent from local clipboard.")
					return true // swallow the press bytes
				}
				suppressRel = false // forwarding the press; forward its release too
			}
			if ev.Release && suppressRel {
				suppressRel = false
				return true
			}
			if len(ev.Data) > 0 {
				return forward(ev.Data)
			}
			return true
		}
		armIdle()
		for {
			select {
			case chunk, ok := <-readCh:
				if !ok {
					return
				}
				for _, ev := range scanner.Feed(chunk) {
					// Detach chord: Ctrl-X then d. Anything else after a
					// pending Ctrl-X is forwarded together, so the host
					// TUI's own Ctrl-X chords (e.g. Ctrl-X e) keep working.
					if leaderBuf != nil {
						if ev.Leader {
							leaderBuf = append(leaderBuf, ev.Data...)
							continue
						}
						if len(ev.Data) == 1 && ev.Data[0] == 'd' && !ev.Release {
							writeMu.Lock()
							werr := WriteFrame(tun.Stdin(), FrameSessionDetach, 0, nil)
							writeMu.Unlock()
							if werr != nil {
								return
							}
							detached.Store(true)
							finish()
							return
						}
						if !forward(leaderBuf) {
							return
						}
						leaderBuf = nil
					}
					if ev.Leader {
						leaderBuf = append([]byte(nil), ev.Data...)
						continue
					}
					if ev.Release && leaderBuf != nil {
						continue // release of a swallowed chord prefix
					}
					if !handle(ev) {
						return
					}
				}
				armIdle()
			case <-idleC:
				idleC = nil
				for _, ev := range scanner.FlushPendingEscape() {
					if !handle(ev) {
						return
					}
				}
				armIdle()
			case <-readErr:
				return
			}
		}
	}()

	// Window size: send the current size now, then on every SIGWINCH. On
	// attach the daemon applies the minimum across all attached clients;
	// the nudge below forces the child to repaint fully.
	stopResize := watchResize(stdoutFD, func(cols, rows int) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = WriteFrame(tun.Stdin(), FrameResize, 0, EncodeResize(cols, rows))
	})
	defer stopResize()
	if cols, rows, err := term.GetSize(stdoutFD); err == nil {
		_ = WriteFrame(tun.Stdin(), FrameResize, 0, EncodeResize(cols, rows))
	}
	if chosen != 0 {
		if cols, rows, err := term.GetSize(stdoutFD); err == nil && cols > 2 && rows > 2 {
			time.Sleep(50 * time.Millisecond)
			writeMu.Lock()
			_ = WriteFrame(tun.Stdin(), FrameResize, 0, EncodeResize(cols, rows-1))
			writeMu.Unlock()
			time.Sleep(50 * time.Millisecond)
			writeMu.Lock()
			_ = WriteFrame(tun.Stdin(), FrameResize, 0, EncodeResize(cols, rows))
			writeMu.Unlock()
		}
	}

	select {
	case <-done:
	case <-endedCh:
	case <-ctx.Done():
	}

	if !detached.Load() {
		_ = WriteFrame(tun.Stdin(), FrameBye, 0, nil)
	}
	tun.Close()
	_, _ = os.Stdout.WriteString(terminalResetSeq)
	_ = term.Restore(stdinFD, oldState)
	if detached.Load() {
		fmt.Fprintln(os.Stderr, "\nDetached — the session keeps running. Reattach with: kit remote --host "+name)
	} else {
		fmt.Fprintln(os.Stderr, "\nRemote session ended.")
	}
	return nil
}

// chooseSession lists the host's live sessions and, when any exist, asks
// which one to attach to. Returns the logical session id to attach to, or
// 0 for a new session. Sends nothing but the list request; the caller
// performs the attach after its pumps are running.
func chooseSession(tun *Tunnel, ctrlCh chan Frame, endedCh chan struct{}) (uint64, error) {
	if err := WriteFrame(tun.Stdin(), FrameSessionList, 0, nil); err != nil {
		return 0, err
	}
	var reply *Frame
	select {
	case f := <-ctrlCh:
		if f.Type == FrameSessionListReply {
			reply = &f
		}
	case <-time.After(10 * time.Second):
		return 0, fmt.Errorf("daemon did not answer the session list")
	case <-endedCh:
		return 0, fmt.Errorf("remote session ended")
	}
	if reply == nil {
		return 0, nil
	}
	var sessions []sessionInfo
	if err := json.Unmarshal(reply.Payload, &sessions); err != nil {
		return 0, fmt.Errorf("daemon: bad session list: %w", err)
	}
	if len(sessions) == 0 {
		return 0, nil // nothing live: straight to a new session
	}

	entries := make([]ui.SessionEntry, len(sessions))
	for i, ses := range sessions {
		started, _ := time.Parse(time.RFC3339, ses.Started)
		entries[i] = ui.SessionEntry{ID: ses.ID, Clients: ses.Clients, Started: started, Cwd: ses.Cwd}
	}
	choice, err := ui.RunSessionPicker(entries)
	if err != nil {
		return 0, err
	}
	if choice < 0 {
		return 0, nil // new session
	}
	fmt.Fprintf(os.Stderr, "DEBUG: choice=%d sessions=%+v\n", choice, sessions)
	return sessions[choice].ID, nil
}
