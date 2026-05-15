package ui

import (
	"fmt"
	"strings"
	"testing"
)

// fakeItem is a deterministic MessageItem for ScrollList tests.
type fakeItem struct {
	id    string
	lines int
}

func (f *fakeItem) ID() string { return f.id }
func (f *fakeItem) Render(_ int) string {
	if f.lines <= 0 {
		return ""
	}
	parts := make([]string, f.lines)
	for i := range parts {
		parts[i] = fmt.Sprintf("%s-line-%d", f.id, i)
	}
	return strings.Join(parts, "\n")
}
func (f *fakeItem) Height() int { return f.lines }

// makeItems builds n fake items of `lines` height each.
func makeItems(n, lines int) []MessageItem {
	out := make([]MessageItem, n)
	for i := range out {
		out[i] = &fakeItem{id: fmt.Sprintf("item-%d", i), lines: lines}
	}
	return out
}

// TestScrollList_MouseDownPreventsAutoScroll verifies the core fix for the
// copy-selection drift bug: while the user has the mouse button held
// (drag-selecting), incoming content updates must NOT shift the viewport,
// because doing so moves the highlighted content out from under the cursor.
func TestScrollList_MouseDownPreventsAutoScroll(t *testing.T) {
	sl := NewScrollList(80, 10)
	sl.SetItems(makeItems(20, 2)) // 40 lines of content into a 10-line viewport
	// Capture the auto-scrolled-to-bottom position.
	startOffsetIdx := sl.offsetIdx
	startOffsetLine := sl.offsetLine

	// User clicks somewhere in the visible area, starting a drag-select.
	if !sl.HandleMouseDown(5, 3) {
		t.Fatalf("HandleMouseDown should accept a click inside the viewport")
	}
	if !sl.IsMouseDown() {
		t.Fatalf("IsMouseDown should be true after HandleMouseDown")
	}

	// New content arrives. With autoScroll still true, SetItems would
	// normally call GotoBottom() and shift the viewport. The fix should
	// suppress that while MouseDown is held.
	sl.SetItems(makeItems(30, 2)) // 60 lines now
	if sl.offsetIdx != startOffsetIdx || sl.offsetLine != startOffsetLine {
		t.Errorf("viewport scrolled during active drag: was (%d,%d), now (%d,%d)",
			startOffsetIdx, startOffsetLine, sl.offsetIdx, sl.offsetLine)
	}

	// User releases the mouse — drag is over.
	sl.HandleMouseUp()
	if sl.IsMouseDown() {
		t.Fatalf("IsMouseDown should be false after HandleMouseUp")
	}

	// After release, a fresh content update should resume auto-scrolling
	// (move the offset to track the new bottom).
	afterReleaseIdx := sl.offsetIdx
	afterReleaseLine := sl.offsetLine
	sl.SetItems(makeItems(50, 2))
	if sl.offsetIdx == afterReleaseIdx && sl.offsetLine == afterReleaseLine {
		t.Errorf("autoscroll did not resume after MouseUp: offset stuck at (%d,%d)",
			afterReleaseIdx, afterReleaseLine)
	}
}

// TestScrollList_DragDisablesAutoScroll verifies that any successful
// HandleMouseDrag call clears autoScroll, even when HandleMouseDown didn't
// observe it (e.g. a stale wheel-down event set it back to true mid-stream).
func TestScrollList_DragDisablesAutoScroll(t *testing.T) {
	sl := NewScrollList(80, 10)
	sl.SetItems(makeItems(20, 2))

	// Begin a selection.
	if !sl.HandleMouseDown(5, 3) {
		t.Fatalf("HandleMouseDown failed")
	}
	// Simulate an external code path that re-enabled autoScroll while
	// MouseDown is still held (the precise condition that caused drift).
	sl.autoScroll = true

	// Drag motion should hard-lock the viewport again.
	if !sl.HandleMouseDrag(10, 4) {
		t.Fatalf("HandleMouseDrag failed")
	}
	if sl.autoScroll {
		t.Errorf("HandleMouseDrag must clear autoScroll to prevent mid-drag jumps")
	}
}

// TestScrollList_SetItemsRespectsMouseDown is the most direct regression
// test: even with autoScroll enabled and new content appended at the
// bottom, SetItems must not move the viewport while a mouse drag is in
// progress. This is what caused the "highlighting shifts by 1+ rows
// during streaming" symptom reported by the user.
func TestScrollList_SetItemsRespectsMouseDown(t *testing.T) {
	sl := NewScrollList(80, 5)
	sl.SetItems(makeItems(10, 2)) // 20 lines into a 5-line viewport
	// At bottom.
	preIdx, preLine := sl.offsetIdx, sl.offsetLine

	// Hold mouse down (no actual drag needed).
	if !sl.HandleMouseDown(0, 0) {
		t.Fatalf("HandleMouseDown failed")
	}

	// Append several more items as if streaming. With the bug, each
	// SetItems would call GotoBottom and shift the offset.
	for n := 11; n <= 15; n++ {
		sl.SetItems(makeItems(n, 2))
		if sl.offsetIdx != preIdx || sl.offsetLine != preLine {
			t.Fatalf("viewport drifted during streaming with mouse held: "+
				"start=(%d,%d) now=(%d,%d) after adding item %d",
				preIdx, preLine, sl.offsetIdx, sl.offsetLine, n)
		}
	}
}
