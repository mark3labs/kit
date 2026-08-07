package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/pkg/extensions/test"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func submonLoad(t *testing.T) *test.Harness {
	t.Helper()
	h := test.New(t)
	h.LoadFile("./subagent-monitor.go")
	if _, err := h.Emit(extensions.SessionStartEvent{SessionID: "test-session"}); err != nil {
		t.Fatalf("SessionStart should not error: %v", err)
	}
	return h
}

func submonStart(t *testing.T, h *test.Harness, callID, task string) {
	t.Helper()
	if _, err := h.Emit(extensions.SubagentStartEvent{ToolCallID: callID, Task: task}); err != nil {
		t.Fatalf("SubagentStart(%s): %v", callID, err)
	}
}

func submonChunk(t *testing.T, h *test.Harness, e extensions.SubagentChunkEvent) {
	t.Helper()
	if _, err := h.Emit(e); err != nil {
		t.Fatalf("SubagentChunk(%s): %v", e.ToolCallID, err)
	}
}

func submonEnd(t *testing.T, h *test.Harness, callID, task, response, errMsg string) {
	t.Helper()
	ev := extensions.SubagentEndEvent{
		ToolCallID: callID, Task: task, Response: response, ErrorMsg: errMsg,
	}
	if _, err := h.Emit(ev); err != nil {
		t.Fatalf("SubagentEnd(%s): %v", callID, err)
	}
}

// submonWidget returns the monitor widget, failing if it was never installed.
func submonWidget(t *testing.T, h *test.Harness) extensions.WidgetConfig {
	t.Helper()
	w, ok := h.Context().GetWidget("submon")
	if !ok {
		t.Fatal("widget \"submon\" was never set")
	}
	return w
}

// submonDraw invokes the widget's Render callback across the Yaegi boundary.
func submonDraw(t *testing.T, h *test.Harness, width int) string {
	t.Helper()
	w := submonWidget(t, h)
	if w.Content.Render == nil {
		t.Fatal("widget has no Render callback; expected the new render path")
	}
	return w.Content.Render(width)
}

// submonWidgetGone reports whether the strip has been removed from the UI.
func submonWidgetGone(h *test.Harness) bool {
	return !h.Context().HasWidget("submon")
}

// submonOpen fires the registered shortcut, the same path the TUI uses. The
// mock resolves every overlay as cancelled, so this returns once the dialog
// would have been dismissed.
func submonOpen(t *testing.T, h *test.Harness) {
	t.Helper()
	handlers := h.Runner().GetShortcutHandlers()
	fn, ok := handlers["ctrl+alt+s"]
	if !ok {
		t.Fatalf("shortcut ctrl+alt+s not registered; have %v", handlers)
	}
	fn()
}

// submonLastOverlay returns the most recent overlay, failing if none was shown.
func submonLastOverlay(t *testing.T, h *test.Harness) extensions.OverlayConfig {
	t.Helper()
	all := h.Context().Overlays
	if len(all) == 0 {
		t.Fatal("no overlay was shown")
	}
	return all[len(all)-1]
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// TestSubagentMonitor_SessionStart verifies OnSessionStart initializes state
// without panicking and properly guards nil ctx calls.
func TestSubagentMonitor_SessionStart(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("./subagent-monitor.go")

	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test-session"})
	if err != nil {
		t.Fatalf("SessionStart should not error: %v", err)
	}
}

// TestSubagentMonitor_SubagentLifecycle verifies the full subagent lifecycle
// creates entries and emits widget updates.
func TestSubagentMonitor_SubagentLifecycle(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "test task")

	for i := range 3 {
		submonChunk(t, h, extensions.SubagentChunkEvent{
			ToolCallID: "call-1", ChunkType: "text",
			Content: fmt.Sprintf("line %d\n", i),
		})
	}
	submonChunk(t, h, extensions.SubagentChunkEvent{
		ToolCallID: "call-1", ChunkType: "tool_call", ToolName: "bash",
	})
	submonEnd(t, h, "call-1", "test task", "done", "")
}

