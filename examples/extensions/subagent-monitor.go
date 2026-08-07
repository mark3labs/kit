//go:build ignore

// subagent-monitor — live dashboard for spawned subagents, with a real modal
//
// Shows a horizontal strip of columns above the composer, one per subagent,
// each streaming that agent's real-time output. Press ctrl+alt+s (or run
// /subagents) to open the full transcript in a floating modal dialog.
//
// The modal is a genuine overlay: Kit composites it over the conversation
// rather than inserting it into the layout, and it brings its own scrolling,
// incremental search and action bar. That means this extension registers
// exactly ONE key — ctrl+alt+s — and intercepts nothing:
//
//	j k ↑ ↓          scroll             pgup pgdn   page
//	ctrl+u ctrl+d    half page          g G         top / bottom
//	/ n N            search             ← → tab     choose an action
//	enter            run the action     esc         close
//
// The action bar walks between subagents and re-reads a running agent's
// output, so everything is reachable without a custom keymap.
//
// The strip shows only the agents that are still working, and disappears
// entirely once the last one finishes — a column that outlived its agent would
// hold screen space for work that is already done. Finished transcripts are
// not lost: the modal keeps them for the rest of the turn, so ctrl+alt+s still
// opens them after the strip is gone. The next agent turn clears the history.
//
// Kit UI features used here:
//   - ctx.ShowOverlay draws the transcript as a composited modal. It blocks,
//     so it runs on the shortcut/command goroutine, never the TUI loop.
//     Overlay text keeps ANSI escapes, so the transcript stays colored.
//   - WidgetContent.Render(width) draws the strip per frame, so the columns
//     reflow with the terminal instead of using a fixed width.
//   - WidgetContent.RefreshHz holds Kit's frame clock open only while a
//     subagent is running, which is what moves the spinner and the elapsed
//     timer. It drops back to 0 when everything is idle, so a quiet session
//     still costs nothing.
//   - ctx.GetTheme() is read inside Render, so every color follows the user's
//     theme and a /theme switch repaints live.
//
// Install:
//
//	kit -e examples/extensions/subagent-monitor.go
//
// YAEGI NOTE: every helper is declared ABOVE the code that references it.
// A bare reference to a function declared later in the file silently yields
// zero values.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	ext "kit/ext"
)

// ---------------------------------------------------------------------------
// Tunables
// ---------------------------------------------------------------------------

const (
	submonWidgetID = "submon"

	submonMinCol   = 26 // narrowest a strip column may be before we drop one
	submonColGap   = 2  // blank columns between strip columns
	submonTailRows = 4  // transcript lines shown per strip column

	submonMaxTranscript = 4000 // per-subagent line cap (oldest dropped)
	submonMaxEntries    = 24   // total subagents retained for browsing

	submonRunningHz = 10 // frame clock rate while a subagent is running

	// Action labels. The result is matched on the label, never the index, so
	// a host that returns a zero-value result closes the dialog instead of
	// looping.
	submonActPrev    = "◀ Previous"
	submonActNext    = "Next ▶"
	submonActRefresh = "Refresh"
	submonActClose   = "Close"
)

// submonSpinner is the frame set for the running indicator.
var submonSpinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// submonLine is one transcript row. kind drives the color.
type submonLine struct {
	kind string // "text" | "tool" | "ok" | "err" | "meta"
	text string
}

type submonEntry struct {
	id      int
	callID  string
	agent   string // named agent (e.g. "explore"), "" for the default agent
	task    string
	started time.Time
	elapsed time.Duration
	status  string // "running" | "done" | "failed"

	lines   []submonLine
	textBuf string // partial line accumulator for streamed text

	tools  int
	errs   int
	result string // final response or error message
}

// ---------------------------------------------------------------------------
// Package state
//
// submonMu guards everything below. Event handlers write from Kit's dispatcher
// goroutine while Render reads from the TUI goroutine, so the lock is load
// bearing. Render only ever holds it long enough to snapshot.
// ---------------------------------------------------------------------------

