//go:build !windows

package daemon

import (
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestPTYSizerNudgeKeepsALaterResize is the regression test for a session
// that stayed drawn at a stale size. A redraw nudge used to restore the size
// it captured before its sleep, so a resize that landed in the gap was
// undone and never re-applied.
func TestPTYSizerNudgeKeepsALaterResize(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Skipf("no pty available: %v", err)
	}
	defer func() { _ = ptmx.Close(); _ = tty.Close() }()

	var s ptySizer
	if err := s.resize(ptmx, winSize{cols: 80, rows: 24}); err != nil {
		t.Fatalf("resize: %v", err)
	}
	s.nudge(ptmx, winSize{cols: 80, rows: 24})

	// A real resize during the nudge gap.
	if err := s.resize(ptmx, winSize{cols: 160, rows: 50}); err != nil {
		t.Fatalf("resize: %v", err)
	}
	if last := waitNudge(&s); last != (winSize{cols: 160, rows: 50}) {
		t.Fatalf("recorded size = %+v, want 160x50", last)
	}

	ws, err := pty.GetsizeFull(tty)
	if err != nil {
		t.Fatalf("getsize: %v", err)
	}
	if ws.Cols != 160 || ws.Rows != 50 {
		t.Fatalf("pty size after nudge = %dx%d, want 160x50", ws.Cols, ws.Rows)
	}
}

// TestPTYSizerNudgeRestoresTheSize checks the nudge itself: one row less,
// then the original size back.
func TestPTYSizerNudgeRestoresTheSize(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Skipf("no pty available: %v", err)
	}
	defer func() { _ = ptmx.Close(); _ = tty.Close() }()

	var s ptySizer
	s.nudge(ptmx, winSize{cols: 100, rows: 30})
	if ws, _ := pty.GetsizeFull(tty); ws.Rows != 29 {
		t.Fatalf("rows during nudge = %d, want 29", ws.Rows)
	}
	if last := waitNudge(&s); last != (winSize{cols: 100, rows: 30}) {
		t.Fatalf("recorded size = %+v, want 100x30", last)
	}
	if ws, _ := pty.GetsizeFull(tty); ws.Cols != 100 || ws.Rows != 30 {
		t.Fatalf("pty size after nudge = %dx%d, want 100x30", ws.Cols, ws.Rows)
	}
}

// waitNudge waits out a nudge and returns the size the sizer recorded.
// It reads that size under the sizer lock, so the nudge goroutine's last
// PTY access happens before the test closes the PTY.
func waitNudge(s *ptySizer) winSize {
	time.Sleep(redrawNudgeGap * 3)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}
