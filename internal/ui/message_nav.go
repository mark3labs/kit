package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// --------------------------------------------------------------------------
// Message navigation — select a message in the scrollback and inspect it
// --------------------------------------------------------------------------

// InspectableItem is implemented by MessageItems that can expose their
// original source text. The scrollback stores a display-oriented rendering
// (styled, width-wrapped, and frequently truncated); the message inspector
// needs the underlying text so the user can read what was elided.
//
// MessageItems that do not implement this interface are still selectable —
// the inspector falls back to their rendered content.
type InspectableItem interface {
	// RawContent returns the untruncated source text for the message.
	RawContent() string
	// Role returns the message role, used to pick a title and decide
	// whether the inspector should render the body as markdown.
	Role() string
}

// selectionBorderChars are the glyphs used to frame the selected message.
// A thin rounded box keeps the highlight legible without competing with the
// per-role gutter glyphs that already occupy column 0 of every block.
const (
	selBorderTopLeft     = "╭"
	selBorderTopRight    = "╮"
	selBorderBottomLeft  = "╰"
	selBorderBottomRight = "╯"
	selBorderHorizontal  = "─"
	selBorderVertical    = "│"
)

// selectionBorderOverhead is the number of extra lines a selection border
// adds to an item (one for the top edge, one for the bottom edge).
const selectionBorderOverhead = 2

// selectionTabWidth is the number of columns a tab occupies when a selected
// message is framed. It matches the terminal's default tab stop.
const selectionTabWidth = 8

// expandTabs replaces tabs with spaces, advancing to the next tab stop so the
// result occupies the same columns the terminal would use. Width-aware so
// wide runes and ANSI sequences before a tab are accounted for.
func expandTabs(s string, tabWidth int) string {
	if !strings.Contains(s, "\t") || tabWidth <= 0 {
		return s
	}

	var out strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}
		col := 0
		for j, part := range strings.Split(line, "\t") {
			// Every segment after the first was preceded by a tab. Keyed on
			// the segment index rather than the column, so a line that opens
			// with a tab (column zero) still advances to the first stop.
			if j > 0 {
				pad := tabWidth - (col % tabWidth)
				out.WriteString(strings.Repeat(" ", pad))
				col += pad
			}
			out.WriteString(part)
			col += xansi.StringWidth(part)
		}
	}
	return out.String()
}

// enterMessageNav switches the model into scrollback navigation mode,
// selecting the last selectable message. It is a no-op when there is
// nothing to select.
func (m *AppModel) enterMessageNav() {
	if m.scrollList == nil {
		return
	}

	idx := m.lastSelectableIndex()
	if idx < 0 {
		m.printSystemMessage("No messages to navigate.")
		return
	}

	// A live text selection would render highlight spans on top of the
	// navigation border, so clear it on entry.
	m.scrollList.ClearSelection()

	// Navigation suspends the current state rather than replacing it: the
	// agent keeps running while the user browses, so stateWorking has to
	// survive the round trip or the activity row never comes back.
	m.navReturnState = m.state
	if m.navReturnState != stateWorking {
		m.navReturnState = stateInput
	}

	m.state = stateMessageNav
	m.selectMessage(idx)

	// Navigation implies the user wants to look at history: pin the
	// viewport so incoming stream output can't yank it away.
	m.scrollList.autoScroll = false
	m.layoutDirty = true
}

// exitMessageNav leaves navigation mode, clears the selection border, and
// returns to the state navigation suspended — stateWorking when the agent is
// still running, stateInput otherwise.
func (m *AppModel) exitMessageNav() {
	if m.scrollList != nil {
		m.scrollList.SetSelectedIndex(-1)
		// Resume following new output only if the user is already at the end.
		if m.scrollList.AtBottom() {
			m.scrollList.autoScroll = true
		}
	}
	m.state = m.navReturnState
	m.navReturnState = stateInput
	m.layoutDirty = true
}

// agentWorking reports whether an agent turn is in flight, independent of
// which mode currently owns the keyboard. Message navigation takes over
// m.state for the duration of the browse, parking the working flag in
// navReturnState, so callers that care about agent liveness (the activity
// row, turn-state notifications) must ask here rather than compare m.state.
func (m *AppModel) agentWorking() bool {
	if m.navActive() {
		return m.navReturnState == stateWorking
	}
	return m.state == stateWorking
}

