// Package selection provides character-level text selection for terminal UIs.
//
// It handles converting mouse coordinates (in terminal cells) to character
// positions within rendered ANSI-styled text, supporting multi-byte characters,
// wide characters (CJK, emoji), and word/line selection via double/triple click.
//
// The approach is modeled after Charm's crush: all coordinate calculations use
// display columns (terminal cells), not byte offsets or rune counts. The
// ultraviolet ScreenBuffer provides the bridge between rendered ANSI strings
// and individual character cells.
package selection

import (
	"image"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/displaywidth"
	"github.com/clipperhouse/uax29/v2/words"
)

// DoubleClickThreshold is the maximum time between clicks for multi-click.
const DoubleClickThreshold = 400 * time.Millisecond

// ClickTolerance is the pixel/cell tolerance for multi-click detection.
const ClickTolerance = 2

// State tracks the full state of a mouse text selection.
type State struct {
	// Whether a mouse button is currently held down.
	MouseDown bool

	// Position where mouse was first pressed (viewport-relative).
	MouseDownItemIdx int
	MouseDownLineIdx int
	MouseDownCol     int

	// Current drag position (viewport-relative).
	DragItemIdx int
	DragLineIdx int
	DragCol     int

	// Multi-click detection.
	LastClickTime time.Time
	LastClickX    int
	LastClickY    int
	ClickCount    int
}

// Range represents a normalized (start <= end) selection range.
type Range struct {
	StartItemIdx int
	StartLine    int
	StartCol     int
	EndItemIdx   int
	EndLine      int
	EndCol       int
}

// IsEmpty returns true if the range selects nothing.
func (r Range) IsEmpty() bool {
	return r.StartItemIdx < 0 || r.EndItemIdx < 0 ||
		(r.StartItemIdx == r.EndItemIdx && r.StartLine == r.EndLine && r.StartCol == r.EndCol)
}

// NewState creates a new empty selection state.
func NewState() State {
	return State{
		MouseDownItemIdx: -1,
		DragItemIdx:      -1,
	}
}

// Clear resets all selection state.
func (s *State) Clear() {
	s.MouseDown = false
	s.MouseDownItemIdx = -1
	s.MouseDownLineIdx = 0
	s.MouseDownCol = 0
	s.DragItemIdx = -1
	s.DragLineIdx = 0
	s.DragCol = 0
	s.LastClickTime = time.Time{}
	s.LastClickX = 0
	s.LastClickY = 0
	s.ClickCount = 0
}

// HasSelection returns true if there is a non-empty active selection.
func (s *State) HasSelection() bool {
	return s.MouseDownItemIdx >= 0 && s.DragItemIdx >= 0 && !s.GetRange().IsEmpty()
}

// GetRange returns the normalized selection range (start <= end).
func (s *State) GetRange() Range {
	if s.MouseDownItemIdx < 0 || s.DragItemIdx < 0 {
		return Range{StartItemIdx: -1, EndItemIdx: -1}
	}

	downItem := s.MouseDownItemIdx
	downLine := s.MouseDownLineIdx
	downCol := s.MouseDownCol
	dragItem := s.DragItemIdx
	dragLine := s.DragLineIdx
	dragCol := s.DragCol

	// Determine if dragging forward or backward.
	forward := dragItem > downItem ||
		(dragItem == downItem && dragLine > downLine) ||
		(dragItem == downItem && dragLine == downLine && dragCol >= downCol)

	if forward {
		return Range{
			StartItemIdx: downItem,
			StartLine:    downLine,
			StartCol:     downCol,
			EndItemIdx:   dragItem,
			EndLine:      dragLine,
			EndCol:       dragCol,
		}
	}
	return Range{
		StartItemIdx: dragItem,
		StartLine:    dragLine,
		StartCol:     dragCol,
		EndItemIdx:   downItem,
		EndLine:      downLine,
		EndCol:       downCol,
	}
}

