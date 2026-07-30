package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/ui/style"
)

// --------------------------------------------------------------------------
// applySelectionBorder
// --------------------------------------------------------------------------

// TestApplySelectionBorder_Geometry verifies the framed result has exactly
// the geometry the ScrollList height cache and mouse hit-testing assume:
// two extra rows and a width that never exceeds the viewport.
func TestApplySelectionBorder_Geometry(t *testing.T) {
	const width = 20
	content := "hello\nworld"

	got := applySelectionBorder(content, width, nil, "", "")
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

	got := applySelectionBorder(content, width, nil, "", "")
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
	if got := applySelectionBorder(content, 3, nil, "", ""); got != content {
		t.Errorf("got %q, want passthrough %q", got, content)
	}
}

// TestApplySelectionBorder_EdgeLabelsKeepGeometry verifies the frame's edge
// labels are spliced into the border rather than added to it: they must not
// change the row count or the width the height cache depends on.
func TestApplySelectionBorder_EdgeLabelsKeepGeometry(t *testing.T) {
	const width = 40
	content := "hello\nworld"

	bare := applySelectionBorder(content, width, nil, "", "")
	labelled := applySelectionBorder(content, width, nil, "Tool Result · 3/9", "enter to open")

	if got, want := lipgloss.Height(labelled), lipgloss.Height(bare); got != want {
		t.Errorf("labelled height = %d, want %d", got, want)
	}
	for i, line := range strings.Split(labelled, "\n") {
		if w := xansi.StringWidth(line); w != width {
			t.Errorf("row %d width = %d, want %d (%q)", i, w, width, xansi.Strip(line))
		}
	}

	lines := strings.Split(xansi.Strip(labelled), "\n")
	if !strings.Contains(lines[0], "Tool Result · 3/9") {
		t.Errorf("top edge missing the label: %q", lines[0])
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, "enter to open") {
		t.Errorf("bottom edge missing the hint: %q", last)
	}
}