// navActive reports whether message navigation currently owns the session,
// either directly or underneath the message inspector it opens (the
// inspector is a modal that restores navigation when dismissed).
func (m *AppModel) navActive() bool {
	switch m.state {
	case stateMessageNav:
		return true
	case stateOverlay:
		return m.preOverlayState == stateMessageNav
	case statePrompt:
		return m.prePromptState == stateMessageNav
	}
	return false
}

// setAgentState applies an agent lifecycle transition (stateWorking on turn
// start, stateInput on turn end).
//
// These transitions are driven by asynchronous agent events, which can land
// at any moment — including while the user is browsing the scrollback. In
// that case the transition is recorded as the state navigation will return
// to instead of being applied, so a background turn can neither tear down
// navigation nor be forgotten when the user leaves it.
func (m *AppModel) setAgentState(s appState) {
	if m.navActive() {
		m.navReturnState = s
		return
	}
	m.state = s
}

// selectMessage moves the selection to idx and scrolls it into view.
func (m *AppModel) selectMessage(idx int) {
	if m.scrollList == nil {
		return
	}
	m.scrollList.SetSelectedIndex(idx)
	m.scrollList.EnsureVisible(idx)
	m.layoutDirty = true
}

// isSelectableIndex reports whether the item at idx can be selected.
// Items that render to nothing (e.g. a reasoning block before any reasoning
// has streamed) occupy no rows, so framing them would draw a border around
// empty space and let the cursor "disappear".
func (m *AppModel) isSelectableIndex(idx int) bool {
	if m.scrollList == nil {
		return false
	}
	item := m.scrollList.ItemAt(idx)
	if item == nil {
		return false
	}
	return strings.TrimSpace(item.Render(m.scrollList.width)) != ""
}

// lastSelectableIndex returns the index of the last selectable message,
// or -1 when the scrollback has none.
func (m *AppModel) lastSelectableIndex() int {
	if m.scrollList == nil {
		return -1
	}
	for idx := m.scrollList.Len() - 1; idx >= 0; idx-- {
		if m.isSelectableIndex(idx) {
			return idx
		}
	}
	return -1
}

// firstSelectableIndex returns the index of the first selectable message,
// or -1 when the scrollback has none.
func (m *AppModel) firstSelectableIndex() int {
	if m.scrollList == nil {
		return -1
	}
	for idx := range m.scrollList.Len() {
		if m.isSelectableIndex(idx) {
			return idx
		}
	}
	return -1
}

// moveMessageSelection moves the selection by delta items, skipping
// unselectable (empty) entries and stopping at the ends of the list.
func (m *AppModel) moveMessageSelection(delta int) {
	if m.scrollList == nil || delta == 0 {
		return
	}

	current := m.scrollList.SelectedIndex()
	if current < 0 {
		if idx := m.lastSelectableIndex(); idx >= 0 {
			m.selectMessage(idx)
		}
		return
	}

	step := 1
	if delta < 0 {
		step = -1
	}

	for idx := current + step; idx >= 0 && idx < m.scrollList.Len(); idx += step {
		if m.isSelectableIndex(idx) {
			m.selectMessage(idx)
			return
		}
	}
	// No selectable neighbour in that direction — keep the current selection.
}

