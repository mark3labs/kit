package daemon

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// Frame types (protocol v1, shared by every kit build — including the
// retired Rust kit-tunnel sidecar). Data/resize/bye travel end-to-end,
// multiplexed by session id; ping/pong are reserved for a future
// keepalive; the 0x1x handshake frames are consumed during the transport
// handshake and never reach the session frame loop (see iroh.go).
type FrameType byte

const (
	FrameData   FrameType = 0x01
	FrameResize FrameType = 0x02
	FrameBye    FrameType = 0x03
	FramePing   FrameType = 0x04
	FramePong   FrameType = 0x05

	// Client -> daemon clipboard image transfer (chunked; see
	// internal/daemon/clipboard.go for the payload layout). Travels
	// end-to-end like DATA/RESIZE; consumed by the daemon, never written
	// to the session PTY.
	FrameClipboard FrameType = 0x06

	// Session lifecycle (travels end-to-end like CLIPBOARD). Sessions are
	// LOGICAL: they outlive client connections, so a client can detach
	// (Ctrl+X d) and later reattach, and several clients can attach to
	// the same session (shared tmux-style view). Wire session ids are per
	// connection; the daemon maps them to logical sessions.
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
	// FrameSessionSpec describes the command line a NEW session should be
	// started as: a working directory, the arguments to pass, and the
	// environment variables to layer on the daemon's own (JSON, see
	// SessionSpec). Without it a new session always starts in the daemon
	// user's home directory behind the directory picker, which is right
	// for 'kit attach' but wrong for a plain 'kit' in a project directory.
	// Sent before SESSION_ATTACH, like TERMINAL; a daemon too old to know
	// the frame drops it and starts the session the old way.
	FrameSessionSpec FrameType = 0x0f // client -> daemon: JSON SessionSpec

	// Historical frame types (retired with the Rust kit-tunnel sidecar,
	// which owned the transport in a subprocess). SESSION_OPEN/CLOSED
	// introduced and retired wire connections on the sidecar's stdio; the
	// 0x30/0x4x frames were the daemon<->sidecar consultation channel that
	// kept authentication and pairing policy in Go. The transport now runs
	// in-process (iroh.go), which makes them unnecessary — the values stay
	// reserved so protocol v1 keeps one authoritative number space.
	FrameSessionOpen   FrameType = 0x16
	FrameSessionClosed FrameType = 0x17
	FrameAuthRequest   FrameType = 0x30
	FrameAuthPayload   FrameType = 0x31
	FrameAuthDecision  FrameType = 0x32
	// FramePairRequest and FramePairCancel are still used in-process: the
	// pairing window surfaces attempts to the operator loop as these
	// frames (see iroh_pair.go and pair.go).
	FramePairRequest  FrameType = 0x40
	FramePairDecision FrameType = 0x41
	FramePairCancel   FrameType = 0x42
)

const frameHeaderSize = 7 // type byte + u32 session + u16 big-endian length

// maxPayload is the frame size limit: a u16 length field.
const maxPayload = 65535

// chunkSize is how much PTY/terminal data we pack into one DATA frame.
// Small enough to stay well under maxPayload, large enough to keep frame
// overhead negligible for full-screen redraws.
const chunkSize = 16 * 1024

// Frame is one relayed message. Session 0 is used by clients (the daemon
// stamps the assigned wire id on arrival) and in tests.
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
	// Multiplexer names the terminal multiplexer the client runs inside
	// ("tmux", "screen", "zellij"), or is empty for a bare terminal. A
	// multiplexer's pane variables describe a process's own pane and so
	// never cross the wire; without this the child probes a terminal that
	// answers from behind one and then draws graphics the multiplexer
	// discards. See termgfx.RemoteMultiplexerEnv.
	Multiplexer string `json:"multiplexer,omitempty"`
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

// SessionSpec describes how to start a NEW session, so a session hosted
// by the daemon begins where the user ran the command rather than in the
// daemon's home directory.
//
// The daemon inherits nothing from the client: it is a separate process,
// usually started by a service manager, with its own working directory
// and its own environment. A session spawned from it therefore has to be
// told all three parts explicitly.
//
// A spec is honoured only on the LOCAL socket. A working directory and an
// argument list from another machine name nothing that exists on this
// one, and accepting argv over the network would turn a paired client
// into arbitrary execution. The local socket is already restricted to the
// daemon's own user, so a spec there grants no more than running kit
// directly would.
type SessionSpec struct {
	// Cwd is the directory the session starts in. An empty or missing
	// directory falls back to the daemon user's home.
	Cwd string `json:"cwd,omitempty"`
	// Args are the arguments to pass to the session's kit process,
	// without the program name.
	Args []string `json:"args,omitempty"`
	// Env holds environment variables to layer on the daemon's own. Keys
	// the daemon owns are ignored; see reservedSpecEnv.
	Env map[string]string `json:"env,omitempty"`
	// Pick asks for the working-directory picker, rooted at Cwd, instead
	// of starting straight away.
	//
	// This is what `kit attach` asks for: the user said which MACHINE they
	// wanted a session on, not which directory, so they are still offered
	// the choice — but offered it where they are standing rather than in
	// their home directory. A routed `kit` leaves it false, because there
	// the directory is the whole point.
	Pick bool `json:"pick,omitempty"`
}

// EncodeSessionSpec renders a SESSION_SPEC payload.
func EncodeSessionSpec(spec SessionSpec) ([]byte, error) {
	return json.Marshal(spec)
}

// DecodeSessionSpec parses a SESSION_SPEC payload.
func DecodeSessionSpec(payload []byte) (SessionSpec, error) {
	var spec SessionSpec
	if err := json.Unmarshal(payload, &spec); err != nil {
		return SessionSpec{}, fmt.Errorf("daemon: bad session spec payload: %w", err)
	}
	return spec, nil
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
