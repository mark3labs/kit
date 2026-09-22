package daemon

import "sync"

// scrollback remembers a session's most recent output.
//
// A client that attaches to a running session inherits a screen somebody
// else's terminal already drew. The old answer was to make the child
// repaint by nudging the PTY size, which works — but only because the
// child is still there to answer, and only after a round trip.
//
// After a daemon restart neither holds. The adopting daemon has no idea
// what is on the screen, and the child is very likely sitting idle
// waiting for input, so a nudge produces a repaint of an unchanged frame
// at best and a blank terminal at worst. Keeping the recent bytes in the
// supervisor — the one process that was there the whole time — lets a
// reattaching client be handed the screen immediately, and the nudge that
// follows simply corrects it.
//
// This is a byte buffer, not a terminal emulator. Replaying the tail of a
// stream can land mid-escape-sequence, and the client's terminal will
// discard that fragment; what it cannot do is corrupt the session, since
// the child's own repaint follows and is authoritative. Emulating the
// screen properly would mean carrying a full terminal implementation in
// every supervisor, which is a great deal of machinery for a frame that
// is about to be redrawn anyway.
type scrollback struct {
	mu    sync.Mutex
	buf   []byte
	limit int
}

func newScrollback(limit int) *scrollback {
	if limit <= 0 {
		limit = scrollbackLimit
	}
	return &scrollback{limit: limit, buf: make([]byte, 0, min(limit, 64*1024))}
}

// Write appends output, discarding the oldest bytes past the limit.
func (s *scrollback) Write(p []byte) {
	if len(p) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// A write larger than the whole buffer replaces it: everything before
	// it is off the screen by definition.
	if len(p) >= s.limit {
		s.buf = append(s.buf[:0], p[len(p)-s.limit:]...)
		return
	}
	if len(s.buf)+len(p) > s.limit {
		drop := len(s.buf) + len(p) - s.limit
		s.buf = append(s.buf[:0], s.buf[drop:]...)
	}
	s.buf = append(s.buf, p...)
}

// Bytes returns a copy of the remembered output, so the caller can hold it
// while the session keeps writing.
func (s *scrollback) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buf) == 0 {
		return nil
	}
	out := make([]byte, len(s.buf))
	copy(out, s.buf)
	return out
}

// Reset forgets everything, for a session whose screen is known to be
// gone (a full clear).
func (s *scrollback) Reset() {
	s.mu.Lock()
	s.buf = s.buf[:0]
	s.mu.Unlock()
}
