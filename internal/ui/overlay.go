package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/clipboard"
	"github.com/mark3labs/kit/internal/ui/style"
)

// ---------------------------------------------------------------------------
// Overlay dialog — modal overlay rendered by AppModel when active
// ---------------------------------------------------------------------------

// overlayResult carries the synchronous outcome of an overlay dialog update.
// A non-nil value means the overlay is done (completed or cancelled); nil
// means the overlay is still active.
type overlayResult struct {
	completed bool
	cancelled bool
	action    string
	index     int
}

// overlayDialog holds the state of an active modal overlay dialog. It is
// created when an OverlayRequestEvent arrives and destroyed when the user
// completes or cancels. The AppModel owns the overlay and routes messages
// to it while in stateOverlay.
type overlayDialog struct {
	title       string
	content     string
	markdown    bool
	borderColor string
	background  string
	actions     []string
	selAction   int // selected action index
	scrollOff   int // scroll offset for content body
	totalLines  int // total body lines (computed on render)
	pageSize    int // body rows visible in the last render, for paging keys
	width       int // terminal width
	height      int // terminal height
	dialogWidth int // configured dialog width (0 = auto)
	maxHeight   int // configured max height (0 = auto)
	anchor      string

	// dismissOnly marks an overlay that has no consumer for its result —
	// the message inspector, which the UI opens for reading only. Enter and
	// Esc then close the dialog with no observable difference, so the key
	// hint advertises a single "close" action instead of implying that
	// dismiss and cancel lead to different outcomes.
	//
	// Extension overlays leave this false: ShowOverlay reports Enter as
	// completed and Esc as cancelled, and extensions may branch on it.
	dismissOnly bool

	// ---- Reader affordances (set by the message inspector) ----------------

	// accent tints the frame and the title so the dialog reads as "this
	// message, zoomed in" rather than as a generic box. Nil falls back to
	// theme.Info, which is what extension overlays get.
	accent color.Color

	// icon is a glyph placed before the title, matching the role marker the
	// scrollback uses for the same message.
	icon string

	// meta is a right-aligned annotation on the title row — the tool name,
	// a line count, a duration. It is the first thing dropped when the row
	// is too narrow for both halves.
	meta string

	// copyText is what the copy binding puts on the clipboard: the message's
	// source text, not the rendered form. Empty disables the binding, so
	// extension overlays get no copy hint they cannot honour.
	copyText string

	// wrapMarks draws a continuation glyph on rows produced by wrapping, so
	// a long tool result line is not mistaken for several source lines.
	wrapMarks bool

	// scrollbar draws a position track down the body's right-hand column.
	scrollbar bool

	// lineNumbers draws a source-line gutter down the left of the body.
	//
	// It applies only to literal text. Markdown is reflowed prose, where a
	// number per line means nothing, and a body renderer has already laid its
	// output out to an exact width — often with a gutter of its own.
	lineNumbers bool

	// body, when non-nil, produces the dialog body at the given text width,
	// replacing content.
	//
	// The inspector re-renders tool bodies — diffs, gutter-numbered code,
	// filled panels — and those are laid out to an exact width, which is not
	// known until the frame is being drawn and changes when the terminal is
	// resized. A pre-rendered string would be laid out to the wrong width the
	// moment either happened.
	body func(width int) string

	// bodyCache memoizes the resolved and wrapped body. Re-running markdown or
	// a full uncapped diff on every frame is far too expensive for a render
	// path; the cache is keyed on everything that can change the result.
	bodyCache      []bodyRow
	bodyCacheWidth int
	bodyCacheGen   uint64
	bodyCacheValid bool

	// bodyGutter is the columns the line-number gutter takes out of the body's
	// text width in the last resolved body. Zero when numbering is off.
	bodyGutter int

	// copied latches a one-shot footer acknowledgement after a copy. It is
	// cleared by the next keystroke: an indicator that never goes away stops
	// being an acknowledgement and becomes decoration.
	copied bool

	// ---- Incremental search ----------------------------------------------

	// searching is true while the query line is open and absorbing keys.
	searching bool
	// query is the committed (or in-progress) search string.
	query string
	// hits holds body-row indices matching query, ascending. Recomputed
	// whenever the query or the wrapped body changes.
	hits []int
	// hitIdx is the position within hits of the currently focused match.
	hitIdx int
}

