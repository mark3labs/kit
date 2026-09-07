package daemon

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/tmc/go-iroh/iroh"
)

// remoteListener hosts the daemon's stable endpoint and serves remote
// sessions in-process. Each verified connection becomes one wire
// connection in the shared session table — the same shape as a local
// socket client, so everything above the transport is shared.
//
// Authentication policy stays where it always was, in Go: the handshake
// verifies the client's ed25519 signature against the pairing allowlist
// directly, with no consultation channel in between.
type remoteListener struct {
	h     *irohEndpoint
	table *sessionTable

	mu       sync.Mutex
	nextWire uint32
	active   int

	backoff *handshakeBackoff
	rejects chan struct{}
}

// listenRemote binds the daemon's stable endpoint from its identity seed.
// The endpoint id clients pinned at pairing time IS this seed's ed25519
// public key, and iroh's QUIC handshake proves we hold the secret.
func listenRemote(ctx context.Context, seed []byte, table *sessionTable) (*remoteListener, error) {
	secret, err := secretFromSeed(seed)
	if err != nil {
		return nil, err
	}
	h, err := bindEndpoint(ctx, secret, true)
	if err != nil {
		return nil, err
	}
	return &remoteListener{
		h:       h,
		table:   table,
		backoff: &handshakeBackoff{},
		rejects: make(chan struct{}, rejectBudget),
	}, nil
}

// endpointID reports the endpoint id as 64 hex chars — the form clients
// store in their host book.
func (l *remoteListener) endpointID() string {
	b := l.h.ep.ID().Bytes()
	return hex.EncodeToString(b[:])
}

func (l *remoteListener) close() { l.h.close() }

// run accepts remote connections until ctx ends or the endpoint closes.
// The session cap is enforced here, atomically, so concurrent peers
// cannot all pass the check.
func (l *remoteListener) run(ctx context.Context) {
	for {
		conn, err := l.h.ep.Accept(ctx)
		if err != nil {
			return
		}
		l.mu.Lock()
		over := l.active >= maxRemoteSessions
		if !over {
			l.active++
		}
		l.mu.Unlock()
		if over {
			select {
			case l.rejects <- struct{}{}:
				go l.rejectSessionFull(ctx, conn)
			default:
				// Beyond the polite-rejection budget, close the peer
				// immediately: unbounded rejection goroutines are their
				// own resource leak under a connection flood.
				_ = conn.Close()
			}
			continue
		}
		go l.handleConn(ctx, conn)
	}
}

// rejectSessionFull politely refuses a peer when the session cap is
// reached. Fully bounded: the wait uses the handshake timeout, and the
// denial is finished before the connection drops so the peer sees it.
func (l *remoteListener) rejectSessionFull(ctx context.Context, conn *iroh.Conn) {
	defer func() { <-l.rejects }()
	defer func() { _ = conn.Close() }()
	sctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	stream, err := conn.AcceptStream(sctx)
	if err != nil {
		return
	}
	denyStream(stream, "too many sessions")
	log.Warn("daemon: remote client denied", "reason", "too many sessions")
}

// handleConn drives one remote connection: handshake, then the shared
// frame loop until the peer goes away. Every exit path releases the
// session slot and unbinds the wire connection; the logical sessions it
// was attached to keep running detached.
func (l *remoteListener) handleConn(ctx context.Context, conn *iroh.Conn) {
	defer func() {
		l.mu.Lock()
		l.active--
		l.mu.Unlock()
	}()
	defer func() { _ = conn.Close() }()

	// Bound the stream open: a peer that connects but never opens its bi
	// stream would otherwise hold its reserved slot forever.
	sctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	stream, err := conn.AcceptStream(sctx)
	cancel()
	if err != nil {
		log.Debug("daemon: remote peer opened no stream", "error", err)
		return
	}

	// Backoff runs in this connection's own goroutine: a failing peer
	// delays itself, never the accept loop.
	if d := l.backoff.delay(); d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return
		}
	}

	wire, err := l.serverHandshake(stream)
	if err != nil {
		l.backoff.failure()
		log.Warn("daemon: remote handshake failed", "error", err)
		return
	}
	l.backoff.success()

	// Register the connection BEFORE serving frames, so replies (attach
	// acks, session lists) can find their way back out.
	sink := newFrameSink(stream)
	l.table.conns.addRemote(wire, sink)
	log.Info("remote client connected", "wire", wire)
	defer func() {
		sink.close()
		l.table.conns.remove(wire)
		l.table.detachWire(wire)
		log.Info("remote client disconnected", "wire", wire)
	}()

	_ = l.table.runFrameSource(ctx, stream, wire)
}

