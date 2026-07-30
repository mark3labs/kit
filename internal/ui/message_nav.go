package ui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/clipboard"
	"github.com/mark3labs/kit/internal/ui/style"
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

// selectionFrameHint is the affordance spliced into the bottom edge of the
// selected message's frame. Navigation is a transient mode, so the one action
// that is not obvious from the highlight is advertised where the eye already
// is rather than only in the status bar.
const selectionFrameHint = "enter to open"

// selectionLabelInset is the number of horizontal border characters drawn
// before a label spliced into the frame's top edge, and after one spliced into
// the bottom edge. One glyph is enough to keep the label clear of the corner
// without making it look detached.
const selectionLabelInset = 1

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

// --------------------------------------------------------------------------
// Role presentation
// --------------------------------------------------------------------------

// rolePresentation describes how a message's role is announced — in the
// inspector's title bar and in the navigation frame's label.
//
// The colour is the one the scrollback already assigns to that role's gutter,
// so a tinted inspector reads as "this message, zoomed in" rather than as a
// separate piece of chrome with its own palette.
type rolePresentation struct {
	// title names the role in prose ("You", "Tool Result").
	title string
	// icon is a narrow glyph placed before the title.
	icon string
	// color tints the inspector's frame and title.
	color color.Color
	// markdown reports whether the body is safe to render as markdown.
	// Literal output — diffs, logs, JSON — is not: reflowing mangles it.
	markdown bool
}

// presentationForRole maps a scrollback role to its inspector presentation.
//
// Glyphs are deliberately narrow (single-cell) BMP characters: a
// double-width or variation-selector glyph measures differently across
// terminals, and the title row's right-aligned meta is padded against that
// measurement.
func presentationForRole(role string, theme style.Theme) rolePresentation {
	switch role {
	case "user":
		// User text is authored as markdown and is safe to render as such.
		// Accent is the colour of the user block's gutter.
		return rolePresentation{title: "You", icon: "›", color: theme.Accent, markdown: true}
	case "assistant":
		return rolePresentation{title: "Assistant", icon: "✦", color: theme.Primary, markdown: true}
	case "reasoning":
		return rolePresentation{title: "Reasoning", icon: "◇", color: theme.Muted}
	case "tool":
		return rolePresentation{title: "Tool Result", icon: "▸", color: theme.Tool}
	case "bash", "shell":
		return rolePresentation{title: "Command Output", icon: "$", color: theme.Tool}
	case "error":
		return rolePresentation{title: "Error", icon: "×", color: theme.Error}
	case "system":
		return rolePresentation{title: "System", icon: "◈", color: theme.System}
	case "logo":
		// The splash is a banner, not a message. It is selectable because it
		// occupies rows like anything else, so it needs a name — "Logo" would
		// be the title-cased fallback, which describes the implementation
		// rather than what the reader is looking at.
		return rolePresentation{title: "Session", icon: "◈", color: theme.System}
	}

	if role == "" {
		return rolePresentation{title: "Message", icon: "·", color: theme.Info}
	}
	// Unknown roles come from extensions. Title-case the first rune rather
	// than the first byte so a non-ASCII role name is not cut in half.
	runes := []rune(role)
	title := strings.ToUpper(string(runes[0])) + string(runes[1:])
	return rolePresentation{title: title, icon: "·", color: theme.Info}
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
		m.scrollList.SetSelectionFrame("", "")
		// Resume following new output only if the user is already at the end.
		if m.scrollList.AtBottom() {
			m.scrollList.autoScroll = true
		}
	}
	m.state = m.navReturnState
	m.navReturnState = stateInput
	m.navNotice = ""
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

// selectMessage moves the selection to idx and scrolls it into view, updating
// the frame's edge labels to name what is now selected.
func (m *AppModel) selectMessage(idx int) {
	if m.scrollList == nil {
		return
	}
	m.scrollList.SetSelectedIndex(idx)
	m.scrollList.SetSelectionFrame(m.selectionLabelFor(idx), selectionFrameHint)
	m.scrollList.EnsureVisible(idx)
	m.layoutDirty = true
}

// selectionLabelFor builds the frame's top-edge label for the item at idx:
// its role and its position in the run of selectable messages, e.g.
// "Tool Result · 12/37".
//
// The position counts only selectable items, because those are the only ones
// the cursor can land on — numbering against the raw item count would make the
// counter skip.
func (m *AppModel) selectionLabelFor(idx int) string {
	if m.scrollList == nil {
		return ""
	}

	item := m.scrollList.ItemAt(idx)
	if item == nil {
		return ""
	}

	label := presentationForRole(itemRole(item), style.GetTheme()).title

	pos, total := m.selectablePosition(idx)
	if total > 1 {
		label += fmt.Sprintf(" · %d/%d", pos, total)
	}
	return label
}