// Dialog layout constants.
const (
	// dialogTabWidth is the number of columns a tab occupies inside the
	// dialog body. It matches the scrollback's selection frame
	// (selectionTabWidth) so indentation does not shift when a message is
	// opened in the inspector.
	dialogTabWidth = selectionTabWidth

	// dialogContMarker prefixes a row produced by wrapping a longer source
	// line. Two columns wide, which is what the wrap budget reserves.
	dialogContMarker = "↪ "

	// dialogScrollbarGap is the blank column between the body text and the
	// scrollbar track, plus the track itself.
	dialogScrollbarGap = 2

	// dialogGutterSep separates the line-number gutter from the body text.
	dialogGutterSep = "│"

	// dialogPageOverlap is the number of rows a page-sized jump keeps from
	// the previous screen, so the reader has an anchor to pick up from.
	dialogPageOverlap = 1
)

// Scrollbar glyphs. A filled block for the thumb reads as position at a
// glance; the light vertical for the track stays out of the way.
const (
	dialogScrollThumb = "█"
	dialogScrollTrack = "│"
)

// bodyRow is one rendered row of the dialog body.
type bodyRow struct {
	// text is the row's content, already wrapped to the body width.
	text string
	// cont marks a row produced by wrapping, rather than by a newline in
	// the source.
	cont bool
	// num is the 1-based source line the row came from. Continuation rows
	// repeat their source line's number, which is what lets the gutter leave
	// them blank instead of counting them as new lines.
	num int
}

// newOverlayDialog creates an overlay dialog from an OverlayRequestEvent's
// parameters.
func newOverlayDialog(title, content string, markdown bool, borderColor, background string, width, maxHeight int, anchor string, actions []string, termWidth, termHeight int) *overlayDialog {
	return &overlayDialog{
		title:       title,
		content:     content,
		markdown:    markdown,
		borderColor: borderColor,
		background:  background,
		actions:     actions,
		dialogWidth: width,
		maxHeight:   maxHeight,
		anchor:      anchor,
		width:       termWidth,
		height:      termHeight,
	}
}

// Init returns the initial command for the overlay. Currently no-op.
func (o *overlayDialog) Init() tea.Cmd {
	return nil
}

// Update handles messages for the overlay dialog. It returns a non-nil
// *overlayResult when the user completes or cancels. The returned tea.Cmd
// carries any side effect the keypress asked for (currently a clipboard
// write); overlays produce no other async work.
func (o *overlayDialog) Update(msg tea.Msg) (*overlayResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.width = msg.Width
		o.height = msg.Height
		return nil, nil

	case tea.MouseWheelMsg:
		// The scrollback underneath scrolls on the wheel, so a reader who
		// opened the inspector to read *more* of a message would otherwise
		// find the wheel dead exactly where it matters most.
		const wheelLines = 3
		switch msg.Button {
		case tea.MouseWheelUp:
			o.scrollBy(-wheelLines)
		case tea.MouseWheelDown:
			o.scrollBy(wheelLines)
		}
		return nil, nil

	case tea.KeyPressMsg:
		return o.handleKey(msg)
	}
	return nil, nil
}

// scrollBy moves the body offset by delta rows, clamping at the top. The
// bottom is clamped in Render, which is the only place the visible row count
// is known.
func (o *overlayDialog) scrollBy(delta int) {
	o.scrollOff = max(o.scrollOff+delta, 0)
}

