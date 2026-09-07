package daemon

import (
	"bufio"
	"context"
	"errors"
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
	// The long timeout covers the human decision on the host.
	hostID, err := dialPairWindow(ctx, seed, kp.Pub, 150*time.Second)
	if err != nil {
		return err
	}
	if len(hostID) != 64 {
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
// pairing code is involved.
//
// The verified frame stream is handed to RunClient, which owns everything
// above the transport: session choice, raw mode, the input pumps and the
// chord table. The local socket path (kit attach) uses the same code.
func RunHost(ctx context.Context, name string, opts AttachOptions) error {
	entry, err := GetHost(name)
	if err != nil {
		return err
	}
	conn, err := dialHost(ctx, name, entry)
	if err != nil {
		return err
	}
	defer conn.Close()

	if opts.Name == "" {
		opts.Name = name
	}
	// Sessions on this daemon are tagged with the saved host name by the
	// hub picker, so a choice carrying a different host is a cross-host
	// switch.
	opts.Host = name
	if opts.Reattach == "" {
		// The hint is completed with the session id, and only 'kit attach'
		// takes one: 'kit remote --host X 1' is not a valid command line.
		opts.Reattach = "kit attach --host " + name
	}
	return RunClient(ctx, conn, opts)
}

// dialHost brings up a verified connection to a paired host.
func dialHost(ctx context.Context, name string, entry HostEntry) (*remoteConn, error) {
	return dialHostQuiet(ctx, name, entry, false)
}

// dialHostQuiet is dialHost with control over the progress message. The
// hub picker queries every paired host while it owns the alt screen, so a
// per-host "Connecting…" line would be drawn straight into the picker.
func dialHostQuiet(ctx context.Context, name string, entry HostEntry, quiet bool) (*remoteConn, error) {
	clientSeed, err := LoadClientIdentity()
	if err != nil {
		return nil, err
	}

	if !quiet {
		fmt.Fprintln(os.Stderr, "Connecting to daemon…")
	}
	conn, err := dialHostIroh(ctx, entry.EndpointID, clientSeed)
	if err != nil {
		var denied *deniedError
		switch {
		case errors.As(err, &denied) && strings.Contains(denied.reason, "client not paired"):
			return nil, fmt.Errorf("the host no longer knows this machine — pair again with 'kit remote --pair <code>'")
		case errors.Is(err, errHostUnresolved):
			return nil, fmt.Errorf("could not resolve the daemon's endpoint (is 'kit daemon' running on the host?)")
		case errors.Is(err, errHostTimeout):
			return nil, fmt.Errorf("%s did not answer — check that 'kit daemon' is running there, or that the network allows the connection", name)
		}
		return nil, fmt.Errorf("daemon: %w", err)
	}
	_ = TouchHost(name)
	return conn, nil
}

// ListHostSessions queries one paired host's live sessions without
// attaching, for the multi-host picker. The whole exchange is bounded by
// timeout: an unreachable host must not stall a picker that has other
// hosts to show.
//
// ctx cancels the query early. The picker queries every paired host at
// once, so a caller that gives up must be able to take the dials down
// without waiting out the timeout.
func ListHostSessions(ctx context.Context, name string, timeout time.Duration) ([]SessionEntry, error) {
	entry, err := GetHost(name)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	conn, err := dialHostQuiet(ctx, name, entry, true)
	if err != nil {
		return nil, err
	}
	// Teardown is detached from the deadline-bound query path: Close says
	// BYE (a short pause) and shuts the endpoint down, and neither must
	// stretch a picker that is already done waiting.
	defer func() { go conn.Close() }()

	cc := newClientConn(conn)
	go cc.readLoop()
	// Bound the reply by what is left of the caller's timeout: the picker
	// queries hosts one at a time, so a host that stops replying must not
	// stretch the wait past the deadline the caller asked for.
	return cc.listSessionsWithin(time.Until(deadline))
}
