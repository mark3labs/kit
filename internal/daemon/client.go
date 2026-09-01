package daemon

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"strings"
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
const terminalResetSeq = "\x1b[?25h" +
	"\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l" +
	"\x1b[?2004l" +
	"\x1b[<u\x1b[<u\x1b[<u"

// altScreenEnter and altScreenLeave bracket a client's whole attachment.
//
// The client owns the alternate screen for the same reason tmux and ssh
// do: a session renders inline, so without it the session's output stays
// in the normal buffer after a detach and the returning shell prompt
// draws straight over it. Holding the alt screen across session switches
// also stops the terminal flashing between two sessions.
const (
	altScreenEnter = "\x1b[?1049h"
	altScreenLeave = "\x1b[?1049l"
)

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
		return pairFailure(err, tun.LastStatuses())
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

// pairFailure turns a sidecar failure into advice. The sidecar reports the
// transport truth ("connect to daemon: timed out"), which says nothing to
// the person at the keyboard: the usual cause is a pairing window that is
// no longer serving the code, so point at the fix instead of the symptom.
func pairFailure(err error, statuses string) error {
	switch {
	case strings.Contains(statuses, "no daemon is live for this pairing code"),
		strings.Contains(statuses, "No addressing information available"):
		return fmt.Errorf("no host is listening for that pairing code — check the code, or run 'kit daemon pair' on the host for a fresh one")
	case strings.Contains(statuses, "connect to daemon"),
		strings.Contains(statuses, "timed out"):
		return fmt.Errorf("could not reach the host's pairing window — it may have closed (run 'kit daemon pair' again on the host), or the network is blocking the connection")
	case strings.Contains(statuses, "endpoint bind"):
		return fmt.Errorf("could not open a local network endpoint: %s", statuses)
	}
	return fmt.Errorf("pairing failed: %w (last: %s)", err, statuses)
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
// pairing code is involved.
//
// The sidecar stream is handed to RunClient, which owns everything above
// the transport: session choice, raw mode, the input pumps and the chord
// table. The local socket path (kit attach) uses the same code.
func RunHost(ctx context.Context, name string, opts AttachOptions) error {
	entry, err := GetHost(name)
	if err != nil {
		return err
	}
	tun, err := dialHost(ctx, name, entry)
	if err != nil {
		return err
	}
	defer tun.Close()

	if opts.Name == "" {
		opts.Name = name
	}
	// Sessions on this daemon are tagged with the saved host name by the
	// hub picker, so a choice carrying a different host is a cross-host
	// switch.
	opts.Host = name
	if opts.Reattach == "" {
		opts.Reattach = "kit remote --host " + name
	}
	return RunClient(ctx, tunnelStream{tun}, opts)
}

// tunnelStream adapts a sidecar Tunnel to the io.ReadWriter the client
// speaks: frames out on the sidecar's stdin, frames in on its stdout.
type tunnelStream struct{ t *Tunnel }

func (s tunnelStream) Read(p []byte) (int, error)  { return s.t.Stdout().Read(p) }
func (s tunnelStream) Write(p []byte) (int, error) { return s.t.Stdin().Write(p) }

// dialHost brings up a verified sidecar connection to a paired host.
func dialHost(ctx context.Context, name string, entry HostEntry) (*Tunnel, error) {
	return dialHostQuiet(ctx, name, entry, false)
}

// dialHostQuiet is dialHost with control over the progress message. The
// hub picker queries every paired host while it owns the alt screen, so a
// per-host "Connecting…" line would be drawn straight into the picker.
func dialHostQuiet(ctx context.Context, name string, entry HostEntry, quiet bool) (*Tunnel, error) {
	clientSeed, err := LoadClientIdentity()
	if err != nil {
		return nil, err
	}

	if !quiet {
		fmt.Fprintln(os.Stderr, "Connecting to daemon…")
	}
	tun, err := StartTunnel(ctx, TunnelOptions{
		Mode: "dial-host",
		Args: []string{
			"--endpoint-id", entry.EndpointID,
			"--timeout", "35",
		},
		Env: []string{"KIT_TUNNEL_CLIENT_SEED=" + fmt.Sprintf("%x", clientSeed)},
	})
	if err != nil {
		return nil, err
	}

	if _, err := tun.WaitStatus(ctx, "VERIFIED", 40*time.Second); err != nil {
		last := tun.LastStatuses()
		tun.Close()
		switch {
		case strings.Contains(last, "client not paired"):
			return nil, fmt.Errorf("the host no longer knows this machine — pair again with 'kit remote --pair <code>'")
		case strings.Contains(last, "No addressing information available"):
			return nil, fmt.Errorf("could not resolve the daemon's endpoint (is 'kit daemon' running on the host?)")
		case strings.Contains(last, "timed out"):
			return nil, fmt.Errorf("could not reach the daemon (network or relay issue)")
		}
		return nil, fmt.Errorf("daemon: %w", err)
	}
	_ = TouchHost(name)
	return tun, nil
}

// ListHostSessions queries one paired host's live sessions without
// attaching, for the multi-host picker. The whole exchange is bounded by
// timeout: an unreachable host must not stall a picker that has other
// hosts to show.
func ListHostSessions(name string, timeout time.Duration) ([]SessionEntry, error) {
	entry, err := GetHost(name)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	tun, err := dialHostQuiet(ctx, name, entry, true)
	if err != nil {
		return nil, err
	}
	defer tun.Close()

	conn := newClientConn(tunnelStream{tun})
	go conn.readLoop()
	// Bound the reply by what is left of the caller's timeout: the picker
	// queries hosts one at a time, so a host that stops replying must not
	// stretch the wait past the deadline the caller asked for.
	return conn.listSessionsWithin(time.Until(deadline))
}
