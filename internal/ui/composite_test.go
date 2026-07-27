package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

// fullScreenBase builds a w x h base view where every row is a repeated
// marker character, so any cell the compositor wrongly overwrites is visible.
func fullScreenBase(w, h int, ch string) string {
	rows := make([]string, h)
	for i := range rows {
		rows[i] = strings.Repeat(ch, w)
	}
	return strings.Join(rows, "\n")
}

// TestCompositeOverlay_PreservesContentBesideBox is the core guarantee: the
// box replaces only the cells it covers, so the view remains visible to the
// left and right of it. The previous whole-line merge blanked the full width
// of every row the box touched.
func TestCompositeOverlay_PreservesContentBesideBox(t *testing.T) {
	const w, h = 40, 9
	base := fullScreenBase(w, h, "x")
	box := "╭────╮\n│ hi │\n╰────╯"

	got := compositeOverlay(base, box, 15, 3, w, h)
	lines := strings.Split(got, "\n")

	if len(lines) != h {
		t.Fatalf("got %d rows, want %d", len(lines), h)
	}

	// Rows the box covers keep base content on both sides.
	for _, row := range []int{3, 4, 5} {
		plain := xansi.Strip(lines[row])
		if !strings.HasPrefix(plain, strings.Repeat("x", 15)) {
			t.Errorf("row %d: content to the LEFT of the box was erased: %q", row, plain)
		}
		if !strings.HasSuffix(strings.TrimRight(plain, " "), strings.Repeat("x", 19)) {
			t.Errorf("row %d: content to the RIGHT of the box was erased: %q", row, plain)
		}
		if !strings.Contains(plain, "│") && !strings.Contains(plain, "╭") && !strings.Contains(plain, "╰") {
			t.Errorf("row %d: box not drawn: %q", row, plain)
		}
	}

	// Rows outside the box are untouched.
	for _, row := range []int{0, 1, 2, 6, 7, 8} {
		if plain := xansi.Strip(lines[row]); plain != strings.Repeat("x", w) {
			t.Errorf("row %d outside the box was modified: %q", row, plain)
		}
	}
}

// TestCompositeOverlay_PreservesANSI verifies styling survives on both sides
// of the splice — the compositor works on cells, not raw strings.
func TestCompositeOverlay_PreservesANSI(t *testing.T) {
	base := strings.Join([]string{
		"\x1b[31m" + strings.Repeat("r", 30) + "\x1b[0m",
		"\x1b[31m" + strings.Repeat("r", 30) + "\x1b[0m",
		"\x1b[31m" + strings.Repeat("r", 30) + "\x1b[0m",
	}, "\n")

	got := compositeOverlay(base, "\x1b[32mBOX\x1b[0m", 10, 1, 30, 3)
	row := strings.Split(got, "\n")[1]

	if !strings.Contains(row, "BOX") {
		t.Fatalf("box missing: %q", row)
	}
	// Red (31) from the base and green (32) from the box coexist on the row.
	if !strings.Contains(row, "31") {
		t.Errorf("base styling lost: %q", row)
	}
	if !strings.Contains(row, "32") {
		t.Errorf("box styling lost: %q", row)
	}
}

// TestCompositeOverlay_ClampsToScreen verifies an out-of-bounds or oversized
// box is pinned on screen rather than drawn into the void.
func TestCompositeOverlay_ClampsToScreen(t *testing.T) {
	const w, h = 20, 5
	base := fullScreenBase(w, h, "x")
	box := "╭──╮\n╰──╯"

	// Far past the right/bottom edge.
	got := compositeOverlay(base, box, 500, 500, w, h)
	if !strings.Contains(got, "╭") {
		t.Error("box was clipped out of existence")
	}
	for i, line := range strings.Split(got, "\n") {
		if got := xansi.StringWidth(line); got > w {
			t.Errorf("row %d width %d exceeds screen width %d", i, got, w)
		}
	}

	// Negative coordinates.
	if got := compositeOverlay(base, box, -50, -50, w, h); !strings.Contains(got, "╭") {
		t.Error("box lost at negative coordinates")
	}
}

