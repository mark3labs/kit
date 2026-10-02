package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/style"
)

// ShellRunItem is the transcript block for one bang-mode shell command — the
// `!cmd` and `!!cmd` prefixes in the composer.
//
// It exists in two states. While the command runs, the output grows as bytes
// arrive and the visible window follows the end, because the newest line is the
// one the user is waiting for. Once the command finishes, the window snaps back
// to the start and an expansion control reveals everything, because a finished
// command is read from the top like any other output.
//
// Colour is kept. The command's own escape sequences are remapped onto the
// active theme and drawn inside the block, which is why the body is styled one
// line at a time: a program is free to end a line with `\x1b[0m`, and that
// reset would clear any style wrapped around the whole block.
type ShellRunItem struct {
	id        string
	command   string
	output    strings.Builder
	exitCode  int
	pending   bool
	timedOut  bool
	excluded  bool
	expanded  bool
	hOffset   int
	maxColumn int

	// newlines counts the line breaks in output. It is maintained on append
	// rather than counted on demand, because counting walks the whole buffer and
	// the buffer grows on every chunk.
	newlines int

	// window is the normalized visible window, and windowSize the output length
	// it was built from. The render path runs on every frame while output
	// arrives, and the window only changes when the output does, so caching it
	// turns a per-frame parse into a per-chunk one.
	window       string
	windowHidden int
	windowSize   int
	windowValid  bool
	// windowByteCapped records that the expanded view ran into its byte budget,
	// so the notice has to say so rather than claim only lines were dropped.
	windowByteCapped bool

	cachedRender string
	cachedWidth  int
	cachedOutLen int
	cachedValid  bool
	themeStamp
}

var (
	_ MessageItem     = (*ShellRunItem)(nil)
	_ InspectableItem = (*ShellRunItem)(nil)
)

// shellCollapsedLines is how many output lines the block shows before the user
// expands it. The cap is small on purpose: a live block that grows to the
// height of the viewport pushes the conversation above it out of sight on every
// frame, and a finished command that fills the screen hides the turn it belongs
// to.
const shellCollapsedLines = 12

// shellHScrollStep is the number of columns a horizontal scroll moves. Program
// output has long lines — a wide table, a single-line JSON blob — and truncation
// alone throws away the right-hand end, where the values are.
const shellHScrollStep = 5

// shellBlockChrome is how many columns the shell block spends before any text:
// the gutter glyph, the indent beside it, the row's own left padding inside the
// panel, and the right margin.
const shellBlockChrome = style.ContentOffset + 1 + style.RightMargin

// shellWindowMaxBytes bounds the window when it has to be cut on a byte rather
// than a line. A command can emit one enormous line — a minified bundle, a
// single-line JSON document — and normalizing all of it would put milliseconds
// on every frame for a block that shows a screenful.
const shellWindowMaxBytes = 64 << 10

// shellExpandedMaxLines is how many lines the expanded view shows.
//
// Expanding is a deliberate user action, so the budget is far larger than the
// collapsed one. It is still a budget, and it is a line budget rather than a
// byte budget because the cost is per line: each one is measured, truncated and
// styled, and sixty thousand of those put seconds on a frame.
const shellExpandedMaxLines = 400

// shellExpandedMaxBytes bounds the expanded view by size as well, for the same
// reason lastWindowBytes bounds the collapsed one: one enormous line must not be
// normalized whole.
const shellExpandedMaxBytes = 256 << 10

// NewShellRunItem creates a block for a command that is about to start. The item
// is pending until MarkComplete or MarkFailed is called.
func NewShellRunItem(id, command string, excludedFromContext bool) *ShellRunItem {
	return &ShellRunItem{
		id:       id,
		command:  command,
		pending:  true,
		excluded: excludedFromContext,
	}
}

// ID returns the item's stable identifier.
func (s *ShellRunItem) ID() string { return s.id }

