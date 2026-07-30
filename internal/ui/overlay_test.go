package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/style"
)

// --------------------------------------------------------------------------
// Body geometry
// --------------------------------------------------------------------------

// longBody returns n numbered lines, long enough that most of them wrap.
func longBody(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "diagnostic line %d that is sometimes quite long indeed and wraps around\n", i)
	}
	return b.String()
}

// readerDialog returns a dialog configured the way the message inspector
// configures one.
func readerDialog(content string, dialogWidth, termHeight int) *overlayDialog {
	o := newOverlayDialog("Error", content, false, "", "",
		dialogWidth, termHeight-3, "center", nil, dialogWidth+4, termHeight)
	o.dismissOnly = true
	o.lineNumbers = true
	o.scrollbar = true
	o.wrapMarks = true
	return o
}

// TestOverlayBody_RowsAreExactlyInnerWidth is the invariant the gutter, the
// continuation marker and the scrollbar all depend on: every body row occupies
// exactly the content width, so the track lands in the same column on each one.
//
// The box style pads short rows out to the content width, which hides an
// undersized row in the finished dialog — the scrollbar simply drifts inward.
// Measuring renderBody directly is what catches it.
func TestOverlayBody_RowsAreExactlyInnerWidth(t *testing.T) {
	theme := style.GetTheme()

	// Line counts spanning one, three and four digits of gutter.
	for _, lines := range []int{9, 150, 4000} {
		for _, dw := range []int{40, 60, 81, 120} {
			o := readerDialog(longBody(lines), dw, 26)
			o.Render()

			innerWidth := max(dw-5, 6)
			textWidth := max(innerWidth-dialogScrollbarGap, 1)
			rows := o.resolveBody(textWidth)

			visible := rows[:min(6, len(rows))]
			body := o.renderBody(visible, textWidth, innerWidth, true, len(visible), theme.Info, theme)

			for i, line := range strings.Split(body, "\n") {
				if w := xansi.StringWidth(line); w != innerWidth {
					t.Errorf("lines=%d dw=%d: row %d width = %d, want %d (%q)",
						lines, dw, i, w, innerWidth, xansi.Strip(line))
				}
			}
		}
	}
}

// TestOverlayBody_GutterIsAsNarrowAsItDraws pins the gutter's width to the
// columns it actually paints.
//
// The separator is a box-drawing character, so sizing the gutter with len()
// claims three columns for a glyph that draws one. The sizing and the rendering
// agreed on that mistake, so the geometry held and nothing looked broken — the
// gutter just carried two dead columns taken from the text.
func TestOverlayBody_GutterIsAsNarrowAsItDraws(t *testing.T) {
	o := readerDialog(longBody(150), 80, 30) // 150 lines -> 3 digits
	o.Render()

	// 3 digits + separator + one space.
	if want := 3 + 1 + 1; o.bodyGutter != want {
		t.Errorf("bodyGutter = %d, want %d", o.bodyGutter, want)
	}

	rows := o.resolveBody(max(80-5-dialogScrollbarGap, 1))
	for _, row := range rows[:min(3, len(rows))] {
		if got := lipgloss.Width(o.gutterCell(row)); got != o.bodyGutter {
			t.Errorf("gutter cell is %d columns, want %d", got, o.bodyGutter)
		}
	}
}

