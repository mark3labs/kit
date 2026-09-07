package daemon

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/tmc/go-iroh/iroh"
	"github.com/tmc/go-iroh/key"
	"github.com/tmc/go-iroh/netaddr"
)

// The pairing window transport: a bootstrap endpoint derived from the
// one-time code (see SeedFromCode). Knowledge of the code is what makes
// the endpoint findable, and the HMAC tag in the hello proves it before
// the human is ever bothered. The window serves attempts concurrently — a
// rejection, an abandoned client or a failed handshake leaves it open —
// and closes on the first successful pairing, which burns the code.

// pairWindow is the host side of a pairing window. Pair requests surface
// on frames as FramePairRequest/FramePairCancel (the same shapes the
// operator loop always consumed); RunPairWindow answers them with decide.
type pairWindow struct {
	h    *irohEndpoint
	seed []byte

	// frames carries PAIR_REQUEST {c_nonce|client_pub} and PAIR_CANCEL
	// {corr} to the operator loop.
	frames chan Frame
	// failed reports a fatal transport error, once.
	failed chan error
	// paired is closed after a successful pairing has been delivered to
	// the client (confirmation drained).
	paired chan struct{}

	mu      sync.Mutex
	pending map[[8]byte]chan pairVerdict

	pairedOnce sync.Once
}

// pairVerdict is the operator's answer to one pairing attempt.
type pairVerdict struct {
	allow  bool
	reason string
	hostID []byte // the daemon's stable endpoint id (32 bytes), accept only
}

// openPairWindow binds the bootstrap endpoint and starts serving pairing
// attempts until ctx ends or the first success.
func openPairWindow(ctx context.Context, seed []byte) (*pairWindow, error) {
	secret, err := secretFromSeed(seed)
	if err != nil {
		return nil, err
	}
	h, err := bindEndpoint(ctx, secret, true)
	if err != nil {
		return nil, err
	}
	w := &pairWindow{
		h:       h,
		seed:    seed,
		frames:  make(chan Frame, 16),
		failed:  make(chan error, 1),
		paired:  make(chan struct{}),
		pending: make(map[[8]byte]chan pairVerdict),
	}
	go w.run(ctx)
	return w, nil
}

func (w *pairWindow) close() { w.h.close() }

func (w *pairWindow) fail(err error) {
	select {
	case w.failed <- err:
	default:
	}
}

func (w *pairWindow) markPaired() {
	w.pairedOnce.Do(func() { close(w.paired) })
}

func (w *pairWindow) isPaired() bool {
	select {
	case <-w.paired:
		return true
	default:
		return false
	}
}

// decide answers a pending pairing attempt. corr is the 8-byte correlation
// key (first bytes of the client nonce) echoed from the request frame. An
// accept carries the daemon's stable endpoint id for the client to store.
//
// The return value reports whether a LIVE attempt took the verdict. False
// means the attempt is gone — its decision window timed out, or the client
// disconnected — and the verdict went nowhere; the caller must not treat
// the pairing as delivered.
func (w *pairWindow) decide(corr []byte, allow bool, reason, hostEndpointID string) bool {
	if len(corr) < 8 {
		return false
	}
	v := pairVerdict{allow: allow, reason: reason}
	if allow {
		id, err := hex.DecodeString(hostEndpointID)
		if err != nil || len(id) != ed25519PubLen {
			v = pairVerdict{allow: false, reason: "host identity error"}
		} else {
			v.hostID = id
		}
	}
	w.mu.Lock()
	ch := w.pending[[8]byte(corr[:8])]
	w.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- v:
		return true
	default:
		return false
	}
}

// run is the window's accept loop. The endpoint must reach a relay before
// the code-derived id is resolvable, so that wait is part of opening the
// window and its failure is fatal.
func (w *pairWindow) run(ctx context.Context) {
	if err := w.h.waitOnline(ctx, 30*time.Second); err != nil {
		if ctx.Err() == nil {
			w.fail(fmt.Errorf("could not reach the iroh relay network: %w", err))
		}
		return
	}

	backoff := &handshakeBackoff{}
	slots := make(chan struct{}, maxPairAttempts)
	for {
		conn, err := w.h.ep.Accept(ctx)
		if err != nil {
			// Window closed: ctx expired, RunPairWindow tore it down
			// after a success, or the endpoint died underneath us.
			if ctx.Err() == nil && !w.isPaired() {
				w.fail(fmt.Errorf("pairing endpoint closed: %w", err))
			}
			return
		}
		select {
		case slots <- struct{}{}:
		default:
			log.Warn("daemon: pairing attempt denied", "reason", "too many attempts")
			_ = conn.Close()
			continue
		}
		go func(conn *iroh.Conn) {
			defer func() { <-slots }()
			if w.handleAttempt(ctx, conn, backoff) {
				w.markPaired()
			}
		}(conn)
	}
}