// TestApplySelectionBorder_DropsUnfittableLabel verifies a label too wide for
// the edge is dropped whole rather than truncated to a fragment, and that the
// frame stays exactly as wide either way.
func TestApplySelectionBorder_DropsUnfittableLabel(t *testing.T) {
	const width = 14
	label := "An Extremely Long Role Name"

	got := applySelectionBorder("body", width, nil, label, "")
	for i, line := range strings.Split(got, "\n") {
		if w := xansi.StringWidth(line); w != width {
			t.Errorf("row %d width = %d, want %d", i, w, width)
		}
	}
	if strings.Contains(xansi.Strip(got), "Extremely") {
		t.Error("an unfittable label must be dropped, not clipped into the edge")
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

// TestInspectorSourceText_PrefersRawOverTruncatedRender is the core guarantee
// of the feature: the inspector must show the full source text, not the
// truncated rendering the scrollback displays.
func TestInspectorSourceText_PrefersRawOverTruncatedRender(t *testing.T) {
	raw := strings.Repeat("full output line\n", 50)
	item := NewStyledMessageItem("id", "tool", raw, "truncated ... (truncated)")

	if content := inspectorSourceText(item, 80); content != raw {
		t.Errorf("content = %q, want the raw text", content)
	}

	pres := presentationForRole(itemRole(item), style.GetTheme())
	if pres.title != "Tool Result" {
		t.Errorf("title = %q, want %q", pres.title, "Tool Result")
	}
	if pres.markdown {
		t.Error("tool output must not be rendered as markdown (it would mangle diffs and logs)")
	}
}

// TestInspectorSourceText_FallsBackToRender verifies items with no raw
// content still display something rather than an empty dialog.
func TestInspectorSourceText_FallsBackToRender(t *testing.T) {
	item := NewStyledMessageItem("id", "assistant", "", "rendered body")

	if content := inspectorSourceText(item, 80); content != "rendered body" {
		t.Errorf("content = %q, want the rendered fallback", content)
	}
}

// TestPresentationForRole covers the role→title/markdown mapping, and pins the
// rule that only authored prose is reflowed as markdown.
func TestPresentationForRole(t *testing.T) {
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
		{"", "Message", false},
	}

	theme := style.GetTheme()
	for _, tt := range tests {
		pres := presentationForRole(tt.role, theme)
		if pres.title != tt.wantTitle {
			t.Errorf("role %q: title = %q, want %q", tt.role, pres.title, tt.wantTitle)
		}
		if pres.markdown != tt.wantMD {
			t.Errorf("role %q: markdown = %v, want %v", tt.role, pres.markdown, tt.wantMD)
		}
		if pres.icon == "" {
			t.Errorf("role %q: no icon", tt.role)
		}
		// The title row right-aligns its meta against a width measured with
		// lipgloss, so a multi-cell glyph would shift the alignment.
		if w := lipgloss.Width(pres.icon); w != 1 {
			t.Errorf("role %q: icon %q is %d cells wide, want 1", tt.role, pres.icon, w)
		}
	}
}

// TestToolRawContent_IncludesFullResult verifies tool raw content carries the
// complete result plus the call metadata needed to interpret it.
func TestToolRawContent_IncludesFullResult(t *testing.T) {
	result := strings.Repeat("line\n", 100)
	got := toolRawContent("read", `{"path":"main.go"}`, result, false)

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
	got := toolRawContent("bash", "{}", "boom", true)
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
	for line := range strings.SplitSeq(o.Render(), "\n") {
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

	// A position indicator is shown; its exact form depends on how much room
	// the footer has ("(1–20 of 200 lines)" or the compact "20/200").
	first := xansi.Strip(o.Render())
	if !strings.Contains(first, "of 200 lines") && !strings.Contains(first, "/200") {
		t.Errorf("expected a scroll position indicator, got:\n%s", first)
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

// TestOverlayDialog_FooterIsInsideTheBox pins the footer's placement. Hints
// rendered below the border would be a bare band of padding that the
// compositor draws as opaque cells, cutting a blank strip through whatever
// sits behind the dialog.
func TestOverlayDialog_FooterIsInsideTheBox(t *testing.T) {
	body := make([]string, 200)
	for i := range body {
		body[i] = "line"
	}
	o := newOverlayDialog("Tool Result", strings.Join(body, "\n"), false, "", "", 0, 0, "center", nil, 100, 30)
	o.dismissOnly = true

	lines := strings.Split(o.Render(), "\n")
	last := xansi.Strip(lines[len(lines)-1])
	if !strings.HasPrefix(last, "╰") {
		t.Errorf("box must end with its bottom border, got %q", last)
	}

	// The hint text sits on a row framed by the border.
	var found bool
	for _, line := range lines {
		plain := xansi.Strip(line)
		if strings.Contains(plain, "close") {
			found = true
			if !strings.HasPrefix(plain, "│") || !strings.HasSuffix(plain, "│") {
				t.Errorf("hint row is not enclosed by the border: %q", plain)
			}
		}
	}
	if !found {
		t.Error("no hint row rendered")
	}
}

// TestOverlayDialog_RespectsMaxHeight guards the footer's height accounting:
// the row is part of the box, so the chrome budget must include it.
func TestOverlayDialog_RespectsMaxHeight(t *testing.T) {
	body := make([]string, 500)
	for i := range body {
		body[i] = "line"
	}

	for _, termH := range []int{12, 20, 30, 50} {
		o := newOverlayDialog("T", strings.Join(body, "\n"), false, "", "", 0, 0, "center", nil, 100, termH)
		got := lipgloss.Height(o.Render())
		if want := termH * 80 / 100; got > want {
			t.Errorf("termH=%d: box height %d exceeds the %d budget", termH, got, want)
		}
		if got > termH {
			t.Errorf("termH=%d: box height %d exceeds the terminal", termH, got)
		}
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

// --------------------------------------------------------------------------
// Tool argument display
// --------------------------------------------------------------------------

// TestToolRawContent_KeepsFullBashCommand is the argument-side counterpart to
// the result-side truncation guarantee: the scrollback header elides a long
// command with "...", so the inspector must carry the whole thing.
func TestToolRawContent_KeepsFullBashCommand(t *testing.T) {
	cmd := `cd /home/space_cowboy/Workspace/kit && btca ask -r https://github.com/owainlewis/neo ` +
		`-q "Show the exact workflow tool definition and the internal/workflow package: ` +
		`the Store/state model, item statuses, how transitions are validated"`

	args, err := json.Marshal(map[string]string{"command": cmd})
	if err != nil {
		t.Fatal(err)
	}

	// The header summary is truncated...
	if got := formatToolParams(string(args), 80); !strings.HasSuffix(got, "...") {
		t.Fatalf("expected a truncated header summary, got %q", got)
	}

	// ...but the inspector keeps the command verbatim.
	raw := toolRawContent("bash", string(args), "output", false)
	if !strings.Contains(raw, cmd) {
		t.Errorf("full command missing from inspector content:\n%s", raw)
	}
}

// TestFormatToolArgsForInspector_StringsStayCopyPasteable verifies string
// arguments are not JSON-escaped. Encoding them would turn every quote into
// \" and every newline into \n, defeating the purpose of showing the command.
func TestFormatToolArgsForInspector_StringsStayCopyPasteable(t *testing.T) {
	args := `{"command":"echo \"hello world\" | grep hello"}`

	got := formatToolArgsForInspector(args)
	want := `command: echo "hello world" | grep hello`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if strings.Contains(got, `\"`) {
		t.Error("quotes must not be escaped")
	}
}

// TestFormatToolArgsForInspector_MultilineAndStructured covers the two
// remaining shapes: multi-line strings start on their own line, and
// non-string values fall back to indented JSON.
func TestFormatToolArgsForInspector_MultilineAndStructured(t *testing.T) {
	got := formatToolArgsForInspector(`{"content":"line one\nline two"}`)
	if !strings.HasPrefix(got, "content:\n") || !strings.Contains(got, "line two") {
		t.Errorf("multi-line value not laid out on its own line: %q", got)
	}

	got = formatToolArgsForInspector(`{"limit":50,"paths":["a","b"]}`)
	if !strings.Contains(got, "limit: 50") {
		t.Errorf("numeric value missing: %q", got)
	}
	if !strings.Contains(got, `"a"`) || !strings.Contains(got, `"b"`) {
		t.Errorf("array value missing: %q", got)
	}
}

// TestFormatToolArgsForInspector_NonObjectPassthrough verifies a non-JSON
// argument payload is shown as-is rather than silently dropped.
func TestFormatToolArgsForInspector_NonObjectPassthrough(t *testing.T) {
	if got := formatToolArgsForInspector("not json"); got != "not json" {
		t.Errorf("got %q, want passthrough", got)
	}
}

// --------------------------------------------------------------------------
// Tab handling
// --------------------------------------------------------------------------

// TestApplySelectionBorder_ExpandsTabs guards border alignment for
// tab-indented content. Width calculations count a tab as one cell but the
// terminal advances to the next tab stop, so an unexpanded tab pushes the
// right border past the frame — visible on any selected grep result.
func TestApplySelectionBorder_ExpandsTabs(t *testing.T) {
	const width = 40
	content := "plain line\n\tindented\ncol\tsep\tvals"

	got := applySelectionBorder(content, width, nil, "", "")
	for i, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "\t") {
			t.Errorf("row %d still contains a raw tab: %q", i, line)
		}
		if w := xansi.StringWidth(line); w != width {
			t.Errorf("row %d width = %d, want %d", i, w, width)
		}
	}
}

// TestExpandTabs_AdvancesToTabStops verifies tabs align to stops rather than
// expanding to a fixed run of spaces, matching terminal behaviour.
func TestExpandTabs_AdvancesToTabStops(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a\tb", "a       b"},                // col 1 -> pad 7 -> col 8
		{"\tx", "        x"},                 // col 0 -> pad 8
		{"1234567\tz", "1234567 z"},          // col 7 -> pad 1 -> col 8
		{"12345678\tz", "12345678        z"}, // col 8 -> pad 8 -> col 16
		{"no tabs", "no tabs"},
	}
	for _, tt := range tests {
		if got := expandTabs(tt.in, 8); got != tt.want {
			t.Errorf("expandTabs(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestOverlayDialog_ExpandsTabsInBody verifies the dialog body carries no raw
// tabs, which would otherwise wrap at a width the measurement did not predict.
func TestOverlayDialog_ExpandsTabsInBody(t *testing.T) {
	body := strings.Repeat("path/file.go:12:\tfunc Thing() error {\n", 40)
	o := newOverlayDialog("Tool Result", body, false, "", "", 0, 0, "center", nil, 100, 30)

	out := o.Render()
	if strings.Contains(out, "\t") {
		t.Error("dialog body contains raw tabs; wrapping measurements will not match the render")
	}
	boxW := lipgloss.Width(out)
	for i, line := range strings.Split(out, "\n") {
		if w := xansi.StringWidth(line); w != boxW {
			t.Errorf("row %d width %d != box width %d", i, w, boxW)
		}
	}
}

// TestScrollList_ClampedSelectionHeightInvalidated guards the scroll maths
// after a list shrink. The item that ends up under the clamped selection may
// have a cached height measured without the selection border; leaving it
// stale throws offsets off by selectionBorderOverhead until it next renders.
func TestScrollList_ClampedSelectionHeightInvalidated(t *testing.T) {
	sl := NewScrollList(40, 20)
	items := makeItems(10, 3)
	sl.SetItems(items)

	// Measure item 2 unselected so its height is cached without a border.
	base := sl.itemHeight(2)
	sl.SetSelectedIndex(9)

	// Shrink so the selection clamps onto the already-cached item 2.
	sl.SetItems(items[:3])
	if got := sl.SelectedIndex(); got != 2 {
		t.Fatalf("selected index = %d, want 2 after clamp", got)
	}

	if got := sl.itemHeight(2); got != base+selectionBorderOverhead {
		t.Errorf("clamped selection height = %d, want %d (stale cache not invalidated)",
			got, base+selectionBorderOverhead)
	}
}

// --------------------------------------------------------------------------
// Navigation vs. a running agent
// --------------------------------------------------------------------------

// navTestModel returns a model with a populated scrollback, ready to enter
// message navigation.
func navTestModel(t *testing.T) *AppModel {
	t.Helper()
	m, _, _ := newTestAppModel(&stubAppController{})
	m.printSystemMessage("first")
	m.printSystemMessage("second")
	return m
}

// TestMessageNav_ExitRestoresWorkingState is the regression guard for the
// activity row vanishing for the rest of a turn: navigation used to return
// unconditionally to stateInput, so browsing the scrollback mid-turn
// permanently convinced the UI the agent had stopped.
func TestMessageNav_ExitRestoresWorkingState(t *testing.T) {
	m := navTestModel(t)
	m.state = stateWorking

	m.enterMessageNav()
	if m.state != stateMessageNav {
		t.Fatalf("state = %v, want stateMessageNav", m.state)
	}
	if !m.agentWorking() {
		t.Fatal("agentWorking() = false while navigating during a live turn")
	}

	m.exitMessageNav()
	if m.state != stateWorking {
		t.Fatalf("state after exit = %v, want stateWorking", m.state)
	}
}

// TestMessageNav_ExitFromIdleReturnsToInput verifies the ordinary path is
// unchanged: navigating while the agent is idle still lands in stateInput.
func TestMessageNav_ExitFromIdleReturnsToInput(t *testing.T) {
	m := navTestModel(t)

	m.enterMessageNav()
	m.exitMessageNav()

	if m.state != stateInput {
		t.Fatalf("state after exit = %v, want stateInput", m.state)
	}
	if m.agentWorking() {
		t.Fatal("agentWorking() = true after an idle navigation session")
	}
}

// TestMessageNav_ActivityRowSurvivesNavigation verifies the liveness row keeps
// rendering while the user browses: the agent is still running, and an empty
// row reads as a stalled agent.
func TestMessageNav_ActivityRowSurvivesNavigation(t *testing.T) {
	m := navTestModel(t)
	m.state = stateWorking
	m.turnStartedAt = time.Now()

	working := m.renderActivityRow()
	if strings.TrimSpace(working) == "" {
		t.Fatal("activity row empty while working")
	}

	m.enterMessageNav()
	if got := m.renderActivityRow(); strings.TrimSpace(got) == "" {
		t.Fatal("activity row empty while navigating during a live turn")
	}
}

// TestMessageNav_TurnEndWhileNavigating verifies that a turn finishing in the
// background neither tears navigation down nor is forgotten: the user stays in
// the scrollback, and exiting lands in the idle state.
func TestMessageNav_TurnEndWhileNavigating(t *testing.T) {
	m := navTestModel(t)
	m.state = stateWorking
	m.enterMessageNav()

	m = sendMsg(m, app.StepCompleteEvent{ResponseText: "done"})

	if m.state != stateMessageNav {
		t.Fatalf("state = %v, want stateMessageNav (navigation torn down by StepComplete)", m.state)
	}
	if m.agentWorking() {
		t.Fatal("agentWorking() = true after the turn completed")
	}

	m.exitMessageNav()
	if m.state != stateInput {
		t.Fatalf("state after exit = %v, want stateInput", m.state)
	}
}

// TestMessageNav_TurnStartWhileNavigating verifies the mirror case: a turn
// starting in the background (queued message, extension-triggered run) leaves
// navigation in place, and exiting reveals the working state.
func TestMessageNav_TurnStartWhileNavigating(t *testing.T) {
	m := navTestModel(t)
	m.enterMessageNav()

	m = sendMsg(m, app.SpinnerEvent{Show: true})

	if m.state != stateMessageNav {
		t.Fatalf("state = %v, want stateMessageNav (navigation torn down by SpinnerEvent)", m.state)
	}
	if !m.agentWorking() {
		t.Fatal("agentWorking() = false after a turn started in the background")
	}

	m.exitMessageNav()
	if m.state != stateWorking {
		t.Fatalf("state after exit = %v, want stateWorking", m.state)
	}
}

// TestMessageNav_InspectorKeepsWorkingState covers the inspector overlay
// opened from navigation: the dialog is a detour inside the browse, so the
// suspended working state has to survive both hops (nav → inspector → nav →
// input) or the activity row disappears for the rest of the turn.
func TestMessageNav_InspectorKeepsWorkingState(t *testing.T) {
	m := navTestModel(t)
	m.state = stateWorking
	m.enterMessageNav()
	m.inspectSelectedMessage()

	if m.state != stateOverlay {
		t.Fatalf("state = %v, want stateOverlay after inspect", m.state)
	}
	if !m.agentWorking() {
		t.Fatal("agentWorking() = false with the inspector open during a live turn")
	}
	if strings.TrimSpace(m.renderActivityRow()) == "" {
		t.Fatal("activity row empty with the inspector open during a live turn")
	}

	// Dismiss the inspector: it restores navigation, which still owes the
	// caller the working state.
	m.resolveOverlay(app.OverlayResponse{Cancelled: true})
	if m.state != stateMessageNav {
		t.Fatalf("state after dismiss = %v, want stateMessageNav", m.state)
	}

	m.exitMessageNav()
	if m.state != stateWorking {
		t.Fatalf("state after exit = %v, want stateWorking", m.state)
	}
}

// --------------------------------------------------------------------------
// Selection frame labels
// --------------------------------------------------------------------------

// TestMessageNav_SelectionFrameIsLabelled verifies the frame names what is
// selected and where it sits in the run of messages. Navigation is a transient
// mode, and a bare highlight says nothing about what it has landed on.
func TestMessageNav_SelectionFrameIsLabelled(t *testing.T) {
	m := navTestModel(t)
	m.refreshContent()
	m.enterMessageNav()

	label := m.scrollList.selectionLabel
	if label == "" {
		t.Fatal("no selection label set on entering navigation")
	}
	if !strings.Contains(label, "System") {
		t.Errorf("label %q does not name the role", label)
	}
	if !strings.Contains(label, "/") {
		t.Errorf("label %q carries no position counter", label)
	}
	if m.scrollList.selectionHint != selectionFrameHint {
		t.Errorf("hint = %q, want %q", m.scrollList.selectionHint, selectionFrameHint)
	}

	// The label reaches the painted frame.
	if !strings.Contains(xansi.Strip(m.scrollList.View()), label) {
		t.Error("selection label absent from the rendered frame")
	}

	// Leaving navigation clears it, so no stale label survives into a
	// selection-free scrollback.
	m.exitMessageNav()
	if m.scrollList.selectionLabel != "" || m.scrollList.selectionHint != "" {
		t.Error("frame labels outlived navigation")
	}
}

// TestMessageNav_SelectablePositionCountsOnlySelectable verifies the counter
// numbers the messages the cursor can actually land on. Counting empty items
// too would make it skip numbers as the user moves.
func TestMessageNav_SelectablePositionCountsOnlySelectable(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.scrollList.SetItems([]MessageItem{
		&fakeItem{id: "a", lines: 2},
		&fakeItem{id: "blank", lines: 0},
		&fakeItem{id: "b", lines: 2},
	})

	pos, total := m.selectablePosition(2)
	if total != 2 {
		t.Errorf("total = %d, want 2 (the empty item must not be counted)", total)
	}
	if pos != 2 {
		t.Errorf("pos = %d, want 2", pos)
	}
}

// --------------------------------------------------------------------------
// Role jumping and copy
// --------------------------------------------------------------------------

// navRoleModel builds a scrollback alternating user turns and tool results.
func navRoleModel(t *testing.T) *AppModel {
	t.Helper()
	m, _, _ := newTestAppModel(&stubAppController{})
	for i := range 3 {
		m.messages = append(m.messages,
			NewStyledMessageItem(fmt.Sprintf("u%d", i), "user",
				fmt.Sprintf("question %d", i), fmt.Sprintf("question %d", i)),
			NewStyledMessageItem(fmt.Sprintf("t%d", i), "tool",
				fmt.Sprintf("result %d", i), fmt.Sprintf("result %d", i)),
		)
	}
	m.refreshContent()
	return m
}

// TestMessageNav_JumpToRole verifies u/U step between the user's own turns.
// "Scroll back to what I asked" is the dominant reason to open navigation, and
// stepping there one tool result at a time is a lot of keypresses.
func TestMessageNav_JumpToRole(t *testing.T) {
	m := navRoleModel(t)
	m.enterMessageNav()

	// Starts on the last item (a tool result at index 5).
	if got := m.scrollList.SelectedIndex(); got != 5 {
		t.Fatalf("initial selection = %d, want 5", got)
	}

	m.jumpToRole("user", -1)
	if got := m.scrollList.SelectedIndex(); got != 4 {
		t.Errorf("after one jump back: selection = %d, want 4", got)
	}
	m.jumpToRole("user", -1)
	if got := m.scrollList.SelectedIndex(); got != 2 {
		t.Errorf("after two jumps back: selection = %d, want 2", got)
	}

	m.jumpToRole("user", 1)
	if got := m.scrollList.SelectedIndex(); got != 4 {
		t.Errorf("after jumping forward: selection = %d, want 4", got)
	}

	// Running out of user turns leaves the selection put rather than
	// snapping to an end.
	m.selectMessage(0)
	m.jumpToRole("user", -1)
	if got := m.scrollList.SelectedIndex(); got != 0 {
		t.Errorf("selection = %d, want 0 (no earlier user turn to jump to)", got)
	}
}

// TestMessageNav_CopySelected verifies the copy binding takes the message's
// source text and reports the result without touching the scrollback.
//
// Printing an acknowledgement as a message would append an item to the very
// list being navigated, shifting the position counter and possibly the
// selection itself.
func TestMessageNav_CopySelected(t *testing.T) {
	m := navRoleModel(t)
	before := m.scrollList.Len()
	m.enterMessageNav()

	if cmd := m.copySelectedMessage(); cmd == nil {
		t.Fatal("copy produced no clipboard command")
	}
	if m.navNotice == "" {
		t.Error("copy was not acknowledged")
	}
	if m.scrollList.Len() != before {
		t.Errorf("scrollback grew from %d to %d items; the notice must not be a message",
			before, m.scrollList.Len())
	}
	if !strings.Contains(xansi.Strip(m.renderStatusBar()), "Copied") {
		t.Error("status bar does not carry the copy acknowledgement")
	}

	// Leaving navigation retires the notice.
	m.exitMessageNav()
	if m.navNotice != "" {
		t.Error("notice outlived navigation")
	}
}

// --------------------------------------------------------------------------
// Rich inspector bodies
// --------------------------------------------------------------------------

// TestInspector_RendersToolBodyUncapped is the point of the whole detour: the
// inspector opens because the transcript elided something, so it must re-render
// the tool's own body with the caps lifted rather than fall back to plain text
// and discard the panels and gutters.
func TestInspector_RendersToolBodyUncapped(t *testing.T) {
	const lines = 120
	var result strings.Builder
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&result, "internal/ui/file.go:%d:match\n", i)
	}

	info := ToolCallInfo{
		Name:   "grep",
		Args:   `{"pattern":"match"}`,
		Result: result.String(),
	}

	// The transcript caps this at maxLsLines.
	capped := renderToolBodyLimited(info.Name, info.Args, info.Result, 90, scrollbackLimits())
	if got := lipgloss.Height(capped); got > maxLsLines+2 {
		t.Fatalf("transcript body is %d rows; expected it to be capped near %d", got, maxLsLines)
	}

	full := renderToolInspectorBody(info, 90)
	if got := lipgloss.Height(full); got < lines {
		t.Errorf("inspector body is %d rows, want at least %d (caps not lifted)", got, lines)
	}
	// Still the rich rendering, not raw text: the panel fill is an escape code.
	if !strings.Contains(full, "\x1b[") {
		t.Error("inspector body lost its styling")
	}
	// And the arguments the transcript header elides are present in full.
	if !strings.Contains(xansi.Strip(full), `pattern: match`) {
		t.Error("inspector body omits the tool arguments")
	}
}

// TestInspector_ToolBodyFitsTheGivenWidth guards the geometry contract between
// the dialog and the tool renderers: the dialog wraps to exactly the width it
// asked for, so a body that exceeds it has every row split in two.
func TestInspector_ToolBodyFitsTheGivenWidth(t *testing.T) {
	// Tabs are the specific hazard: renderers measure one as a single cell,
	// but the terminal advances it to the next stop.
	result := strings.Repeat("path/to/file.go:12:\tfunc Thing() error {\n", 30)

	for _, tool := range []string{"grep", "ls", "find", "bash", "read"} {
		info := ToolCallInfo{Name: tool, Args: `{"path":"x.go","pattern":"p","command":"c"}`, Result: result}
		for _, width := range []int{40, 72, 120} {
			body := renderToolInspectorBody(info, width)
			if strings.Contains(body, "\t") {
				t.Errorf("%s@%d: body still contains a raw tab", tool, width)
			}
			for i, line := range strings.Split(body, "\n") {
				if w := xansi.StringWidth(line); w > width {
					t.Errorf("%s@%d: row %d is %d columns wide (%q)",
						tool, width, i, w, xansi.Strip(line))
				}
			}
		}
	}
}

// TestInspector_ErrorsShowVerbatim verifies a failed call shows its whole
// result rather than a styled excerpt, and is presented as an error.
func TestInspector_ErrorsShowVerbatim(t *testing.T) {
	result := "FAIL\n" + strings.Repeat("stack frame\n", 50)
	info := ToolCallInfo{Name: "bash", Args: `{"command":"go test"}`, Result: result, IsError: true}

	body := xansi.Strip(renderToolInspectorBody(info, 80))
	if strings.Count(body, "stack frame") != 50 {
		t.Errorf("error body holds %d of 50 stack frames", strings.Count(body, "stack frame"))
	}
}

// TestInspector_FailedToolIsTintedAsAnError verifies a failed call takes the
// error frame colour. The reader opened it to find out what went wrong, and the
// frame is the fastest way to answer that.
func TestInspector_FailedToolIsTintedAsAnError(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	theme := style.GetTheme()

	for _, isErr := range []bool{false, true} {
		raw := toolRawContent("bash", `{"command":"go test"}`, "output", isErr)
		m.messages = []MessageItem{
			NewStyledMessageItem("t", "tool", raw, raw).WithToolCall(ToolCallInfo{
				Name: "bash", Args: `{"command":"go test"}`, Result: "output", IsError: isErr,
			}),
		}
		m.refreshContent()
		m.enterMessageNav()
		m.inspectSelectedMessage()

		if m.overlay == nil {
			t.Fatal("inspector did not open")
		}
		want := theme.Tool
		if isErr {
			want = theme.Error
		}
		if got := m.overlay.frameColor(theme); got != want {
			t.Errorf("isError=%v: frame colour = %v, want %v", isErr, got, want)
		}
		if isErr && !strings.Contains(m.overlay.meta, "error") {
			t.Errorf("failed call not marked in the meta: %q", m.overlay.meta)
		}
		m.resolveOverlay(app.OverlayResponse{Cancelled: true})
		m.exitMessageNav()
	}
}

// TestInspector_ConfiguresReaderAffordances pins the inspector's setup. Each of
// these is the difference between a reader and a generic dialog, and each was
// missing at some point.
func TestInspector_ConfiguresReaderAffordances(t *testing.T) {
	m := navRoleModel(t)
	m.enterMessageNav()
	m.inspectSelectedMessage()

	o := m.overlay
	if o == nil {
		t.Fatal("inspector did not open")
	}
	if !o.dismissOnly {
		t.Error("inspector is read-only; Enter and Esc must not imply different outcomes")
	}
	if !o.scrollbar {
		t.Error("no scrollbar")
	}
	if !o.wrapMarks {
		t.Error("no continuation markers")
	}
	if !o.lineNumbers {
		t.Error("no line-number gutter")
	}
	if o.copyText == "" {
		t.Error("nothing to copy")
	}
	if o.icon == "" {
		t.Error("no role glyph")
	}
	if o.accent == nil {
		t.Error("no role tint")
	}
}