// Role returns the message role, which the inspector uses to label the block.
func (s *ShellRunItem) Role() string { return "shell" }

// Spinning reports whether the command is still running, so the transcript
// advances the item on its animation tick.
func (s *ShellRunItem) Spinning() bool { return s.pending }

// AppendOutput adds streamed output. It runs on the Bubble Tea goroutine, so the
// item needs no lock of its own.
func (s *ShellRunItem) AppendOutput(chunk string) {
	if !s.pending || chunk == "" {
		return
	}
	s.output.WriteString(chunk)
	s.newlines += strings.Count(chunk, "\n")
	s.invalidate()
}

// MarkComplete ends the block with the exit code and the final output.
//
// final replaces the streamed text rather than appending to it, because the
// runner collected the same bytes: appending would print everything twice.
func (s *ShellRunItem) MarkComplete(output string, exitCode int) {
	s.setOutput(output)
	s.exitCode = exitCode
	s.pending = false
	s.invalidate()
}

// MarkTimedOut ends the block because the command exceeded its timeout. The
// output collected before the kill is kept, so the user can see how far it got.
func (s *ShellRunItem) MarkTimedOut(output string) {
	s.setOutput(output)
	s.exitCode = 124
	s.timedOut = true
	s.pending = false
	s.invalidate()
}

// MarkFailed ends the block because the command never started.
func (s *ShellRunItem) MarkFailed(message string) {
	s.setOutput(message)
	s.exitCode = -1
	s.pending = false
	s.invalidate()
}

// setOutput replaces the retained output and recounts the line breaks.
func (s *ShellRunItem) setOutput(output string) {
	s.output.Reset()
	s.output.WriteString(output)
	s.newlines = strings.Count(output, "\n")
}

// ToggleExpanded flips between the capped window and the full output. It
// reports the new state so the caller can invalidate what it cached.
func (s *ShellRunItem) ToggleExpanded() bool {
	s.expanded = !s.expanded
	s.invalidate()
	return s.expanded
}

// Expanded reports whether the full output is shown.
func (s *ShellRunItem) Expanded() bool { return s.expanded }

// ScrollHorizontalBy moves the visible column window by one step in the given
// direction: negative scrolls left towards the start of the line, positive
// scrolls right. It reports whether the window moved, so a key handler can pass
// the key on when it was already at its limit.
func (s *ShellRunItem) ScrollHorizontalBy(direction int) bool {
	if s.hOffset == 0 && direction < 0 {
		return false
	}
	before := s.hOffset
	s.hOffset = max(0, s.hOffset+direction*shellHScrollStep)
	if s.maxColumn > 0 {
		s.hOffset = min(s.hOffset, s.maxColumn)
	}
	if s.hOffset == before {
		return false
	}
	s.invalidate()
	return true
}

// CopyText returns the block as plain text for the clipboard: the header, then
// the output with every escape sequence removed. A clipboard cannot carry the
// styling a user cannot select anyway.
func (s *ShellRunItem) CopyText() string {
	return s.RawContent()
}

func (s *ShellRunItem) invalidate() {
	s.cachedValid = false
	s.windowValid = false
}