// serverHandshake runs the daemon side of the reconnect handshake
// (protocol v1, main-endpoint):
//
//	client -> CLIENT_HELLO {ver, c_nonce, client_pub}
//	server -> SERVER_HELLO {ver, s_nonce}
//	client -> CLIENT_AUTH  {ed25519_sig("kit-remote-v1-auth"|c_nonce|s_nonce)}
//	server -> SERVER_OK {} | DENIED {reason}
//	server -> SESSION_ASSIGN {wire id}
//
// On success it returns the wire id assigned to this connection.
func (l *remoteListener) serverHandshake(stream *iroh.Stream) (uint32, error) {
	hello, err := readFrameTimeout(stream, handshakeTimeout)
	if err != nil {
		return 0, fmt.Errorf("read hello: %w", err)
	}
	if hello.Type != frameClientHello || len(hello.Payload) != 2+nonceLen+ed25519PubLen {
		denyStream(stream, "bad hello frame")
		return 0, errors.New("malformed client hello")
	}
	if ver := binary.BigEndian.Uint16(hello.Payload[:2]); ver != remoteProtocolVersion {
		denyStream(stream, fmt.Sprintf("version %d", ver))
		return 0, fmt.Errorf("version mismatch: %d", ver)
	}
	cNonce := hello.Payload[2 : 2+nonceLen]
	clientPub := hello.Payload[2+nonceLen:]

	sNonce, err := randomNonce()
	if err != nil {
		return 0, err
	}
	reply := make([]byte, 2+nonceLen)
	binary.BigEndian.PutUint16(reply[:2], remoteProtocolVersion)
	copy(reply[2:], sNonce)
	if err := WriteFrame(stream, frameServerHello, 0, reply); err != nil {
		return 0, fmt.Errorf("write server hello: %w", err)
	}

	auth, err := readFrameTimeout(stream, handshakeTimeout)
	if err != nil {
		return 0, fmt.Errorf("read auth: %w", err)
	}
	if auth.Type != frameClientAuth || len(auth.Payload) != ed25519.SignatureSize {
		denyStream(stream, "bad auth frame")
		return 0, errors.New("malformed client auth")
	}
	if ok, reason := authorizeRemoteClient(clientPub, cNonce, sNonce, auth.Payload); !ok {
		denyStream(stream, reason)
		return 0, fmt.Errorf("unauthorized client: %s", reason)
	}

	// Tell the client which wire id this connection carries. Old clients
	// echo it on their frames; the daemon stamps arriving frames itself
	// either way, so both behaviours work.
	wire := l.allocWire()
	if err := WriteFrame(stream, frameServerOK, 0, nil); err != nil {
		return 0, fmt.Errorf("write verdict: %w", err)
	}
	assign := make([]byte, 4)
	binary.BigEndian.PutUint32(assign, wire)
	if err := WriteFrame(stream, frameSessionAssign, 0, assign); err != nil {
		return 0, fmt.Errorf("write session assignment: %w", err)
	}
	return wire, nil
}

// allocWire hands out remote wire ids from the space below localWireBase,
// so the two allocators can never collide (see transport.go). It wraps
// back to 1 rather than passing through 0.
func (l *remoteListener) allocWire() uint32 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nextWire++
	if l.nextWire == 0 || l.nextWire >= localWireBase {
		l.nextWire = 1
	}
	return l.nextWire
}

// authorizeRemoteClient checks a reconnect handshake signature against the
// pairing allowlist. This is the authentication policy that used to
// answer the sidecar's AUTH consultation; it now runs inline.
func authorizeRemoteClient(clientPub, cNonce, sNonce, sig []byte) (bool, string) {
	fp := Fingerprint(clientPub)
	entry, authorized, err := LookupClient(fp)
	if err != nil {
		return false, "allowlist error"
	}
	if !authorized {
		log.Warn("client not paired", "fp", fp)
		return false, "client not paired — run 'kit daemon pair' on the host"
	}
	pub, err := hex.DecodeString(entry.PubKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false, "corrupt allowlist entry"
	}
	msg := append([]byte(signContext), cNonce...)
	msg = append(msg, sNonce...)
	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		log.Warn("bad client signature", "fp", fp)
		return false, "bad signature"
	}
	_ = TouchClient(fp)
	log.Info("client authorized", "fp", fp)
	return true, ""
}