// TestOverlayBody_GutterNumbersSourceLines verifies the gutter counts source
// lines rather than screen rows: a wrapped continuation belongs to the line
// above it, so it repeats that number and leaves the cell blank.
func TestOverlayBody_GutterNumbersSourceLines(t *testing.T) {
	content := "short\n" +
		strings.Repeat("wrap ", 40) + "\n" +
		"after"

	o := readerDialog(content, 50, 30)
	o.Render()

	rows := o.resolveBody(max(50-5-dialogScrollbarGap, 1))
	if o.bodyGutter == 0 {
		t.Fatal("line numbering did not engage for literal content")
	}

	var nums []int
	var conts []bool
	for _, r := range rows {
		nums = append(nums, r.num)
		conts = append(conts, r.cont)
	}

	if nums[0] != 1 || conts[0] {
		t.Errorf("first row: num=%d cont=%v, want 1/false", nums[0], conts[0])
	}
	if nums[1] != 2 || conts[1] {
		t.Errorf("second row: num=%d cont=%v, want 2/false", nums[1], conts[1])
	}
	// The long line must have produced at least one continuation, tagged with
	// the same source number.
	if !conts[2] || nums[2] != 2 {
		t.Errorf("third row: num=%d cont=%v, want 2/true", nums[2], conts[2])
	}
	// The last row is the third source line, whatever the wrapping did.
	if last := len(rows) - 1; nums[last] != 3 || conts[last] {
		t.Errorf("last row: num=%d cont=%v, want 3/false", nums[last], conts[last])
	}
}

// TestOverlayBody_GutterSkippedForMarkdown pins the rule that numbering is for
// literal output only. Markdown is reflowed prose, where a number per rendered
// line corresponds to nothing the reader can refer to.
func TestOverlayBody_GutterSkippedForMarkdown(t *testing.T) {
	o := newOverlayDialog("Assistant", "para one\n\npara two\n", true, "", "",
		60, 20, "center", nil, 64, 24)
	o.lineNumbers = true
	o.Render()

	if o.bodyGutter != 0 {
		t.Errorf("bodyGutter = %d for markdown, want 0", o.bodyGutter)
	}
}

// TestOverlayBody_LazyRendererOwnsItsWidth verifies a body renderer is asked
// for the text width it must fill, and that its output is passed through
// without the tab expansion applied to plain content.
//
// Tool bodies are laid out to an exact width with tabs already accounted for;
// expanding a tab afterwards pushes the row past the frame, and every line then
// wraps in two.
func TestOverlayBody_LazyRendererOwnsItsWidth(t *testing.T) {
	const dw = 80
	var gotWidth int

	o := readerDialog("", dw, 30)
	o.body = func(width int) string {
		gotWidth = width
		// A row of exactly `width` columns containing a tab. Expanded, it
		// would overflow.
		return "a\tb" + strings.Repeat("x", width-3)
	}
	o.lineNumbers = false // a renderer supplies its own gutter if it wants one

	o.Render()

	innerWidth := max(dw-5, 6)
	if want := innerWidth - dialogScrollbarGap; gotWidth != want {
		t.Errorf("renderer asked for width %d, want %d", gotWidth, want)
	}
	if o.totalLines != 1 {
		t.Errorf("totalLines = %d, want 1 (the row was re-wrapped)", o.totalLines)
	}
}

// TestOverlayBody_CachedAcrossFrames verifies the body is resolved once per
// width rather than on every frame. An uncapped diff or a markdown pass is far
// too expensive to repeat sixty times a second.
func TestOverlayBody_CachedAcrossFrames(t *testing.T) {
	var calls int

	o := readerDialog("", 80, 30)
	o.body = func(width int) string {
		calls++
		return longBody(30)
	}

	o.Render()
	o.Render()
	o.Render()

	if calls != 1 {
		t.Errorf("body renderer called %d times across three frames, want 1", calls)
	}

	// A width change must invalidate it: the old layout was for a width that
	// no longer exists.
	o.dialogWidth = 60
	o.Render()
	if calls != 2 {
		t.Errorf("body renderer called %d times after a resize, want 2", calls)
	}
}

// --------------------------------------------------------------------------
// Scrollbar
// --------------------------------------------------------------------------

