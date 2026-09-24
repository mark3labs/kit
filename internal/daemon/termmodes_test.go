package daemon

import (
	"strings"
	"testing"
)

// feedString is Feed for a literal sequence.
func feedString(m *termModes, s string) { m.Feed([]byte(s)) }

// TestTermModesReplaysTheMouseTheChildEnabled is the regression test for
// the bug this tracker exists for: a client attaching to a running
// session never saw the child turn mouse reporting on, so the wheel
// scrolled nothing and text could not be selected.
func TestTermModesReplaysTheMouseTheChildEnabled(t *testing.T) {
	m := newTermModes()
	// What Bubble Tea writes for MouseModeCellMotion on its first frame.
	feedString(m, "\x1b[?25l\x1b[?2004h\x1b[?1002h\x1b[?1006h\x1b[?1004hframe")

	replay := string(m.Replay())
	for _, want := range []string{"\x1b[?1002h", "\x1b[?1006h", "\x1b[?2004h", "\x1b[?1004h", "\x1b[?25l"} {
		if !strings.Contains(replay, want) {
			t.Errorf("replay %q is missing %q", replay, want)
		}
	}
	if strings.Contains(replay, "frame") {
		t.Errorf("replay %q carries screen content; it must carry modes only", replay)
	}
}

// TestTermModesFollowsTheChildOff covers a child that turns a mode off
// again: the attaching client must be told the CURRENT state, not every
// state the session has been in.
func TestTermModesFollowsTheChildOff(t *testing.T) {
	m := newTermModes()
	feedString(m, "\x1b[?1002h\x1b[?1006h")
	feedString(m, "\x1b[?1002l\x1b[?1003l\x1b[?1006l") // a picker with the mouse off

	replay := string(m.Replay())
	if strings.Contains(replay, "\x1b[?1002h") || strings.Contains(replay, "\x1b[?1006h") {
		t.Errorf("replay %q re-enables a mouse the child turned off", replay)
	}
	if !strings.Contains(replay, "\x1b[?1002l") {
		t.Errorf("replay %q does not carry the reset the child sent", replay)
	}
}

// TestTermModesReadsSequencesSplitAcrossChunks covers the ordinary case
// on a PTY: the child writes a frame in one call and the reader gets it
// in pieces, which can land anywhere — including inside a sequence.
func TestTermModesReadsSequencesSplitAcrossChunks(t *testing.T) {
	m := newTermModes()
	for _, chunk := range []string{"\x1b", "[?10", "02", "h\x1b[?1", "006h"} {
		feedString(m, chunk)
	}
	replay := string(m.Replay())
	if !strings.Contains(replay, "\x1b[?1002h") || !strings.Contains(replay, "\x1b[?1006h") {
		t.Errorf("replay %q lost a sequence split across reads", replay)
	}
}

// TestTermModesReadsMultiParameterModes covers CSI ? 1002 ; 1006 h, which
// sets several modes in one sequence.
func TestTermModesReadsMultiParameterModes(t *testing.T) {
	m := newTermModes()
	feedString(m, "\x1b[?1002;1006;2004h")
	replay := string(m.Replay())
	for _, want := range []string{"\x1b[?1002h", "\x1b[?1006h", "\x1b[?2004h"} {
		if !strings.Contains(replay, want) {
			t.Errorf("replay %q is missing %q", replay, want)
		}
	}
}

// TestTermModesNeverReplaysTheAltScreenOrSyncOutput pins the two modes
// that must not be replayed. The alt screen belongs to the client for the
// whole attachment, and synchronized output is a bracket around one frame:
// replaying "on" would leave the terminal waiting for an end-of-frame it
// is never sent, which freezes the screen.
func TestTermModesNeverReplaysTheAltScreenOrSyncOutput(t *testing.T) {
	m := newTermModes()
	feedString(m, "\x1b[?1049h\x1b[?2026h\x1b[?1002h")

	replay := string(m.Replay())
	if strings.Contains(replay, "1049") {
		t.Errorf("replay %q fights the client for the alt screen", replay)
	}
	if strings.Contains(replay, "2026") {
		t.Errorf("replay %q replays synchronized output, which would freeze the screen", replay)
	}
	if !strings.Contains(replay, "\x1b[?1002h") {
		t.Errorf("replay %q dropped the mouse mode with them", replay)
	}
}

// TestTermModesTracksTheKeyboardStack covers the kitty keyboard protocol:
// pushes stack and pops unwind, and the replay reproduces what is left.
func TestTermModesTracksTheKeyboardStack(t *testing.T) {
	m := newTermModes()
	feedString(m, "\x1b[>1u\x1b[>5u")
	if got := string(m.Replay()); !strings.Contains(got, "\x1b[>1u\x1b[>5u") {
		t.Errorf("replay %q does not reproduce the pushed keyboard stack", got)
	}

	feedString(m, "\x1b[<u") // pop one
	if got := string(m.Replay()); strings.Contains(got, "\x1b[>5u") {
		t.Errorf("replay %q keeps a popped keyboard entry", got)
	}

	feedString(m, "\x1b[<9u") // pop more than are left
	if got := string(m.Replay()); strings.Contains(got, "\x1b[>") {
		t.Errorf("replay %q keeps a keyboard entry after the stack was emptied", got)
	}
}