// TestSubagentMonitor_MultipleSubagents verifies multiple parallel subagents.
func TestSubagentMonitor_MultipleSubagents(t *testing.T) {
	h := submonLoad(t)

	for i := 1; i <= 3; i++ {
		submonStart(t, h, fmt.Sprintf("call-%d", i), fmt.Sprintf("task %d", i))
	}
	for i := 1; i <= 3; i++ {
		submonChunk(t, h, extensions.SubagentChunkEvent{
			ToolCallID: fmt.Sprintf("call-%d", i), ChunkType: "text",
			Content: fmt.Sprintf("output from agent %d\n", i),
		})
	}
	out := submonDraw(t, h, 120)
	for i := 1; i <= 3; i++ {
		if !strings.Contains(out, fmt.Sprintf("#%d", i)) {
			t.Errorf("strip is missing subagent #%d:\n%s", i, out)
		}
	}

	for i := 1; i <= 3; i++ {
		submonEnd(t, h, fmt.Sprintf("call-%d", i), fmt.Sprintf("task %d", i), "completed", "")
	}
	if !submonWidgetGone(h) {
		t.Error("strip survived the last subagent finishing")
	}
}

// TestSubagentMonitor_ConcurrentSubagents verifies no panics or data races when
// subagents emit events concurrently while the TUI renders.
func TestSubagentMonitor_ConcurrentSubagents(t *testing.T) {
	h := submonLoad(t)

	// A renderer racing the event handlers, which is exactly the production
	// arrangement: Render runs on the TUI goroutine, handlers on Kit's
	// dispatcher.
	stopRender := make(chan struct{})
	renderDone := make(chan struct{})
	go func() {
		defer close(renderDone)
		for {
			select {
			case <-stopRender:
				return
			default:
				if w, ok := h.Context().GetWidget("submon"); ok && w.Content.Render != nil {
					_ = w.Content.Render(100)
				}
			}
		}
	}()

	done := make(chan struct{}, 5)
	for i := range 5 {
		go func(idx int) {
			defer func() { done <- struct{}{} }()

			callID := fmt.Sprintf("concurrent-%d", idx)
			_, _ = h.Emit(extensions.SubagentStartEvent{
				ToolCallID: callID, Task: fmt.Sprintf("concurrent task %d", idx),
			})
			for j := range 20 {
				_, _ = h.Emit(extensions.SubagentChunkEvent{
					ToolCallID: callID, ChunkType: "text",
					Content: fmt.Sprintf("agent %d chunk %d\n", idx, j),
				})
			}
			_, _ = h.Emit(extensions.SubagentEndEvent{ToolCallID: callID, Response: "done"})
		}(i)
	}
	for range 5 {
		<-done
	}
	close(stopRender)
	<-renderDone
}

