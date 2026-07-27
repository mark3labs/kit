package ui

import (
	"strings"
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