// TestTermModesIsEmptyUntilTheChildSetsSomething keeps a brand new
// session from writing anything to the client that started it: that
// client watches the child set every mode itself.
func TestTermModesIsEmptyUntilTheChildSetsSomething(t *testing.T) {
	m := newTermModes()
	feedString(m, "hello \x1b[31mworld\x1b[0m\r\n")
	if got := m.Replay(); len(got) != 0 {
		t.Errorf("Replay() = %q, want nothing for a session that set no modes", got)
	}
}

// TestTermModesIgnoresModesInsideAnOSCString guards the one way a mode
// scanner can be fooled: a string payload that happens to read like a
// sequence. OSC and DCS payloads cannot contain an ESC, so the scanner
// must not find one in them.
func TestTermModesIgnoresModesInsideAnOSCString(t *testing.T) {
	m := newTermModes()
	feedString(m, "\x1b]52;c;WzsxMDAyaA==\x07\x1b[?1006h")
	replay := string(m.Replay())
	if !strings.Contains(replay, "\x1b[?1006h") {
		t.Errorf("replay %q lost the sequence after an OSC string", replay)
	}
	if strings.Contains(replay, "1002") {
		t.Errorf("replay %q read a mode out of an OSC payload", replay)
	}
}

// TestTermModesDropsAnOverlongSequence keeps binary output — kitty
// graphics payloads, a pasted file — from buffering without bound.
func TestTermModesDropsAnOverlongSequence(t *testing.T) {
	m := newTermModes()
	feedString(m, "\x1b["+strings.Repeat("0", maxCSISeq*4)+"h")
	feedString(m, "\x1b[?1002h")
	if got := string(m.Replay()); !strings.Contains(got, "\x1b[?1002h") {
		t.Errorf("replay %q lost the sequence after an overlong one", got)
	}
	m.mu.Lock()
	buffered := len(m.buf)
	m.mu.Unlock()
	if buffered > maxCSISeq {
		t.Errorf("buffered %d bytes of a malformed sequence, want at most %d", buffered, maxCSISeq)
	}
}

// TestTermModesReplayIsStable keeps the output deterministic: the modes
// live in a map, and an unordered replay would be untestable and would
// churn the wire for no reason.
func TestTermModesReplayIsStable(t *testing.T) {
	m := newTermModes()
	feedString(m, "\x1b[?2004h\x1b[?1006h\x1b[?25l\x1b[?1002h")
	first := string(m.Replay())
	for range 8 {
		if got := string(m.Replay()); got != first {
			t.Fatalf("Replay() = %q, want the stable %q", got, first)
		}
	}
	if want := "\x1b[?25l\x1b[?2004h" +
		"\x1b[?1005l\x1b[?1015l\x1b[?1016l\x1b[?1006h" +
		"\x1b[?9l\x1b[?1000l\x1b[?1001l\x1b[?1002l\x1b[?1003l\x1b[?1002h"; first != want {
		t.Errorf("Replay() = %q, want %q", first, want)
	}
}

// TestTermModesMouseSurvivesAnEarlierMouseOff is the regression test for
// sessions that lost the mouse wheel after a reattach. Bubble Tea turns
// the mouse off with ?1002l?1003l?1006l (a picker with no mouse) and on
// again with ?1002h?1006h. A tracker that kept each mode number on its own
// replayed "?1002h ... ?1003l", and a terminal treats a reset of ANY
// tracking mode as "mouse off" — so the client got no mouse at all.
func TestTermModesMouseSurvivesAnEarlierMouseOff(t *testing.T) {
	m := newTermModes()
	feedString(m, "\x1b[?1002l\x1b[?1003l\x1b[?1006l") // picker: mouse off
	feedString(m, "\x1b[?1002h\x1b[?1006h")            // main view: mouse on

	replay := string(m.Replay())
	set := strings.LastIndex(replay, "\x1b[?1002h")
	if set < 0 {
		t.Fatalf("replay %q does not turn mouse tracking on", replay)
	}
	for _, n := range []string{"9", "1000", "1001", "1002", "1003"} {
		if i := strings.LastIndex(replay, "\x1b[?"+n+"l"); i > set {
			t.Errorf("replay %q resets mode %s after it sets 1002, which turns the mouse off", replay, n)
		}
	}
	if strings.LastIndex(replay, "\x1b[?1006h") < strings.LastIndex(replay, "\x1b[?1006l") {
		t.Errorf("replay %q leaves SGR mouse encoding off", replay)
	}
}

// TestTermModesMouseTrackingIsOneSetting covers a child that moves from
// one tracking mode to another: only the last one set is replayed as on.
func TestTermModesMouseTrackingIsOneSetting(t *testing.T) {
	m := newTermModes()
	feedString(m, "\x1b[?1003h\x1b[?1002h")
	replay := string(m.Replay())
	if strings.Contains(replay, "\x1b[?1003h") {
		t.Errorf("replay %q turns on a tracking mode the child replaced", replay)
	}
	if !strings.HasSuffix(replay, "\x1b[?1002h") {
		t.Errorf("replay %q does not end with the active tracking mode", replay)
	}
}