// TestCompositeOverlay_TrimsUnstyledTrailingRows guards the regression where
// a popup's bottom margin — a row of plain spaces — punched a blank band
// through the content below the box.
func TestCompositeOverlay_TrimsUnstyledTrailingRows(t *testing.T) {
	const w, h = 20, 5
	base := fullScreenBase(w, h, "x")

	// Box with two unstyled padding rows below it.
	box := "╭──╮\n╰──╯\n    \n    "

	got := compositeOverlay(base, box, 0, 0, w, h)
	lines := strings.Split(got, "\n")

	for _, row := range []int{2, 3} {
		if plain := xansi.Strip(lines[row]); plain != strings.Repeat("x", w) {
			t.Errorf("row %d was erased by trailing padding: %q", row, plain)
		}
	}
}

// TestTrimTrailingBlankLines_KeepsStyledRows verifies a row carrying a
// background colour is preserved — it is part of the box's appearance, not
// stray padding.
func TestTrimTrailingBlankLines_KeepsStyledRows(t *testing.T) {
	styled := "box\n\x1b[48;2;13;13;13m   \x1b[m"
	if got := trimTrailingBlankLines(styled); got != styled {
		t.Errorf("styled trailing row was dropped:\ngot  %q\nwant %q", got, styled)
	}

	plain := "box\n   \n  "
	if got := trimTrailingBlankLines(plain); got != "box" {
		t.Errorf("plain trailing rows not trimmed: %q", got)
	}

	if got := trimTrailingBlankLines("   "); got != "" {
		t.Errorf("all-blank input should trim to empty, got %q", got)
	}
}

// TestCompositeCentered_Centres verifies the box lands in the middle.
func TestCompositeCentered_Centres(t *testing.T) {
	const w, h = 21, 7
	base := fullScreenBase(w, h, "x")
	box := "abc" // 3 wide, 1 tall

	got := compositeCentered(base, box, w, h)
	lines := strings.Split(got, "\n")

	mid := xansi.Strip(lines[3]) // (7-1)/2 = 3
	if !strings.Contains(mid, "abc") {
		t.Fatalf("box not on the centre row: %q", mid)
	}
	if idx := strings.Index(mid, "abc"); idx != (w-3)/2 {
		t.Errorf("box at column %d, want %d", idx, (w-3)/2)
	}
}

// TestCompositeAnchored_Anchors covers the three vertical anchors extensions
// can request.
func TestCompositeAnchored_Anchors(t *testing.T) {
	const w, h = 20, 10
	base := fullScreenBase(w, h, "x")
	box := "BOX"

	rowOf := func(rendered string) int {
		for i, line := range strings.Split(rendered, "\n") {
			if strings.Contains(xansi.Strip(line), "BOX") {
				return i
			}
		}
		return -1
	}

	if got := rowOf(compositeAnchored(base, box, "top-center", w, h)); got != 1 {
		t.Errorf("top-center row = %d, want 1", got)
	}
	if got := rowOf(compositeAnchored(base, box, "bottom-center", w, h)); got != h-2 {
		t.Errorf("bottom-center row = %d, want %d", got, h-2)
	}
	if got := rowOf(compositeAnchored(base, box, "center", w, h)); got != (h-1)/2 {
		t.Errorf("center row = %d, want %d", got, (h-1)/2)
	}
}

// TestCompositeOverlay_EmptyBoxIsNoop verifies an inactive overlay leaves the
// view untouched.
func TestCompositeOverlay_EmptyBoxIsNoop(t *testing.T) {
	base := fullScreenBase(10, 3, "x")
	for _, box := range []string{"", "   ", "\n\n"} {
		if got := compositeOverlay(base, box, 0, 0, 10, 3); got != base {
			t.Errorf("box %q modified the base view", box)
		}
	}
}

// TestPopupList_RenderHasNoUnstyledTrailingRow pins the root cause of the
// blank-band regression at its source.
func TestPopupList_RenderHasNoUnstyledTrailingRow(t *testing.T) {
	p := NewPopupList("Commands", []PopupItem{{Label: "/help", Description: "show help"}}, 100, 30)

	lines := strings.Split(p.Render(), "\n")
	last := lines[len(lines)-1]
	if strings.TrimSpace(xansi.Strip(last)) == "" && !strings.Contains(last, "\x1b") {
		t.Errorf("popup ends with an unstyled blank row that would erase content: %q", last)
	}
	if !strings.Contains(last, "╰") {
		t.Errorf("expected the bottom border to be the last row, got %q", xansi.Strip(last))
	}
}