// TestOverlayScrollbar_ThumbTracksPosition verifies the thumb reaches both
// ends of the track and never leaves it: a thumb that parks at the top for the
// first several rows, or runs past the bottom, misreports the position it
// exists to report.
func TestOverlayScrollbar_ThumbTracksPosition(t *testing.T) {
	const viewport = 10

	o := readerDialog(longBody(200), 80, 30)
	o.Render() // establishes totalLines
	total := o.totalLines
	maxOff := total - viewport

	if maxOff <= 0 {
		t.Fatalf("body of %d rows does not exceed the %d-row viewport", total, viewport)
	}

	// At the top the thumb starts at row 0; at the bottom it ends at the
	// last row.
	o.scrollOff = 0
	if start, _ := o.thumbRange(viewport); start != 0 {
		t.Errorf("thumb start at top = %d, want 0", start)
	}
	o.scrollOff = maxOff
	if _, end := o.thumbRange(viewport); end != viewport {
		t.Errorf("thumb end at bottom = %d, want %d", end, viewport)
	}

	// In between it must stay inside the track and move monotonically.
	prev := -1
	for off := 0; off <= maxOff; off++ {
		o.scrollOff = off
		start, end := o.thumbRange(viewport)
		if start < 0 || end > viewport || start >= end {
			t.Fatalf("offset %d: thumb [%d,%d) outside track [0,%d)", off, start, end, viewport)
		}
		if start < prev {
			t.Errorf("offset %d: thumb moved backwards (%d after %d)", off, start, prev)
		}
		prev = start
	}
}

// TestOverlayScrollbar_ReservesColumnsWhenBodyFits guards against a jump: the
// track's columns are reserved whether or not the body currently scrolls, so a
// body that grows past the viewport does not re-wrap and shift sideways.
func TestOverlayScrollbar_ReservesColumnsWhenBodyFits(t *testing.T) {
	const dw = 60
	innerWidth := max(dw-5, 6)

	short := readerDialog("one\ntwo\n", dw, 30)
	short.Render()
	if got := short.bodyCacheWidth; got != innerWidth-dialogScrollbarGap {
		t.Errorf("short body wrapped to %d, want %d (track columns not reserved)",
			got, innerWidth-dialogScrollbarGap)
	}
}

// --------------------------------------------------------------------------
// Keys, paging, copy
// --------------------------------------------------------------------------

// press sends a key to the dialog and returns the resulting command.
func press(o *overlayDialog, key string) (*overlayResult, tea.Cmd) {
	return o.Update(tea.KeyPressMsg{Code: keyCodeOf(key), Text: keyTextOf(key)})
}

// keyCodeOf maps the key names used in these tests to their rune codes. Named
// keys are passed through the dedicated helpers below.
func keyCodeOf(key string) rune {
	if len([]rune(key)) == 1 {
		return []rune(key)[0]
	}
	return 0
}

func keyTextOf(key string) string {
	if len([]rune(key)) == 1 {
		return key
	}
	return ""
}

// TestOverlayKeys_PagingMovesAPageAtATime verifies PgDn advances by roughly a
// screen rather than a line, and keeps a row of overlap so the reader has an
// anchor to pick up from.
func TestOverlayKeys_PagingMovesAPageAtATime(t *testing.T) {
	o := readerDialog(longBody(400), 80, 30)
	o.Render()

	page := o.pageSize
	if page < 4 {
		t.Fatalf("viewport of %d rows is too small to page", page)
	}

	o.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	o.Render()
	if got, want := o.scrollOff, page-dialogPageOverlap; got != want {
		t.Errorf("after PgDn scrollOff = %d, want %d", got, want)
	}

	o.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	o.Render()
	if o.scrollOff != 0 {
		t.Errorf("after PgUp scrollOff = %d, want 0", o.scrollOff)
	}

	// Half-page steps are half a screen, and never zero.
	if got, want := o.halfPage(), max(page/2, 1); got != want {
		t.Errorf("halfPage = %d, want %d", got, want)
	}
}

// TestOverlayKeys_WheelScrolls verifies the wheel reaches the dialog. The
// scrollback underneath scrolls on the wheel, so a dead wheel over a reader
// opened to see *more* of a message is the worst place for it.
func TestOverlayKeys_WheelScrolls(t *testing.T) {
	o := readerDialog(longBody(200), 80, 30)
	o.Render()

	o.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	o.Render()
	if o.scrollOff == 0 {
		t.Error("wheel down did not scroll the dialog")
	}

	down := o.scrollOff
	o.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	o.Render()
	if o.scrollOff >= down {
		t.Errorf("wheel up did not scroll back (%d then %d)", down, o.scrollOff)
	}
}