var (
	submonMu      sync.Mutex
	submonEntries []*submonEntry
	submonNextID  int

	// submonAgentByCall records the named agent for each subagent tool call,
	// keyed by ToolCallID, for the case where OnToolCall wins the race.
	submonAgentByCall map[string]string

	// submonModalOpen guards against a second modal: ShowOverlay blocks, and
	// Kit rejects an overlay raised while one is already up.
	submonModalOpen bool

	submonCtx    ext.Context
	submonHasCtx bool

	// submonInstalledHz is the RefreshHz of the currently installed widget.
	// Tracked so the widget is only re-Set when something actually changed.
	submonInstalledHz = -1
	submonInstalledOn bool
	submonAccent      string
)

func submonReset() {
	submonEntries = nil
	submonNextID = 1
	submonAgentByCall = map[string]string{}
	submonModalOpen = false
	submonInstalledHz = -1
	submonInstalledOn = false
}

// ---------------------------------------------------------------------------
// String helpers (ANSI aware)
// ---------------------------------------------------------------------------

// submonRuneLen returns the printable width of s, ignoring ANSI escapes.
func submonRuneLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			// Truecolor escapes end at 'm'; that is the only form emitted here.
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

func submonSpaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

// submonPad right-pads s to w printable columns.
func submonPad(s string, w int) string {
	return s + submonSpaces(w-submonRuneLen(s))
}

// submonClean makes one line of arbitrary tool output safe to lay out in a
// fixed-width column.
//
// Column widths are computed in runes, so anything that occupies more terminal
// cells than it has runes breaks the arithmetic: the joined row overflows the
// widget, wraps, and silently adds a line — which changes the strip's height
// without a widget update for Kit to re-measure on. Tabs are the common case
// in real tool output (source code, go test output); other control characters
// would corrupt the row outright.
func submonClean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case r == '\n' || r == '\r':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f:
			// Drop other C0 controls, including stray ANSI introducers: the
			// column colors are applied here, so incoming escapes are noise.
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// submonTrunc shortens plain (unstyled) text to w columns with an ellipsis.
func submonTrunc(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return ""
	}
	return string(r[:w-1]) + "…"
}

func submonFmtDur(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 0 {
		secs = 0
	}
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	return fmt.Sprintf("%dm%02ds", secs/60, secs%60)
}

func submonClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// submonPlural renders a count with a naively pluralized noun.
func submonPlural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// ---------------------------------------------------------------------------
// Domain helpers
// ---------------------------------------------------------------------------

// submonAgentLabel returns the display title for a subagent.
func submonAgentLabel(agent string) string {
	if strings.TrimSpace(agent) == "" {
		return "default agent"
	}
	return agent
}

// submonArgSummary picks a readable one-liner out of a tool's JSON arguments,
// preferring the field that identifies what the tool acted on.
func submonArgSummary(argsJSON string) string {
	if strings.TrimSpace(argsJSON) == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return ""
	}
	for _, k := range []string{"command", "file_path", "path", "pattern", "query", "prompt", "url"} {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(strings.ReplaceAll(v, "\n", " "))
		}
	}
	return ""
}

// submonElapsed reports how long an entry has been running, frozen once it has
// finished. Computed on demand so the timer advances between events.
func submonElapsed(e *submonEntry) time.Duration {
	if e.elapsed > 0 {
		return e.elapsed
	}
	if e.started.IsZero() {
		return 0
	}
	return time.Since(e.started)
}

// submonFind returns the entry for a tool call id. Caller holds submonMu.
func submonFind(callID string) *submonEntry {
	for _, e := range submonEntries {
		if e.callID == callID {
			return e
		}
	}
	return nil
}

// submonPush appends a transcript line, enforcing the per-entry cap.
// Caller holds submonMu.
func submonPush(e *submonEntry, kind, text string) {
	text = submonClean(strings.TrimRight(text, "\r\n"))
	if strings.TrimSpace(text) == "" {
		return
	}
	e.lines = append(e.lines, submonLine{kind: kind, text: text})
	if n := len(e.lines); n > submonMaxTranscript {
		e.lines = e.lines[n-submonMaxTranscript:]
	}
}