// TestSubagentMonitor_SessionShutdown verifies shutdown doesn't panic even with
// nil ctx functions.
func TestSubagentMonitor_SessionShutdown(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "test task")

	if _, err := h.Emit(extensions.SessionShutdownEvent{}); err != nil {
		t.Fatalf("SessionShutdown should not error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Newer UI features
// ---------------------------------------------------------------------------

// TestSubagentMonitor_UsesRenderCallback verifies the widget draws through
// Content.Render rather than the older static Text field, so the layout can
// reflow with the terminal width.
func TestSubagentMonitor_UsesRenderCallback(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "map the session flow")

	w := submonWidget(t, h)
	if w.Content.Render == nil {
		t.Fatal("widget should render through Content.Render")
	}
	if w.Content.Text != "" {
		t.Errorf("Content.Text should be unused, got %q", w.Content.Text)
	}

	narrow := w.Content.Render(40)
	wide := w.Content.Render(160)
	if narrow == wide {
		t.Error("render output should reflow with width")
	}
	for line := range strings.SplitSeq(wide, "\n") {
		if n := len([]rune(stripANSI(line))); n > 160 {
			t.Errorf("line exceeds the given width (%d > 160): %q", n, line)
		}
	}
}

// submonRenderedHeight reports the height Kit will actually paint.
//
// Kit boxes widget content and DROPS trailing empty lines in the process, so
// counting newlines in the raw string overstates the height of a widget whose
// last rows are blank. This mirrors that trimming so the test measures what
// the terminal shows.
func submonRenderedHeight(s string) int {
	if s == "" {
		return 0
	}
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return len(lines)
}

// TestSubagentMonitor_StripHeightIsConstant verifies the widget paints the same
// number of rows regardless of how many subagents are running or how much
// output they have produced.
//
// Kit measures widget height only when a widget update arrives. A strip that
// changed height between updates would be painted against a stale budget and
// push the status bar off the bottom of the screen — which is exactly what
// happened when blank tail rows were emitted as empty strings and silently
// trimmed away.
func TestSubagentMonitor_StripHeightIsConstant(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "first task")

	// Baseline: a fresh subagent with no output yet, so every transcript row
	// is blank. This is the case that used to collapse.
	want := submonRenderedHeight(submonDraw(t, h, 120))
	if want == 0 {
		t.Fatal("strip rendered nothing for a running subagent")
	}

	for i := range 30 {
		submonChunk(t, h, extensions.SubagentChunkEvent{
			ToolCallID: "call-1", ChunkType: "text",
			Content: fmt.Sprintf("noisy line %d\n", i),
		})
	}
	if got := submonRenderedHeight(submonDraw(t, h, 120)); got != want {
		t.Errorf("height changed as output arrived: %d, want %d", got, want)
	}

	for i := 2; i <= 6; i++ {
		submonStart(t, h, fmt.Sprintf("call-%d", i), fmt.Sprintf("task %d", i))
		if got := submonRenderedHeight(submonDraw(t, h, 120)); got != want {
			t.Errorf("height changed with %d subagents: %d, want %d", i, got, want)
		}
	}

	// Narrow enough that the columns must be windowed.
	if got := submonRenderedHeight(submonDraw(t, h, 60)); got != want {
		t.Errorf("height changed when windowed: %d, want %d", got, want)
	}

	// A finished agent leaving the strip must not change the height either.
	submonEnd(t, h, "call-1", "first task", "done", "")
	if got := submonRenderedHeight(submonDraw(t, h, 120)); got != want {
		t.Errorf("height changed when a column left: %d, want %d", got, want)
	}
}

// TestSubagentMonitor_RefreshHzOnlyWhileRunning verifies the frame clock is
// held open for the spinner and elapsed timer, and released when the work
// stops. An always-on RefreshHz would stop Kit ever idling.
func TestSubagentMonitor_RefreshHzOnlyWhileRunning(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "task")

	if hz := submonWidget(t, h).Content.RefreshHz; hz <= 0 {
		t.Errorf("RefreshHz = %d while running, want > 0", hz)
	}

	// The widget is removed outright when the last agent finishes, which
	// releases the clock more completely than RefreshHz=0 would.
	submonEnd(t, h, "call-1", "task", "done", "")
	if !submonWidgetGone(h) {
		t.Errorf("widget survived completion with RefreshHz = %d",
			submonWidget(t, h).Content.RefreshHz)
	}
}

// ---------------------------------------------------------------------------
// Retention
// ---------------------------------------------------------------------------

// TestSubagentMonitor_StripClearsWhenWorkFinishes verifies the strip vanishes
// as soon as the last subagent finishes, rather than holding screen space for
// work that is already done.
func TestSubagentMonitor_StripClearsWhenWorkFinishes(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "first")
	submonStart(t, h, "call-2", "second")

	// One finishing leaves the strip up for the other, minus its column.
	submonEnd(t, h, "call-1", "first", "done", "")
	out := submonDraw(t, h, 120)
	if strings.Contains(out, "first") {
		t.Errorf("finished subagent kept its column:\n%s", out)
	}
	if !strings.Contains(out, "second") {
		t.Errorf("running subagent lost its column:\n%s", out)
	}
	if !strings.Contains(out, "1 done") {
		t.Errorf("header should point at the retained history:\n%s", out)
	}

	// The last one finishing removes the widget entirely.
	submonEnd(t, h, "call-2", "second", "done", "")
	if !submonWidgetGone(h) {
		t.Errorf("strip survived the last subagent finishing:\n%s", submonDraw(t, h, 120))
	}
}