// handleAttempt serves one pairing connection. It returns true only for a
// completed pairing; everything else leaves the window open for the next
// attempt, under the shared guess backoff.
func (w *pairWindow) handleAttempt(ctx context.Context, conn *iroh.Conn, backoff *handshakeBackoff) bool {
	defer func() { _ = conn.Close() }()

	if d := backoff.delay(); d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return false
		}
	}

	sctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	stream, err := conn.AcceptStream(sctx)
	cancel()
	if err != nil {
		log.Debug("daemon: pairing peer opened no stream", "error", err)
		return false
	}

	hello, err := readFrameTimeout(stream, handshakeTimeout)
	if err != nil {
		backoff.failure()
		log.Debug("daemon: pairing hello not read", "error", err)
		return false
	}
	// ver(u16) | c_nonce(32) | client_pub(32) | tag(32)
	if hello.Type != framePairClientHello || len(hello.Payload) != 2+nonceLen+ed25519PubLen+tagLen {
		backoff.failure()
		log.Warn("daemon: pairing attempt denied", "reason", "malformed pair hello")
		return false
	}
	if ver := binary.BigEndian.Uint16(hello.Payload[:2]); ver != remoteProtocolVersion {
		backoff.failure()
		denyStream(stream, fmt.Sprintf("version %d", ver))
		return false
	}
	cNonce := hello.Payload[2 : 2+nonceLen]
	clientPub := hello.Payload[2+nonceLen : 2+nonceLen+ed25519PubLen]
	tag := hello.Payload[2+nonceLen+ed25519PubLen:]
	expect, err := pairingTag(w.seed, pairRoleClient, cNonce, nil)
	if err != nil || !constantTimeEqual(tag, expect) {
		backoff.failure()
		denyStream(stream, "bad pairing tag")
		log.Warn("daemon: pairing attempt denied", "reason", "bad pairing tag")
		return false
	}

	// The peer knows the code. Ask the human, and stay cancellable: a
	// client that disappears while the operator is thinking must not
	// leave a stale question blocking the window.
	corr := [8]byte(cNonce[:8])
	verdictCh := make(chan pairVerdict, 1)
	w.mu.Lock()
	w.pending[corr] = verdictCh
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.pending, corr)
		w.mu.Unlock()
	}()

	req := make([]byte, 0, nonceLen+ed25519PubLen)
	req = append(req, cNonce...)
	req = append(req, clientPub...)
	select {
	case w.frames <- Frame{Type: FramePairRequest, Payload: req}:
	case <-ctx.Done():
		return false
	}

	var v pairVerdict
	select {
	case v = <-verdictCh:
	case <-time.After(pairDecisionTimeout):
		backoff.failure()
		denyStream(stream, "pairing window closed")
		return false
	case <-conn.Context().Done():
		// Withdraw the stale question from the operator's terminal.
		select {
		case w.frames <- Frame{Type: FramePairCancel, Payload: bytes.Clone(corr[:])}:
		default:
		}
		backoff.failure()
		log.Info("daemon: pairing attempt abandoned", "reason", "client disconnected")
		return false
	case <-ctx.Done():
		return false
	}

	if !v.allow {
		backoff.failure()
		reason := v.reason
		if reason == "" {
			reason = "rejected on the host"
		}
		denyStream(stream, reason)
		return false
	}

	backoff.success()
	sNonce, err := randomNonce()
	if err != nil {
		return false
	}
	tag2, err := pairingTag(w.seed, pairRoleServer, cNonce, sNonce)
	if err != nil {
		return false
	}
	ok := make([]byte, 0, nonceLen+tagLen+ed25519PubLen)
	ok = append(ok, sNonce...)
	ok = append(ok, tag2...)
	ok = append(ok, v.hostID...)
	if WriteFrame(stream, framePairServerOK, 0, ok) != nil {
		return false
	}
	// Deliver the confirmation as finished data and hold while the peer
	// drains it; a dropped stream would reset and lose the frame.
	_ = stream.Close()
	time.Sleep(verdictDrainTime)
	return true
}

