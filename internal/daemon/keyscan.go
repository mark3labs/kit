package daemon

import (
	"strconv"
	"strings"
	"time"
)

// Kitty keyboard protocol scanning for the remote client.
//
// The host TUI (Bubble Tea v2) enables the kitty keyboard protocol on the
// user's terminal through the PTY (`CSI = 3 ; 1 u`: disambiguate + report
// event types). Once active, a Ctrl-V keystroke no longer arrives as the
// legacy single byte 0x16 but as CSI sequences:
//
//	press   ESC [ 118 ; 5 u
//	release ESC [ 118 ; 5 : 3 u
//	repeat  ESC [ 118 ; 5 : 2 u
//
// keyScanner is an incremental parser over the client's stdin stream that
// reports Ctrl-V presses in both encodings so the client can attach local
// clipboard images. Everything else — including split sequences — is
// forwarded byte-identical, with two safety rules:
//
//   - A lone Escape (a complete key press on its own) is flushed as
//     passthrough when the next read arrives; it would otherwise be stuck
//     in the partial-sequence buffer forever, eating the user's Esc key.
//   - A CSI sequence that exceeds maxCSILen without a final byte is
//     malformed input (e.g. pasted binary): it is flushed as passthrough
//     instead of buffering without bound.
//
// The scanner never decides suppression: paste and release events carry
// their original wire bytes, and the client marks a release for suppression
// only after an image interception actually succeeded.

const (
	// keyV is the kitty key code for 'v'.
	keyV = 118
	// kittyModCtrl is the ctrl bit in the kitty modifier encoding (the
	// modifier value is 1 + shift|alt|ctrl).
	kittyModCtrl = 4
	// maxCSILen bounds a partial CSI sequence: real sequences are far
	// shorter; anything longer is treated as malformed passthrough.
	maxCSILen = 64
	// escIdleFlush is how long a pending lone Escape waits for its
	// sequence continuation before it is flushed as an Esc key press. A
	// CSI sequence split across reads arrives within microseconds; a
	// standalone Esc key press is followed by human-scale silence.
	escIdleFlush = 50 * time.Millisecond
)

// kittyEventKind classifies a decoded CSI-u event for the 'v' key.
type kittyEventKind int

const (
	kittyPress kittyEventKind = iota
	kittyRepeat
	kittyRelease
)

type keyEvent struct {
	// Paste is true when the event is a Ctrl-V press (or repeat) in any
	// supported encoding. Data holds the original wire bytes.
	Paste bool
	// Release is true for a Ctrl-V release event. Data holds the original
	// wire bytes.
	Release bool
	// Data is the original wire bytes for passthrough.
	Data []byte
}

// keyScanner is an incremental CSI-u/legacy key scanner.
type keyScanner struct {
	buf   []byte    // pending partial escape sequence
	inCSI bool      // saw ESC [ — accumulating until a final byte
	escAt time.Time // when the pending lone ESC was read (zero = none)
}

// Feed consumes one stdin chunk and returns the decoded events in wire
// order. A Ctrl-V press/repeat event is reported for both the kitty and
// legacy encodings, carrying the original bytes; a lone Escape pending
// from a previous chunk is flushed as passthrough when new input arrives.
func (k *keyScanner) Feed(chunk []byte) []keyEvent {
	var events []keyEvent
	var other []byte
	var emitOther = func() {
		if len(other) > 0 {
			events = append(events, keyEvent{Data: other})
			other = nil
		}
	}

	// A lone Escape pending from a previous chunk flushes as an Esc key
	// press once it has been idle past escIdleFlush. A CSI sequence split
	// right after ESC arrives within microseconds and keeps composing —
	// flushing on chunk arrival alone would break that valid split.
	if len(k.buf) == 1 && k.buf[0] == 0x1b && !k.inCSI && !k.escAt.IsZero() &&
		time.Since(k.escAt) >= escIdleFlush {
		events = append(events, keyEvent{Data: append([]byte(nil), k.buf...)})
		k.buf = k.buf[:0]
		k.escAt = time.Time{}
	}

	i := 0
	for i < len(chunk) {
		b := chunk[i]
		switch {
		case len(k.buf) > 0 && !k.inCSI:
			// After ESC: '[' introduces a CSI sequence; anything else is
			// a two-byte legacy escape — pass both through.
			if b == '[' {
				k.buf = append(k.buf, b)
				k.inCSI = true
				k.escAt = time.Time{}
			} else {
				other = append(other, k.buf...)
				other = append(other, b)
				k.buf = k.buf[:0]
				k.escAt = time.Time{}
			}
		case len(k.buf) > 0 && k.inCSI:
			// Inside a CSI sequence: params/intermediates are 0x20-0x3f,
			// the final byte is 0x40-0x7e.
			if len(k.buf) > maxCSILen {
				// Malformed oversized sequence — flush as passthrough and
				// treat this byte in ground state.
				other = append(other, k.buf...)
				k.buf = k.buf[:0]
				k.inCSI = false
				continue
			}
			k.buf = append(k.buf, b)
			if b >= 0x40 && b <= 0x7e {
				// Final byte: decode the sequence, then consume it.
				seq := append([]byte(nil), k.buf...)
				k.buf = k.buf[:0]
				k.inCSI = false
				i++
				paste, release, handled := k.decodeCSI(seq)
				if handled {
					emitOther()
					if paste {
						events = append(events, keyEvent{Paste: true, Data: seq})
						continue
					}
					if release {
						events = append(events, keyEvent{Release: true, Data: seq})
						continue
					}
				}
				other = append(other, seq...)
				continue
			}
		case b == 0x1b:
			// Start of a potential escape sequence.
			k.buf = append(k.buf[:0], b)
			k.escAt = time.Now()
		case b == pasteKey:
			// Legacy encoding of Ctrl-V.
			emitOther()
			events = append(events, keyEvent{Paste: true, Data: []byte{pasteKey}})
		default:
			other = append(other, b)
		}
		i++
	}
	emitOther()
	return events
}

// decodeCSI inspects a complete CSI sequence. It reports ctrl+v press,
// repeat and release events; everything else is passthrough.
func (k *keyScanner) decodeCSI(seq []byte) (paste, release, handled bool) {
	// Shape: ESC [ params final; params are 0x30-0x3f, final 0x40-0x7e.
	if len(seq) < 3 || seq[0] != 0x1b || seq[1] != '[' {
		return false, false, false
	}
	final := seq[len(seq)-1]
	if final != 'u' {
		return false, false, false
	}
	params := strings.Split(string(seq[2:len(seq)-1]), ";")
	if len(params) == 0 {
		return false, false, false
	}
	// First parameter: key code (with optional :alternate).
	key, _, ok := strings.Cut(params[0], ":")
	if !ok {
		key = params[0]
	}
	if key != strconv.Itoa(keyV) {
		return false, false, false
	}
	mod := 1
	event := kittyPress
	if len(params) > 1 {
		modPart, evPart, hasEv := strings.Cut(params[1], ":")
		if m, err := strconv.Atoi(modPart); err == nil {
			mod = m
		}
		if hasEv {
			if e, err := strconv.Atoi(evPart); err == nil {
				switch e {
				case 2:
					event = kittyRepeat
				case 3:
					event = kittyRelease
				}
			}
		}
	}
	if (mod-1)&kittyModCtrl == 0 {
		return false, false, false // no ctrl held — plain 'v'
	}
	switch event {
	case kittyPress, kittyRepeat:
		return true, false, true
	case kittyRelease:
		return false, true, true
	}
	return false, false, false
}
