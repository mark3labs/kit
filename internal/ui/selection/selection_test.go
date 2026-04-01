package selection

import (
	"testing"
	"time"
)

func TestNewState(t *testing.T) {
	s := NewState()
	if s.MouseDownItemIdx != -1 {
		t.Errorf("expected MouseDownItemIdx -1, got %d", s.MouseDownItemIdx)
	}
	if s.DragItemIdx != -1 {
		t.Errorf("expected DragItemIdx -1, got %d", s.DragItemIdx)
	}
	if s.MouseDown {
		t.Error("expected MouseDown false")
	}
	if s.HasSelection() {
		t.Error("expected no selection on new state")
	}
}

func TestClear(t *testing.T) {
	s := NewState()
	s.MouseDown = true
	s.MouseDownItemIdx = 2
	s.DragItemIdx = 3
	s.ClickCount = 2
	s.Clear()

	if s.MouseDown {
		t.Error("expected MouseDown false after clear")
	}
	if s.MouseDownItemIdx != -1 {
		t.Errorf("expected MouseDownItemIdx -1 after clear, got %d", s.MouseDownItemIdx)
	}
	if s.DragItemIdx != -1 {
		t.Errorf("expected DragItemIdx -1 after clear, got %d", s.DragItemIdx)
	}
	if s.ClickCount != 0 {
		t.Errorf("expected ClickCount 0 after clear, got %d", s.ClickCount)
	}
}

func TestGetRange_Forward(t *testing.T) {
	s := NewState()
	s.MouseDownItemIdx = 0
	s.MouseDownLineIdx = 1
	s.MouseDownCol = 5
	s.DragItemIdx = 0
	s.DragLineIdx = 3
	s.DragCol = 10

	r := s.GetRange()
	if r.StartItemIdx != 0 || r.StartLine != 1 || r.StartCol != 5 {
		t.Errorf("unexpected start: item=%d line=%d col=%d", r.StartItemIdx, r.StartLine, r.StartCol)
	}
	if r.EndItemIdx != 0 || r.EndLine != 3 || r.EndCol != 10 {
		t.Errorf("unexpected end: item=%d line=%d col=%d", r.EndItemIdx, r.EndLine, r.EndCol)
	}
}

func TestGetRange_Backward(t *testing.T) {
	s := NewState()
	s.MouseDownItemIdx = 2
	s.MouseDownLineIdx = 5
	s.MouseDownCol = 20
	s.DragItemIdx = 0
	s.DragLineIdx = 1
	s.DragCol = 3

	r := s.GetRange()
	// Should be normalized: drag position becomes start
	if r.StartItemIdx != 0 || r.StartLine != 1 || r.StartCol != 3 {
		t.Errorf("unexpected start: item=%d line=%d col=%d", r.StartItemIdx, r.StartLine, r.StartCol)
	}
	if r.EndItemIdx != 2 || r.EndLine != 5 || r.EndCol != 20 {
		t.Errorf("unexpected end: item=%d line=%d col=%d", r.EndItemIdx, r.EndLine, r.EndCol)
	}
}

func TestGetRange_SameLine(t *testing.T) {
	s := NewState()
	s.MouseDownItemIdx = 1
	s.MouseDownLineIdx = 2
	s.MouseDownCol = 10
	s.DragItemIdx = 1
	s.DragLineIdx = 2
	s.DragCol = 20

	r := s.GetRange()
	if r.IsEmpty() {
		t.Error("expected non-empty range")
	}
	if r.StartCol != 10 || r.EndCol != 20 {
		t.Errorf("expected cols 10-20, got %d-%d", r.StartCol, r.EndCol)
	}
}

func TestRangeIsEmpty(t *testing.T) {
	// Same point
	r := Range{StartItemIdx: 0, StartLine: 0, StartCol: 5, EndItemIdx: 0, EndLine: 0, EndCol: 5}
	if !r.IsEmpty() {
		t.Error("expected same-point range to be empty")
	}

	// Negative item idx
	r = Range{StartItemIdx: -1, EndItemIdx: -1}
	if !r.IsEmpty() {
		t.Error("expected negative item idx range to be empty")
	}

	// Valid range
	r = Range{StartItemIdx: 0, StartLine: 0, StartCol: 0, EndItemIdx: 0, EndLine: 0, EndCol: 5}
	if r.IsEmpty() {
		t.Error("expected valid range to not be empty")
	}
}