// TestSubagentMonitor_FinishedTranscriptsSurviveInTheModal verifies removing a
// column does not discard its output: the transcript is still reachable after
// the strip is gone.
func TestSubagentMonitor_FinishedTranscriptsSurviveInTheModal(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "summarise the tests")
	submonChunk(t, h, extensions.SubagentChunkEvent{
		ToolCallID: "call-1", ChunkType: "text", Content: "counted the tests\n",
	})
	submonEnd(t, h, "call-1", "summarise the tests", "42 tests pass", "")

	if !submonWidgetGone(h) {
		t.Fatal("expected the strip to be gone before checking the modal")
	}

	submonOpen(t, h)
	ov := submonLastOverlay(t, h)
	body := stripANSI(ov.Content.Text)
	for _, want := range []string{"summarise the tests", "counted the tests", "42 tests pass"} {
		if !strings.Contains(body, want) {
			t.Errorf("transcript lost %q after the strip cleared:\n%s", want, body)
		}
	}
}

// TestSubagentMonitor_NewTurnClearsFinished verifies the strip does not grow
// without bound: the next agent turn clears completed entries.
func TestSubagentMonitor_NewTurnClearsFinished(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "old task")
	submonEnd(t, h, "call-1", "old task", "done", "")

	if _, err := h.Emit(extensions.AgentStartEvent{Prompt: "next question"}); err != nil {
		t.Fatalf("AgentStart: %v", err)
	}

	if _, ok := h.Context().GetWidget("submon"); ok {
		if out := submonDraw(t, h, 100); strings.Contains(out, "old task") {
			t.Errorf("finished entry survived the new turn:\n%s", out)
		}
	}
}

// TestSubagentMonitor_RunningSurvivesNewTurn verifies pruning never drops a
// subagent that is still in flight.
func TestSubagentMonitor_RunningSurvivesNewTurn(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "long running task")

	if _, err := h.Emit(extensions.AgentStartEvent{Prompt: "next"}); err != nil {
		t.Fatalf("AgentStart: %v", err)
	}

	if out := submonDraw(t, h, 100); !strings.Contains(out, "long running task") {
		t.Errorf("running subagent was pruned:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Keybindings
// ---------------------------------------------------------------------------

// TestSubagentMonitor_ShortcutIsSafe verifies the only global binding neither
// is reserved by Kit nor shadows a built-in.
func TestSubagentMonitor_ShortcutIsSafe(t *testing.T) {
	h := submonLoad(t)

	shortcuts := h.Runner().GetShortcuts()
	if len(shortcuts) != 1 {
		t.Fatalf("expected exactly one global shortcut, got %d: %v", len(shortcuts), shortcuts)
	}
	for key := range shortcuts {
		normalized, warning, err := extensions.ValidateShortcutKey(key)
		if err != nil {
			t.Errorf("shortcut %q is unusable: %v", key, err)
		}
		if warning != "" {
			t.Errorf("shortcut %q shadows a built-in: %s", key, warning)
		}
		if normalized != key {
			t.Errorf("shortcut %q is not stored normalized (want %q)", key, normalized)
		}
	}
	if _, ok := shortcuts["ctrl+alt+s"]; !ok {
		t.Errorf("expected ctrl+alt+s, got %v", shortcuts)
	}
}

// TestSubagentMonitor_NeverInterceptsTheEditor verifies the extension never
// installs an editor interceptor.
//
// The modal is a real overlay with its own key handling, so no key needs to be
// intercepted. An interceptor is the one mechanism that could swallow a
// keystroke Kit expects, which makes its absence worth asserting.
func TestSubagentMonitor_NeverInterceptsTheEditor(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "task")
	submonOpen(t, h)
	submonEnd(t, h, "call-1", "task", "done", "")
	submonOpen(t, h)

	if h.Context().EditorConfig != nil {
		t.Error("extension installed an editor interceptor; it should rely on the overlay")
	}
}

// TestSubagentMonitor_CommandRegistered verifies the keyboard-free entry point.
func TestSubagentMonitor_CommandRegistered(t *testing.T) {
	h := submonLoad(t)
	for _, c := range h.RegisteredCommands() {
		if c.Name == "subagents" {
			return
		}
	}
	t.Errorf("/subagents command not registered; have %v", h.RegisteredCommands())
}

// ---------------------------------------------------------------------------
// The modal
// ---------------------------------------------------------------------------

// TestSubagentMonitor_OpensRealOverlay verifies the transcript is shown through
// ctx.ShowOverlay — a composited modal — rather than being drawn into a widget.
func TestSubagentMonitor_OpensRealOverlay(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "inspect the tree")
	submonEnd(t, h, "call-1", "inspect the tree", "found it", "")

	submonOpen(t, h)

	ov := submonLastOverlay(t, h)
	if !strings.Contains(ov.Title, "#1") {
		t.Errorf("overlay title should identify the subagent, got %q", ov.Title)
	}
	if ov.Content.Text == "" {
		t.Error("overlay has no content")
	}
	if len(ov.Actions) == 0 {
		t.Error("overlay should offer at least a close action")
	}
}