func (o *overlayDialog) handleKey(msg tea.KeyPressMsg) (*overlayResult, tea.Cmd) {
	key := msg.String()

	// The search line owns the keyboard while it is open, so a query can
	// contain the very letters that are navigation keys everywhere else.
	if o.searching {
		return o.handleSearchKey(msg)
	}

	// Any keystroke retires the copy acknowledgement, including the one
	// being handled below.
	if key != "y" {
		o.copied = false
	}

	switch key {
	case "esc":
		// A committed query is dismissed before the dialog is: Esc reads as
		// "back out of the thing I just did", and closing the whole reader
		// on the first Esc after a search loses the reader's place.
		if o.query != "" {
			o.clearSearch()
			return nil, nil
		}
		return &overlayResult{cancelled: true}, nil

	case "enter":
		if len(o.actions) > 0 {
			action := ""
			if o.selAction < len(o.actions) {
				action = o.actions[o.selAction]
			}
			return &overlayResult{completed: true, action: action, index: o.selAction}, nil
		}
		// No actions — Enter dismisses (not cancelled).
		return &overlayResult{completed: true, action: "", index: -1}, nil

	// Content scrolling
	case "up", "k":
		o.scrollBy(-1)
	case "down", "j":
		// Clamped in Render; allow incrementing freely.
		o.scrollBy(1)
	case "pgup", "ctrl+b":
		o.scrollBy(-o.page())
	case "pgdown", "ctrl+f", " ":
		o.scrollBy(o.page())
	case "ctrl+u":
		o.scrollBy(-o.halfPage())
	case "ctrl+d":
		o.scrollBy(o.halfPage())
	case "home", "g":
		o.scrollOff = 0
	case "end", "G":
		// Set to a large value; Render will clamp.
		o.scrollOff = o.totalLines

	// Copy the message source. The rendered form is full of escape codes and
	// hard-wrapped at the dialog width, so it is the raw text that goes on
	// the clipboard.
	case "y":
		if o.copyText == "" {
			return nil, nil
		}
		o.copied = true
		return nil, clipboard.CopyToClipboard(o.copyText)

	// Search
	case "/":
		if o.searchable() {
			o.searching = true
			o.query = ""
			o.hits = nil
			o.hitIdx = 0
		}
	case "n":
		o.stepHit(1)
	case "N":
		o.stepHit(-1)

	// Action navigation
	case "left", "h":
		if len(o.actions) > 0 && o.selAction > 0 {
			o.selAction--
		}
	case "right", "l":
		if len(o.actions) > 0 && o.selAction < len(o.actions)-1 {
			o.selAction++
		}
	case "tab":
		if len(o.actions) > 0 {
			o.selAction = (o.selAction + 1) % len(o.actions)
		}
	}
	return nil, nil
}

// handleSearchKey absorbs keys while the query line is open. Enter commits the
// query and jumps to the first match; Esc abandons it and restores the reader
// to where it was.
func (o *overlayDialog) handleSearchKey(msg tea.KeyPressMsg) (*overlayResult, tea.Cmd) {
	switch msg.String() {
	case "esc":
		o.searching = false
		o.clearSearch()
		return nil, nil

	case "enter":
		o.searching = false
		if o.query == "" {
			o.clearSearch()
		}
		// Hits and the jump are resolved in Render, which is the only place
		// the wrapped body — and therefore the row indices — exists.
		return nil, nil

	case "backspace":
		if r := []rune(o.query); len(r) > 0 {
			o.query = string(r[:len(r)-1])
		}
		return nil, nil

	case "ctrl+u":
		o.query = ""
		return nil, nil
	}

	// Printable input extends the query. Modified and named keys (arrows,
	// function keys) are ignored rather than injected as literal text.
	if text := msg.Text; text != "" {
		o.query += text
	}
	return nil, nil
}

// clearSearch drops the query and its match list.
func (o *overlayDialog) clearSearch() {
	o.query = ""
	o.hits = nil
	o.hitIdx = 0
}

// searchable reports whether searching the body is worth offering. A body that
// fits on screen has nothing to find that the reader cannot already see.
func (o *overlayDialog) searchable() bool {
	return o.totalLines > o.pageSize && o.pageSize > 0
}

// stepHit moves the focus to the next or previous match, wrapping around, and
// scrolls it into view. No-op without a committed query.
func (o *overlayDialog) stepHit(delta int) {
	if len(o.hits) == 0 {
		return
	}
	o.hitIdx = (o.hitIdx + delta + len(o.hits)) % len(o.hits)
	o.revealRow(o.hits[o.hitIdx])
}

// revealRow scrolls so body row idx is visible, placing it a third of the way
// down the viewport. Centring it exactly is disorienting when stepping through
// matches; a third leaves useful context both above and below.
func (o *overlayDialog) revealRow(idx int) {
	if o.pageSize <= 0 {
		o.scrollOff = idx
		return
	}
	if idx >= o.scrollOff && idx < o.scrollOff+o.pageSize {
		return // already on screen
	}
	o.scrollOff = max(idx-o.pageSize/3, 0)
}

// page returns the row count a page-sized jump moves, keeping a row of overlap
// so the reader has an anchor. Falls back to a small step before the first
// render has established a viewport height.
func (o *overlayDialog) page() int {
	if o.pageSize <= dialogPageOverlap {
		return max(o.pageSize, 1)
	}
	return o.pageSize - dialogPageOverlap
}

// halfPage returns the row count a half-page jump moves.
func (o *overlayDialog) halfPage() int {
	return max(o.pageSize/2, 1)
}

