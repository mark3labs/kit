package daemon

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Frame types shared with contrib/kit-tunnel/src/main.rs (protocol v2).
// Data/resize/bye are relayed end-to-end by the tunnel, multiplexed by
// session id; ping/pong are reserved for a future keepalive; the 0x1x
// handshake/session frames never cross the tunnel boundary — the serve-side
// tunnel emits SESSION_OPEN/CLOSED on its stdout to introduce and retire
// sessions to the Go daemon.
type FrameType byte

const (
	FrameData   FrameType = 0x01
	FrameResize FrameType = 0x02
	FrameBye    FrameType = 0x03
	FramePing   FrameType = 0x04
	FramePong   FrameType = 0x05

	// Tunnel -> daemon session lifecycle (serve side only).
	FrameSessionOpen   FrameType = 0x16
	FrameSessionClosed FrameType = 0x17

	// Pairing-model control frames on the tunnel stdio (v3, protocol v3 in
	// the sidecar). They never cross the iroh connection; they are the
	// daemon<->sidecar consultation channel that keeps all policy in Go.
	//
	// Reconnect authentication (main endpoint):
	//   AUTH_REQUEST  sidecar->daemon {c_nonce, s_nonce, client_pub}  (8+8+32)
	//   AUTH_PAYLOAD  sidecar->daemon {signature}                     (64)
	//   AUTH_DECISION daemon->sidecar {0|1}
	// Pairing (bootstrap endpoint):
	//   PAIR_REQUEST  sidecar->daemon {c_nonce, client_pub}           (8+32)
	//   PAIR_DECISION daemon->sidecar {0|1, host_endpoint_id?}        (1 or 33)
	FrameAuthRequest  FrameType = 0x30
	FrameAuthPayload  FrameType = 0x31
	FrameAuthDecision FrameType = 0x32
	FramePairRequest  FrameType = 0x40
	FramePairDecision FrameType = 0x41
)

const frameHeaderSize = 7 // type byte + u32 session + u16 big-endian length

// maxPayload matches the tunnel's limit: u16 length field.
const maxPayload = 65535

// chunkSize is how much PTY/terminal data we pack into one DATA frame.
// Small enough to stay well under maxPayload, large enough to keep frame
// overhead negligible for full-screen redraws.
const chunkSize = 16 * 1024

// Frame is one relayed message. Session 0 is used on the client side (the
// dial tunnel rewrites ids in both directions) and in tests.
type Frame struct {
	Type    FrameType
	Session uint32
	Payload []byte
}

// ReadFrame reads one frame. Returns io.EOF when the reader ends cleanly on
// a frame boundary (peer closed).
func ReadFrame(r io.Reader) (Frame, error) {
	var hdr [frameHeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			return Frame{}, io.EOF
		}
		return Frame{}, err
	}
	length := binary.BigEndian.Uint16(hdr[5:7])
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		if err == io.ErrUnexpectedEOF {
			return Frame{}, io.EOF
		}
		return Frame{}, err
	}
	return Frame{
		Type:    FrameType(hdr[0]),
		Session: binary.BigEndian.Uint32(hdr[1:5]),
		Payload: payload,
	}, nil
}

// WriteFrame writes one frame.
func WriteFrame(w io.Writer, t FrameType, session uint32, payload []byte) error {
	if len(payload) > maxPayload {
		return fmt.Errorf("daemon: frame payload %d exceeds %d", len(payload), maxPayload)
	}
	buf := make([]byte, frameHeaderSize+len(payload))
	buf[0] = byte(t)
	binary.BigEndian.PutUint32(buf[1:5], session)
	binary.BigEndian.PutUint16(buf[5:7], uint16(len(payload)))
	copy(buf[frameHeaderSize:], payload)
	_, err := w.Write(buf)
	return err
}

// EncodeResize packs cols/rows into a RESIZE payload.
func EncodeResize(cols, rows int) []byte {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], uint16(cols))
	binary.BigEndian.PutUint16(payload[2:4], uint16(rows))
	return payload
}

// DecodeResize unpacks a RESIZE payload.
func DecodeResize(payload []byte) (cols, rows int, err error) {
	if len(payload) != 4 {
		return 0, 0, fmt.Errorf("daemon: bad resize payload %d bytes", len(payload))
	}
	return int(binary.BigEndian.Uint16(payload[0:2])),
		int(binary.BigEndian.Uint16(payload[2:4])), nil
}

// WriteDataFrames splits b into chunkSize DATA frames tagged with session.
func WriteDataFrames(w io.Writer, session uint32, b []byte) error {
	for len(b) > 0 {
		n := min(chunkSize, len(b))
		if err := WriteFrame(w, FrameData, session, b[:n]); err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}