// visibleWindow returns the normalized text the block draws, and how many lines
// it left out.
//
// Which end is shown depends on the state: live output follows the end,
// because the newest line is what the user is waiting for; finished output
// starts at the beginning, because a finished command is read like any other.
//
// The result is cached against the output length. Every state change clears the
// cache, so a window is rebuilt at most once per chunk rather than once per
// frame.
func (s *ShellRunItem) visibleWindow(raw string) (string, int) {
	if s.windowValid && s.windowSize == s.output.Len() {
		return s.window, s.windowHidden
	}

	total := s.newlines + 1
	window := raw
	hidden := 0
	byteCapped := false

	switch {
	case s.expanded:
		window = firstWindowBytes(style.FirstLines(raw, shellExpandedMaxLines), shellExpandedMaxBytes)
		if window != raw {
			hidden = max(0, total-(strings.Count(window, "\n")+1))
			byteCapped = true
		}
	case s.pending:
		hidden = max(0, total-shellCollapsedLines)
		window = lastWindowBytes(raw, shellCollapsedLines, shellWindowMaxBytes)
	default:
		if total > shellCollapsedLines {
			hidden = total - shellCollapsedLines
			window = style.FirstLines(raw, shellCollapsedLines)
		}
	}

	// The reset closes any colour the command opened before the window began.
	// Without it, a colour that spans the cut would bleed across the whole block.
	window = style.NormalizeOutput(style.CompleteANSIFrom("\x1b[0m" + window))

	s.window = window
	s.windowHidden = hidden
	s.windowByteCapped = byteCapped
	s.windowSize = s.output.Len()
	s.windowValid = true
	return window, hidden
}

// firstWindowBytes returns the leading window of raw: at most maxBytes.
//
// Like lastWindowBytes it prefers a line boundary, for the same reason. The
// budget is a ceiling rather than a target: the window comes out shorter when
// honouring a boundary costs a few bytes, and that is the right trade against a
// cut that lands inside an escape sequence.
func firstWindowBytes(raw string, maxBytes int) string {
	if len(raw) <= maxBytes {
		return raw
	}

	end := maxBytes
	if nl := strings.LastIndexByte(raw[:end], '\n'); nl >= 0 {
		return raw[:nl+1]
	}

	// One enormous first line. Snap forward to the next rune boundary.
	for end < len(raw) && !utf8.RuneStart(raw[end]) {
		end++
	}
	return raw[:end]
}

// lastWindowBytes returns the trailing window of raw: at most count lines, and
// never more than maxBytes.
//
// Lines are the preferred cut because an escape sequence never spans a newline,
// so a line boundary cannot land inside one. The byte cap only applies when a
// single line exceeds it, and the cut then snaps forward to a rune boundary and
// a leading sequence fragment, which is what style.CompleteANSIFrom removes.
func lastWindowBytes(raw string, count, maxBytes int) string {
	if raw == "" {
		return ""
	}

	// A trailing newline terminates the last line rather than separating two, so
	// it must not consume one of the count. Without this the window is one line
	// short whenever the command ended its output with a newline, which is most
	// of the time.
	end := len(raw)
	if raw[end-1] == '\n' {
		end--
	}

	start := 0
	if count > 0 && end > 0 {
		start = end
		complete := true
		for range count {
			i := strings.LastIndexByte(raw[:start], '\n')
			if i < 0 {
				complete = false
				break
			}
			start = i
		}
		if complete {
			// start sits on the newline that ends the line above the window.
			start++
		} else {
			// Fewer than count lines exist, so the window is the whole output.
			start = 0
		}
	}

	if len(raw)-start <= maxBytes {
		return raw[start:]
	}

	// Too many bytes. Prefer a newline inside the budget: it keeps the cut on a
	// line boundary, where an escape sequence cannot straddle it.
	cut := len(raw) - maxBytes
	if raw[cut] == '\n' {
		cut++
	} else if nl := strings.IndexByte(raw[cut:], '\n'); nl >= 0 && nl < maxBytes {
		cut += nl + 1
	}

	// One enormous line. Snap forward to the next rune boundary so the cut does
	// not split a multi-byte character.
	for cut < len(raw) && !utf8.RuneStart(raw[cut]) {
		cut++
	}
	return raw[cut:]
}

// RawContent returns the header and the complete output, without the display
// cap, for the message inspector.
func (s *ShellRunItem) RawContent() string {
	var b strings.Builder
	b.WriteString(s.header())
	out := s.output.String()
	if out != "" {
		b.WriteString("\n\n")
		b.WriteString(out)
	} else {
		b.WriteString("\n\n(no output)")
	}
	if s.timedOut {
		b.WriteString("\n\n(timed out)")
	} else if s.exitCode != 0 {
		fmt.Fprintf(&b, "\n\nExit code: %d", s.exitCode)
	}
	return b.String()
}