// Render returns the overlay dialog as a styled string for full-view
// composition. The dialog is a bordered box centered (or anchored)
// horizontally within the terminal width.
func (o *overlayDialog) Render() string {
	theme := style.GetTheme()

	// Calculate dialog dimensions, clamped to terminal bounds.
	termW := max(o.width, 10)
	termH := max(o.height, 5)

	dw := o.dialogWidth
	if dw == 0 {
		dw = termW * 60 / 100
	}
	dw = clamp(dw, min(24, termW), termW-2)

	mh := o.maxHeight
	if mh == 0 {
		mh = termH * 80 / 100
	}
	mh = clamp(mh, min(6, termH), termH)

	// Inner width accounts for border (2) + horizontal padding (2 left + 1 right).
	// lipgloss's Width() sets the total rendered width including the border,
	// so the dialog style below is given the full dw and the content area is
	// what remains after the frame and padding.
	innerWidth := max(dw-5, 6)

	// Resolve the frame colour once: the border, the title and the scrollbar
	// thumb all share it, which is what makes the tint read as one signal
	// rather than three decorations.
	accent := o.frameColor(theme)

	// Chrome: border(2) + padTop(1) + padBottom(1) + blank(1) + footer(1) = 6.
	// The footer row carries the scroll indicator and the key hints; both
	// live inside the border so no part of the dialog is a bare strip of
	// padding that would erase the view behind it.
	chromeLines := 6
	if o.title != "" {
		chromeLines += 2 // title line + separator line
	}
	if len(o.actions) > 0 {
		chromeLines += 2 // separator line + action bar
	}
	if o.searching || o.query != "" {
		chromeLines++ // search line
	}

	maxBodyLines := max(mh-chromeLines, 1)

	// The scrollbar occupies the rightmost columns, so the text is laid out to
	// what remains. Whether the body scrolls is only knowable after wrapping,
	// and re-wrapping to a different width would change the row count that
	// decided it — so the reservation is made whenever a scrollbar is possible
	// at all, and the track is simply left blank when the body turns out to
	// fit.
	textWidth := innerWidth
	if o.scrollbar {
		textWidth = max(innerWidth-dialogScrollbarGap, 1)
	}

	rows := o.resolveBody(textWidth)
	o.totalLines = len(rows)
	o.pageSize = maxBodyLines

	// Resolve search hits against the wrapped rows, then pull the focused
	// match into view. Both have to happen here: row indices only exist once
	// the body has been wrapped to this frame's width.
	o.refreshHits(rows)

	scrollable := len(rows) > maxBodyLines
	visible := rows
	if scrollable {
		// Clamp scroll offset.
		maxOff := len(rows) - maxBodyLines
		o.scrollOff = clamp(o.scrollOff, 0, maxOff)
		visible = rows[o.scrollOff : o.scrollOff+maxBodyLines]
	} else {
		o.scrollOff = 0
	}

	// Build the content to render inside the border.
	var parts []string

	// Title row: icon + title on the left, meta on the right.
	if o.title != "" {
		parts = append(parts, o.renderTitle(innerWidth, accent, theme))
		parts = append(parts, lipgloss.NewStyle().
			Foreground(theme.Muted).
			Render(repeatRune('─', innerWidth)))
	}

	// Search line, directly above the body it filters.
	if o.searching || o.query != "" {
		parts = append(parts, o.renderSearchLine(innerWidth, accent, theme))
	}

	// Body content.
	parts = append(parts, "")
	parts = append(parts, o.renderBody(visible, textWidth, innerWidth, scrollable, maxBodyLines, accent, theme))

	// Action bar.
	if len(o.actions) > 0 {
		parts = append(parts, lipgloss.NewStyle().
			Foreground(theme.Muted).
			Render(repeatRune('─', innerWidth)))

		var actionParts []string
		for i, a := range o.actions {
			if i == o.selAction {
				actionParts = append(actionParts,
					lipgloss.NewStyle().Bold(true).Foreground(theme.Accent).Render("> "+a))
			} else {
				actionParts = append(actionParts,
					lipgloss.NewStyle().Foreground(theme.Text).Render("  "+a))
			}
		}
		parts = append(parts, strings.Join(actionParts, "    "))
	}

	// Footer: scroll position on the left, key hints on the right.
	parts = append(parts, o.renderFooter(scrollable, maxBodyLines, innerWidth, theme))

	innerContent := strings.Join(parts, "\n")

	// Build the dialog box style. Width() is the total rendered width
	// including the border, so dw is passed through unmodified — subtracting
	// the border here would shrink the content area below innerWidth and
	// wrap the title separator and action bar onto extra lines.
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Width(dw).
		Padding(1, 1, 1, 2).
		Foreground(theme.Text)

	// The dialog fills its own surface. Without a background the box reads as
	// a hole punched in the transcript rather than a panel floating above it,
	// and stray scrollback text shows through wherever a body row falls short
	// of the full width. theme.Background is one step off the terminal's own,
	// which is what the session and model pickers use.
	switch {
	case o.background != "":
		dialogStyle = dialogStyle.Background(lipgloss.Color(o.background))
	default:
		dialogStyle = dialogStyle.Background(theme.Background)
	}

	dialog := dialogStyle.Render(innerContent)

	// The box is returned unpositioned; the caller composites it over the
	// main view at the anchor point (see compositeAnchored). Positioning
	// here by padding with spaces and newlines would produce an opaque
	// full-screen block that erases whatever is behind it.
	return dialog
}