// dialPairWindow is the client side of pairing: it proves knowledge of the
// one-time code to the host's bootstrap endpoint and returns the daemon's
// stable endpoint id (64 hex chars) on a human-approved pairing.
//
// decisionTimeout covers the human answering on the host; the network
// steps are bounded tighter so a dead code fails in seconds, not minutes.
func dialPairWindow(ctx context.Context, seed []byte, clientPub ed25519.PublicKey, decisionTimeout time.Duration) (string, error) {
	bootstrapSecret, err := secretFromSeed(seed)
	if err != nil {
		return "", err
	}
	serverID := bootstrapSecret.Public().EndpointID()

	// The transport identity is ephemeral; what the host stores is the
	// signing public key carried in the hello.
	ephemeral, err := key.GenerateSecretKey()
	if err != nil {
		return "", fmt.Errorf("daemon: generate dial key: %w", err)
	}
	h, err := bindEndpoint(ctx, ephemeral, false)
	if err != nil {
		return "", err
	}
	defer h.close()
	if err := h.waitOnline(ctx, 30*time.Second); err != nil {
		return "", fmt.Errorf("could not reach the iroh relay network — check your internet connection: %w", err)
	}

	dctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	conn, err := h.ep.Connect(dctx, netaddr.NewEndpointAddr(serverID), remoteALPN)
	cancel()
	if err != nil {
		if errors.Is(err, iroh.ErrNoAddress) {
			return "", errors.New("no host is listening for that pairing code — check the code, or run 'kit daemon pair' on the host for a fresh one")
		}
		return "", errors.New("could not reach the host's pairing window — it may have closed (run 'kit daemon pair' again on the host), or the network is blocking the connection")
	}
	defer func() { _ = conn.Close() }()

	octx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	stream, err := conn.OpenStreamSync(octx)
	cancel()
	if err != nil {
		return "", fmt.Errorf("daemon: open pairing stream: %w", err)
	}

	cNonce, err := randomNonce()
	if err != nil {
		return "", err
	}
	tag, err := pairingTag(seed, pairRoleClient, cNonce, nil)
	if err != nil {
		return "", err
	}
	hello := make([]byte, 0, 2+nonceLen+ed25519PubLen+tagLen)
	hello = binary.BigEndian.AppendUint16(hello, remoteProtocolVersion)
	hello = append(hello, cNonce...)
	hello = append(hello, clientPub...)
	hello = append(hello, tag...)
	if err := WriteFrame(stream, framePairClientHello, 0, hello); err != nil {
		return "", fmt.Errorf("daemon: write pair hello: %w", err)
	}

	reply, err := readFrameTimeout(stream, decisionTimeout)
	if err != nil {
		return "", errors.New("pairing timed out (was the request accepted on the host?)")
	}
	switch {
	case reply.Type == framePairServerOK && len(reply.Payload) == nonceLen+tagLen+ed25519PubLen:
		sNonce := reply.Payload[:nonceLen]
		expect, err := pairingTag(seed, pairRoleServer, cNonce, sNonce)
		if err != nil {
			return "", err
		}
		if !constantTimeEqual(reply.Payload[nonceLen:nonceLen+tagLen], expect) {
			return "", errors.New("daemon: the host failed tag verification")
		}
		// The stable endpoint id the client stores for codeless
		// reconnection. iroh's QUIC handshake authenticates the daemon
		// against it, so dialing it later cannot be hijacked.
		return hex.EncodeToString(reply.Payload[nonceLen+tagLen:]), nil
	case reply.Type == frameDenied:
		reason := string(reply.Payload)
		switch {
		case strings.Contains(reason, "rejected on the host"):
			return "", errors.New("the pairing request was rejected on the host")
		case strings.Contains(reason, "bad pairing tag"):
			return "", errors.New("the host rejected the pairing code")
		default:
			return "", fmt.Errorf("pairing denied: %s", reason)
		}
	default:
		return "", fmt.Errorf("daemon: unexpected pairing frame %#x", byte(reply.Type))
	}
}
