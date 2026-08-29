package daemon

import (
	"bufio"
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
	// always denies.
	Prompt func(fp string) bool
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

	for {
		select {
		case <-pctx.Done():
			fmt.Println("  Pairing window closed.")
			return nil
		default:
		}
		frame, err := ReadFrame(tun.Stdout())
		if err != nil {
			// Tunnel ended: window expired (Go ctx killed it) or crash.
			if pctx.Err() != nil {
				fmt.Println("  Pairing window closed.")
				return nil
			}
			return fmt.Errorf("daemon: pairing tunnel ended: %w", err)
		}
		switch frame.Type {
		case FramePairRequest:
			// Payload: c_nonce(32) | client_pub(32). The correlation key
			// echoed in the decision is the first 8 bytes of c_nonce.
			if len(frame.Payload) != 32+32 {
				continue
			}
			clientPub := frame.Payload[32:]
			corr := frame.Payload[0:8]
			fp := Fingerprint(clientPub)

			fmt.Printf("  Pairing request from client %s\n", fingerprintShort(fp))
			allowed := opts.prompt(fp)
			if !allowed {
				fmt.Println("  Rejected.")
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
			fmt.Fprintf(os.Stderr, "pair window statuses: %s\n", tun.LastStatuses())
			return nil
		}
	}
}

// prompt asks on the terminal. Non-interactive contexts always deny:
// pairing is an inherently human decision, and an unattended daemon must
// not approve anything.
func (opts PairWindowOptions) prompt(fp string) bool {
	if opts.Prompt != nil {
		return opts.Prompt(fp)
	}
	if !term.IsTerminal(os.Stdin.Fd()) {
		log.Warn("daemon: pairing request denied — no terminal to confirm on; run 'kit daemon pair' interactively", "fp", fp)
		return false
	}
	fmt.Printf("  Accept? [y/N]: ")
	line := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		line <- strings.TrimSpace(text)
	}()
	answer := <-line
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