// resolveBody produces the wrapped body rows for the given text width,
// memoizing the result.
//
// Wrapping happens here rather than being left to the box style. The style
// would wrap over-long lines itself, at which point one source line occupies
// several rows and every downstream number is wrong: totalLines under-counts,
// the height budget lets through more rows than it allowed (a grep result with
// long paths grew a 30-row terminal's dialog to 48 rows), and scrolling moves
// by source line rather than by the row the reader actually sees. Wrapping here
// makes one row equal exactly one rendered line, so the budget, the offset and
// the "of N lines" counter all agree with the display.
func (o *overlayDialog) resolveBody(textWidth int) []bodyRow {
	gen := style.ThemeGeneration()
	if o.bodyCacheValid && o.bodyCacheWidth == textWidth && o.bodyCacheGen == gen {
		return o.bodyCache
	}

	bodyText := o.content
	numbered := false
	switch {
	case o.body != nil:
		// A body renderer lays its output out to exactly textWidth, tabs
		// included, so it must not be touched afterwards — expanding a tab in
		// a finished row pushes it past the width and wrapBodyRows then splits
		// every line in two. Renderers are handed tab-free input instead.
		bodyText = strings.TrimRight(o.body(textWidth), "\n")
	default:
		if o.markdown {
			bodyText = style.ToMarkdown(bodyText, textWidth)
		}
		bodyText = strings.TrimRight(bodyText, "\n")

		// Expand tabs before measuring. Wrapping counts a tab as one cell but
		// the box style renders it as several, so a line wrapped to exactly
		// the content width would overflow and be wrapped a second time by the
		// style — silently doubling the height of tab-indented output such as
		// grep matches. Expanding first makes the two agree.
		bodyText = expandTabs(bodyText, dialogTabWidth)

		numbered = o.lineNumbers && !o.markdown
	}

	// The gutter is sized from the source line count, which is known before
	// wrapping. Sizing it from the wrapped row count instead would be circular:
	// the count depends on the width, which depends on the gutter.
	o.bodyGutter = 0
	if numbered {
		digits := len(strconv.Itoa(strings.Count(bodyText, "\n") + 1))
		o.bodyGutter = digits + gutterSepWidth() + 1
	}

	o.bodyCache = wrapBodyRows(bodyText, textWidth-o.bodyGutter, o.wrapMarks)
	o.bodyCacheWidth = textWidth
	o.bodyCacheGen = gen
	o.bodyCacheValid = true
	return o.bodyCache
}

// frameColor resolves the colour shared by the border, the title and the
// scrollbar thumb. An explicit borderColor (extension overlays) wins, then the
// role accent the inspector supplies, then the generic dialog amber.
func (o *overlayDialog) frameColor(theme style.Theme) color.Color {
	if o.borderColor != "" {
		return lipgloss.Color(o.borderColor)
	}
	if o.accent != nil {
		return o.accent
	}
	return theme.Info
}

// renderTitle builds the title row: the role glyph and title on the left, the
// meta annotation right-aligned. The meta is dropped rather than wrapped when
// the two cannot share the row — a title that spills onto a second line throws
// the height budget off by one.
func (o *overlayDialog) renderTitle(innerWidth int, accent color.Color, theme style.Theme) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(accent)

	left := o.title
	if o.icon != "" {
		left = o.icon + " " + o.title
	}
	rendered := titleStyle.Render(left)

	if o.meta == "" {
		return rendered
	}

	metaStyle := lipgloss.NewStyle().Foreground(theme.VeryMuted)
	gap := innerWidth - lipgloss.Width(left) - lipgloss.Width(o.meta)
	if gap < 2 {
		return rendered
	}
	return rendered + strings.Repeat(" ", gap) + metaStyle.Render(o.meta)
}

