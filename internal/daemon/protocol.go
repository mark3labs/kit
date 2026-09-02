package daemon

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// Frame types shared with contrib/kit-tunnel/src/main.rs (protocol v1).
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

	// Client -> daemon clipboard image transfer (chunked; see
	// internal/daemon/clipboard.go for the payload layout). Relayed by the
	// sidecar verbatim like DATA/RESIZE; consumed by the daemon, never
	// written to the session PTY.
	FrameClipboard FrameType = 0x06

	// Session lifecycle (additive, relayed verbatim by the sidecar like
	// CLIPBOARD). Sessions are LOGICAL: they outlive client connections,
	// so a client can detach (Ctrl+X d) and later reattach, and several
	// clients can attach to the same session (shared tmux-style view).
	// Wire session ids are per sidecar connection; the daemon maps them to
	// logical sessions.
	FrameSessionDetach    FrameType = 0x07 // client -> daemon: unbind me, keep the session
	FrameSessionList      FrameType = 0x08 // client -> daemon: list live sessions (payload empty)
	FrameSessionListReply FrameType = 0x09 // daemon -> client: JSON [{id,clients,started,cwd,name}]
	FrameSessionAttach    FrameType = 0x0a // client -> daemon: {logical id u64 BE}
	FrameSessionAttachAck FrameType = 0x0b // daemon -> client: {logical id u64 BE, ok 0|1}
	// FrameSessionRedraw asks the daemon to make the session's child
	// repaint. A client that has just attached inherits a screen the child
	// already drew, so without this the terminal stays blank until the
	// next keystroke. The daemon nudges the PTY size, which is what makes
	// a full-screen TUI redraw; doing it daemon-side keeps the two size
	// changes off the network, where the round trip made the old
	// client-side version of this trick unreliable.
	FrameSessionRedraw FrameType = 0x0c // client -> daemon: repaint (payload empty)
	// FrameSessionRename sets a session's display name so a list of many
	// sessions stays readable.
	FrameSessionRename FrameType = 0x0d // client -> daemon: {id u64 BE, name UTF-8}
	// FrameTerminal describes the CLIENT's terminal to the daemon (JSON,
	// see TerminalInfo). A daemon owns a PTY, not a terminal, and a PTY
	// reports no colour depth and answers no background-colour query, so
	// without this a session's child describes the daemon's own
	// environment — under a service manager, no terminal at all. Sent
	// before SESSION_ATTACH so a new session's child is spawned already
	// describing the terminal it will be seen in.
	FrameTerminal FrameType = 0x0e // client -> daemon: JSON TerminalInfo

	// Tunnel -> daemon session lifecycle (serve side only).
	FrameSessionOpen   FrameType = 0x16
	FrameSessionClosed FrameType = 0x17

	// Pairing-model control frames on the tunnel stdio (protocol v1 in
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
	//   PAIR_CANCEL   sidecar->daemon {corr}                          (8)
	FrameAuthRequest  FrameType = 0x30
	FrameAuthPayload  FrameType = 0x31
	FrameAuthDecision FrameType = 0x32
	FramePairRequest  FrameType = 0x40
	FramePairDecision FrameType = 0x41
	// FramePairCancel withdraws a pending pairing question: the client
	// behind it disconnected, so the prompt on the host's terminal is
	// stale and must not block the window.
	FramePairCancel FrameType = 0x42
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

// TerminalInfo describes the client terminal a session is rendered in.
// The daemon cannot observe any of it: it owns the PTY, and a PTY answers
// no capability queries of its own.
//
// Term and ColorTerm are forwarded into the child's environment the way
// ssh forwards TERM, so colour-depth detection sees the user's terminal.
// Background is the terminal's own background colour, which decides
// whether a theme renders its light or its dark palette; the client
// resolves it locally because the OSC query that answers it must otherwise
// cross a PTY and a network before the first frame is drawn.
type TerminalInfo struct {
	Term      string `json:"term,omitempty"`
	ColorTerm string `json:"colorterm,omitempty"`
	// Background is the terminal background as "#rrggbb", or
	// BackgroundUnknown when the terminal was asked and did not answer.
	// The two are distinct on purpose: "asked, no answer" tells the child
	// not to spend its own startup asking again, where an empty value (a
	// client too old to probe at all) leaves it free to try.
	Background string `json:"background,omitempty"`
}

// BackgroundUnknown marks a terminal that was asked for its background
// colour and did not answer.
const BackgroundUnknown = "unknown"

// EncodeTerminalInfo renders a TERMINAL payload.
func EncodeTerminalInfo(info TerminalInfo) ([]byte, error) {
	return json.Marshal(info)
}

// DecodeTerminalInfo parses a TERMINAL payload.
func DecodeTerminalInfo(payload []byte) (TerminalInfo, error) {
	var info TerminalInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		return TerminalInfo{}, fmt.Errorf("daemon: bad terminal payload: %w", err)
	}
	return info, nil
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