// submonFeedText buffers streamed text and emits whole lines only, so the
// transcript reads as prose instead of one row per network chunk.
// Caller holds submonMu.
func submonFeedText(e *submonEntry, chunk string) {
	e.textBuf += chunk
	for {
		i := strings.IndexByte(e.textBuf, '\n')
		if i < 0 {
			break
		}
		submonPush(e, "text", e.textBuf[:i])
		e.textBuf = e.textBuf[i+1:]
	}
}

// submonFlushText commits any partial line. Caller holds submonMu.
func submonFlushText(e *submonEntry) {
	if strings.TrimSpace(e.textBuf) != "" {
		submonPush(e, "text", e.textBuf)
	}
	e.textBuf = ""
}

// submonRunningEntries returns the entries still in flight, in start order.
//
// The strip shows only these: a column that lingered after its agent finished
// would hold screen space for work that is already done. Finished entries stay
// in submonEntries so the modal can still show their transcript.
// Caller holds submonMu.
func submonRunningEntries() []*submonEntry {
	out := []*submonEntry{}
	for _, e := range submonEntries {
		if e.status == "running" {
			out = append(out, e)
		}
	}
	return out
}

// submonRunning counts entries still in flight. Caller holds submonMu.
func submonRunning() int {
	return len(submonRunningEntries())
}

// submonDesiredHz is the frame rate the widget needs right now: animation only
// while something is actually running. Caller holds submonMu.
func submonDesiredHz() int {
	if submonRunning() > 0 {
		return submonRunningHz
	}
	return 0
}

// submonKindColor maps a transcript line kind to a theme color.
func submonKindColor(th ext.ThemeColors, kind string) string {
	switch kind {
	case "tool":
		return th.Tool
	case "ok":
		return th.Success
	case "err":
		return th.Error
	case "meta":
		return th.VeryMuted
	}
	return th.Text
}

// submonStatusGlyph returns the leading indicator for an entry, animated for
// running agents using the shared frame clock's cadence.
func submonStatusGlyph(th ext.ThemeColors, e *submonEntry) string {
	switch e.status {
	case "done":
		return th.ANSI(th.Success, "✓")
	case "failed":
		return th.ANSI(th.Error, "✗")
	}
	// Derive the frame from wall clock so every column spins together.
	idx := int(time.Since(e.started)/(100*time.Millisecond)) % len(submonSpinner)
	if idx < 0 {
		idx = 0
	}
	return th.ANSI(th.Accent, submonSpinner[idx])
}

// ---------------------------------------------------------------------------
// Strip rendering
// ---------------------------------------------------------------------------

// submonRenderColumn draws one fixed-width column. Caller holds submonMu.
func submonRenderColumn(th ext.ThemeColors, e *submonEntry, w int) []string {
	rows := []string{}

	// ---- title: status glyph, agent name, elapsed -------------------------
	glyph := submonStatusGlyph(th, e)
	dur := submonFmtDur(submonElapsed(e))
	nameW := w - submonRuneLen(dur) - 4 // marker + glyph + two spaces
	label := th.ANSI(th.Text, submonTrunc(submonAgentLabel(e.agent), submonClamp(nameW, 1, w)))

	title := " " + glyph + " " + label
	title += submonSpaces(w-submonRuneLen(title)-submonRuneLen(dur)) + th.ANSI(th.VeryMuted, dur)
	rows = append(rows, submonPad(title, w))

	// ---- task -------------------------------------------------------------
	num := fmt.Sprintf("#%d ", e.id)
	task := submonTrunc(e.task, submonClamp(w-len(num)-2, 1, w))
	rows = append(rows, submonPad("  "+th.ANSI(th.VeryMuted, num)+th.ANSI(th.Muted, task), w))

	// ---- transcript tail --------------------------------------------------
	tail := e.lines
	if len(tail) > submonTailRows {
		tail = tail[len(tail)-submonTailRows:]
	}
	for _, l := range tail {
		body := submonTrunc(l.text, submonClamp(w-2, 1, w))
		rows = append(rows, submonPad("  "+th.ANSI(submonKindColor(th, l.kind), body), w))
	}
	for len(rows) < submonTailRows+2 {
		if len(rows) == 2 && len(e.lines) == 0 {
			rows = append(rows, submonPad("  "+th.ANSI(th.VeryMuted, "waiting…"), w))
		} else {
			rows = append(rows, submonSpaces(w))
		}
	}
	return rows
}

