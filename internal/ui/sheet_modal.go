package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/clipboard"
	"github.com/mark3labs/kit/internal/ui/selection"
	"github.com/mark3labs/kit/internal/ui/style"
)

// sheetModal is a scrollable read-only sheet composited over the transcript.
// /help and the info commands (/tools, /servers, /usage, /session,
// /shortcuts, /jobs) show their content in it.
//
// The body is built on each render, so a sheet that shows live state (job
// status, usage) stays current while it is open.
//
// The text can be selected with the mouse, as in the scrollback: drag to
// select, double click for a word, triple click for a line. Release copies
// the selection to the clipboard.
type sheetModal struct {
	title    string
	subtitle string
	// maxW caps the box width.
	maxW int
	// body returns the body lines. Every line must be exactly width wide.
	body func(width int, s helpStyles) []string

	scrollOff int
	// pageSize and maxOff come from the last render; the key handler pages
	// and clamps with them.
	pageSize int
	maxOff   int

	// Geometry of the last render, in screen cells: the top-left cell of
	// the body viewport and its width. Mouse events map through it.
	bodyX, bodyY, bodyW int
	// lines are all body lines of the last render. A selection addresses
	// them by index, so it stays on the same text while the sheet scrolls.
	lines []string

	// sel uses item index 0 for every line: the body is one item.
	sel selection.State
	// copied is the rune count of the last copied selection, for the footer.
	copied int
}

// Sheet geometry shared by every sheet.
const (
	sheetPadX = 3
	sheetPadY = 1
	// sheetInfoMaxW is the width cap for the info sheets. They hold less
	// than /help, so a narrower box keeps them easy to read.
	sheetInfoMaxW = 110
	// sheetDragScroll is how many lines a drag past the top or bottom edge
	// scrolls per mouse motion event.
	sheetDragScroll = 1
)

// sheetKeys are the bindings an open sheet answers to. The footer hint is
// rendered from them.
var sheetKeys = struct {
	Up, Down, PageUp, PageDown, Top, Bottom, Close key.Binding
}{
	Up:       bind("↑/k", "up", "up", "k"),
	Down:     bind("↓/j", "down", "down", "j"),
	PageUp:   bind("pgup", "page up", "pgup", "shift+space"),
	PageDown: bind("pgdn/space", "page down", "pgdown", "space"),
	Top:      bind("g", "top", "home", "g"),
	Bottom:   bind("G", "bottom", "end", "G"),
	Close:    bind("esc/q", "close", "esc", "q", "enter", "ctrl+c"),
}

// openSheet shows a sheet with the given title, subtitle and body builder.
// It replaces a sheet that is already open.
func (m *AppModel) openSheet(title, subtitle string, maxW int, body func(width int, s helpStyles) []string) {
	m.sheet = &sheetModal{
		title:    title,
		subtitle: subtitle,
		maxW:     maxW,
		body:     body,
		sel:      selection.NewState(),
	}
}

// updateSheet handles input while a sheet is open. The sheet owns the
// keyboard and the mouse; every other message passes through to the app.
func (m *AppModel) updateSheet(msg tea.Msg) (handled bool, cmd tea.Cmd) {
	sh := m.sheet
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, sheetKeys.Close):
			m.sheet = nil
		case key.Matches(msg, sheetKeys.Up):
			sh.scrollBy(-1)
		case key.Matches(msg, sheetKeys.Down):
			sh.scrollBy(1)
		case key.Matches(msg, sheetKeys.PageUp):
			sh.scrollBy(-max(1, sh.pageSize-1))
		case key.Matches(msg, sheetKeys.PageDown):
			sh.scrollBy(max(1, sh.pageSize-1))
		case key.Matches(msg, sheetKeys.Top):
			sh.scrollOff = 0
		case key.Matches(msg, sheetKeys.Bottom):
			sh.scrollOff = sh.maxOff
		}
		return true, nil
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			sh.scrollBy(-3)
		case tea.MouseWheelDown:
			sh.scrollBy(3)
		}
		return true, nil
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			sh.mouseDown(msg.X, msg.Y, time.Now())
		}
		return true, nil
	case tea.MouseMotionMsg:
		sh.mouseDrag(msg.X, msg.Y)
		return true, nil
	case tea.MouseReleaseMsg:
		if text := sh.mouseUp(); text != "" {
			m.clipboardFailed = false
			return true, clipboard.CopySelection(text)
		}
		return true, nil
	}
	return false, nil
}

func (sh *sheetModal) scrollBy(delta int) {
	sh.scrollOff = max(0, min(sh.maxOff, sh.scrollOff+delta))
}