// renderSearchLine draws the query line. While the query is being typed it
// carries a cursor block; once committed it reports the match count so a
// query that found nothing says so instead of appearing to have done nothing.
func (o *overlayDialog) renderSearchLine(innerWidth int, accent color.Color, theme style.Theme) string {
	promptStyle := lipgloss.NewStyle().Foreground(accent)
	queryStyle := lipgloss.NewStyle().Foreground(theme.Text)
	countStyle := lipgloss.NewStyle().Foreground(theme.VeryMuted)

	left := promptStyle.Render("/") + queryStyle.Render(o.query)
	plainWidth := 1 + lipgloss.Width(o.query)

	if o.searching {
		left += lipgloss.NewStyle().Foreground(accent).Render("▏")
		plainWidth++
	}

	var count string
	switch {
	case o.query == "":
	case len(o.hits) == 0:
		count = "no matches"
	default:
		count = fmt.Sprintf("%d/%d", o.hitIdx+1, len(o.hits))
	}
	if count == "" {
		return left
	}

	gap := innerWidth - plainWidth - lipgloss.Width(count)
	if gap < 2 {
		return left
	}
	return left + strings.Repeat(" ", gap) + countStyle.Render(count)
}

// renderBody draws the visible body rows, each padded to the text width, with
// the line-number gutter down the left and the scrollbar track down the right.
//
// Rows are padded explicitly rather than left for the box style to pad: the
// scrollbar has to land in a fixed column on every row, and a row whose text
// falls short would otherwise pull the track left.
func (o *overlayDialog) renderBody(visible []bodyRow, textWidth, innerWidth int, scrollable bool, maxBodyLines int, accent color.Color, theme style.Theme) string {
	contStyle := lipgloss.NewStyle().Foreground(theme.VeryMuted)
	gutterStyle := lipgloss.NewStyle().Foreground(theme.VeryMuted)
	thumbStyle := lipgloss.NewStyle().Foreground(accent)
	trackStyle := lipgloss.NewStyle().Foreground(theme.MutedBorder)
	hitStyle := lipgloss.NewStyle().Foreground(theme.Background).Background(accent)

	thumbStart, thumbEnd := o.thumbRange(maxBodyLines)
	rowWidth := textWidth - o.bodyGutter

	out := make([]string, 0, len(visible))
	for i, row := range visible {
		text := row.text
		if o.query != "" {
			text = highlightMatches(text, o.query, hitStyle)
		}

		// The continuation marker is rendered, not wrapped into, the text:
		// the row was wrapped to rowWidth-2 precisely to leave room here.
		if row.cont {
			text = contStyle.Render(dialogContMarker) + text
		}

		if pad := rowWidth - lipgloss.Width(row.text) - o.contWidth(row); pad > 0 {
			text += strings.Repeat(" ", pad)
		}

		if o.bodyGutter > 0 {
			text = gutterStyle.Render(o.gutterCell(row)) + text
		}

		if !o.scrollbar {
			out = append(out, text)
			continue
		}

		// The gap column, then the track. When the body fits, the reserved
		// columns stay blank rather than shifting the text back — a body
		// that grows past the viewport would otherwise re-wrap and jump.
		bar := " "
		if scrollable {
			if i >= thumbStart && i < thumbEnd {
				bar = thumbStyle.Render(dialogScrollThumb)
			} else {
				bar = trackStyle.Render(dialogScrollTrack)
			}
		}
		out = append(out, text+" "+bar)
	}

	return strings.Join(out, "\n")
}

// gutterSepWidth is the display width of the gutter separator.
//
// It must be measured rather than counted in bytes: the separator is a
// box-drawing character, so len() reports three for a glyph that draws one. The
// sizing and the rendering both used len(), so they agreed and the geometry
// held — but every gutter carried two dead columns, and the two would have
// disagreed the moment either was corrected on its own.
func gutterSepWidth() int {
	return lipgloss.Width(dialogGutterSep)
}

// gutterCell renders one line-number cell, exactly bodyGutter columns wide.
// Continuation rows get a blank number but keep the separator, so the gutter
// reads as a continuous rule and the numbers count source lines rather than
// screen rows.
func (o *overlayDialog) gutterCell(row bodyRow) string {
	digits := o.bodyGutter - gutterSepWidth() - 1
	if digits < 1 {
		return strings.Repeat(" ", max(o.bodyGutter, 0))
	}

	num := strings.Repeat(" ", digits)
	if !row.cont {
		num = fmt.Sprintf("%*d", digits, row.num)
	}
	return num + dialogGutterSep + " "
}

