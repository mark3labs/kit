package daemon

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
)

// PairWindowOptions controls `kit daemon pair`. Zero values are valid.
type PairWindowOptions struct {
	// Code forces a specific pairing code instead of a random one.
	// Intended for tests.
	Code string
	// Prompt overrides the interactive accept/reject decision (tests).
	// When nil, the decision is made on the terminal; a non-TTY stdin
	// always denies. Implementations should return false when ctx ends.
	Prompt func(ctx context.Context, fp string) bool
	// Window bounds the pairing window. Zero means the default (10 min).
	Window time.Duration
}

// RunPairWindow opens a one-time pairing window: it derives an ephemeral
// bootstrap endpoint from a fresh code, shows the code, and — when a
// client presents it — asks the user to accept or reject on this
// terminal. On accept the client's public key joins the allowlist and the
// client learns this daemon's endpoint id; the code is then burned.
//
// The window is independent of the main daemon process: pairing writes
// the allowlist to disk, and `kit daemon` (running or not) picks it up on
// the next connection attempt.
func RunPairWindow(ctx context.Context, opts PairWindowOptions) error {
	if _, err := FindTunnelBinary(); err != nil {
		return err
	}

	window := opts.Window
	if window <= 0 {
		window = pairWindowTime
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

	daemonSeed, err := LoadDaemonIdentity()
	if err != nil {
		return err
	}
	// The endpoint id the client will store is the daemon identity's
	// ed25519 public key: iroh endpoint ids ARE ed25519 public keys, and
	// the QUIC handshake proves the peer holds the matching secret.
	priv := ed25519.NewKeyFromSeed(daemonSeed)
	hostEndpointID := hex.EncodeToString(priv.Public().(ed25519.PublicKey))

	pctx, cancel := context.WithTimeout(ctx, window)
	defer cancel()

	fmt.Println()
	fmt.Println("  Pair a client with this host")
	fmt.Println()
	fmt.Printf("  Pairing code: %s\n", FormatCode(code))
	fmt.Printf("  On the client run: kit remote --pair %s\n", code)
	fmt.Printf("  This window closes in %s or after one successful pairing.\n", window)
	fmt.Println()

	tun, err := StartTunnel(pctx, TunnelOptions{
		Mode: "serve-pair",
		Args: []string{"--timeout", "30"},
		Env:  []string{"KIT_TUNNEL_PAIR_SEED=" + fmt.Sprintf("%x", seed)},
	})
	if err != nil {
		return err
	}
	defer tun.Close()

	// Frames are read by a dedicated goroutine so the window keeps listening
	// while the operator is being asked a question. Reading inline would
	// block the sidecar's whole consultation channel behind one prompt.
	frames := make(chan Frame, 16)
	readErr := make(chan error, 1)
	go func() {
		for {
			frame, err := ReadFrame(tun.Stdout())
			if err != nil {
				readErr <- err
				close(frames)
				return
			}
			frames <- frame
		}
	}()

	// One long-lived reader owns the terminal. A per-prompt reader would
	// leak a goroutine holding os.Stdin for every abandoned question, and
	// those would then race for the operator's next keystroke.
	answers := opts.answerLines(pctx)

	var next *Frame // a request that superseded the one being asked about
	for {
		var frame Frame
		if next != nil {
			frame, next = *next, nil
		} else {
			select {
			case <-pctx.Done():
				fmt.Println("  Pairing window closed.")
				return nil
			case err := <-readErr:
				// Tunnel ended: window expired (Go ctx killed it) or crash.
				if pctx.Err() != nil {
					fmt.Println("  Pairing window closed.")
					return nil
				}
				return fmt.Errorf("daemon: pairing tunnel ended (%s): %w", tun.LastStatuses(), err)
			case f, ok := <-frames:
				if !ok {
					continue // readErr carries the reason
				}
				frame = f
			}
		}

		// Payload: c_nonce(32) | client_pub(32). The correlation key
		// echoed in the decision is the first 8 bytes of c_nonce.
		if frame.Type != FramePairRequest || len(frame.Payload) != 32+32 {
			continue
		}
		clientPub := frame.Payload[32:]
		corr := frame.Payload[0:8]
		fp := Fingerprint(clientPub)

		fmt.Printf("  Pairing request from client %s\n", fingerprintShort(fp))
		allowed, superseded := opts.askOperator(pctx, fp, corr, frames, answers)
		next = superseded
		if !allowed {
			writePairDecision(tun, corr, false, "", hostEndpointID)
			continue
		}
		if _, err := AuthorizeClient(hex.EncodeToString(clientPub)); err != nil {
			log.Error("daemon: authorize failed", "error", err)
			writePairDecision(tun, corr, false, "host error", hostEndpointID)
			continue
		}
		writePairDecision(tun, corr, true, "", hostEndpointID)
		fmt.Println("  Client paired. It can now connect with: kit remote --host <name>")
		fmt.Println()
		// One successful pairing burns the code; end the window. A
		// short grace lets the client drain the confirmation frame
		// before the tunnel teardown closes the connection.
		_, _ = tun.WaitAnyStatus(pctx, 10*time.Second, "PAIRED", "PAIR_DENIED", "CLOSED")
		time.Sleep(2 * time.Second)
		log.Debug("daemon: pair window statuses", "statuses", tun.LastStatuses())
		return nil
	}
}

// answerLines returns a channel of trimmed terminal lines, or nil when the
// decision is not made on this terminal (test override, or no TTY).
func (opts PairWindowOptions) answerLines(ctx context.Context) <-chan string {
	if opts.Prompt != nil || !term.IsTerminal(os.Stdin.Fd()) {
		return nil
	}
	ch := make(chan string, 4)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			text, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			select {
			case ch <- strings.TrimSpace(text):
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// askOperator asks the accept/reject question on the terminal while staying
// responsive to the sidecar. A question is abandoned when the client behind
// it disconnects (PAIR_CANCEL) or when a newer request arrives — otherwise
// one walked-away client would hold the window for its whole decision
// timeout and nobody else could pair. The second return value is a request
// that superseded this one and must be handled next.
//
// Non-interactive contexts always deny: pairing is an inherently human
// decision, and an unattended daemon must not approve anything. A denial is
// also the answer when the window expires while the operator is thinking.
func (opts PairWindowOptions) askOperator(
	ctx context.Context,
	fp string,
	corr []byte,
	frames <-chan Frame,
	answers <-chan string,
) (bool, *Frame) {
	if opts.Prompt != nil {
		return opts.Prompt(ctx, fp), nil
	}
	if answers == nil {
		log.Warn("daemon: pairing request denied — no terminal to confirm on; run 'kit daemon pair' interactively", "fp", fp)
		return false, nil
	}
	fmt.Printf("  Accept? [y/N]: ")
	for {
		select {
		case answer := <-answers:
			allowed := promptDecision(ctx, answer)
			if !allowed {
				fmt.Println("  Rejected. The code stays valid — the window is still open for another attempt.")
			}
			return allowed, nil
		case frame, ok := <-frames:
			if !ok {
				return false, nil
			}
			switch frame.Type {
			case FramePairCancel:
				if len(frame.Payload) >= 8 && bytes.Equal(frame.Payload[:8], corr) {
					fmt.Println()
					fmt.Println("  The client disconnected; question withdrawn. The code stays valid.")
					return false, nil
				}
			case FramePairRequest:
				fmt.Println()
				fmt.Println("  Superseded by a newer pairing request.")
				return false, &frame
			}
		case <-ctx.Done():
			fmt.Println("\n  (window expired) rejected.")
			return false, nil
		}
	}
}

// promptDecision resolves a typed answer against the window context. The
// context check runs after the answer is received: Go's select may pick a
// queued "yes" even when the deadline has already fired, and an answer
// that lands at (or after) expiry is a rejection.
func promptDecision(ctx context.Context, answer string) bool {
	if ctx.Err() != nil {
		return false
	}
	switch strings.ToLower(answer) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// writeDecision answers the sidecar's pairing consultation. Payload:
// correlation key (8) | verdict (1) | host endpoint id (32, accept only)
// | optional reason (deny only).
func writePairDecision(tun *Tunnel, corr []byte, allow bool, reason, hostEndpointID string) {
	out := []byte{}
	out = append(out, corr...)
	if allow {
		out = append(out, 1)
		id, err := hex.DecodeString(hostEndpointID)
		if err != nil || len(id) != ed25519PubLen {
			out = out[:8]
			out = append(out, 0)
			out = append(out, "host identity error"...)
		} else {
			out = append(out, id...)
		}
	} else {
		out = append(out, 0)
		out = append(out, reason...)
	}
	_ = WriteFrame(tun.Stdin(), FramePairDecision, 0, out)
}
