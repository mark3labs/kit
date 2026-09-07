package daemon

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tmc/go-iroh/iroh"
	"github.com/tmc/go-iroh/key"
	"github.com/tmc/go-iroh/netaddr"
)

// Client-side transport for `kit remote --host <name>`: dial the pinned
// endpoint id, authenticate with the client signing key, and hand the
// verified frame stream to RunClient.

// Sentinel errors dialHostQuiet maps to user-facing advice.
var (
	// errHostUnresolved: discovery found no address for the endpoint id —
	// the usual meaning is that no daemon is publishing it right now.
	errHostUnresolved = errors.New("daemon endpoint not resolvable")
	// errHostTimeout: the dial or the handshake ran out of time.
	errHostTimeout = errors.New("daemon did not answer")
)

// deniedError carries the daemon's handshake denial reason.
type deniedError struct{ reason string }

func (e *deniedError) Error() string { return "connection denied: " + e.reason }

// remoteConn is one verified client connection to a paired daemon. It is
// the io.ReadWriter RunClient speaks: protocol frames pass through
// verbatim, and the daemon stamps wire ids on arrival.
type remoteConn struct {
	h      *irohEndpoint
	conn   *iroh.Conn
	stream *iroh.Stream
	once   sync.Once
}

func (r *remoteConn) Read(p []byte) (int, error)  { return r.stream.Read(p) }
func (r *remoteConn) Write(p []byte) (int, error) { return r.stream.Write(p) }

// Close says goodbye and tears the connection down. The BYE frame lets
// the daemon detach this client at once instead of waiting out the QUIC
// idle timeout; the short pause lets it reach the wire before the
// connection close overtakes it.
func (r *remoteConn) Close() {
	r.once.Do(func() {
		_ = WriteFrame(r.stream, FrameBye, 0, nil)
		_ = r.stream.Close()
		time.Sleep(250 * time.Millisecond)
		_ = r.conn.Close()
		r.h.close()
	})
}

// dialHostIroh connects to a paired daemon by its stored endpoint id and
// runs the client side of the reconnect handshake. The transport identity
// is ephemeral; the application identity is the ed25519 signing seed, and
// iroh authenticates the daemon against the endpoint id pinned at pairing
// time.
func dialHostIroh(ctx context.Context, endpointIDHex string, clientSeed []byte) (*remoteConn, error) {
	idBytes, err := hex.DecodeString(strings.TrimSpace(endpointIDHex))
	if err != nil || len(idBytes) != ed25519PubLen {
		return nil, fmt.Errorf("daemon: bad stored endpoint id — pair again with 'kit remote --pair <code>'")
	}
	endpointID, err := key.EndpointIDFromSlice(idBytes)
	if err != nil {
		return nil, fmt.Errorf("daemon: bad stored endpoint id: %w", err)
	}
	if len(clientSeed) != ed25519.SeedSize {
		return nil, errors.New("daemon: corrupt client identity seed")
	}
	signing := ed25519.NewKeyFromSeed(clientSeed)

	ephemeral, err := key.GenerateSecretKey()
	if err != nil {
		return nil, fmt.Errorf("daemon: generate dial key: %w", err)
	}
	h, err := bindEndpoint(ctx, ephemeral, false)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			h.close()
		}
	}()
	if err := h.waitOnline(ctx, 30*time.Second); err != nil {
		return nil, fmt.Errorf("could not reach the iroh relay network — check your internet connection: %w", err)
	}

	dctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	conn, err := h.ep.Connect(dctx, netaddr.NewEndpointAddr(endpointID), remoteALPN)
	cancel()
	if err != nil {
		if errors.Is(err, iroh.ErrNoAddress) {
			return nil, errHostUnresolved
		}
		if ctx.Err() == nil {
			return nil, fmt.Errorf("%w: %v", errHostTimeout, err)
		}
		return nil, ctx.Err()
	}

	octx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	stream, err := conn.OpenStreamSync(octx)
	cancel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: no stream", errHostTimeout)
	}

	if err := clientHandshake(stream, signing); err != nil {
		_ = conn.Close()
		return nil, err
	}

	ok = true
	return &remoteConn{h: h, conn: conn, stream: stream}, nil
}

// clientHandshake authenticates this machine to the daemon by signing the
// handshake transcript with the pairing key (protocol v1; the exact frames
// are documented on remoteListener.serverHandshake).
func clientHandshake(stream *iroh.Stream, signing ed25519.PrivateKey) error {
	cNonce, err := randomNonce()
	if err != nil {
		return err
	}
	hello := make([]byte, 0, 2+nonceLen+ed25519PubLen)
	hello = binary.BigEndian.AppendUint16(hello, remoteProtocolVersion)
	hello = append(hello, cNonce...)
	hello = append(hello, signing.Public().(ed25519.PublicKey)...)
	if err := WriteFrame(stream, frameClientHello, 0, hello); err != nil {
		return fmt.Errorf("daemon: write hello: %w", err)
	}

	reply, err := readFrameTimeout(stream, handshakeTimeout+5*time.Second)
	if err != nil {
		return fmt.Errorf("%w: no handshake reply", errHostTimeout)
	}
	if reply.Type == frameDenied {
		return &deniedError{reason: string(reply.Payload)}
	}
	if reply.Type != frameServerHello || len(reply.Payload) < 2+nonceLen {
		return errors.New("daemon: malformed server hello")
	}
	if ver := binary.BigEndian.Uint16(reply.Payload[:2]); ver != remoteProtocolVersion {
		return fmt.Errorf("daemon: version mismatch: the host speaks protocol %d", ver)
	}
	sNonce := reply.Payload[2 : 2+nonceLen]

	msg := make([]byte, 0, len(signContext)+2*nonceLen)
	msg = append(msg, signContext...)
	msg = append(msg, cNonce...)
	msg = append(msg, sNonce...)
	if err := WriteFrame(stream, frameClientAuth, 0, ed25519.Sign(signing, msg)); err != nil {
		return fmt.Errorf("daemon: write auth: %w", err)
	}

	verdict, err := readFrameTimeout(stream, handshakeTimeout+5*time.Second)
	if err != nil {
		return fmt.Errorf("%w: no verdict", errHostTimeout)
	}
	switch verdict.Type {
	case frameServerOK:
	case frameDenied:
		return &deniedError{reason: string(verdict.Payload)}
	default:
		return fmt.Errorf("daemon: unexpected handshake frame %#x", byte(verdict.Type))
	}

	// The daemon assigns this connection's wire id right after the
	// verdict. It stamps arriving frames itself, so the value is consumed
	// and dropped — but it must be consumed, or it would surface as an
	// unknown frame in the client's read loop.
	assign, err := readFrameTimeout(stream, handshakeTimeout)
	if err != nil {
		return fmt.Errorf("%w: no session assignment", errHostTimeout)
	}
	if assign.Type != frameSessionAssign || len(assign.Payload) != 4 {
		return errors.New("daemon: malformed session assignment")
	}
	return nil
}