// IsLineInRange checks if a specific line within an item falls inside the
// selection range. Returns (inRange, startCol, endCol) where startCol == -1
// means the entire line is selected. startCol == endCol means no selection
// on this line.
func IsLineInRange(r Range, itemIdx, lineIdx int) (bool, int, int) {
	if r.IsEmpty() {
		return false, 0, 0
	}

	// Outside item range entirely.
	if itemIdx < r.StartItemIdx || itemIdx > r.EndItemIdx {
		return false, 0, 0
	}

	// Single-item selection.
	if r.StartItemIdx == r.EndItemIdx {
		if itemIdx != r.StartItemIdx {
			return false, 0, 0
		}
		if lineIdx < r.StartLine || lineIdx > r.EndLine {
			return false, 0, 0
		}
		if r.StartLine == r.EndLine {
			// Single line: specific column range.
			return true, r.StartCol, r.EndCol
		}
		if lineIdx == r.StartLine {
			return true, r.StartCol, -1 // from startCol to end of line
		}
		if lineIdx == r.EndLine {
			return true, 0, r.EndCol // from start of line to endCol
		}
		return true, -1, -1 // full line (middle of multi-line selection)
	}

	// Multi-item selection.
	if itemIdx == r.StartItemIdx {
		if lineIdx < r.StartLine {
			return false, 0, 0
		}
		if lineIdx == r.StartLine {
			return true, r.StartCol, -1
		}
		return true, -1, -1 // full line
	}
	if itemIdx == r.EndItemIdx {
		if lineIdx > r.EndLine {
			return false, 0, 0
		}
		if lineIdx == r.EndLine {
			return true, 0, r.EndCol
		}
		return true, -1, -1 // full line
	}

	// Middle item: fully selected.
	return true, -1, -1
}

// FindWordBoundaries finds the start and end column of the word at the given
// column position in a plain-text line (ANSI codes already stripped).
// Returns (startCol, endCol) where endCol is exclusive.
// Uses UAX#29 word segmentation and display-width-aware column tracking.
func FindWordBoundaries(line string, col int) (startCol, endCol int) {
	if line == "" || col < 0 {
		return 0, 0
	}

	// Segment the line into words using UAX#29.
	lineCol := 0
	iter := words.FromString(line)
	for iter.Next() {
		token := iter.Value()
		tokenWidth := displaywidth.String(token)

		graphemeStart := lineCol
		graphemeEnd := lineCol + tokenWidth
		lineCol += tokenWidth

		// If clicked before this token, no word here.
		if col < graphemeStart {
			return col, col
		}

		// If clicked within this token, return its boundaries.
		if col >= graphemeStart && col < graphemeEnd {
			// Whitespace tokens produce empty selection.
			if strings.TrimSpace(token) == "" {
				return col, col
			}
			return graphemeStart, graphemeEnd
		}
	}

	return col, col
}

// HighlightLine applies reverse-video highlighting to a portion of a rendered
// line (which may contain ANSI escape codes). startCol/endCol are in display
// columns. If startCol == -1, the entire line is highlighted. If startCol ==
// endCol, returns the line unchanged.
//
// Uses ultraviolet ScreenBuffer for cell-level ANSI manipulation.
func HighlightLine(line string, startCol, endCol int) string {
	if line == "" {
		return line
	}

	lineWidth := xansi.StringWidth(line)
	if lineWidth == 0 {
		return line
	}

	// Full-line highlight.
	if startCol == -1 {
		startCol = 0
		endCol = lineWidth
	}

	if startCol >= endCol || startCol >= lineWidth {
		return line
	}
	if endCol > lineWidth {
		endCol = lineWidth
	}

	// Parse the styled line into a cell buffer.
	area := image.Rect(0, 0, lineWidth, 1)
	buf := uv.NewScreenBuffer(lineWidth, 1)
	styled := uv.NewStyledString(line)
	styled.Draw(&buf, area)

	// Apply reverse attribute to cells in the selection range.
	if buf.Height() > 0 {
		bufLine := buf.Line(0)
		for x := startCol; x < endCol && x < len(bufLine); x++ {
			cell := bufLine.At(x)
			if cell != nil {
				cell.Style.Attrs |= uv.AttrReverse
			}
		}
	}

	return buf.Render()
}

// ExtractText extracts plain text from a rendered ANSI string within the given
// column range on a single line. Uses ultraviolet to parse ANSI and extract
// character content.
func ExtractText(line string, startCol, endCol int) string {
	if line == "" {
		return ""
	}

	lineWidth := xansi.StringWidth(line)
	if lineWidth == 0 {
		return ""
	}

	// Full-line extraction.
	if startCol == -1 {
		startCol = 0
		endCol = lineWidth
	}

	if startCol >= endCol || startCol >= lineWidth {
		return ""
	}
	if endCol > lineWidth {
		endCol = lineWidth
	}

	// Parse to cell buffer.
	area := image.Rect(0, 0, lineWidth, 1)
	buf := uv.NewScreenBuffer(lineWidth, 1)
	styled := uv.NewStyledString(line)
	styled.Draw(&buf, area)

	var sb strings.Builder
	if buf.Height() > 0 {
		bufLine := buf.Line(0)
		for x := startCol; x < endCol && x < len(bufLine); x++ {
			cell := bufLine.At(x)
			if cell != nil && cell.Content != "" {
				sb.WriteString(cell.Content)
			}
		}
	}

	return sb.String()
}