// Height returns the number of lines the item occupies at this width.
func (s *ShellRunItem) Height() int {
	rendered := s.cachedRender
	if !s.cachedValid || rendered == "" {
		rendered = s.Render(0)
	}
	if rendered == "" {
		return 0
	}
	return strings.Count(rendered, "\n") + 1
}

// Render returns the styled block.
//
// The result is cached against the width and the output length. Nothing else in
// the block changes over time — the running marker is static text, not an
// animation — so a frame that arrives without new output is free. That matters
// because the transcript redraws at display rate while a command runs, and most
// of those frames carry no new bytes.
func (s *ShellRunItem) Render(width int) string {
	// The cache holds colour codes baked in by the theme that produced it, so a
	// theme switch has to drop it.
	if s.stale() {
		s.invalidate()
		s.stamp()
	}
	if s.cachedValid && s.cachedWidth == width && s.cachedOutLen == s.output.Len() {
		return s.cachedRender
	}

	out := s.render(width)
	s.cachedRender = out
	s.cachedWidth = width
	s.cachedOutLen = s.output.Len()
	s.cachedValid = true
	return out
}

// header is the `$ command` line, plus the context marker and the exit status.
func (s *ShellRunItem) header() string {
	styles := style.GetCachedStyles()

	header := "$ " + s.command
	if s.excluded {
		// The marker is part of the line rather than a separate badge, so it
		// survives a copy and cannot be lost to truncation.
		header += "  (excluded from context)"
	}
	switch {
	case s.pending:
		return styles.BashHeader.Render(header)
	case s.timedOut:
		return styles.ToolError.Render(header + "  (timed out)")
	case s.exitCode != 0:
		return styles.ToolError.Render(fmt.Sprintf("%s  (exit %d)", header, s.exitCode))
	default:
		return styles.BashHeader.Render(header)
	}
}

func (s *ShellRunItem) render(width int) string {
	// The header can be arbitrarily long — a pasted one-liner — so it is
	// clamped here. Nothing else in the block is drawn without a budget, and an
	// unclamped header pushes the frame wider than the terminal.
	header := xansi.Truncate(s.header(), max(width-style.ContentOffset, 1), "…")

	parts := []string{header}

	if body := s.renderOutput(width); body != "" {
		parts = append(parts, "", body)
	}

	// The running marker is pinned below the output, so the eye stays on the
	// line being written to instead of on a fixed position the output keeps
	// pushing down.
	if s.pending {
		parts = append(parts, style.GetCachedStyles().SpinnerDim.Render("running…"))
	}

	return strings.Join(parts, "\n") + "\n" + strings.Repeat("\n", style.BlockGap)
}