// submonRenderStrip draws the multi-column overview of the RUNNING subagents.
//
// Returns "" when nothing is running, which is what makes the strip disappear
// the moment the last agent finishes. Caller holds submonMu.
func submonRenderStrip(th ext.ThemeColors, width int) string {
	live := submonRunningEntries()
	if len(live) == 0 {
		return ""
	}
	if width < submonMinCol {
		width = submonMinCol
	}

	// How many columns fit, and how wide each one is. One column is held back
	// so a full-width row can never reach the edge: a row that exactly fills
	// the content width wraps, which would add a line and change the height.
	avail := width - 1
	cols := (avail + submonColGap) / (submonMinCol + submonColGap)
	cols = submonClamp(cols, 1, len(live))
	colW := (avail - submonColGap*(cols-1)) / cols

	// Newest first when they don't all fit: the agent that just started is
	// the one with something new to show.
	visible := live
	if cols < len(visible) {
		visible = visible[len(visible)-cols:]
	}

	// ---- header row -------------------------------------------------------
	// The finished count is kept as a pointer to the history the modal holds:
	// those columns are gone from the strip but their transcripts are not.
	done := len(submonEntries) - len(live)
	parts := []string{fmt.Sprintf("%d running", len(live))}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("%d done", done))
	}
	head := th.ANSIBold(th.Accent, "SUBAGENTS") + th.ANSI(th.VeryMuted, "  "+strings.Join(parts, " · "))

	hint := "ctrl+alt+s open"
	if cols < len(live) {
		hint = fmt.Sprintf("showing %d of %d · ", cols, len(live)) + hint
	}
	hint = th.ANSI(th.VeryMuted, submonTrunc(hint, width))
	// Leave one column spare: a row that exactly fills the content width
	// wraps, costing a blank line of scrollback.
	pad := width - submonRuneLen(head) - submonRuneLen(hint) - 1
	if pad < 1 {
		pad = 1
	}

	out := []string{head + submonSpaces(pad) + hint, ""}

	// ---- columns ----------------------------------------------------------
	grid := [][]string{}
	for _, e := range visible {
		grid = append(grid, submonRenderColumn(th, e, colW))
	}
	gap := submonSpaces(submonColGap)
	for row := 0; row < submonTailRows+2; row++ {
		line := ""
		for ci := range grid {
			if ci > 0 {
				line += gap
			}
			line += grid[ci][row]
		}
		// Trailing spaces are trimmed to keep the terminal clean, but a row
		// that empties out completely must not become "": Kit drops trailing
		// EMPTY lines when it boxes widget content, so a column with little
		// output yet would silently render shorter than the same widget once
		// it fills up. The height would then change without a widget update
		// to make Kit re-measure, and the status bar would be pushed off the
		// bottom. A single space keeps every row and the height constant.
		if line = strings.TrimRight(line, " "); line == "" {
			line = " "
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// ---------------------------------------------------------------------------
// Widget plumbing
// ---------------------------------------------------------------------------

// submonRender is the widget body, called by Kit on every frame. It must stay
// cheap: it snapshots under the lock and formats from local state only.
func submonRender(width int) string {
	submonMu.Lock()
	defer submonMu.Unlock()

	if !submonHasCtx {
		return ""
	}
	// Live read: a /theme switch repaints in the new colors on the next frame.
	return submonRenderStrip(submonCtx.GetTheme(), width)
}

// submonSyncWidget installs, updates or removes the widget to match state.
//
// The widget exists only while at least one subagent is running, so it is
// removed as soon as the last one finishes rather than lingering with stale
// columns. Their transcripts remain available through the modal.
//
// Caller must NOT hold submonMu.
func submonSyncWidget() {
	submonMu.Lock()
	has := submonHasCtx
	ctx := submonCtx
	show := submonRunning() > 0
	hz := submonDesiredHz()
	installedOn := submonInstalledOn
	installedHz := submonInstalledHz
	accent := submonAccent
	submonMu.Unlock()

	if !has || ctx.SetWidget == nil {
		return
	}

	if !show {
		if installedOn && ctx.RemoveWidget != nil {
			ctx.RemoveWidget(submonWidgetID)
		}
		submonMu.Lock()
		submonInstalledOn = false
		submonInstalledHz = -1
		submonMu.Unlock()
		return
	}

	// Style is captured at install time (unlike Content.Render, which runs per
	// frame), so re-Set when the rate or the theme accent changes. The strip
	// is a constant height, so nothing else here can affect the layout.
	current := ctx.GetTheme().Accent
	if installedOn && hz == installedHz && current == accent {
		return
	}
	ctx.SetWidget(ext.WidgetConfig{
		ID:        submonWidgetID,
		Placement: ext.WidgetAbove,
		Priority:  0,
		Content: ext.WidgetContent{
			Render:    func(width int) string { return submonRender(width) },
			RefreshHz: hz,
		},
		Style: ext.WidgetStyle{BorderColor: current},
	})

	submonMu.Lock()
	submonInstalledOn = true
	submonInstalledHz = hz
	submonAccent = current
	submonMu.Unlock()
}

// ---------------------------------------------------------------------------
// Modal transcript
// ---------------------------------------------------------------------------

// submonOverlayBody renders one subagent's full transcript for the modal.
//
// The overlay wraps long lines itself and preserves ANSI, so this emits
// colored text and leaves layout to Kit. Caller holds submonMu.
func submonOverlayBody(th ext.ThemeColors, e *submonEntry) string {
	out := []string{}

	// ---- summary ----------------------------------------------------------
	out = append(out, th.ANSI(th.Text, e.task))
	meta := submonPlural(e.tools, "tool")
	if e.errs > 0 {
		meta += " · " + submonPlural(e.errs, "error")
	}
	meta += " · " + submonFmtDur(submonElapsed(e))
	if e.status == "running" {
		meta += " · still running"
	}
	out = append(out, th.ANSI(th.VeryMuted, meta), "")

	// ---- transcript -------------------------------------------------------
	if len(e.lines) == 0 {
		out = append(out, th.ANSI(th.VeryMuted, "(no output yet)"))
	}
	for _, l := range e.lines {
		out = append(out, th.ANSI(submonKindColor(th, l.kind), l.text))
	}

	// ---- result -----------------------------------------------------------
	if e.status != "running" && strings.TrimSpace(e.result) != "" {
		color, tag := th.Success, "result"
		if e.status == "failed" {
			color, tag = th.Error, "error"
		}
		out = append(out, "", th.ANSI(th.VeryMuted, "── "+tag+" ──"), th.ANSI(color, e.result))
	}
	return strings.Join(out, "\n")
}

// submonOverlayTitle labels the dialog with which agent is shown and where it
// sits in the list.
func submonOverlayTitle(e *submonEntry, index, total int) string {
	title := fmt.Sprintf("#%d %s · %s", e.id, submonAgentLabel(e.agent), e.status)
	if total > 1 {
		title += fmt.Sprintf("  (%d/%d)", index+1, total)
	}
	return title
}

// submonOverlayActions builds the action bar for the current position.
func submonOverlayActions(index, total int, running bool) []string {
	actions := []string{}
	if total > 1 {
		if index > 0 {
			actions = append(actions, submonActPrev)
		}
		if index < total-1 {
			actions = append(actions, submonActNext)
		}
	}
	// The dialog snapshots its content, so a running agent needs a way to
	// pull the latest output without closing and reopening.
	if running {
		actions = append(actions, submonActRefresh)
	}
	return append(actions, submonActClose)
}

// submonShowModal displays the transcript and walks between subagents until
// the user closes it.
//
// ShowOverlay BLOCKS until the dialog is dismissed, so this must run on a
// goroutine — never on the TUI event loop. Shortcut handlers and slash
// commands already provide one.
func submonShowModal(ctx ext.Context, index int) {
	if ctx.ShowOverlay == nil {
		return
	}
	for {
		submonMu.Lock()
		total := len(submonEntries)
		if total == 0 {
			submonMu.Unlock()
			return
		}
		index = submonClamp(index, 0, total-1)
		e := submonEntries[index]
		th := ctx.GetTheme()
		title := submonOverlayTitle(e, index, total)
		body := submonOverlayBody(th, e)
		actions := submonOverlayActions(index, total, e.status == "running")
		submonMu.Unlock()

		width := 0
		if w, _ := ctx.GetTerminalSize(); w > 0 {
			width = submonClamp(w-8, 40, 160)
		}

		res := ctx.ShowOverlay(ext.OverlayConfig{
			Title:   title,
			Content: ext.WidgetContent{Text: body},
			Style:   ext.OverlayStyle{BorderColor: th.Accent},
			Width:   width,
			Anchor:  ext.OverlayCenter,
			Actions: actions,
		})

		// Match on the label, never the index: a cancelled dialog or a host
		// that returns a zero value must close rather than loop.
		if res.Cancelled {
			return
		}
		switch res.Action {
		case submonActPrev:
			index--
		case submonActNext:
			index++
		case submonActRefresh:
			// Fall through and redraw from current state.
		default:
			return
		}
	}
}

// submonOpenModal is the entry point behind the shortcut and slash command.
// Caller must NOT hold submonMu.
func submonOpenModal(ctx ext.Context) {
	submonMu.Lock()
	submonCtx = ctx
	submonHasCtx = true
	empty := len(submonEntries) == 0
	busy := submonModalOpen
	if !empty && !busy {
		submonModalOpen = true
	}
	submonMu.Unlock()

	if empty {
		if ctx.PrintInfo != nil {
			ctx.PrintInfo("No subagents yet — the strip appears when the agent spawns one.")
		}
		return
	}
	// Kit rejects an overlay raised while another is open, which would return
	// immediately as cancelled and look like the key did nothing.
	if busy {
		return
	}

	// Open on the first agent still working, since that is the one the user
	// is most likely asking about. Falls back to the oldest when all are done.
	submonMu.Lock()
	start := 0
	for i, e := range submonEntries {
		if e.status == "running" {
			start = i
			break
		}
	}
	submonMu.Unlock()

	submonShowModal(ctx, start)

	submonMu.Lock()
	submonModalOpen = false
	submonMu.Unlock()
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func Init(api ext.API) {
	submonMu.Lock()
	submonReset()
	submonMu.Unlock()

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		submonMu.Lock()
		submonCtx = ctx
		submonHasCtx = true
		submonReset()
		submonMu.Unlock()
		if ctx.RemoveWidget != nil {
			ctx.RemoveWidget(submonWidgetID)
		}
	})

	// A new turn clears the previous turn's finished subagents, so results
	// stay readable until the user moves on. Never prunes while the modal is
	// up, which would pull the transcript out from under the reader.
	api.OnAgentStart(func(_ ext.AgentStartEvent, ctx ext.Context) {
		submonMu.Lock()
		submonCtx = ctx
		submonHasCtx = true
		if !submonModalOpen {
			kept := []*submonEntry{}
			for _, e := range submonEntries {
				if e.status == "running" {
					kept = append(kept, e)
				}
			}
			submonEntries = kept
		}
		submonMu.Unlock()
		submonSyncWidget()
	})

	api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
		submonMu.Lock()
		submonCtx = ctx
		submonHasCtx = true
		submonMu.Unlock()
		submonSyncWidget()
	})

	// ── ToolCall: resolve the named agent ───────────────────────────────────
	// SubagentStartEvent doesn't carry the agent name, so it has to come from
	// the raw tool-call arguments.
	//
	// Ordering matters here and is counter-intuitive: SubagentStart is derived
	// from the SDK event bus and lands BEFORE the extension ToolCall event, so
	// the entry usually exists already and this handler backfills it. The map
	// covers the opposite order. Verified against a live run:
	//
	//	SubagentStart id=toolu_01L9… → ToolCall tool=subagent id=toolu_01L9…
	api.OnToolCall(func(e ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if e.ToolName != "subagent" || e.ParsedArgs == nil {
			return nil
		}
		a, ok := e.ParsedArgs["agent"].(string)
		if !ok {
			return nil
		}
		submonMu.Lock()
		submonAgentByCall[e.ToolCallID] = a
		if entry := submonFind(e.ToolCallID); entry != nil {
			entry.agent = a
		}
		submonMu.Unlock()
		submonSyncWidget()
		return nil
	})

	// ── SubagentStart ────────────────────────────────────────────────────────
	api.OnSubagentStart(func(e ext.SubagentStartEvent, ctx ext.Context) {
		submonMu.Lock()
		submonCtx = ctx
		submonHasCtx = true

		entry := &submonEntry{
			id:      submonNextID,
			callID:  e.ToolCallID,
			agent:   submonAgentByCall[e.ToolCallID],
			task:    submonClean(e.Task),
			started: time.Now(),
			status:  "running",
		}
		submonNextID++
		submonEntries = append(submonEntries, entry)
		if n := len(submonEntries); n > submonMaxEntries {
			submonEntries = submonEntries[n-submonMaxEntries:]
		}
		submonMu.Unlock()

		submonSyncWidget()
	})

	// ── SubagentChunk ────────────────────────────────────────────────────────
	api.OnSubagentChunk(func(e ext.SubagentChunkEvent, ctx ext.Context) {
		submonMu.Lock()
		submonCtx = ctx
		submonHasCtx = true

		entry := submonFind(e.ToolCallID)
		if entry == nil {
			submonMu.Unlock()
			return
		}

		switch e.ChunkType {
		case "text":
			submonFeedText(entry, e.Content)
		case "reasoning":
			// Thinking is noise on the strip but useful in the transcript.
			submonFlushText(entry)
			submonPush(entry, "meta", "· "+strings.TrimSpace(e.Content))
		case "tool_call":
			submonFlushText(entry)
			entry.tools++
			line := "→ " + e.ToolName
			if s := submonArgSummary(e.ToolArgs); s != "" {
				line += "  " + s
			}
			submonPush(entry, "tool", line)
		case "tool_result":
			submonFlushText(entry)
			if e.IsError {
				entry.errs++
				submonPush(entry, "err", "✗ "+e.ToolName+"  "+strings.TrimSpace(e.ToolResult))
			} else {
				submonPush(entry, "ok", "✓ "+e.ToolName)
			}
		}
		submonMu.Unlock()

		submonSyncWidget()
	})

	// ── SubagentEnd ──────────────────────────────────────────────────────────
	// Entries are kept (not deleted) so the output stays readable; the next
	// OnAgentStart clears them.
	api.OnSubagentEnd(func(e ext.SubagentEndEvent, ctx ext.Context) {
		submonMu.Lock()
		submonCtx = ctx
		submonHasCtx = true

		if entry := submonFind(e.ToolCallID); entry != nil {
			submonFlushText(entry)
			entry.elapsed = time.Since(entry.started)
			if e.ErrorMsg != "" {
				entry.status = "failed"
				entry.result = e.ErrorMsg
				submonPush(entry, "err", "✗ "+e.ErrorMsg)
			} else {
				entry.status = "done"
				entry.result = e.Response
				submonPush(entry, "meta", "— finished in "+submonFmtDur(entry.elapsed))
			}
		}
		delete(submonAgentByCall, e.ToolCallID)
		submonMu.Unlock()

		submonSyncWidget()
	})

	// ── Entry points ─────────────────────────────────────────────────────────
	// ctrl+alt+s is neither reserved nor bound by Kit, so it shadows nothing.
	// Shortcut handlers run on their own goroutine, which is what lets the
	// blocking ShowOverlay call be made from here.
	api.RegisterShortcut(ext.ShortcutDef{
		Key:         "ctrl+alt+s",
		Description: "Open the subagent transcript",
	}, func(ctx ext.Context) {
		submonOpenModal(ctx)
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "subagents",
		Description: "Open the subagent transcript (same as ctrl+alt+s)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			submonOpenModal(ctx)
			return "", nil
		},
	})

	// ── SessionShutdown ──────────────────────────────────────────────────────
	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		submonMu.Lock()
		submonReset()
		submonMu.Unlock()
		// Guard ctx access — may be nil during shutdown.
		if ctx.RemoveWidget != nil {
			ctx.RemoveWidget(submonWidgetID)
		}
	})
}