func TestHasSelection(t *testing.T) {
	s := NewState()
	if s.HasSelection() {
		t.Error("new state should have no selection")
	}

	// Set up a valid selection
	s.MouseDownItemIdx = 0
	s.MouseDownLineIdx = 0
	s.MouseDownCol = 0
	s.DragItemIdx = 0
	s.DragLineIdx = 0
	s.DragCol = 10
	if !s.HasSelection() {
		t.Error("expected selection to exist")
	}

	// Same point = no selection
	s.DragCol = 0
	if s.HasSelection() {
		t.Error("same point should not be a selection")
	}
}

func TestIsLineInRange_SingleItem_SingleLine(t *testing.T) {
	r := Range{
		StartItemIdx: 1, StartLine: 2, StartCol: 5,
		EndItemIdx: 1, EndLine: 2, EndCol: 15,
	}

	// Exact line
	ok, sc, ec := IsLineInRange(r, 1, 2)
	if !ok || sc != 5 || ec != 15 {
		t.Errorf("expected (true, 5, 15), got (%v, %d, %d)", ok, sc, ec)
	}

	// Wrong line
	ok, _, _ = IsLineInRange(r, 1, 0)
	if ok {
		t.Error("line 0 should not be in range")
	}

	// Wrong item
	ok, _, _ = IsLineInRange(r, 0, 2)
	if ok {
		t.Error("item 0 should not be in range")
	}
}

func TestIsLineInRange_SingleItem_MultiLine(t *testing.T) {
	r := Range{
		StartItemIdx: 0, StartLine: 1, StartCol: 5,
		EndItemIdx: 0, EndLine: 4, EndCol: 10,
	}

	// Start line
	ok, sc, ec := IsLineInRange(r, 0, 1)
	if !ok || sc != 5 || ec != -1 {
		t.Errorf("start line: expected (true, 5, -1), got (%v, %d, %d)", ok, sc, ec)
	}

	// Middle line
	ok, sc, ec = IsLineInRange(r, 0, 2)
	if !ok || sc != -1 || ec != -1 {
		t.Errorf("middle line: expected (true, -1, -1), got (%v, %d, %d)", ok, sc, ec)
	}

	// End line
	ok, sc, ec = IsLineInRange(r, 0, 4)
	if !ok || sc != 0 || ec != 10 {
		t.Errorf("end line: expected (true, 0, 10), got (%v, %d, %d)", ok, sc, ec)
	}
}

func TestIsLineInRange_MultiItem(t *testing.T) {
	r := Range{
		StartItemIdx: 0, StartLine: 3, StartCol: 5,
		EndItemIdx: 2, EndLine: 1, EndCol: 10,
	}

	// First item, start line
	ok, sc, ec := IsLineInRange(r, 0, 3)
	if !ok || sc != 5 || ec != -1 {
		t.Errorf("first item start: expected (true, 5, -1), got (%v, %d, %d)", ok, sc, ec)
	}

	// First item, line after start
	ok, sc, ec = IsLineInRange(r, 0, 5)
	if !ok || sc != -1 || ec != -1 {
		t.Errorf("first item after: expected (true, -1, -1), got (%v, %d, %d)", ok, sc, ec)
	}

	// Middle item, any line
	ok, sc, ec = IsLineInRange(r, 1, 0)
	if !ok || sc != -1 || ec != -1 {
		t.Errorf("middle item: expected (true, -1, -1), got (%v, %d, %d)", ok, sc, ec)
	}

	// Last item, end line
	ok, sc, ec = IsLineInRange(r, 2, 1)
	if !ok || sc != 0 || ec != 10 {
		t.Errorf("last item end: expected (true, 0, 10), got (%v, %d, %d)", ok, sc, ec)
	}

	// Last item, line after end
	ok, _, _ = IsLineInRange(r, 2, 5)
	if ok {
		t.Error("line after end in last item should not be in range")
	}
}

func TestFindWordBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		col       int
		wantStart int
		wantEnd   int
	}{
		{
			name:      "simple word",
			line:      "hello world",
			col:       2,
			wantStart: 0,
			wantEnd:   5,
		},
		{
			name:      "second word",
			line:      "hello world",
			col:       7,
			wantStart: 6,
			wantEnd:   11,
		},
		{
			name:      "on space",
			line:      "hello world",
			col:       5,
			wantStart: 5,
			wantEnd:   5,
		},
		{
			name:      "empty line",
			line:      "",
			col:       0,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "negative col",
			line:      "hello",
			col:       -1,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "past end",
			line:      "hello",
			col:       10,
			wantStart: 10,
			wantEnd:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := FindWordBoundaries(tt.line, tt.col)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("FindWordBoundaries(%q, %d) = (%d, %d), want (%d, %d)",
					tt.line, tt.col, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestExtractText_PlainText(t *testing.T) {
	line := "Hello, World!"
	text := ExtractText(line, 0, 5)
	if text != "Hello" {
		t.Errorf("expected 'Hello', got %q", text)
	}

	text = ExtractText(line, 7, 12)
	if text != "World" {
		t.Errorf("expected 'World', got %q", text)
	}
}

func TestExtractText_FullLine(t *testing.T) {
	line := "Hello"
	text := ExtractText(line, -1, -1)
	if text != "Hello" {
		t.Errorf("expected 'Hello', got %q", text)
	}
}

func TestExtractText_Empty(t *testing.T) {
	text := ExtractText("", 0, 5)
	if text != "" {
		t.Errorf("expected empty string, got %q", text)
	}
}

func TestExtractText_OutOfBounds(t *testing.T) {
	line := "Hi"
	text := ExtractText(line, 5, 10)
	if text != "" {
		t.Errorf("expected empty string for out of bounds, got %q", text)
	}
}

func TestHighlightLine_PlainText(t *testing.T) {
	line := "Hello, World!"
	result := HighlightLine(line, 0, 5)
	// Should produce a non-empty result different from input (has ANSI codes)
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Should still contain the text content
	if len(result) < len(line) {
		t.Error("result should be at least as long as input (ANSI codes add length)")
	}
}

func TestHighlightLine_Empty(t *testing.T) {
	result := HighlightLine("", 0, 5)
	if result != "" {
		t.Errorf("expected empty for empty input, got %q", result)
	}
}

func TestHighlightLine_NoSelection(t *testing.T) {
	line := "Hello"
	result := HighlightLine(line, 3, 3)
	// Same startCol and endCol = no change
	if result != line {
		t.Errorf("expected no change for zero-width selection, got %q", result)
	}
}

// TestMultiClickDetection verifies the click counting logic.
func TestMultiClickDetection(t *testing.T) {
	s := NewState()
	now := time.Now()

	// First click
	s.LastClickTime = now
	s.LastClickX = 10
	s.LastClickY = 5
	s.ClickCount = 1

	// Second click within threshold
	later := now.Add(200 * time.Millisecond)
	if later.Sub(s.LastClickTime) <= DoubleClickThreshold {
		if abs(10-s.LastClickX) <= ClickTolerance && abs(5-s.LastClickY) <= ClickTolerance {
			s.ClickCount++
		}
	}
	if s.ClickCount != 2 {
		t.Errorf("expected click count 2, got %d", s.ClickCount)
	}

	// Third click
	s.LastClickTime = later
	later2 := later.Add(200 * time.Millisecond)
	if later2.Sub(s.LastClickTime) <= DoubleClickThreshold {
		if abs(10-s.LastClickX) <= ClickTolerance && abs(5-s.LastClickY) <= ClickTolerance {
			s.ClickCount++
		}
	}
	if s.ClickCount != 3 {
		t.Errorf("expected click count 3, got %d", s.ClickCount)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