// TestSubagentMonitor_OverlayShowsFullOutput verifies the modal contains the
// whole transcript, including lines long since scrolled off the strip.
func TestSubagentMonitor_OverlayShowsFullOutput(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "inspect the tree")

	for i := range 12 {
		submonChunk(t, h, extensions.SubagentChunkEvent{
			ToolCallID: "call-1", ChunkType: "text",
			Content: fmt.Sprintf("transcript line %02d\n", i),
		})
	}
	submonChunk(t, h, extensions.SubagentChunkEvent{
		ToolCallID: "call-1", ChunkType: "tool_call",
		ToolName: "grep", ToolArgs: `{"pattern":"TreeManager"}`,
	})

	// The strip only has room for the last few lines while it is up.
	if strip := submonDraw(t, h, 100); strings.Contains(strip, "transcript line 00") {
		t.Error("strip should show only the tail, not the whole transcript")
	}

	submonEnd(t, h, "call-1", "inspect the tree", "found it", "")
	submonOpen(t, h)
	body := stripANSI(submonLastOverlay(t, h).Content.Text)

	for _, want := range []string{
		"transcript line 00",  // the head, unreachable from the strip
		"transcript line 11",  // the tail
		"grep", "TreeManager", // tool calls with their arguments
		"found it", // the final result
	} {
		if !strings.Contains(body, want) {
			t.Errorf("overlay body missing %q:\n%s", want, body)
		}
	}
}

// TestSubagentMonitor_OverlayDegradesWithoutATheme verifies the transcript
// stays readable when no theme is available.
//
// Colors come from ctx.GetTheme(), and ThemeColors.ANSI returns text unchanged
// for an empty color rather than emitting a malformed escape. A headless host
// reports no theme at all, so this is the path that must not leak escape codes
// into the dialog.
func TestSubagentMonitor_OverlayDegradesWithoutATheme(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "task")
	submonChunk(t, h, extensions.SubagentChunkEvent{
		ToolCallID: "call-1", ChunkType: "tool_result",
		ToolName: "bash", IsError: true, ToolResult: "exit status 1",
	})
	submonOpen(t, h)

	body := submonLastOverlay(t, h).Content.Text
	if strings.Contains(body, "\033[") {
		t.Errorf("unset theme leaked escape codes into the dialog:\n%q", body)
	}
	if !strings.Contains(body, "exit status 1") {
		t.Errorf("body lost its content:\n%s", body)
	}
}

// TestSubagentMonitor_OverlayNavigationActions verifies the action bar is the
// navigation mechanism, and that it is bounded at both ends of the list.
func TestSubagentMonitor_OverlayNavigationActions(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "first")
	submonStart(t, h, "call-2", "second")
	submonOpen(t, h)

	actions := submonLastOverlay(t, h).Actions
	joined := strings.Join(actions, "|")
	if !strings.Contains(joined, "Next") {
		t.Errorf("expected a Next action with 2 subagents, got %v", actions)
	}
	if strings.Contains(joined, "Previous") {
		t.Errorf("first subagent should not offer Previous, got %v", actions)
	}
	if !strings.Contains(joined, "Close") {
		t.Errorf("expected a Close action, got %v", actions)
	}
}

