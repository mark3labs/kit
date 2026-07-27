package ui

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/app"
)

// appToolResult builds a ToolResultEvent for the raw-content tests.
func appToolResult(name, args, result string, isErr bool) app.ToolResultEvent {
	return app.ToolResultEvent{
		ToolName: name,
		ToolArgs: args,
		Result:   result,
		IsError:  isErr,
	}
}

// --------------------------------------------------------------------------
// applySelectionBorder
// --------------------------------------------------------------------------

// TestApplySelectionBorder_Geometry verifies the framed result has exactly
// the geometry the ScrollList height cache and mouse hit-testing assume:
// two extra rows and a width that never exceeds the viewport.
func TestApplySelectionBorder_Geometry(t *testing.T) {
	const width = 20
	content := "hello\nworld"

	got := applySelectionBorder(content, width, nil)
	lines := strings.Split(got, "\n")

	wantLines := 2 + selectionBorderOverhead
	if len(lines) != wantLines {
		t.Fatalf("got %d lines, want %d\n%q", len(lines), wantLines, got)
	}

	for i, line := range lines {
		if w := xansi.StringWidth(line); w != width {
			t.Errorf("line %d width = %d, want %d (%q)", i, w, width, line)
		}
	}
}

// TestApplySelectionBorder_TruncatesOverWideLines ensures a line wider than
// the frame is clipped rather than pushing the right edge out of alignment,
// which would corrupt the fixed-width layout.
func TestApplySelectionBorder_TruncatesOverWideLines(t *testing.T) {
	const width = 12
	content := strings.Repeat("x", 40)

	got := applySelectionBorder(content, width, nil)
	for i, line := range strings.Split(got, "\n") {
		if w := xansi.StringWidth(line); w != width {
			t.Errorf("line %d width = %d, want %d", i, w, width)
		}
	}
}

// TestApplySelectionBorder_NarrowWidthPassthrough verifies that a width with
// no room for a frame returns the content untouched instead of producing
// negative-width padding.
func TestApplySelectionBorder_NarrowWidthPassthrough(t *testing.T) {
	content := "abc"
	if got := applySelectionBorder(content, 3, nil); got != content {
		t.Errorf("got %q, want passthrough %q", got, content)
	}
}

// --------------------------------------------------------------------------
// ScrollList selection integration
// --------------------------------------------------------------------------

// TestScrollList_SelectionChangesItemHeight verifies the height cache tracks
// the selection border. If the cached height went stale, the scrollback
// layout and mouse hit-testing would drift by two rows.
func TestScrollList_SelectionChangesItemHeight(t *testing.T) {
	sl := NewScrollList(40, 30)
	sl.SetItems(makeItems(5, 3))

	base := sl.itemHeight(2)
	if base != 3 {
		t.Fatalf("unselected height = %d, want 3", base)
	}

	sl.SetSelectedIndex(2)
	if got := sl.itemHeight(2); got != base+selectionBorderOverhead {
		t.Errorf("selected height = %d, want %d", got, base+selectionBorderOverhead)
	}

	// Neighbours must be unaffected.
	if got := sl.itemHeight(1); got != base {
		t.Errorf("neighbour height = %d, want %d", got, base)
	}

	// Clearing the selection restores the original height.
	sl.SetSelectedIndex(-1)
	if got := sl.itemHeight(2); got != base {
		t.Errorf("height after clearing = %d, want %d", got, base)
	}
}

// TestScrollList_ViewRendersSelectionBorder verifies the border actually
// reaches the painted output.
func TestScrollList_ViewRendersSelectionBorder(t *testing.T) {
	sl := NewScrollList(40, 30)
	sl.SetItems(makeItems(3, 2))

	if strings.Contains(sl.View(), selBorderTopLeft) {
		t.Fatal("unselected view should not contain a selection border")
	}

	sl.SetSelectedIndex(1)
	view := sl.View()
	for _, glyph := range []string{selBorderTopLeft, selBorderTopRight, selBorderBottomLeft, selBorderBottomRight} {
		if !strings.Contains(view, glyph) {
			t.Errorf("view missing border glyph %q", glyph)
		}
	}
}

// TestScrollList_ViewHeightStableWithSelection guards the invariant that
// View() always emits exactly `height` lines, so the composer and footer
// stay pinned regardless of the selection border.
func TestScrollList_ViewHeightStableWithSelection(t *testing.T) {
	const height = 12
	sl := NewScrollList(40, height)
	sl.SetItems(makeItems(10, 3))

	for _, sel := range []int{-1, 0, 5, 9} {
		sl.SetSelectedIndex(sel)
		got := len(strings.Split(sl.View(), "\n"))
		if got != height {
			t.Errorf("selected=%d: View() produced %d lines, want %d", sel, got, height)
		}
	}
}