// selectablePosition returns the 1-based position of idx among selectable
// items, and how many there are.
//
// Selectability is decided by rendering, but item renders are memoized, so the
// walk is cheap after the first pass — and it only runs on a selection change,
// never per frame.
func (m *AppModel) selectablePosition(idx int) (pos, total int) {
	if m.scrollList == nil {
		return 0, 0
	}
	for i := range m.scrollList.Len() {
		if !m.isSelectableIndex(i) {
			continue
		}
		total++
		if i == idx {
			pos = total
		}
	}
	return pos, total
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

// jumpToRole moves the selection to the nearest message with the given role in
// the direction of step (-1 backwards, +1 forwards). It is a no-op when there
// is none, leaving the selection where it was rather than snapping to an end.
func (m *AppModel) jumpToRole(role string, step int) {
	if m.scrollList == nil || step == 0 {
		return
	}

	current := m.scrollList.SelectedIndex()
	if current < 0 {
		current = m.scrollList.Len()
	}
	if step > 0 {
		step = 1
	} else {
		step = -1
	}

	for idx := current + step; idx >= 0 && idx < m.scrollList.Len(); idx += step {
		item := m.scrollList.ItemAt(idx)
		if item == nil || itemRole(item) != role {
			continue
		}
		if m.isSelectableIndex(idx) {
			m.selectMessage(idx)
			return
		}
	}
}

// copySelectedMessage puts the selected message's source text on the clipboard.
//
// The source is copied rather than the rendering: the latter carries the escape
// codes, gutter glyphs and hard wrapping the transcript needed, none of which
// survives a paste usefully.
func (m *AppModel) copySelectedMessage() tea.Cmd {
	if m.scrollList == nil {
		return nil
	}

	item := m.scrollList.ItemAt(m.scrollList.SelectedIndex())
	if item == nil {
		return nil
	}

	text := inspectorSourceText(item, m.scrollList.width)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// Reported in the status bar rather than as a scrollback message: printing
	// one would append an item to the very list being navigated, shifting the
	// selection's position counter under the reader.
	m.navNotice = fmt.Sprintf("Copied %d chars to clipboard.", len(text))
	return clipboard.CopyToClipboard(text)
}

// inspectSelectedMessage opens the selected message's full source text in
// the scrollable overlay dialog.
//
// The scrollback stores a display rendering that is frequently truncated (tool
// results are capped at ~10 lines, diffs at 20, and so on), so the inspector
// re-renders from source: a tool call is drawn again with its line caps lifted,
// keeping the diff colouring and gutters, and everything else falls back to its
// raw text. Dismissing the dialog returns to navigation mode via
// preOverlayState.
func (m *AppModel) inspectSelectedMessage() {
	if m.scrollList == nil {
		return
	}

	idx := m.scrollList.SelectedIndex()
	item := m.scrollList.ItemAt(idx)
	if item == nil {
		return
	}

	theme := style.GetTheme()
	pres := presentationForRole(itemRole(item), theme)
	info, isTool := toolCallOf(item)

	// A failed tool call is presented as an error rather than as a tool: the
	// reader opened it to find out what went wrong, and the frame colour is the
	// fastest way to answer that.
	if isTool && info.IsError {
		err := presentationForRole("error", theme)
		pres.icon, pres.color = err.icon, err.color
	}

	content := inspectorSourceText(item, m.scrollList.width)
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
		pres.title, content, pres.markdown,
		"", "",
		dialogWidth, dialogHeight, "center",
		nil,
		m.width, m.height,
	)
	if m.overlay == nil {
		return
	}

	// Read-only dialog: Enter and Esc both just close it.
	m.overlay.dismissOnly = true
	m.overlay.accent = pres.color
	m.overlay.icon = pres.icon
	m.overlay.scrollbar = true
	m.overlay.wrapMarks = true
	// Literal output gets a source-line gutter so "the error on line 340" is
	// findable. The dialog ignores this for markdown and for re-rendered tool
	// bodies, which carry gutters of their own where they need them.
	m.overlay.lineNumbers = true
	// The clipboard gets the source text, never the rendering: the latter is
	// woven through with escape codes and hard-wrapped to the dialog width.
	m.overlay.copyText = content
	m.overlay.meta = inspectorMeta(item, content)

	// A tool call is re-rendered rather than shown as text, so the inspector
	// keeps the diff panels, line-number gutters and filled backgrounds the
	// transcript uses — just without the line caps that sent the reader here.
	if isTool {
		m.overlay.body = func(width int) string {
			return renderToolInspectorBody(info, width)
		}
	}
}

// itemRole returns a scrollback item's role, or "" for items that do not
// declare one.
func itemRole(item MessageItem) string {
	if inspectable, ok := item.(InspectableItem); ok {
		return inspectable.Role()
	}
	return ""
}

// toolCallOf returns the structured tool call an item displays, if any.
func toolCallOf(item MessageItem) (ToolCallInfo, bool) {
	if ti, ok := item.(ToolInspectable); ok {
		return ti.ToolCall()
	}
	return ToolCallInfo{}, false
}

