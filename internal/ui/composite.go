package ui

import (
	"image"
	"strings"

	"bloomfx.dev/bloom"
	"charm.land/lipgloss/v2"
)

// --------------------------------------------------------------------------
// Overlay compositing
// --------------------------------------------------------------------------

// overlayRect returns where a box goes on a termW×termH screen. The box is
// centred horizontally; anchor sets its row: "top-center", "bottom-center", or
// the centre for anything else. The box must already be trimmed with
// trimTrailingBlankLines, so that it is measured at the size it is drawn at.
//
// The rectangle is clamped, so a box larger than the terminal is pinned to
// the top left rather than drawn off-screen. ok is false when there is
// nothing to draw.
func overlayRect(box, anchor string, termW, termH int) (r image.Rectangle, ok bool) {
	if termW <= 0 || termH <= 0 || strings.TrimSpace(box) == "" {
		return image.Rectangle{}, false
	}
	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)
	x := (termW - boxW) / 2

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
	return overlayRectAt(box, x, y, termW, termH), true
}

// anchorAboveInput is the anchor of a box that belongs to the composer: it
// goes against the top edge of the composer, at its left end. The layout
// places it, since the row of the composer is not known before the frame is
// laid out; see surfaceLayers.
const anchorAboveInput = "above-input"

// boxAtHeight returns a bordered box drawn at a height that is not its own,
// for a frame in which the box is on its way from one height to another.
//
// tail is the number of rows at the foot of the box that stay whole: its
// footer and its border. A box that is too tall loses rows from above its
// foot, and a box that is too short gets blank rows there, so at every height
// the box is closed and its footer is where the eye left it. The second row
// of the box, the padding below its top border, is the blank row.
func boxAtHeight(box string, rows, tail int) string {
	lines := strings.Split(box, "\n")
	n := len(lines)
	if rows <= 0 || rows >= layoutUnbounded || rows == n || tail <= 0 || n < tail+2 {
		return box
	}
	head, foot := lines[:n-tail], lines[n-tail:]
	out := make([]string, 0, max(rows, tail+1))
	if rows < n {
		// The top border always stays, so the box is never open.
		out = append(out, head[:min(max(rows-tail, 1), len(head))]...)
	} else {
		out = append(out, head...)
		for range rows - n {
			out = append(out, lines[1])
		}
	}
	return strings.Join(append(out, foot...), "\n")
}

// overlayRectAt returns the rectangle of a box whose top-left cell is asked
// for at x, y, clamped to the screen.
func overlayRectAt(box string, x, y, termW, termH int) image.Rectangle {
	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)
	x = clamp(x, 0, max(termW-boxW, 0))
	y = clamp(y, 0, max(termH-boxH, 0))
	return image.Rect(x, y, x+boxW, y+boxH)
}

// compositeOverlay draws a rendered box on top of a full-screen base view at
// the given top-left cell coordinates and returns the merged frame.
//
// The box is a bloom layer over the base, so only the cells the box actually
// covers are replaced. Content above, below, and — critically — to the left
// and right of the box survives, with its ANSI styling intact on both sides
// of the splice.
//
// The naive alternative (merging whole lines, preferring the overlay whenever
// its line is non-blank) blanks the full width of every row the box occupies,
// which is what made popups appear to punch a hole through the transcript.
//
// x and y are clamped so a box larger than the terminal is pinned to the top
// left rather than drawn off-screen.
//
// AppModel.View does not call this: it builds one overlay for all of its
// surfaces, with transitions, from the same geometry (see surfaceLayers).
// This is the stateless form, for a component that draws one box on its own.
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

	return bloom.Render(termW, termH, bloom.Overlay(
		region(base),
		bloom.Layer(region(box)).Place(overlayRectAt(box, x, y, termW, termH)),
	))
}

// compositeCentered draws a box centred over the base view.
func compositeCentered(base, box string, termW, termH int) string {
	return compositeAnchored(base, box, "", termW, termH)
}

// compositeAnchored draws a box over the base view using one of the named
// vertical anchors ("top-center", "bottom-center", or centred by default).
// The box is always centred horizontally.
func compositeAnchored(base, box, anchor string, termW, termH int) string {
	if strings.TrimSpace(box) == "" {
		return base
	}
	// Trim before measuring: centring on the untrimmed height would place
	// the box using a taller size than the one actually drawn and shift it
	// off true centre.
	box = trimTrailingBlankLines(box)
	r, ok := overlayRect(box, anchor, termW, termH)
	if !ok {
		return base
	}
	return compositeOverlay(base, box, r.Min.X, r.Min.Y, termW, termH)
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