// TestScrollList_EmptyItemNotFramed verifies zero-height items stay
// zero-height when selected. Framing empty content would draw a box around
// nothing and desynchronise height accounting with View().
func TestScrollList_EmptyItemNotFramed(t *testing.T) {
	sl := NewScrollList(40, 20)
	sl.SetItems([]MessageItem{
		&fakeItem{id: "empty", lines: 0},
		&fakeItem{id: "full", lines: 2},
	})

	sl.SetSelectedIndex(0)
	if got := sl.renderItem(0); got != "" {
		t.Errorf("empty item rendered %q, want empty string", got)
	}
	if got := sl.itemHeight(0); got != 0 {
		t.Errorf("empty item height = %d, want 0", got)
	}
}

// TestScrollList_SelectionClampedOnShrink verifies a selection past the end
// of a shrunken list (e.g. after /clear) is pulled back into range instead
// of naming a message that no longer exists.
func TestScrollList_SelectionClampedOnShrink(t *testing.T) {
	sl := NewScrollList(40, 20)
	sl.SetItems(makeItems(10, 2))
	sl.SetSelectedIndex(9)

	sl.SetItems(makeItems(3, 2))
	if got := sl.SelectedIndex(); got >= 3 {
		t.Errorf("selected index = %d, want < 3 after shrink", got)
	}

	// Rendering must stay safe with the clamped selection.
	if lines := len(strings.Split(sl.View(), "\n")); lines != 20 {
		t.Errorf("View() produced %d lines, want 20", lines)
	}

	// Emptying the list entirely must not leave a positive index.
	sl.SetItems(nil)
	if got := sl.SelectedIndex(); got >= 0 {
		t.Errorf("selected index = %d, want negative for an empty list", got)
	}
}

// TestScrollList_EnsureVisible checks that navigating to an off-screen item
// scrolls it into view from either direction.
func TestScrollList_EnsureVisible(t *testing.T) {
	sl := NewScrollList(40, 10)
	sl.SetItems(makeItems(20, 3)) // 60 lines into a 10-line viewport

	// Target above the viewport.
	sl.EnsureVisible(0)
	if sl.offsetIdx != 0 || sl.offsetLine != 0 {
		t.Errorf("after EnsureVisible(0): offset = (%d,%d), want (0,0)", sl.offsetIdx, sl.offsetLine)
	}

	// Target below the viewport must become visible.
	sl.EnsureVisible(15)
	if !isItemVisible(sl, 15) {
		t.Errorf("item 15 not visible after EnsureVisible; offset = (%d,%d)", sl.offsetIdx, sl.offsetLine)
	}
}

// isItemVisible reports whether any part of item idx is inside the viewport.
func isItemVisible(sl *ScrollList, idx int) bool {
	if idx < sl.offsetIdx {
		return false
	}
	y := -sl.offsetLine
	for i := sl.offsetIdx; i < idx; i++ {
		y += sl.itemHeight(i)
	}
	return y < sl.height
}

// --------------------------------------------------------------------------
// Inspector content resolution
// --------------------------------------------------------------------------

// TestMessageInspectorContent_PrefersRawOverTruncatedRender is the core
// guarantee of the feature: the inspector must show the full source text,
// not the truncated rendering the scrollback displays.
func TestMessageInspectorContent_PrefersRawOverTruncatedRender(t *testing.T) {
	raw := strings.Repeat("full output line\n", 50)
	item := NewStyledMessageItem("id", "tool", raw, "truncated ... (truncated)")

	title, content, markdown := messageInspectorContent(item, 80)

	if content != raw {
		t.Errorf("content = %q, want the raw text", content)
	}
	if title != "Tool Result" {
		t.Errorf("title = %q, want %q", title, "Tool Result")
	}
	if markdown {
		t.Error("tool output must not be rendered as markdown (it would mangle diffs and logs)")
	}
}

// TestMessageInspectorContent_FallsBackToRender verifies items with no raw
// content still display something rather than an empty dialog.
func TestMessageInspectorContent_FallsBackToRender(t *testing.T) {
	item := NewStyledMessageItem("id", "assistant", "", "rendered body")

	_, content, _ := messageInspectorContent(item, 80)
	if content != "rendered body" {
		t.Errorf("content = %q, want the rendered fallback", content)
	}
}