// pointAt maps a screen cell to a body line index and a column in it. With
// clamp, a point outside the viewport maps to the nearest visible cell;
// without it, such a point reports ok=false.
func (sh *sheetModal) pointAt(x, y int, clamp bool) (line, col int, ok bool) {
	if len(sh.lines) == 0 || sh.pageSize == 0 {
		return 0, 0, false
	}
	row, col := y-sh.bodyY, x-sh.bodyX
	inside := row >= 0 && row < sh.pageSize && col >= 0 && col < sh.bodyW
	if !inside && !clamp {
		return 0, 0, false
	}
	row = max(0, min(sh.pageSize-1, row))
	col = max(0, min(sh.bodyW, col))
	line = sh.scrollOff + row
	if line >= len(sh.lines) {
		if !clamp {
			return 0, 0, false
		}
		line = len(sh.lines) - 1
	}
	return line, col, true
}

// mouseDown starts a selection: a drag on a single click, the word on a
// double click and the line on a triple click. A click outside the body
// clears the selection.
func (sh *sheetModal) mouseDown(x, y int, now time.Time) {
	sh.copied = 0
	line, col, ok := sh.pointAt(x, y, false)
	if !ok {
		sh.sel.Clear()
		return
	}

	s := &sh.sel
	if now.Sub(s.LastClickTime) <= selection.DoubleClickThreshold &&
		abs(x-s.LastClickX) <= selection.ClickTolerance &&
		abs(y-s.LastClickY) <= selection.ClickTolerance {
		s.ClickCount++
	} else {
		s.ClickCount = 1
	}
	s.LastClickTime, s.LastClickX, s.LastClickY = now, x, y

	start, end := col, col
	switch s.ClickCount {
	case 2:
		start, end = selection.FindWordBoundaries(xansi.Strip(sh.lines[line]), col)
		if start == end {
			start, end = col, col
		}
	case 3:
		start, end = 0, sh.bodyW
		s.ClickCount = 0
	}
	s.MouseDown = true
	s.MouseDownItemIdx, s.MouseDownLineIdx, s.MouseDownCol = 0, line, start
	s.DragItemIdx, s.DragLineIdx, s.DragCol = 0, line, end
}

// mouseDrag moves the selection end. A drag past the top or bottom edge
// scrolls the sheet, so a selection can grow past one screen.
func (sh *sheetModal) mouseDrag(x, y int) {
	if !sh.sel.MouseDown {
		return
	}
	switch {
	case y < sh.bodyY:
		sh.scrollBy(-sheetDragScroll)
	case y >= sh.bodyY+sh.pageSize:
		sh.scrollBy(sheetDragScroll)
	}
	line, col, ok := sh.pointAt(x, y, true)
	if !ok {
		return
	}
	sh.sel.DragItemIdx, sh.sel.DragLineIdx, sh.sel.DragCol = 0, line, col
}

// mouseUp ends a drag and returns the selected text, or "" when nothing was
// selected. The selection stays highlighted so the user can see what was
// copied.
func (sh *sheetModal) mouseUp() string {
	if !sh.sel.MouseDown {
		return ""
	}
	sh.sel.MouseDown = false
	if !sh.sel.HasSelection() {
		sh.sel.Clear()
		return ""
	}
	text := sh.selectedText()
	if text == "" {
		sh.sel.Clear()
		return ""
	}
	sh.copied = utf8.RuneCountInString(text)
	return text
}

// selectedText returns the plain text under the selection. The padding that
// fills each line to the box width is not part of the text, so trailing
// spaces are dropped from every line.
func (sh *sheetModal) selectedText() string {
	r := sh.sel.GetRange()
	if r.IsEmpty() {
		return ""
	}
	var out []string
	for i := r.StartLine; i <= r.EndLine && i < len(sh.lines); i++ {
		in, startCol, endCol := selection.IsLineInRange(r, 0, i)
		if !in {
			continue
		}
		out = append(out, strings.TrimRight(selection.ExtractText(sh.lines[i], startCol, endCol), " "))
	}
	return strings.Trim(strings.Join(out, "\n"), "\n")
}

// highlight returns line i with the selected part in reverse video.
func (sh *sheetModal) highlight(i int, line string) string {
	r := sh.sel.GetRange()
	if r.IsEmpty() {
		return line
	}
	in, startCol, endCol := selection.IsLineInRange(r, 0, i)
	if !in {
		return line
	}
	return selection.HighlightLine(line, startCol, endCol)
}