// renderOutput draws the visible window of the command output, one line at a
// time, inside the standard gutter block.
func (s *ShellRunItem) renderOutput(width int) string {
	raw := style.TrimTrailingResets(s.output.String())
	if raw == "" {
		return ""
	}

	// The block spends four columns before any text: the border glyph, the
	// gutter indent, the row's own left padding, and the right margin. What is
	// left is the text budget.
	//
	// The floor is 1 rather than style.MinContentWidth. That constant exists for
	// the tool panels, which are always drawn inside a wider block; here the
	// width is the whole terminal, and honouring a 20-column floor at a
	// 20-column terminal would push the row off screen.
	inner := max(width-shellBlockChrome, 1)

	// Only the window is normalized. Running the pipeline over everything the
	// command ever printed would make a live block quadratic: at a megabyte of
	// coloured output the whole-buffer pass costs milliseconds, and it runs again
	// on the next chunk and the one after. A window is a screenful, so the cost
	// is flat no matter how much the command produces.
	//
	// The window is cut from the raw text, at a line boundary wherever there is
	// one. Escape sequences never span a newline, so a line-start cut cannot land
	// inside one; the byte fallback below is the only case that can, and
	// CompleteANSIFrom is there for it.
	normalized, hidden := s.visibleWindow(raw)

	lines := strings.Split(normalized, "\n")

	truncates := hidden > 0

	// Measure the widest line before truncating, or the horizontal scroll would
	// never have anything to reveal.
	widest := 0
	for _, ln := range lines {
		if w := xansi.StringWidth(ln); w > widest {
			widest = w
		}
	}
	// The scroll marker occupies one of the visible columns, so the last offset
	// that still shows the end of the line is one past the difference. Without
	// the correction the final characters are cut by one column every time.
	s.maxColumn = max(0, widest-inner+1)

	lineStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		Background(style.GetTheme().CodeBg).
		Width(inner)

	var body strings.Builder

	// While the command runs the hidden lines are above the window, so the
	// notice goes first. Once it finishes they are below, so it goes last.
	if truncates && s.pending {
		body.WriteString(s.noticeLine(lineStyle, s.hiddenNotice(hidden)))
	}

	for _, ln := range lines {
		body.WriteString(lineStyle.Render(s.fitLine(ln, inner)))
		body.WriteString("\n")
	}

	if truncates && !s.pending {
		body.WriteString(s.noticeLine(lineStyle, s.hiddenNotice(hidden)))
		body.WriteString("\n")
	}

	// The block carries no foreground of its own. The program sets its own, and
	// a reset inside the output would clear anything wrapped around the whole
	// block anyway; only the border needs a colour.
	block := lipgloss.NewStyle().
		BorderStyle(style.GutterBorder()).
		BorderLeft(true).
		BorderLeftForeground(s.borderColor()).
		PaddingLeft(style.ContentOffset - 1).
		PaddingRight(style.RightMargin)

	return block.Render(strings.TrimRight(body.String(), "\n"))
}

func (s *ShellRunItem) noticeLine(lineStyle lipgloss.Style, text string) string {
	return lineStyle.Render(style.GetCachedStyles().VeryMuted.Render(text))
}

// hiddenNotice describes what the block is not showing. The word differs with
// the state because the hidden lines sit above a live window and below a
// finished one.
func (s *ShellRunItem) hiddenNotice(hidden int) string {
	word := "more lines"
	if s.pending {
		word = "earlier lines"
	}
	if s.windowByteCapped {
		// The expanded view is still bounded, so the reader needs somewhere else
		// to go. Copying hands over the retained output in full.
		return fmt.Sprintf("… %d %s not shown — press y to copy the full output", hidden, word)
	}
	return fmt.Sprintf("… %d %s", hidden, word)
}

// borderColor tints the gutter by outcome. An accent stripe means the output is
// in the conversation; a muted one means the user asked for it to stay out; a
// red one means it failed.
func (s *ShellRunItem) borderColor() color.Color {
	theme := style.GetTheme()
	switch {
	case s.excluded:
		return theme.MutedBorder
	case s.pending:
		return theme.Border
	case s.exitCode != 0:
		return theme.Error
	default:
		return theme.Accent
	}
}

// fitLine applies the horizontal scroll and then the width clamp to one line.
// Both operations are ANSI-aware, so a colour that was open at the cut point is
// closed rather than printed.
func (s *ShellRunItem) fitLine(line string, width int) string {
	if s.hOffset > 0 && xansi.StringWidth(line) > s.hOffset {
		// The marker records that the line continues to the left, so the visible
		// window is never mistaken for the start of the line. It counts as one
		// of the visible columns, which is why maxColumn leaves room for it.
		line = "…" + xansi.Cut(line, s.hOffset, math.MaxInt)
	}
	return xansi.Truncate(line, max(width, 1), "…")
}