// TestMessageInspectorContent_Titles covers the role→title/markdown mapping.
func TestMessageInspectorContent_Titles(t *testing.T) {
	tests := []struct {
		role      string
		wantTitle string
		wantMD    bool
	}{
		{"user", "You", true},
		{"assistant", "Assistant", true},
		{"tool", "Tool Result", false},
		{"error", "Error", false},
		{"system", "System", false},
		{"reasoning", "Reasoning", false},
		{"custom", "Custom", false},
	}

	for _, tt := range tests {
		item := NewStyledMessageItem("id", tt.role, "body", "styled")
		title, _, md := messageInspectorContent(item, 80)
		if title != tt.wantTitle {
			t.Errorf("role %q: title = %q, want %q", tt.role, title, tt.wantTitle)
		}
		if md != tt.wantMD {
			t.Errorf("role %q: markdown = %v, want %v", tt.role, md, tt.wantMD)
		}
	}
}

// TestToolRawContent_IncludesFullResult verifies tool raw content carries the
// complete result plus the call metadata needed to interpret it.
func TestToolRawContent_IncludesFullResult(t *testing.T) {
	result := strings.Repeat("line\n", 100)
	got := toolRawContent(appToolResult("read", `{"path":"main.go"}`, result, false))

	if !strings.Contains(got, "read") {
		t.Error("missing tool name")
	}
	if !strings.Contains(got, "main.go") {
		t.Error("missing tool arguments")
	}
	if !strings.Contains(got, strings.TrimRight(result, "\n")) {
		t.Error("missing full result body")
	}
}

// TestToolRawContent_MarksErrors verifies error results are labelled.
func TestToolRawContent_MarksErrors(t *testing.T) {
	got := toolRawContent(appToolResult("bash", "{}", "boom", true))
	if !strings.Contains(got, "(error)") {
		t.Errorf("error result not marked: %q", got)
	}
}

// --------------------------------------------------------------------------
// Overlay dialog geometry (the inspector's rendering surface)
// --------------------------------------------------------------------------

// TestOverlayDialog_TitleSeparatorFitsOnOneLine guards the dialog width math.
// lipgloss's Width() includes the border, so treating it as border-exclusive
// made the content area two columns narrower than innerWidth and wrapped the
// title separator (and action bar) onto a stray extra line.
func TestOverlayDialog_TitleSeparatorFitsOnOneLine(t *testing.T) {
	o := newOverlayDialog("Tool Result", "body text", false, "", "", 0, 0, "center", nil, 100, 30)

	var seen int
	for _, line := range strings.Split(o.Render(), "\n") {
		plain := strings.TrimSpace(xansi.Strip(line))
		// Separator rows contain only box-drawing dashes between the frame.
		inner := strings.Trim(plain, "│ ")
		if inner != "" && strings.Trim(inner, "─") == "" {
			seen++
			if len(inner) < 10 {
				t.Errorf("separator fragment %q wrapped onto its own line", inner)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no title separator rendered")
	}
}

// TestOverlayDialog_ScrollsLongContent verifies the inspector can page
// through content that exceeds the dialog height — the whole point of
// opening a truncated message.
func TestOverlayDialog_ScrollsLongContent(t *testing.T) {
	body := make([]string, 200)
	for i := range body {
		body[i] = "line"
	}
	o := newOverlayDialog("Tool Result", strings.Join(body, "\n"), false, "", "", 0, 0, "center", nil, 100, 30)

	first := o.Render()
	if !strings.Contains(xansi.Strip(first), "of 200 lines") {
		t.Error("expected a scroll indicator for content taller than the dialog")
	}

	// Scroll to the end; the offset must advance and stay clamped.
	o.scrollOff = o.totalLines
	o.Render()
	if o.scrollOff == 0 {
		t.Error("scroll offset did not advance")
	}
	if o.scrollOff >= o.totalLines {
		t.Errorf("scroll offset %d not clamped below total %d", o.scrollOff, o.totalLines)
	}
}

// TestOverlayDialog_DismissOnlyHint verifies the inspector advertises a
// single close action, while extension overlays keep the dismiss/cancel
// wording that matches the distinct results they report.
func TestOverlayDialog_DismissOnlyHint(t *testing.T) {
	ext := newOverlayDialog("T", "body", false, "", "", 0, 0, "center", nil, 100, 30)
	if got := xansi.Strip(ext.Render()); !strings.Contains(got, "Enter dismiss") ||
		!strings.Contains(got, "Esc cancel") {
		t.Error("extension overlay should keep the dismiss/cancel hints")
	}

	insp := newOverlayDialog("T", "body", false, "", "", 0, 0, "center", nil, 100, 30)
	insp.dismissOnly = true
	got := xansi.Strip(insp.Render())
	if !strings.Contains(got, "Enter/Esc close") {
		t.Errorf("inspector should show a single close hint, got %q", got)
	}
	if strings.Contains(got, "Esc cancel") {
		t.Error("inspector must not imply Esc cancels something")
	}
}