// renderSheet draws the open sheet, scrolled to the current offset, and
// records the geometry that mouse events map through. The box is placed by
// compositeCentered, so the geometry uses the same centring.
func (m *AppModel) renderSheet() string {
	sh := m.sheet
	theme := style.GetTheme()
	s := newHelpStyles(theme)

	boxW := max(20, min(m.width-4, sh.maxW))
	innerW := max(10, boxW-2-2*sheetPadX)

	blank := s.bg.Render(strings.Repeat(" ", innerW))
	header := []string{s.padLine(s.title.Render(xansi.Truncate(sh.title, innerW, "…")), innerW)}
	if sh.subtitle != "" {
		for l := range strings.SplitSeq(xansi.Wrap(sh.subtitle, innerW, ""), "\n") {
			header = append(header, s.padLine(s.subtitle.Render(l), innerW))
		}
	}
	header = append(header, blank)

	body := sh.body(innerW, s)
	if len(body) == 0 {
		body = []string{s.padLine(s.note.Render("Nothing to show."), innerW)}
	}
	// A new width wraps the text anew, so a selection would mark other text.
	if innerW != sh.bodyW {
		sh.sel.Clear()
		sh.copied = 0
	}
	sh.lines = body

	// The box may use the full height less a one-row margin top and bottom.
	maxBoxH := max(8, m.height-2)
	footerH := 2
	viewH := max(1, maxBoxH-2-2*sheetPadY-len(header)-footerH)
	scrollable := len(body) > viewH
	if !scrollable {
		viewH = len(body)
	}
	sh.pageSize = viewH
	sh.maxOff = max(0, len(body)-viewH)
	sh.scrollOff = min(sh.scrollOff, sh.maxOff)

	lines := append([]string{}, header...)
	for i := sh.scrollOff; i < sh.scrollOff+viewH; i++ {
		lines = append(lines, sh.highlight(i, body[i]))
	}
	lines = append(lines, blank, m.sheetFooter(scrollable, len(body), viewH, innerW, s))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		BorderBackground(theme.Background).
		Background(theme.Background).
		Padding(sheetPadY, sheetPadX).
		Render(strings.Join(lines, "\n"))

	boxX := (m.width - lipgloss.Width(box)) / 2
	boxY := (m.height - lipgloss.Height(box)) / 2
	sh.bodyX = boxX + 1 + sheetPadX
	sh.bodyY = boxY + 1 + sheetPadY + len(header)
	sh.bodyW = innerW
	return box
}

// sheetFooter renders the key hints and, on the right, the copy notice or,
// when the sheet scrolls, the position.
func (m *AppModel) sheetFooter(scrollable bool, total, viewH, width int, s helpStyles) string {
	hint := func(k, desc string) string {
		return s.hintKey.Render(k) + s.hintDesc.Render(" "+desc)
	}
	bindingHint := func(b key.Binding) string {
		hb := b.Help()
		return hint(hb.Key, hb.Desc)
	}
	sep := s.hintDesc.Render("   ")
	var parts []string
	if scrollable {
		parts = append(parts, bindingHint(sheetKeys.Up), bindingHint(sheetKeys.Down), bindingHint(sheetKeys.PageDown))
	}
	parts = append(parts, hint("drag", "copy"), bindingHint(sheetKeys.Close))
	left := strings.Join(parts, sep)

	sh := m.sheet
	right := ""
	switch {
	case sh.copied > 0:
		right = s.key.Render(fmt.Sprintf("Copied %d chars", sh.copied))
	case scrollable:
		pct := 100
		if sh.maxOff > 0 {
			pct = sh.scrollOff * 100 / sh.maxOff
		}
		right = s.hintDesc.Render(fmt.Sprintf("%d–%d of %d · %d%%", sh.scrollOff+1, sh.scrollOff+viewH, total, pct))
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return s.padLine(xansi.Truncate(left, width, "…"), width)
	}
	return left + s.bg.Render(strings.Repeat(" ", gap)) + right
}

// sheetText renders plain text as sheet lines: wrapped to width, with tabs
// expanded and escape sequences removed, since they would break the line
// widths.
func sheetText(text string, width int, st lipgloss.Style, s helpStyles) []string {
	text = strings.ReplaceAll(xansi.Strip(strings.ReplaceAll(text, "\r\n", "\n")), "\t", "    ")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return nil
	}
	var out []string
	for l := range strings.SplitSeq(xansi.Wrap(text, width, ""), "\n") {
		out = append(out, s.padLine(st.Render(l), width))
	}
	return out
}

// sheetNameGrid lays names out in columns, top to bottom and then left to
// right, like ls. A name too long for its column is cut.
func sheetNameGrid(names []string, width int, s helpStyles) []string {
	if len(names) == 0 {
		return nil
	}
	const gap = 4
	nameW := 0
	for _, n := range names {
		nameW = max(nameW, lipgloss.Width(n))
	}
	colW := min(width, nameW)
	cols := max(1, min(len(names), (width+gap)/(colW+gap)))
	rows := (len(names) + cols - 1) / cols
	// Re-spread the columns over the rows actually used.
	cols = (len(names) + rows - 1) / rows

	gapCell := s.bg.Render(strings.Repeat(" ", gap))
	out := make([]string, rows)
	for r := range rows {
		var b strings.Builder
		for c := range cols {
			i := c*rows + r
			if i >= len(names) {
				break
			}
			if c > 0 {
				b.WriteString(gapCell)
			}
			b.WriteString(s.padLine(s.key.Render(xansi.Truncate(names[i], colW, "…")), colW))
		}
		out[r] = s.padLine(b.String(), width)
	}
	return out
}