// TestSubagentMonitor_OverlayRefreshOnlyWhileRunning verifies the snapshot can
// be re-read while an agent is still working, and that the action disappears
// once there is nothing left to update.
func TestSubagentMonitor_OverlayRefreshOnlyWhileRunning(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "task")

	submonOpen(t, h)
	if !strings.Contains(strings.Join(submonLastOverlay(t, h).Actions, "|"), "Refresh") {
		t.Error("a running subagent should offer Refresh")
	}

	submonEnd(t, h, "call-1", "task", "done", "")

	submonOpen(t, h)
	if strings.Contains(strings.Join(submonLastOverlay(t, h).Actions, "|"), "Refresh") {
		t.Error("a finished subagent should not offer Refresh")
	}
}

// TestSubagentMonitor_OverlaySingleSubagentHasNoNavigation verifies the action
// bar stays minimal when there is nowhere to navigate.
func TestSubagentMonitor_OverlaySingleSubagentHasNoNavigation(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "only task")
	submonEnd(t, h, "call-1", "only task", "done", "")
	submonOpen(t, h)

	if got := submonLastOverlay(t, h).Actions; len(got) != 1 || got[0] != "Close" {
		t.Errorf("expected only a Close action for a single subagent, got %v", got)
	}
}

// TestSubagentMonitor_FailedSubagent verifies an error is surfaced distinctly.
func TestSubagentMonitor_FailedSubagent(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "risky task")
	submonEnd(t, h, "call-1", "risky task", "", "timeout after 5m")
	submonOpen(t, h)

	ov := submonLastOverlay(t, h)
	if !strings.Contains(ov.Title, "failed") {
		t.Errorf("title should report the failed status, got %q", ov.Title)
	}
	if !strings.Contains(stripANSI(ov.Content.Text), "timeout after 5m") {
		t.Errorf("body should show the error message:\n%s", ov.Content.Text)
	}
}

// TestSubagentMonitor_OpenWithNoSubagents verifies the entry points are inert
// when there is nothing to show, rather than raising an empty dialog.
func TestSubagentMonitor_OpenWithNoSubagents(t *testing.T) {
	h := submonLoad(t)
	submonOpen(t, h)

	if n := len(h.Context().Overlays); n != 0 {
		t.Errorf("raised %d overlays with no subagents, want 0", n)
	}
	infos := h.Context().GetPrintInfos()
	found := false
	for _, s := range infos {
		if strings.Contains(s, "No subagents") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an explanatory message, got %v", infos)
	}
}

// ---------------------------------------------------------------------------
// Agent name resolution
// ---------------------------------------------------------------------------

// TestSubagentMonitor_AgentNameBackfilled verifies the named agent is shown
// even though SubagentStart arrives BEFORE the ToolCall event that carries the
// agent argument. Relying on the documented-but-wrong order made every column
// read "default agent".
func TestSubagentMonitor_AgentNameBackfilled(t *testing.T) {
	h := submonLoad(t)

	// Live ordering: SubagentStart first, ToolCall second.
	submonStart(t, h, "call-1", "map the tree")
	if out := submonDraw(t, h, 100); !strings.Contains(out, "default agent") {
		t.Fatalf("expected the placeholder before the agent is known:\n%s", out)
	}

	if _, err := h.Emit(extensions.ToolCallEvent{
		ToolName:   "subagent",
		ToolCallID: "call-1",
		ParsedArgs: map[string]any{"agent": "explore"},
	}); err != nil {
		t.Fatalf("ToolCall: %v", err)
	}

	out := submonDraw(t, h, 100)
	if !strings.Contains(out, "explore") {
		t.Errorf("agent name was not backfilled onto the running entry:\n%s", out)
	}
	if strings.Contains(out, "default agent") {
		t.Errorf("placeholder still shown after the agent became known:\n%s", out)
	}
}

// TestSubagentMonitor_AgentNameEitherOrder verifies the opposite ordering also
// resolves, so the extension does not depend on which event wins.
func TestSubagentMonitor_AgentNameEitherOrder(t *testing.T) {
	h := submonLoad(t)

	if _, err := h.Emit(extensions.ToolCallEvent{
		ToolName:   "subagent",
		ToolCallID: "call-1",
		ParsedArgs: map[string]any{"agent": "code-reviewer"},
	}); err != nil {
		t.Fatalf("ToolCall: %v", err)
	}
	submonStart(t, h, "call-1", "review the diff")

	if out := submonDraw(t, h, 100); !strings.Contains(out, "code-reviewer") {
		t.Errorf("agent name lost when ToolCall arrives first:\n%s", out)
	}
}