// inspectSelectedMessage opens the selected message's full source text in
// the scrollable overlay dialog.
//
// The scrollback stores a display rendering that is frequently truncated
// (tool results are capped at ~10 lines, diffs at 20, and so on), so the
// inspector shows RawContent when the item can supply it. Dismissing the
// dialog returns to navigation mode via preOverlayState.
func (m *AppModel) inspectSelectedMessage() {
	if m.scrollList == nil {
		return
	}

	idx := m.scrollList.SelectedIndex()
	item := m.scrollList.ItemAt(idx)
	if item == nil {
		return
	}

	title, content, markdown := messageInspectorContent(item, m.scrollList.width)
	if strings.TrimSpace(content) == "" {
		return
	}

	m.preOverlayState = stateMessageNav
	m.state = stateOverlay
	// No response channel: this overlay is driven by the UI, not by a
	// blocking extension goroutine, so resolveOverlay has nothing to notify.
	m.overlayResponseCh = nil

	// The inspector is a reader, not a dialog, so it claims more of the
	// screen than the extension-facing defaults (60% x 80%). Tool output is
	// full of long lines — grep matches carry a path, a line number and the
	// matched source — and every column saved here is a wrapped row the
	// reader doesn't have to scroll past.
	dialogWidth := m.width * 90 / 100
	dialogHeight := m.height * 90 / 100

	m.overlay = newOverlayDialog(
		title, content, markdown,
		"", "",
		dialogWidth, dialogHeight, "center",
		nil,
		m.width, m.height,
	)
	if m.overlay != nil {
		// Read-only dialog: Enter and Esc both just close it.
		m.overlay.dismissOnly = true
	}
}

// messageInspectorContent resolves the title, body, and markdown flag used
// when displaying a message in the inspector.
func messageInspectorContent(item MessageItem, width int) (title, content string, markdown bool) {
	inspectable, ok := item.(InspectableItem)
	if !ok {
		// Fall back to the rendered form for items with no raw source.
		return "Message", item.Render(width), false
	}

	content = inspectable.RawContent()
	if strings.TrimSpace(content) == "" {
		content = item.Render(width)
	}

	role := inspectable.Role()
	switch role {
	case "user":
		// User text is authored as markdown and is safe to render as such.
		return "You", content, true
	case "assistant":
		return "Assistant", content, true
	case "reasoning":
		return "Reasoning", content, false
	case "tool":
		// Tool output is literal text (diffs, logs, JSON): rendering it as
		// markdown would reflow and mangle it.
		return "Tool Result", content, false
	case "bash", "shell":
		return "Command Output", content, false
	case "error":
		return "Error", content, false
	case "system":
		return "System", content, false
	}

	if role == "" {
		role = "Message"
	}
	return strings.ToUpper(role[:1]) + role[1:], content, false
}

// applySelectionBorder frames pre-rendered content in a thin box exactly
// totalWidth columns wide.
//
// The border is drawn manually rather than with a lipgloss border style
// because scrollback content is already fully styled and padded to the
// viewport width: handing it to lipgloss would re-wrap every line and change
// the item's height unpredictably. Drawing the frame by hand keeps the
// geometry exact — the result is always len(lines)+2 rows tall and
// totalWidth columns wide — which is what the ScrollList height cache and
// mouse hit-testing depend on.
//
// Lines wider than the inner width are truncated (ANSI-aware) and shorter
// lines are padded, so the right edge always lines up.
func applySelectionBorder(content string, totalWidth int, clr color.Color) string {
	// Below this width there is no room for a frame plus content.
	if totalWidth < 4 {
		return content
	}

	// Expand tabs before measuring. Width calculations treat a tab as a
	// single cell, but the terminal advances to the next tab stop, so a row
	// padded on the tab-is-one-cell assumption renders wider than the frame
	// and pushes the right border out of alignment. Tool output is full of
	// tabs (grep matches carry the indentation of the matched source).
	content = expandTabs(content, selectionTabWidth)

	innerWidth := totalWidth - 2
	borderStyle := lipgloss.NewStyle().Foreground(clr)

	left := borderStyle.Render(selBorderVertical)
	right := borderStyle.Render(selBorderVertical)

	horizontal := strings.Repeat(selBorderHorizontal, innerWidth)
	top := borderStyle.Render(selBorderTopLeft + horizontal + selBorderTopRight)
	bottom := borderStyle.Render(selBorderBottomLeft + horizontal + selBorderBottomRight)

	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines)+selectionBorderOverhead)
	out = append(out, top)

	for _, line := range lines {
		// Truncate first so an over-wide line can't push the right edge out,
		// then pad so a short line can't pull it in.
		if xansi.StringWidth(line) > innerWidth {
			line = xansi.Truncate(line, innerWidth, "")
		}
		if pad := innerWidth - xansi.StringWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		out = append(out, left+line+right)
	}

	out = append(out, bottom)
	return strings.Join(out, "\n")
}
