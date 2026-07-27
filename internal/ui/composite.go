package ui

import (
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// --------------------------------------------------------------------------
// Overlay compositing
// --------------------------------------------------------------------------

// compositeOverlay draws a rendered box on top of a full-screen base view at
// the given top-left cell coordinates and returns the merged frame.
//
// Compositing happens at the cell level via an ultraviolet ScreenBuffer: the
// base is drawn first, then the box is drawn into its own sub-rectangle, so
// only the cells the box actually covers are replaced. Content above, below,
// and — critically — to the left and right of the box survives, with its ANSI
// styling intact on both sides of the splice.
//
// The naive alternative (merging whole lines, preferring the overlay whenever
// its line is non-blank) blanks the full width of every row the box occupies,
// which is what made popups appear to punch a hole through the transcript.
//
// x and y are clamped so a box larger than the terminal is pinned to the top
// left rather than drawn off-screen.
func compositeOverlay(base, box string, x, y, termW, termH int) string {
	if termW <= 0 || termH <= 0 {
		return base
	}
	if strings.TrimSpace(box) == "" {
		return base
	}

	box = trimTrailingBlankLines(box)
	if box == "" {
		return base
	}

	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)

	x = clamp(x, 0, max(termW-boxW, 0))
	y = clamp(y, 0, max(termH-boxH, 0))

	buf := uv.NewScreenBuffer(termW, termH)
	uv.NewStyledString(base).Draw(&buf, image.Rect(0, 0, termW, termH))
	uv.NewStyledString(box).Draw(&buf, image.Rect(x, y, x+boxW, y+boxH))

	return buf.Render()
}

// compositeCentered draws a box centred over the base view.
func compositeCentered(base, box string, termW, termH int) string {
	if strings.TrimSpace(box) == "" {
		return base
	}
	// Trim before measuring: compositeOverlay trims internally, so centring
	// on the untrimmed height would place the box using a taller size than
	// the one actually drawn and shift it off true centre.
	box = trimTrailingBlankLines(box)
	x := (termW - lipgloss.Width(box)) / 2
	y := (termH - lipgloss.Height(box)) / 2
	return compositeOverlay(base, box, x, y, termW, termH)
}

// compositeAnchored draws a box over the base view using one of the named
// vertical anchors ("top-center", "bottom-center", or centred by default).
// The box is always centred horizontally.
func compositeAnchored(base, box, anchor string, termW, termH int) string {
	if strings.TrimSpace(box) == "" {
		return base
	}

	box = trimTrailingBlankLines(box)
	boxH := lipgloss.Height(box)
	x := (termW - lipgloss.Width(box)) / 2

	var y int
	switch anchor {
	case "top-center":
		// One blank row of breathing room above the box.
		y = 1
	case "bottom-center":
		y = termH - boxH - 1
	default:
		y = (termH - boxH) / 2
	}

	return compositeOverlay(base, box, x, y, termW, termH)
}

// trimTrailingBlankLines drops trailing rows that consist only of plain
// spaces.
//
// Such rows are invisible when a box is placed on an empty canvas, but the
// compositor draws every cell it is given — so an unstyled padding row would
// punch a blank band through the content behind the box. A row carrying an
// escape sequence (e.g. a themed background) is deliberately kept: it is part
// of the box's appearance, not stray padding.
func trimTrailingBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if end == len(lines) {
		return s
	}
	return strings.Join(lines[:end], "\n")
}