// TestOverlayKeys_CopyEmitsClipboardCommand verifies the copy binding produces
// a command, and only when there is something to copy — an extension overlay
// has no source text and must not advertise a key it cannot honour.
func TestOverlayKeys_CopyEmitsClipboardCommand(t *testing.T) {
	o := readerDialog("some content", 80, 30)
	o.copyText = "some content"
	o.Render()

	result, cmd := press(o, "y")
	if result != nil {
		t.Error("copy must not close the dialog")
	}
	if cmd == nil {
		t.Fatal("copy produced no clipboard command")
	}
	if !o.copied {
		t.Error("copy did not latch its acknowledgement")
	}
	if got := xansi.Strip(o.Render()); !strings.Contains(got, "copied") {
		t.Error("footer does not acknowledge the copy")
	}

	// The next keystroke retires the acknowledgement.
	press(o, "j")
	if o.copied {
		t.Error("acknowledgement outlived the next keystroke")
	}

	// With nothing to copy the key is inert and the hint is absent.
	bare := readerDialog("body", 80, 30)
	bare.Render()
	if _, cmd := press(bare, "y"); cmd != nil {
		t.Error("copy fired with no source text")
	}
	if strings.Contains(xansi.Strip(bare.Render()), "y copy") {
		t.Error("copy hint shown for a dialog that cannot copy")
	}
}

// --------------------------------------------------------------------------
// Search
// --------------------------------------------------------------------------

// TestOverlaySearch_FindsAndCyclesMatches covers the whole search flow: the
// query line absorbs typing, Enter commits, the match count is reported, and
// n/N cycle through hits.
func TestOverlaySearch_FindsAndCyclesMatches(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 60; i++ {
		if i%20 == 0 {
			b.WriteString("this line holds the needle\n")
			continue
		}
		fmt.Fprintf(&b, "ordinary line %d\n", i)
	}

	o := readerDialog(b.String(), 80, 24)
	o.Render()

	press(o, "/")
	if !o.searching {
		t.Fatal("'/' did not open the query line")
	}
	for _, r := range "needle" {
		press(o, string(r))
	}
	if o.query != "needle" {
		t.Fatalf("query = %q, want %q", o.query, "needle")
	}

	// While the query line is open, navigation keys are text.
	if o.scrollOff != 0 {
		t.Error("typing a query scrolled the body")
	}

	o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if o.searching {
		t.Error("Enter did not commit the query")
	}
	o.Render()

	if len(o.hits) != 3 {
		t.Fatalf("found %d matches, want 3", len(o.hits))
	}
	if got := xansi.Strip(o.Render()); !strings.Contains(got, "1/3") {
		t.Errorf("match counter not shown: %q", got)
	}

	first := o.hits[o.hitIdx]
	press(o, "n")
	o.Render()
	if o.hits[o.hitIdx] == first {
		t.Error("'n' did not advance to the next match")
	}
	press(o, "N")
	o.Render()
	if o.hits[o.hitIdx] != first {
		t.Error("'N' did not return to the previous match")
	}

	// Esc drops the query rather than closing the reader, so a search does
	// not cost the reader their place.
	result, _ := o.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if result != nil {
		t.Error("Esc closed the dialog instead of clearing the search")
	}
	if o.query != "" || len(o.hits) != 0 {
		t.Error("Esc did not clear the search")
	}

	// A second Esc closes it.
	if result, _ := o.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); result == nil || !result.cancelled {
		t.Error("Esc with no active search should close the dialog")
	}
}

// TestOverlaySearch_ReportsNoMatches verifies a query that finds nothing says
// so, rather than appearing to have done nothing at all.
func TestOverlaySearch_ReportsNoMatches(t *testing.T) {
	o := readerDialog(longBody(80), 80, 24)
	o.Render()
	o.query = "zzz-not-present"
	o.Render()

	if got := xansi.Strip(o.Render()); !strings.Contains(got, "no matches") {
		t.Errorf("footer does not report an empty result: %q", got)
	}
}

