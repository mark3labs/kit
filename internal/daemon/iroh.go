package daemon

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/tmc/go-iroh/dns"
	"github.com/tmc/go-iroh/iroh"
	"github.com/tmc/go-iroh/key"
	"github.com/tmc/go-iroh/relay"
)

// In-process iroh transport. This file owns everything the Rust kit-tunnel
// sidecar used to own: endpoint binding, n0 relay and DNS discovery, and
// the QUIC transport tuning. The wire contract — ALPN, the 7-byte frame
// header, the handshake frames below, and the HKDF/HMAC domains in
// pairing.go — is byte-identical to the sidecar's (protocol v1), so kits
// built before and after the switch interoperate.

const (
	// remoteALPN names kit's remote-session protocol on the iroh endpoint.
	remoteALPN = "kit/remote/1"
	// remoteProtocolVersion is protocol v1: the pairing model.
	remoteProtocolVersion = 1

	// nonceLen and tagLen size the handshake nonces and HMAC tags.
	nonceLen = 32
	tagLen   = 32

	// handshakeTimeout bounds every step of a peer's handshake, so a
	// stalled peer cannot pin a session slot forever.
	handshakeTimeout = 30 * time.Second
	// maxRemoteSessions caps concurrent remote connections on the
	// daemon's endpoint; extra peers are denied politely.
	maxRemoteSessions = 8
	// rejectBudget caps concurrent polite rejections of over-cap peers.
	// Each rejection lives at most one handshake timeout; beyond the
	// budget over-cap peers are closed immediately without a denial.
	rejectBudget = 32
	// maxPairAttempts caps pairing handshakes served at once. The window
	// is a human-scale event; this only stops an unbounded burst.
	maxPairAttempts = 4
	// pairDecisionTimeout bounds how long one pairing attempt waits for
	// the human decision on the host.
	pairDecisionTimeout = 120 * time.Second

	// verdictDrainTime holds a finished stream open so the peer can drain
	// a DENIED or PAIR_SERVER_OK frame before the connection drops.
	// Dropping the stream unfinished would reset it and the peer would
	// only see a connection loss.
	verdictDrainTime = 2 * time.Second
)

// Handshake and session-control frame types. They never reach the session
// frame loop: both ends consume them during the handshake, exactly like
// the sidecar did (main.rs 0x10..0x21).
const (
	frameServerHello     FrameType = 0x10
	frameClientHello     FrameType = 0x11
	frameServerOK        FrameType = 0x12
	frameDenied          FrameType = 0x13
	frameClientAuth      FrameType = 0x14
	frameSessionAssign   FrameType = 0x18
	framePairClientHello FrameType = 0x20
	framePairServerOK    FrameType = 0x21
)

// Pairing-tag HMAC roles. The values are part of the wire contract.
const (
	pairRoleClient = "kit-pair-client"
	pairRoleServer = "kit-pair-server"
)

// remoteTransportConfig tunes QUIC with a keep-alive plus a hard idle
// timeout so a silently vanished peer (killed process, dropped network,
// sleeping laptop) is detected in seconds instead of hanging the other
// side forever. Values match the sidecar's.
func remoteTransportConfig() *iroh.QUICTransportConfig {
	return &iroh.QUICTransportConfig{
		KeepAlivePeriod: 5 * time.Second,
		MaxIdleTimeout:  20 * time.Second,
	}
}

// secretFromSeed builds the iroh endpoint secret from a stored 32-byte
// seed. The endpoint id IS the seed's ed25519 public key.
func secretFromSeed(seed []byte) (key.SecretKey, error) {
	if len(seed) != key.SeedSize {
		return key.SecretKey{}, fmt.Errorf("daemon: endpoint seed must be %d bytes, got %d", key.SeedSize, len(seed))
	}
	return key.NewSecretKey([key.SeedSize]byte(seed)), nil
}

// irohEndpoint bundles a bound endpoint with its discovery services and
// their teardown.
type irohEndpoint struct {
	ep   *iroh.Endpoint
	stop context.CancelFunc
	pub  *iroh.PkarrPublisher // nil on dial-only endpoints
}

