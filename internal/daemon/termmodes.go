package daemon

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Terminal mode tracking for a session's child.
//
// A client that attaches to a session that is ALREADY running inherits a
// child that has long since set up the terminal it thinks it is drawing
// on. Mouse reporting, bracketed paste, focus reporting, cursor
// visibility and the kitty keyboard protocol are all terminal STATE: the
// child turns them on once, with an escape sequence, and never sends it
// again — Bubble Tea writes a mode sequence only when the mode differs
// from the frame before (cursed_renderer.go compares against lastView),
// and a repaint is not a mode change.
//
// The first client of a session sees those sequences because it is
// watching when the child starts. Every later one — a reattach, a session
// switch, a second client on a shared session — attaches to a stream that
// has already moved past them, so its terminal keeps the modes it had
// before: mouse reporting off. The symptom is a session where the wheel
// scrolls nothing, text cannot be selected, and pasting arrives a
// character at a time, while the same session works in the terminal that
// started it.
//
// termModes watches the child's output for the mode sequences it emits
// and remembers the state they leave the terminal in, so an attaching
// client can be handed exactly that state. It is not a terminal emulator:
// it tracks modes, which are small, orderless and idempotent, and nothing
// else. The screen itself is recovered the way it always was, by making
// the child repaint (see nudgeRedraw).
type termModes struct {
	mu sync.Mutex

	// buf holds a CSI sequence being accumulated, without its ESC [
	// introducer. Sequences split across PTY reads are common: the child
	// writes a frame in one go but the PTY hands it over in chunks.
	buf   []byte
	inCSI bool
	// sawESC records an ESC awaiting its introducer byte.
	sawESC bool

	// modes maps a DEC private mode number to its current state.
	modes map[int]bool
	// kitty is the kitty keyboard protocol stack, innermost last. The
	// child pushes flags with CSI > flags u and pops with CSI < n u.
	kitty []int
	// kittySet is the last CSI = flags ; mode u, which sets the current
	// entry rather than pushing a new one.
	kittySet string
	// modifyOther is the last CSI > ... m (modifyOtherKeys), which is a
	// mode in all but name and is set exactly once for the same reason.
	modifyOther string
}

// maxCSISeq bounds an accumulating sequence. Real mode sequences are a
// handful of bytes; anything longer is a sequence we do not care about
// (or binary that merely looked like one) and is dropped rather than
// buffered without bound.
const maxCSISeq = 128

// kittyStackLimit bounds the remembered keyboard stack. Terminals keep a
// stack of their own with a similar limit; a child that pushes without
// popping past this point is misbehaving, and a replay of the innermost
// entries is the best answer available.
const kittyStackLimit = 16

func newTermModes() *termModes {
	return &termModes{modes: make(map[int]bool)}
}

// Feed consumes one chunk of child output. It is safe to call from the
// PTY reader and while another goroutine builds a replay.
func (m *termModes) Feed(p []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range p {
		switch {
		case m.inCSI:
			if len(m.buf) >= maxCSISeq {
				m.buf = m.buf[:0]
				m.inCSI = false
				continue
			}
			m.buf = append(m.buf, b)
			// 0x40-0x7e is the final byte; everything before it is
			// parameters and intermediates.
			if b >= 0x40 && b <= 0x7e {
				m.applyLocked(m.buf)
				m.buf = m.buf[:0]
				m.inCSI = false
			}
		case m.sawESC:
			m.sawESC = false
			if b == '[' {
				m.inCSI = true
				m.buf = m.buf[:0]
			}
			// Anything else introduces a sequence we do not track. OSC
			// and DCS strings are skipped rather than parsed: their
			// payloads cannot contain an ESC, so no string body can be
			// mistaken for a CSI sequence here.
		case b == 0x1b:
			m.sawESC = true
		}
	}
}

// applyLocked records one complete CSI sequence, given without its ESC [
// introducer.
func (m *termModes) applyLocked(seq []byte) {
	if len(seq) < 2 {
		return
	}
	final := seq[len(seq)-1]
	body := string(seq[:len(seq)-1])
	switch body[0] {
	case '?':
		// DEC private modes: CSI ? Ps [; Ps ...] h|l.
		if final != 'h' && final != 'l' {
			return
		}
		set := final == 'h'
		for p := range strings.SplitSeq(body[1:], ";") {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && replayableMode(n) {
				m.modes[n] = set
			}
		}
	case '>':
		switch final {
		case 'u': // kitty keyboard push
			flags := 0
			if v, err := strconv.Atoi(strings.TrimSpace(body[1:])); err == nil {
				flags = v
			}
			if len(m.kitty) < kittyStackLimit {
				m.kitty = append(m.kitty, flags)
			}
		case 'm': // modifyOtherKeys
			m.modifyOther = "\x1b[" + body + "m"
		}
	case '<':
		if final != 'u' { // kitty keyboard pop
			return
		}
		n := 1
		if v, err := strconv.Atoi(strings.TrimSpace(body[1:])); err == nil && v > 0 {
			n = v
		}
		if n > len(m.kitty) {
			n = len(m.kitty)
		}
		m.kitty = m.kitty[:len(m.kitty)-n]
	case '=':
		if final == 'u' { // kitty keyboard set
			m.kittySet = "\x1b[" + body + "u"
		}
	}
}

// replayableMode reports whether a DEC private mode may be replayed to a
// client that is taking over the screen.
//
// Two modes must never be:
//
//   - 1049, the alternate screen. The client owns it for the whole
//     attachment (see altScreenEnter) and enters it before any session
//     draws, so replaying the child's view of it can only fight the
//     client for the screen it is already holding.
//   - 2026, synchronized output. It is not state but a bracket around a
//     single frame, and the tracker sees "on" for as long as a frame is
//     in flight. Replaying that leaves the client's terminal waiting for
//     an end-of-frame that was delivered to somebody else, which freezes
//     the screen until the next frame ends.
func replayableMode(n int) bool {
	switch n {
	case 1049, 2026:
		return false
	}
	return true
}

// Replay returns the sequences that bring a freshly attached client's
// terminal into the state the child believes it is drawing on. It is
// empty when nothing has been recorded yet, which is the normal case for
// the client that starts a session: that one watches the child set every
// mode itself.
//
// Every sequence emitted is idempotent, so a client that is already in
// the right state loses nothing by being sent it again.
func (m *termModes) Replay() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()

	nums := make([]int, 0, len(m.modes))
	for n := range m.modes {
		nums = append(nums, n)
	}
	sort.Ints(nums) // a stable order keeps the output testable

	var b strings.Builder
	for _, n := range nums {
		b.WriteString("\x1b[?")
		b.WriteString(strconv.Itoa(n))
		if m.modes[n] {
			b.WriteString("h")
		} else {
			// A mode the child turned OFF is replayed too: some default
			// to on — 25, the cursor, most visibly — so leaving them out
			// would show a cursor the session hides.
			b.WriteString("l")
		}
	}
	if m.modifyOther != "" {
		b.WriteString(m.modifyOther)
	}
	for _, flags := range m.kitty {
		b.WriteString("\x1b[>")
		b.WriteString(strconv.Itoa(flags))
		b.WriteString("u")
	}
	if m.kittySet != "" {
		b.WriteString(m.kittySet)
	}
	if b.Len() == 0 {
		return nil
	}
	return []byte(b.String())
}