// --------------------------------------------------------------------------
// Overlay dialog body wrapping
// --------------------------------------------------------------------------

// longLineBody builds n lines that are each far wider than any dialog, the
// shape grep and ls output take (path + line number + matched source).
func longLineBody(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "internal/ui/some/deeply/nested/path/file.go:1234:\tfunc SomeReasonablyLongFunctionName(ctx context.Context) error {"
	}
	return strings.Join(lines, "\n")
}

// TestOverlayDialog_WrapsLongLinesWithinBudget guards the bug that made grep
// and ls results unreadable: the box style wrapped over-long lines itself, so
// one source line became several rows and the height budget was blown — a
// 30-row terminal produced a 48-row dialog that overflowed the screen.
func TestOverlayDialog_WrapsLongLinesWithinBudget(t *testing.T) {
	for _, tc := range []struct{ termW, termH int }{
		{120, 30}, {80, 24}, {200, 50}, {60, 20},
	} {
		o := newOverlayDialog("Tool Result", longLineBody(60), false, "", "", 0, 0, "center", nil, tc.termW, tc.termH)

		h := lipgloss.Height(o.Render())
		if want := tc.termH * 80 / 100; h > want {
			t.Errorf("%dx%d: height %d exceeds the %d budget", tc.termW, tc.termH, h, want)
		}
		if h > tc.termH {
			t.Errorf("%dx%d: height %d overflows the terminal", tc.termW, tc.termH, h)
		}
	}
}

// TestOverlayDialog_NoRowExceedsBoxWidth verifies wrapping happens before
// rendering, so no row is clipped by the compositor.
func TestOverlayDialog_NoRowExceedsBoxWidth(t *testing.T) {
	o := newOverlayDialog("Tool Result", longLineBody(40), false, "", "", 0, 0, "center", nil, 100, 30)

	out := o.Render()
	boxW := lipgloss.Width(out)
	for i, line := range strings.Split(out, "\n") {
		if got := xansi.StringWidth(line); got != boxW {
			t.Errorf("row %d width %d != box width %d: %q", i, got, boxW, xansi.Strip(line))
		}
	}
}

// TestOverlayDialog_ScrollCountsDisplayRows verifies the "of N lines" counter
// reports rows the reader actually scrolls through. Counting unwrapped source
// lines made the indicator disagree with the content and left the tail of a
// long result unreachable.
func TestOverlayDialog_ScrollCountsDisplayRows(t *testing.T) {
	const sourceLines = 60
	o := newOverlayDialog("Tool Result", longLineBody(sourceLines), false, "", "", 0, 0, "center", nil, 100, 30)
	o.Render()

	if o.totalLines <= sourceLines {
		t.Errorf("totalLines = %d, want > %d (wrapped rows, not source lines)", o.totalLines, sourceLines)
	}

	// The end of the content must be reachable and stay within budget.
	o.scrollOff = o.totalLines
	end := o.Render()
	if h := lipgloss.Height(end); h > 30 {
		t.Errorf("scrolled-to-end height %d overflows the terminal", h)
	}
	if o.scrollOff >= o.totalLines {
		t.Errorf("scroll offset %d not clamped below %d", o.scrollOff, o.totalLines)
	}
}

// TestOverlayDialog_LastLineReachable walks to the bottom and confirms the
// final line of a long tool result is actually displayed.
func TestOverlayDialog_LastLineReachable(t *testing.T) {
	var lines []string
	for i := 1; i <= 80; i++ {
		lines = append(lines, fmt.Sprintf("internal/ui/path/to/file_%02d.go:%d:\tmatched source text here", i, i))
	}
	o := newOverlayDialog("Tool Result", strings.Join(lines, "\n"), false, "", "", 0, 0, "center", nil, 110, 30)

	// totalLines is computed during Render, so measure before seeking.
	o.Render()
	o.scrollOff = o.totalLines
	if got := xansi.Strip(o.Render()); !strings.Contains(got, "file_80.go") {
		t.Errorf("last line unreachable at maximum scroll:\n%s", got)
	}
}