// TestOverlaySearch_HighlightLeavesStyledRowsAlone verifies inline highlighting
// is confined to plain rows. Splicing a style into a row that already carries
// escape codes terminates the surrounding sequence and bleeds colour across the
// rest of the line.
func TestOverlaySearch_HighlightLeavesStyledRowsAlone(t *testing.T) {
	hit := lipgloss.NewStyle().Reverse(true)

	plain := highlightMatches("find the needle here", "needle", hit)
	if plain == "find the needle here" {
		t.Error("plain row was not highlighted")
	}

	styled := "\x1b[38;2;255;0;0mfind the needle here\x1b[m"
	if got := highlightMatches(styled, "needle", hit); got != styled {
		t.Error("styled row was modified; highlighting must skip rows with escape codes")
	}
}

// TestOverlaySearch_OfferedOnlyWhenUseful verifies the search hint appears only
// for a body that does not fit on screen. Searching what is already fully
// visible finds nothing the reader cannot see.
func TestOverlaySearch_OfferedOnlyWhenUseful(t *testing.T) {
	short := readerDialog("one\ntwo\n", 80, 30)
	short.Render()
	if short.searchable() {
		t.Error("search offered for a body that fits on screen")
	}
	if strings.Contains(xansi.Strip(short.Render()), "/ find") {
		t.Error("search hint shown for a body that fits on screen")
	}

	long := readerDialog(longBody(200), 80, 30)
	long.Render()
	if !long.searchable() {
		t.Error("search not offered for a body that scrolls")
	}
}

// --------------------------------------------------------------------------
// Role tinting
// --------------------------------------------------------------------------

// TestOverlayFrameColor_PrefersExplicitThenRole pins the precedence: an
// extension's chosen border colour wins, then the inspector's role tint, then
// the generic dialog amber.
func TestOverlayFrameColor_PrefersExplicitThenRole(t *testing.T) {
	theme := style.GetTheme()

	plain := newOverlayDialog("T", "b", false, "", "", 0, 0, "center", nil, 80, 24)
	if got := plain.frameColor(theme); got != theme.Info {
		t.Errorf("default frame colour = %v, want theme.Info", got)
	}

	tinted := newOverlayDialog("T", "b", false, "", "", 0, 0, "center", nil, 80, 24)
	tinted.accent = theme.Error
	if got := tinted.frameColor(theme); got != theme.Error {
		t.Errorf("role frame colour = %v, want theme.Error", got)
	}

	explicit := newOverlayDialog("T", "b", false, "#00ff00", "", 0, 0, "center", nil, 80, 24)
	explicit.accent = theme.Error
	if got := explicit.frameColor(theme); got != lipgloss.Color("#00ff00") {
		t.Errorf("explicit frame colour = %v, want the caller's #00ff00", got)
	}
}

// TestOverlayTitle_DropsMetaRatherThanWrapping guards the height budget: the
// title occupies exactly one row, so an annotation that will not share it is
// dropped rather than pushed onto a second line.
func TestOverlayTitle_DropsMetaRatherThanWrapping(t *testing.T) {
	theme := style.GetTheme()

	o := newOverlayDialog("Tool Result", "body", false, "", "", 0, 0, "center", nil, 80, 24)
	o.icon = "▸"
	o.meta = "Grep · 4000 lines"

	wide := o.renderTitle(60, theme.Info, theme)
	if lipgloss.Height(wide) != 1 {
		t.Errorf("wide title occupies %d rows, want 1", lipgloss.Height(wide))
	}
	if !strings.Contains(xansi.Strip(wide), o.meta) {
		t.Error("meta dropped from a row with room for it")
	}

	narrow := o.renderTitle(18, theme.Info, theme)
	if lipgloss.Height(narrow) != 1 {
		t.Errorf("narrow title occupies %d rows, want 1", lipgloss.Height(narrow))
	}
	if strings.Contains(xansi.Strip(narrow), o.meta) {
		t.Error("meta kept on a row too narrow for it")
	}
	if !strings.Contains(xansi.Strip(narrow), "Tool Result") {
		t.Error("title itself was dropped")
	}
}