// contWidth returns the columns a row's continuation marker occupies.
func (o *overlayDialog) contWidth(row bodyRow) int {
	if row.cont {
		return lipgloss.Width(dialogContMarker)
	}
	return 0
}

// thumbRange returns the half-open range of visible row indices covered by the
// scrollbar thumb. The thumb is at least one row tall so it never vanishes on
// a very long body, and it reaches the bottom of the track exactly when the
// body is scrolled to its end.
func (o *overlayDialog) thumbRange(viewport int) (int, int) {
	if viewport <= 0 || o.totalLines <= viewport {
		return 0, viewport
	}

	size := max(viewport*viewport/o.totalLines, 1)
	maxOff := o.totalLines - viewport
	travel := viewport - size

	start := 0
	if maxOff > 0 && travel > 0 {
		// Rounded so the thumb only sits at the extremes when the body
		// actually is: integer truncation would park it at the top for the
		// first several rows of scrolling.
		start = (o.scrollOff*travel + maxOff/2) / maxOff
	}
	start = clamp(start, 0, travel)
	return start, start + size
}

// refreshHits recomputes the match list against rows and pulls the focused
// match into view.
//
// Matching is done on the stripped text so a query is not defeated by the
// escape codes woven through a highlighted or markdown-rendered row.
func (o *overlayDialog) refreshHits(rows []bodyRow) {
	if o.query == "" {
		o.hits = nil
		o.hitIdx = 0
		return
	}

	needle := strings.ToLower(o.query)
	prev := len(o.hits)
	o.hits = nil
	for i, row := range rows {
		if strings.Contains(strings.ToLower(xansi.Strip(row.text)), needle) {
			o.hits = append(o.hits, i)
		}
	}

	if len(o.hits) == 0 {
		o.hitIdx = 0
		return
	}
	o.hitIdx = clamp(o.hitIdx, 0, len(o.hits)-1)

	// A query that just gained its first matches (or was retyped from
	// scratch) jumps to one; an existing query being re-rendered must not
	// yank the viewport away from the reader.
	if prev == 0 {
		o.hitIdx = o.firstHitAtOrAfter(o.scrollOff)
		o.revealRow(o.hits[o.hitIdx])
	}
}

// firstHitAtOrAfter returns the index into hits of the first match at or below
// row, wrapping to the first match when every hit is above it.
func (o *overlayDialog) firstHitAtOrAfter(row int) int {
	for i, h := range o.hits {
		if h >= row {
			return i
		}
	}
	return 0
}

// wrapBodyRows wraps text to width, tagging rows that came from wrapping
// rather than from a newline in the source, and recording the source line each
// row belongs to.
//
// A line that needs wrapping is wrapped a second time, to a budget short by
// the continuation marker, so the marker can be prefixed without pushing the
// row over the width. Lines that fit — the overwhelming majority — are wrapped
// once and keep the full width.
func wrapBodyRows(text string, width int, marks bool) []bodyRow {
	if width < 1 {
		width = 1
	}
	contWidth := max(width-lipgloss.Width(dialogContMarker), 1)

	var rows []bodyRow
	for num, src := range strings.Split(text, "\n") {
		segs := strings.Split(xansi.Wrap(src, width, ""), "\n")
		if marks && len(segs) > 1 {
			segs = strings.Split(xansi.Wrap(src, contWidth, ""), "\n")
		}
		for i, seg := range segs {
			rows = append(rows, bodyRow{
				text: seg,
				cont: marks && i > 0,
				num:  num + 1,
			})
		}
	}
	return rows
}

// highlightMatches wraps every occurrence of query in s with hitStyle.
//
// Only ANSI-free rows are highlighted. Splicing a style into a row that
// already carries escape codes would terminate the surrounding sequence and
// bleed colour across the rest of the line; for those rows the position
// indicator and the match counter carry the information instead. Tool output —
// where searching actually earns its keep — is plain text.
func highlightMatches(s, query string, hitStyle lipgloss.Style) string {
	if query == "" || s == "" || strings.Contains(s, "\x1b") {
		return s
	}

	lowerS := strings.ToLower(s)
	lowerQ := strings.ToLower(query)

	var b strings.Builder
	for {
		i := strings.Index(lowerS, lowerQ)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString(hitStyle.Render(s[i : i+len(query)]))
		s = s[i+len(query):]
		lowerS = lowerS[i+len(query):]
	}
}