// inspectorSourceText returns the untruncated source text for an item, falling
// back to its rendered form for items that keep no source.
func inspectorSourceText(item MessageItem, width int) string {
	inspectable, ok := item.(InspectableItem)
	if !ok {
		return item.Render(width)
	}
	if content := inspectable.RawContent(); strings.TrimSpace(content) != "" {
		return content
	}
	return item.Render(width)
}

// inspectorMeta builds the right-aligned annotation on the inspector's title
// row: what produced the message, and how much of it there is.
//
// For a tool call the name goes here rather than being left as the first line
// of the body, where the reader has to hunt for it while the title says only
// "Tool Result".
func inspectorMeta(item MessageItem, content string) string {
	var parts []string

	if info, ok := toolCallOf(item); ok {
		if name := toolDisplayName(info.Name); name != "" {
			parts = append(parts, name)
		}
		if info.IsError {
			parts = append(parts, "error")
		}
	}

	if n := strings.Count(strings.TrimRight(content, "\n"), "\n") + 1; n > 1 {
		parts = append(parts, fmt.Sprintf("%d lines", n))
	}

	return strings.Join(parts, " · ")
}

// renderToolInspectorBody renders a tool call for the inspector: its arguments
// in full, then the tool's own body with every line cap lifted.
//
// The transcript's header elides arguments to a single truncated line, so the
// full set — a long bash command, a multi-edit payload — is only recoverable
// here. Errors skip the body renderers entirely and show the result verbatim:
// what the reader came for is the whole message, not a styled excerpt of it.
func renderToolInspectorBody(info ToolCallInfo, width int) string {
	theme := style.GetTheme()

	// Tabs are expanded on the way in, not on the way out. The body renderers
	// lay their output out to exactly width, measuring a tab as one cell, and
	// the terminal then advances it to the next stop — so a tab that survives
	// this far pushes its row past the frame. Grep results are full of them.
	result := expandTabs(info.Result, dialogTabWidth)

	var parts []string
	if args := strings.TrimSpace(formatToolArgsForInspector(info.Args)); args != "" {
		parts = append(parts,
			lipgloss.NewStyle().Foreground(theme.Muted).Render(expandTabs(args, dialogTabWidth)),
			"",
		)
	}

	body := result
	if !info.IsError {
		if rendered := renderToolBodyLimited(
			info.Name, info.Args, result, width, inspectorLimits(),
		); strings.TrimSpace(rendered) != "" {
			body = rendered
		}
	}
	parts = append(parts, strings.TrimRight(body, "\n"))

	return strings.Join(parts, "\n")
}

// applySelectionBorder frames pre-rendered content in a thin box exactly
// totalWidth columns wide, with label spliced into the top edge and hint into
// the bottom edge.
//
// The border is drawn manually rather than with a lipgloss border style
// because scrollback content is already fully styled and padded to the
// viewport width: handing it to lipgloss would re-wrap every line and change
// the item's height unpredictably. Drawing the frame by hand keeps the
// geometry exact — the result is always len(lines)+2 rows tall and
// totalWidth columns wide — which is what the ScrollList height cache and
// mouse hit-testing depend on. It is also what makes the edge labels free:
// they replace border characters rather than adding rows.
//
// Lines wider than the inner width are truncated (ANSI-aware) and shorter
// lines are padded, so the right edge always lines up.
func applySelectionBorder(content string, totalWidth int, clr color.Color, label, hint string) string {
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

	top := borderStyle.Render(selBorderTopLeft) +
		frameEdge(label, innerWidth, true, borderStyle,
			lipgloss.NewStyle().Foreground(clr).Bold(true)) +
		borderStyle.Render(selBorderTopRight)

	bottom := borderStyle.Render(selBorderBottomLeft) +
		frameEdge(hint, innerWidth, false, borderStyle,
			lipgloss.NewStyle().Foreground(style.GetTheme().VeryMuted)) +
		borderStyle.Render(selBorderBottomRight)

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

// frameEdge builds one horizontal edge of the selection frame, exactly width
// columns wide, with text inset from the leading corner (leading) or from the
// trailing one.
//
// The label is dropped whole rather than truncated when it will not fit: a
// frame edge reading "─ Tool Res… ─" is noise, and the status bar carries the
// same information for narrow terminals.
func frameEdge(text string, width int, leading bool, borderStyle, textStyle lipgloss.Style) string {
	bar := func(n int) string {
		if n <= 0 {
			return ""
		}
		return borderStyle.Render(strings.Repeat(selBorderHorizontal, n))
	}

	if text == "" {
		return bar(width)
	}

	padded := " " + text + " "
	textW := xansi.StringWidth(padded)

	// Keep at least one border glyph on the far side of the label so it reads
	// as set into the edge rather than as having replaced it.
	if textW+selectionLabelInset+1 > width {
		return bar(width)
	}

	fill := width - textW - selectionLabelInset
	if leading {
		return bar(selectionLabelInset) + textStyle.Render(padded) + bar(fill)
	}
	return bar(fill) + textStyle.Render(padded) + bar(selectionLabelInset)
}
