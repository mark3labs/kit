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

	tun, err := openPairWindow(pctx, seed)
	if err != nil {
		return err
	}
	defer tun.close()

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
			case err := <-tun.failed:
				if pctx.Err() != nil {
					fmt.Println("  Pairing window closed.")
					return nil
				}
				return fmt.Errorf("daemon: pairing window failed: %w", err)
			case f := <-tun.frames:
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
		allowed, superseded := opts.askOperator(pctx, fp, corr, tun.frames, answers)
		next = superseded
		if !allowed {
			tun.decide(corr, false, "", hostEndpointID)
			continue
		}
		// The allowlist write comes first so the entry exists before the
		// client can possibly dial — but it only sticks if the verdict
		// reaches the client. Every undelivered path below rolls a NEWLY
		// created entry back (an already-known client keeps its working
		// pairing) and leaves the window open for another attempt.
		_, createdEntry, err := AuthorizeClient(hex.EncodeToString(clientPub))
		if err != nil {
			log.Error("daemon: authorize failed", "error", err)
			tun.decide(corr, false, "host error", hostEndpointID)
			continue
		}
		if !tun.decide(corr, true, "", hostEndpointID) {
			// The attempt is gone: its decision window timed out or the
			// client disconnected while the operator was thinking.
			rollbackPairing(createdEntry, fp)
			fmt.Println("  The client gave up before the answer arrived. The code stays valid — ask it to try again.")
			continue
		}
		// One successful pairing burns the code; end the window. The
		// transport holds the stream open while the client drains the
		// confirmation and closes tun.paired only after that delivery, so
		// success is only reported once it fires.
		select {
		case <-tun.paired:
			fmt.Println("  Client paired. It can now connect with: kit remote --host <name>")
			fmt.Println()
			return nil
		case <-time.After(15 * time.Second):
			// The verdict was taken but the confirmation never went out
			// (the write stalled or the connection died mid-delivery).
			rollbackPairing(createdEntry, fp)
			fmt.Println("  The pairing confirmation could not be delivered. The code stays valid — ask the client to try again.")
			continue
		case <-pctx.Done():
			// The window expired mid-delivery. Keep the authorization: the
			// human approved this key, and if the client did receive the
			// confirmation, revoking it now would orphan a good pairing.
			// If it did not, it simply pairs again with a fresh code.
			fmt.Println("  Pairing window closed.")
			return nil
		}
	}
}

// rollbackPairing removes an allowlist entry that this window created but
// could not confirm to the client. Entries that existed before the attempt
// are kept: the client behind them already holds a working pairing, and
// AuthorizeClient only refreshed its LastSeen.
func rollbackPairing(created bool, fp string) {
	if !created {
		return
	}
	if _, err := RevokeClient(fp); err != nil {
		log.Warn("daemon: pairing rollback failed", "fp", fp, "error", err)
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
// responsive to the transport. A question is abandoned when the client behind
// it disconnects (FramePairCancel) or when a newer request arrives — otherwise
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