// bindEndpoint binds an iroh endpoint with n0 production relays and DNS
// discovery — the Go equivalent of the sidecar's presets::N0.
//
// serving selects the listener role: the ALPN is registered for accept,
// and the endpoint's relay address is published to the n0 pkarr relay so
// peers can dial it by endpoint id alone. go-iroh does not publish
// address data on its own, so a watcher feeds every address change to the
// publisher.
func bindEndpoint(ctx context.Context, secret key.SecretKey, serving bool) (*irohEndpoint, error) {
	services := &iroh.AddressLookupServices{}
	services.AddResolver(iroh.N0DNSAddressLookup(nil))

	var pub *iroh.PkarrPublisher
	if serving {
		p, err := iroh.N0PkarrPublisher(secret, nil)
		if err != nil {
			return nil, fmt.Errorf("daemon: pkarr publisher: %w", err)
		}
		services.AddPublisher(p)
		pub = p
	}

	opts := []iroh.Option{
		iroh.WithSecretKey(secret),
		iroh.WithRelayMode(relay.ModeDefault()),
		iroh.WithAddressLookup(services),
		iroh.WithTransportConfig(remoteTransportConfig()),
	}
	if serving {
		opts = append(opts, iroh.WithALPNs(remoteALPN))
	}

	ep, err := iroh.Bind(ctx, opts...)
	if err != nil {
		if pub != nil {
			_ = pub.Close()
		}
		return nil, fmt.Errorf("daemon: bind endpoint: %w", err)
	}

	// The publisher outlives ctx on purpose: teardown happens in close(),
	// not when the caller's ctx ends, so a short-lived bind ctx cannot
	// kill the publish loop under a live endpoint.
	pctx, stopPublish := context.WithCancel(context.Background())
	if serving {
		go func() {
			w := ep.WatchAddr()
			for {
				addr, err := w.Updated(pctx)
				if err != nil {
					return
				}
				services.Publish(dns.NewEndpointData(addr.Addrs()...))
			}
		}()
	}

	return &irohEndpoint{ep: ep, stop: stopPublish, pub: pub}, nil
}

// close tears the endpoint down: stop publishing, close the publisher,
// then shut the endpoint down with a bounded grace period.
func (h *irohEndpoint) close() {
	h.stop()
	if h.pub != nil {
		_ = h.pub.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = h.ep.Shutdown(ctx)
}

// waitOnline blocks until the endpoint holds a home-relay connection, ctx
// ends, or the timeout elapses. A timeout of zero or less means no extra
// deadline beyond ctx.
func (h *irohEndpoint) waitOnline(ctx context.Context, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return h.ep.Online(ctx)
}

// randomNonce returns a fresh 32-byte handshake nonce.
func randomNonce() ([]byte, error) {
	b := make([]byte, nonceLen)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("daemon: nonce: %w", err)
	}
	return b, nil
}

// readFrameTimeout reads one frame from an iroh stream under a deadline,
// then clears the deadline for the caller.
func readFrameTimeout(s *iroh.Stream, d time.Duration) (Frame, error) {
	_ = s.SetReadDeadline(time.Now().Add(d))
	defer func() { _ = s.SetReadDeadline(time.Time{}) }()
	return ReadFrame(s)
}

// denyStream writes a DENIED verdict, finishes the stream, and holds it
// open while the peer drains the frame (the sidecar's deny_and_finish).
func denyStream(s *iroh.Stream, reason string) {
	_ = WriteFrame(s, frameDenied, 0, []byte(reason))
	_ = s.Close()
	time.Sleep(verdictDrainTime)
}

// handshakeBackoff throttles handshake failures shared across
// connections: each consecutive failure raises the delay the next peer
// waits before authenticating (500 ms steps, capped at 8 s). The count
// decays after two quiet minutes and resets on any success. The delay is
// applied in the per-connection goroutine, so a failing peer delays
// itself and never blocks the accept loop.
type handshakeBackoff struct {
	mu   sync.Mutex
	n    int
	last time.Time
}

func (b *handshakeBackoff) delay() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.n > 0 && time.Since(b.last) > 2*time.Minute {
		b.n = 0
	}
	d := time.Duration(b.n) * 500 * time.Millisecond
	return min(d, 8*time.Second)
}

func (b *handshakeBackoff) success() {
	b.mu.Lock()
	b.n = 0
	b.last = time.Time{}
	b.mu.Unlock()
}

func (b *handshakeBackoff) failure() {
	b.mu.Lock()
	b.n++
	b.last = time.Now()
	b.mu.Unlock()
}