// TestSubagentMonitor_ElapsedAdvances verifies the timer is computed on demand
// rather than frozen at the last event.
func TestSubagentMonitor_ElapsedAdvances(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "task")

	first := submonDraw(t, h, 100)
	time.Sleep(1100 * time.Millisecond)
	if second := submonDraw(t, h, 100); first == second {
		t.Error("elapsed time did not advance between frames")
	}
}

// TestSubagentMonitor_StripReturnsForNewWork verifies the widget is
// re-installed after having been removed, so a later turn is not left without
// a strip.
func TestSubagentMonitor_StripReturnsForNewWork(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "first")
	submonEnd(t, h, "call-1", "first", "done", "")
	if !submonWidgetGone(h) {
		t.Fatal("expected the strip to be removed after the first agent finished")
	}

	submonStart(t, h, "call-2", "second")
	out := submonDraw(t, h, 120)
	if !strings.Contains(out, "second") {
		t.Errorf("strip did not come back for the new subagent:\n%s", out)
	}
	if strings.Contains(out, "first") {
		t.Errorf("the finished agent reappeared on the strip:\n%s", out)
	}
}

// TestSubagentMonitor_ModalOpensOnRunningAgent verifies the dialog starts on
// work in progress rather than on an older finished entry, which is what the
// user is asking about when they open it mid-turn.
func TestSubagentMonitor_ModalOpensOnRunningAgent(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "already finished")
	submonEnd(t, h, "call-1", "already finished", "done", "")
	submonStart(t, h, "call-2", "still working")

	submonOpen(t, h)

	ov := submonLastOverlay(t, h)
	if !strings.Contains(ov.Title, "running") {
		t.Errorf("modal should open on the running agent, got title %q", ov.Title)
	}
	if !strings.Contains(stripANSI(ov.Content.Text), "still working") {
		t.Errorf("modal opened on the wrong subagent:\n%s", ov.Content.Text)
	}
}

// TestSubagentMonitor_HostileOutputKeepsHeight verifies raw tool output cannot
// change the strip's height.
//
// Column widths are computed in runes, so a tab — which occupies several
// terminal cells — makes the joined row wider than the widget, wrapping it and
// adding a line. Kit would not re-measure, and the status bar would be pushed
// off the bottom. Real tool output (source code, go test) is full of tabs.
func TestSubagentMonitor_HostileOutputKeepsHeight(t *testing.T) {
	h := submonLoad(t)
	submonStart(t, h, "call-1", "task")
	want := submonRenderedHeight(submonDraw(t, h, 120))

	for _, nasty := range []string{
		"\tif err != nil {\n",
		"\t\t\t\treturn fmt.Errorf(\"deeply indented\")\n",
		"col\tsep\tvalues\there\n",
		"bell \x07 and backspace \x08 and vertical tab \x0b\n",
		"a stray escape \x1b[31m sneaking in\n",
	} {
		submonChunk(t, h, extensions.SubagentChunkEvent{
			ToolCallID: "call-1", ChunkType: "text", Content: nasty,
		})
		for _, width := range []int{60, 100, 140} {
			out := submonDraw(t, h, width)
			if got := submonRenderedHeight(out); got != want {
				t.Errorf("width=%d: height %d, want %d after %q", width, got, want, nasty)
			}
			for line := range strings.SplitSeq(out, "\n") {
				if n := len([]rune(stripANSI(line))); n >= width {
					t.Errorf("width=%d: row is %d columns and would wrap: %q", width, n, line)
				}
				// A tab occupies up to 8 cells but counts as one rune, so any
				// tab that survives to the output invalidates the width
				// arithmetic above. The same goes for other C0 controls.
				for _, r := range stripANSI(line) {
					if r == '\t' || r < 0x20 || r == 0x7f {
						t.Errorf("width=%d: control char %q survived into the row: %q",
							width, r, line)
					}
				}
			}
		}
	}
}