// renderFooter builds the dialog's bottom row: the scroll position on the
// left and the key hints on the right, padded to exactly innerWidth.
//
// The row lives inside the border. Rendering hints below the box would leave
// a bare band of padding that the compositor draws as opaque cells, cutting a
// blank strip through the content behind the dialog.
//
// Both halves compete for one row, so the content degrades by priority rather
// than being dropped outright: the verbose "(1–12 of 200 lines)" collapses to
// "12/200", and the "↑/↓ scroll" hint goes before any dismiss hint because a
// visible position indicator already implies the dialog scrolls. The keys
// that close the dialog are the last thing surrendered.
func (o *overlayDialog) renderFooter(scrollable bool, maxBodyLines, innerWidth int, theme style.Theme) string {
	mutedStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	veryMutedStyle := lipgloss.NewStyle().Foreground(theme.VeryMuted)

	var indicators []string

	// A copy acknowledgement displaces the position indicator: it is
	// transient and answers a question the reader just asked, whereas the
	// position is always recoverable by looking at the scrollbar.
	if o.copied {
		indicators = append(indicators, "copied to clipboard", "copied")
	} else if scrollable {
		last := min(o.scrollOff+maxBodyLines, o.totalLines)
		indicators = append(indicators,
			fmt.Sprintf("(%d–%d of %d lines)", o.scrollOff+1, last, o.totalLines),
			fmt.Sprintf("%d/%d", last, o.totalLines),
		)
	}
	indicators = append(indicators, "")

	// Hint sets, widest first.
	hintSets := [][]string{o.hintLabels(scrollable, true), o.hintLabels(scrollable, false)}

	for _, hints := range hintSets {
		hintText := strings.Join(hints, "  ")
		hintW := lipgloss.Width(hintText)

		for _, indicator := range indicators {
			if indicator == "" {
				continue
			}
			if gap := innerWidth - lipgloss.Width(indicator) - hintW; gap >= 2 {
				return veryMutedStyle.Render(indicator) +
					strings.Repeat(" ", gap) +
					mutedStyle.Render(hintText)
			}
		}

		// Hints alone, right-aligned.
		if pad := innerWidth - hintW; pad >= 0 {
			return strings.Repeat(" ", pad) + mutedStyle.Render(hintText)
		}
	}

	// Narrower than even the terse hints — show them and let the box clip.
	return mutedStyle.Render(strings.Join(o.hintLabels(scrollable, false), "  "))
}

// hintLabels returns the key hints for the dialog's current configuration.
// verbose selects the full wording over the terse forms used when the row is
// too narrow to spell the keys out.
//
// Hints are ordered least-essential-first because renderFooter drops the
// widest set before the narrow one, never individual entries: what survives a
// narrow dialog is the tail of this list.
func (o *overlayDialog) hintLabels(scrollable, verbose bool) []string {
	if o.searching {
		if verbose {
			return []string{"Enter find", "Esc cancel"}
		}
		return []string{"↵ find", "esc"}
	}

	var hints []string

	if verbose {
		if scrollable {
			hints = append(hints, "↑/↓ scroll", "PgUp/PgDn page")
		}
		if len(o.hits) > 0 {
			hints = append(hints, "n/N match")
		} else if o.searchable() {
			hints = append(hints, "/ find")
		}
		if o.copyText != "" {
			hints = append(hints, "y copy")
		}
		switch {
		case len(o.actions) > 0:
			hints = append(hints, "←/→ switch", "Enter select", "Esc cancel")
		case o.dismissOnly:
			hints = append(hints, "Enter/Esc close")
		default:
			hints = append(hints, "Enter dismiss", "Esc cancel")
		}
		return hints
	}

	if len(o.hits) > 0 {
		hints = append(hints, "n/N")
	} else if o.searchable() {
		hints = append(hints, "/")
	}
	if o.copyText != "" {
		hints = append(hints, "y copy")
	}
	switch {
	case len(o.actions) > 0:
		hints = append(hints, "↵ select", "esc")
	case o.dismissOnly:
		hints = append(hints, "↵/esc close")
	default:
		hints = append(hints, "↵ ok", "esc")
	}
	return hints
}

// Anchor returns the configured vertical anchor for this dialog.
func (o *overlayDialog) Anchor() string {
	return o.anchor
}
