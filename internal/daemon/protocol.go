package daemon

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Frame types shared with contrib/kit-tunnel/src/main.rs. Data/resize/bye
// are relayed end-to-end by the tunnel; ping/pong are reserved for a future
// keepalive; the 0x1x handshake frames never leave the tunnel.
type FrameType byte

const (
	FrameData   FrameType = 0x01
	FrameResize FrameType = 0x02
	FrameBye    FrameType = 0x03
	FramePing   FrameType = 0x04
	FramePong   FrameType = 0x05
)

const frameHeaderSize = 3 // type byte + u16 big-endian payload length

// maxPayload matches the tunnel's limit: u16 length field.
const maxPayload = 65535

// chunkSize is how much PTY/terminal data we pack into one DATA frame.
// Small enough to stay well under maxPayload, large enough to keep frame
// overhead negligible for full-screen redraws.
const chunkSize = 16 * 1024

// ReadFrame reads one frame. Returns io.EOF when the reader ends cleanly on
// a frame boundary (peer closed).
func ReadFrame(r io.Reader) (FrameType, []byte, error) {
	var hdr [frameHeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			return 0, nil, io.EOF
		}
		return 0, nil, err
	}
	length := binary.BigEndian.Uint16(hdr[1:3])
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		if err == io.ErrUnexpectedEOF {
			return 0, nil, io.EOF
		}
		return 0, nil, err
	}
	return FrameType(hdr[0]), payload, nil
}

// WriteFrame writes one frame and flushes via the underlying writer.
func WriteFrame(w io.Writer, t FrameType, payload []byte) error {
	if len(payload) > maxPayload {
		return fmt.Errorf("daemon: frame payload %d exceeds %d", len(payload), maxPayload)
	}
	buf := make([]byte, frameHeaderSize+len(payload))
	buf[0] = byte(t)
	binary.BigEndian.PutUint16(buf[1:3], uint16(len(payload)))
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

// WriteDataFrames splits b into chunkSize DATA frames and writes them.
func WriteDataFrames(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n := min(chunkSize, len(b))
		if err := WriteFrame(w, FrameData, b[:n]); err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}
