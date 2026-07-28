package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	udiff "github.com/aymanbagabas/go-udiff"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/indaco/herald"

	"github.com/mark3labs/kit/internal/ui/style"
)

// Maximum visible lines per tool type before truncation.
const (
	maxDiffLines  = 20 // side-by-side rows for Edit
	maxCodeLines  = 20 // lines for Read / code blocks
	maxWriteLines = 10 // lines for Write blocks
	maxBashLines  = 20 // lines for Bash output (matches Read)
	maxLsLines    = 20 // lines for Ls directory listings
)

// minDiffContent is the narrowest code column a diff panel is worth drawing.
// Below it the text is too clipped to compare against anything, so the layout
// switches to a unified diff rather than shrinking further.
const minDiffContent = 12

// isShellTool reports if the tool name matches a shell-like tool (bash or
// tools with "shell"/"command" in the name). Used by renderToolBody.
func isShellTool(toolName string) bool {
	return toolName == "bash" ||
		strings.Contains(toolName, "shell") || strings.Contains(toolName, "command")
}

// ---------------------------------------------------------------------------
// Shared tool body geometry
// ---------------------------------------------------------------------------

// toolIndent is the left indent applied to every tool body, placing it at the
// shared content column beneath the tool header's marker.
func toolIndent() string {
	return strings.Repeat(" ", style.ContentOffset)
}

// panelPadding is the interior left padding of a tool body panel. The panel's
// background begins at the content column and its text is inset by this much,
// so the fill reads as a surface rather than as text with a colored margin.
const panelPadding = 1

// toolPanel renders lines inside the background-filled panel shared by the
// bash, ls, find, grep and subagent bodies.
//
// Each of those renderers used to compute the same three quantities by hand —
// panel width, per-line truncation budget, indent — and they disagreed: some
// clamped the width and some did not, and renderBashBody clamped the value it
// computed but then rendered with the unclamped one, which goes negative on a
// very narrow terminal and makes lipgloss emit nothing at all. Deriving them
// once keeps the panels identical and keeps the arithmetic in one place.
type toolPanel struct {
	// width is the panel's total width, including its interior padding.
	width int
	// textWidth is what remains for text after the interior padding.
	textWidth int

	text lipgloss.Style
	err  lipgloss.Style
}

// newToolPanel builds a panel sized for a tool body rendered at the given
// body width (the width handed to the body renderer, already inside the
// tool block).
func newToolPanel(bodyWidth int) toolPanel {
	theme := GetTheme()
	width := max(bodyWidth-style.ContentOffset, style.MinContentWidth)
	return toolPanel{
		width:     width,
		textWidth: max(width-panelPadding, 1),
		text:      lipgloss.NewStyle().Background(theme.CodeBg).PaddingLeft(panelPadding).Width(width),
		err: lipgloss.NewStyle().Foreground(theme.Error).Background(theme.CodeBg).
			PaddingLeft(panelPadding).Width(width),
	}
}

// line renders one panel row, truncated to fit. The row is not indented:
// callers indent the finished block (content plus any caption) as a unit, so
// the caption lands in the same column as the panel above it.
//
// Truncation happens before styling because cutting a string that already
// holds ANSI escapes truncates the escapes with it.
func (p toolPanel) line(s string, isErr bool) string {
	s = truncateLine(s, p.textWidth)
	if isErr {
		return p.err.Render(s)
	}
	return p.text.Render(s)
}

// blank renders an empty panel row, used to separate sections without
// breaking the background fill.
func (p toolPanel) blank() string {
	return p.text.Render("")
}

// captionTypography returns a herald instance that places a muted caption
// beneath a figure. Every tool body that captions its output built this
// inline; they are identical.
func captionTypography() *herald.Typography {
	return herald.New(herald.WithTheme(herald.Theme{
		FigureCaption:         lipgloss.NewStyle().Foreground(GetTheme().Muted),
		FigureCaptionPosition: herald.CaptionBottom,
	}))
}

// renderToolBody dispatches to tool-specific body renderers based on tool name.
// Returns the styled body string, or empty string to fall back to default rendering.
func renderToolBody(toolName, toolArgs, toolResult string, width int) string {
	switch {
	case toolName == "edit":
		if body := renderEditBody(toolArgs, toolResult, width); body != "" {
			return body
		}
	case toolName == "ls":
		if body := renderLsBody(toolResult, width); body != "" {
			return body
		}
	case toolName == "read":
		if body := renderReadBody(toolArgs, toolResult, width); body != "" {
			return body
		}
	case toolName == "write":
		if body := renderWriteBody(toolArgs, toolResult, width); body != "" {
			return body
		}
	case toolName == "find":
		if body := renderFindBody(toolResult, width); body != "" {
			return body
		}
	case toolName == "grep":
		if body := renderGrepBody(toolResult, width); body != "" {
			return body
		}
	case isShellTool(toolName):
		if body := renderBashBody(toolArgs, toolResult, width); body != "" {
			return body
		}
	case toolName == "subagent":
		if body := renderSubagentBody(toolResult, width); body != "" {
			return body
		}
	}
	return "" // fall back to default
}

// ---------------------------------------------------------------------------
// Edit tool — side-by-side diff
// ---------------------------------------------------------------------------

// renderEditBody renders a side-by-side diff from the edits array in toolArgs.
func renderEditBody(toolArgs, toolResult string, width int) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
		return ""
	}

	// Try to extract the starting line number from the unified diff in the result
	startLine := extractDiffStartLine(toolResult)

	editsArr, ok := args["edits"].([]any)
	if !ok || len(editsArr) == 0 {
		return ""
	}

	var results []string
	for _, edit := range editsArr {
		if e, ok := edit.(map[string]any); ok {
			oldText, _ := e["old_text"].(string)
			newText, _ := e["new_text"].(string)
			if oldText != "" || newText != "" {
				diff := renderDiffBlock(oldText, newText, startLine, width)
				if diff != "" {
					results = append(results, diff)
				}
			}
		}
	}
	if len(results) > 0 {
		return strings.Join(results, "\n")
	}
	return ""
}

// diffHunkPattern matches the first @@ hunk header in a unified diff.
// Package-level so it compiles once, not on every edit-tool render.
var diffHunkPattern = regexp.MustCompile(`@@ -(\d+)`)

// totalLinesPattern extracts the total line count from a read-tool footer
// (e.g. "[showing lines 1-100 of 407 total...]"). Package-level so it
// compiles once, not per footer line per render.
var totalLinesPattern = regexp.MustCompile(`of (\d+) total`)

// extractDiffStartLine parses the first @@ hunk header from a unified diff
// result to find the starting line number. Returns 1 if not found.
func extractDiffStartLine(result string) int {
	matches := diffHunkPattern.FindStringSubmatch(result)
	if len(matches) >= 2 {
		if n, err := strconv.Atoi(matches[1]); err == nil && n > 0 {
			return n
		}
	}
	return 1
}

// splitLine holds one row of a side-by-side diff.
type splitLine struct {
	beforeNum  int
	afterNum   int
	beforeText string
	afterText  string
	beforeKind udiff.OpKind
	afterKind  udiff.OpKind
}

// renderDiffBlock renders old→new as a side-by-side diff with colored backgrounds.
func renderDiffBlock(before, after string, startLine int, width int) string {
	// Normalise tabs and ensure trailing newlines
	before = strings.ReplaceAll(before, "\t", "    ")
	after = strings.ReplaceAll(after, "\t", "    ")
	if before != "" && !strings.HasSuffix(before, "\n") {
		before += "\n"
	}
	if after != "" && !strings.HasSuffix(after, "\n") {
		after += "\n"
	}

	edits := udiff.Strings(before, after)
	if len(edits) == 0 {
		return "" // no changes
	}

	unified, err := udiff.ToUnifiedDiff("a", "b", before, edits, 3)
	if err != nil || len(unified.Hunks) == 0 {
		return ""
	}

	// Convert hunks to paired split-lines for side-by-side rendering.
	var lines []splitLine
	for hi, h := range unified.Hunks {
		beforeLine := h.FromLine + startLine - 1
		afterLine := h.ToLine + startLine - 1

		// Hunk separator between hunks
		if hi > 0 {
			lines = append(lines, splitLine{beforeKind: -1, afterKind: -1})
		}

		i := 0
		for i < len(h.Lines) {
			l := h.Lines[i]
			switch l.Kind {
			case udiff.Equal:
				lines = append(lines, splitLine{
					beforeNum: beforeLine, afterNum: afterLine,
					beforeText: l.Content, afterText: l.Content,
					beforeKind: udiff.Equal, afterKind: udiff.Equal,
				})
				beforeLine++
				afterLine++
				i++

			case udiff.Delete:
				// Collect consecutive deletes then inserts and pair them.
				var deletes, inserts []udiff.Line
				for i < len(h.Lines) && h.Lines[i].Kind == udiff.Delete {
					deletes = append(deletes, h.Lines[i])
					i++
				}
				for i < len(h.Lines) && h.Lines[i].Kind == udiff.Insert {
					inserts = append(inserts, h.Lines[i])
					i++
				}
				maxPairs := max(len(deletes), len(inserts))
				for j := range maxPairs {
					sl := splitLine{}
					if j < len(deletes) {
						sl.beforeNum = beforeLine
						sl.beforeText = deletes[j].Content
						sl.beforeKind = udiff.Delete
						beforeLine++
					}
					if j < len(inserts) {
						sl.afterNum = afterLine
						sl.afterText = inserts[j].Content
						sl.afterKind = udiff.Insert
						afterLine++
					}
					lines = append(lines, sl)
				}

			case udiff.Insert:
				lines = append(lines, splitLine{
					afterNum: afterLine, afterText: l.Content,
					afterKind: udiff.Insert,
				})
				afterLine++
				i++
			}
		}
	}

	if len(lines) == 0 {
		return ""
	}

	// Truncate to maxDiffLines visible rows
	var diffHiddenCount int
	if len(lines) > maxDiffLines {
		diffHiddenCount = len(lines) - maxDiffLines
		lines = lines[:maxDiffLines]
	}

	// Layout calculations
	availableWidth := width - style.ContentOffset

	// Gutter width from max line number
	maxLineNum := 1
	for _, l := range lines {
		if l.beforeNum > maxLineNum {
			maxLineNum = l.beforeNum
		}
		if l.afterNum > maxLineNum {
			maxLineNum = l.afterNum
		}
	}
	gutterWidth := max(len(fmt.Sprintf("%d", maxLineNum)), 3)

	// A side-by-side diff needs room for two gutters, two markers, two
	// columns of code and the divider between them. Below that it cannot be
	// drawn at all: the panels have minimum widths, and forcing them into a
	// narrow terminal produced rows wider than the screen, which wrap in the
	// emulator and throw off the scroll list's height accounting. A unified
	// diff says the same thing in one column, so narrow terminals get that
	// instead of a broken two-column layout.
	panelOverhead := gutterWidth + 4 // gutter, its leading space, and " - "
	minSideBySide := 2*(panelOverhead+minDiffContent) + 3
	if availableWidth < minSideBySide {
		return renderUnifiedDiff(lines, diffHiddenCount, availableWidth, gutterWidth)
	}

	panelWidth := (availableWidth - 3) / 2 // " │ " divider
	contentWidth := max(panelWidth-panelOverhead, minDiffContent)

	theme := GetTheme()

	// Styles for each cell type
	gutterInsert := lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.DiffInsertBg)
	gutterDelete := lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.DiffDeleteBg)
	gutterEqual := lipgloss.NewStyle().Foreground(theme.VeryMuted).Background(theme.DiffEqualBg)
	gutterMissing := lipgloss.NewStyle().Background(theme.DiffMissingBg)

	contentInsert := lipgloss.NewStyle().Background(theme.DiffInsertBg)
	contentDelete := lipgloss.NewStyle().Background(theme.DiffDeleteBg)
	contentEqual := lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.DiffEqualBg)
	contentMissing := lipgloss.NewStyle().Background(theme.DiffMissingBg)

	dividerStyle := lipgloss.NewStyle().Foreground(theme.MutedBorder)

	var result []string
	for _, sl := range lines {
		// Hunk separator
		if sl.beforeKind == -1 {
			sep := toolIndent() +
				dividerStyle.Render(padRight("···", panelWidth)) + " " +
				dividerStyle.Render("│") + " " +
				dividerStyle.Render(padRight("···", panelWidth))
			result = append(result, sep)
			continue
		}

		beforeText := strings.TrimRight(sl.beforeText, "\n")
		afterText := strings.TrimRight(sl.afterText, "\n")

		// Left panel (before)
		var left string
		switch {
		case sl.beforeNum > 0 && sl.beforeKind == udiff.Delete:
			gutter := fmt.Sprintf(" %*d", gutterWidth, sl.beforeNum)
			code := padRight(truncateLine(beforeText, contentWidth), contentWidth)
			left = gutterDelete.Render(gutter) + contentDelete.Render(" - "+code)
		case sl.beforeNum > 0 && sl.beforeKind == udiff.Equal:
			gutter := fmt.Sprintf(" %*d", gutterWidth, sl.beforeNum)
			code := padRight(truncateLine(beforeText, contentWidth), contentWidth)
			left = gutterEqual.Render(gutter) + contentEqual.Render("   "+code)
		default:
			left = gutterMissing.Render(padRight("", gutterWidth+1)) +
				contentMissing.Render(padRight("", contentWidth+3))
		}

		// Right panel (after)
		var right string
		switch {
		case sl.afterNum > 0 && sl.afterKind == udiff.Insert:
			gutter := fmt.Sprintf(" %*d", gutterWidth, sl.afterNum)
			code := padRight(truncateLine(afterText, contentWidth), contentWidth)
			right = gutterInsert.Render(gutter) + contentInsert.Render(" + "+code)
		case sl.afterNum > 0 && sl.afterKind == udiff.Equal:
			gutter := fmt.Sprintf(" %*d", gutterWidth, sl.afterNum)
			code := padRight(truncateLine(afterText, contentWidth), contentWidth)
			right = gutterEqual.Render(gutter) + contentEqual.Render("   "+code)
		default:
			right = gutterMissing.Render(padRight("", gutterWidth+1)) +
				contentMissing.Render(padRight("", contentWidth+3))
		}

		row := toolIndent() + left + " " + dividerStyle.Render("│") + " " + right
		result = append(result, row)
	}

	// Truncation hint spanning both panels
	if diffHiddenCount > 0 {
		hint := fmt.Sprintf("...(%d more lines)", diffHiddenCount)
		hintStyle := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(theme.DiffEqualBg).
			Italic(true)
		fullWidth := panelWidth*2 + 3 // both panels + divider
		hintRow := toolIndent() + hintStyle.Width(fullWidth).Render(hint)
		result = append(result, hintRow)
	}

	return strings.Join(result, "\n")
}

// renderUnifiedDiff renders a diff as a single column of changed lines, used
// when the terminal is too narrow for the side-by-side layout.
//
// Deleted and inserted lines are shown on their own rows, marked and tinted
// the same way as their side-by-side counterparts, so the vocabulary is the
// same in both layouts and only the arrangement changes.
func renderUnifiedDiff(lines []splitLine, hiddenCount, availableWidth, gutterWidth int) string {
	theme := GetTheme()

	gutterDelete := lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.DiffDeleteBg)
	gutterInsert := lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.DiffInsertBg)
	gutterEqual := lipgloss.NewStyle().Foreground(theme.VeryMuted).Background(theme.DiffEqualBg)
	contentDelete := lipgloss.NewStyle().Background(theme.DiffDeleteBg)
	contentInsert := lipgloss.NewStyle().Background(theme.DiffInsertBg)
	contentEqual := lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.DiffEqualBg)
	dividerStyle := lipgloss.NewStyle().Foreground(theme.MutedBorder)

	// gutter (with its leading space) + " - " marker + code
	contentWidth := max(availableWidth-gutterWidth-4, 1)

	row := func(num int, marker string, text string, g, c lipgloss.Style) string {
		gutter := fmt.Sprintf(" %*d", gutterWidth, num)
		code := padRight(truncateLine(strings.TrimRight(text, "\n"), contentWidth), contentWidth)
		return toolIndent() + g.Render(gutter) + c.Render(marker+code)
	}

	var result []string
	for _, sl := range lines {
		if sl.beforeKind == -1 {
			result = append(result, toolIndent()+dividerStyle.Render(padRight("···", availableWidth)))
			continue
		}

		// An equal line appears once; a changed line contributes its deletion
		// and its insertion as consecutive rows.
		if sl.beforeNum > 0 && sl.beforeKind == udiff.Equal {
			result = append(result, row(sl.beforeNum, "   ", sl.beforeText, gutterEqual, contentEqual))
			continue
		}
		if sl.beforeNum > 0 && sl.beforeKind == udiff.Delete {
			result = append(result, row(sl.beforeNum, " - ", sl.beforeText, gutterDelete, contentDelete))
		}
		if sl.afterNum > 0 && sl.afterKind == udiff.Insert {
			result = append(result, row(sl.afterNum, " + ", sl.afterText, gutterInsert, contentInsert))
		}
	}

	if hiddenCount > 0 {
		hint := fmt.Sprintf("...(%d more lines)", hiddenCount)
		hintStyle := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(theme.DiffEqualBg).
			Italic(true)
		result = append(result, toolIndent()+hintStyle.Width(availableWidth).Render(hint))
	}

	return strings.Join(result, "\n")
}

// ---------------------------------------------------------------------------
// Ls tool — simple list without gutter
// ---------------------------------------------------------------------------

// renderPlainListBody renders tool output as a plain list with code background
// and no line-number gutter, truncated to maxLsLines. When lines were hidden by
// truncation, caption(total, hidden) supplies the herald.Figure caption text.
// Shared pipeline for renderFindBody, renderGrepBody, and renderLsBody.
func renderPlainListBody(toolResult string, width int, caption func(total, hidden int) string) string {
	content := strings.TrimSpace(toolResult)
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	total := len(lines)

	// Truncate to maxLsLines for display
	var hiddenCount int
	if len(lines) > maxLsLines {
		hiddenCount = len(lines) - maxLsLines
		lines = lines[:maxLsLines]
	}

	panel := newToolPanel(width)

	var rendered []string
	for _, line := range lines {
		rendered = append(rendered, panel.line(line, false))
	}

	content = strings.Join(rendered, "\n")

	if hiddenCount > 0 {
		result := captionTypography().Figure(content, caption(total, hiddenCount))
		// Indent content and caption together so both sit at the content column.
		return indentBlock(result, toolIndent())
	}

	return indentBlock(content, toolIndent())
}

// renderFindBody renders find output as a plain list with code background.
// Similar to ls but with results-specific caption.
func renderFindBody(toolResult string, width int) string {
	return renderPlainListBody(toolResult, width, func(total, hidden int) string {
		count := fmt.Sprintf("%d results", total)
		if total == 1 {
			count = "1 result"
		}
		return count + " · " + fmt.Sprintf("%d more", hidden)
	})
}

// renderGrepBody renders grep output as a plain list with code background.
// Similar to find but with match-specific caption terminology.
func renderGrepBody(toolResult string, width int) string {
	return renderPlainListBody(toolResult, width, func(total, hidden int) string {
		count := fmt.Sprintf("%d matches", total)
		if total == 1 {
			count = "1 match"
		}
		return count + " · " + fmt.Sprintf("%d more", hidden)
	})
}

// renderLsBody renders ls output as a plain list with code background and no
// line-number gutter.
func renderLsBody(toolResult string, width int) string {
	return renderPlainListBody(toolResult, width, func(_, hidden int) string {
		return fmt.Sprintf("%d more entries", hidden)
	})
}

// ---------------------------------------------------------------------------
// Read tool — code block with line numbers + syntax highlighting
// ---------------------------------------------------------------------------

// renderReadBody renders Read tool output using herald.CodeBlock with line numbers
// and syntax highlighting. Uses WithCodeLineNumberOffset to show correct offsets
// based on the Read tool's offset parameter.
func renderReadBody(toolArgs, toolResult string, width int) string {
	if strings.TrimSpace(toolResult) == "" {
		return ""
	}

	// Extract file path and offset from tool args
	var fileName string
	var offset = 1
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err == nil {
		if p, ok := args["path"].(string); ok {
			fileName = p
		}
		if o, ok := args["offset"].(float64); ok {
			offset = int(o)
		}
	}

	// Parse lines to extract pure code content (removing "N: " prefixes)
	rawLines := strings.Split(toolResult, "\n")
	var codeLines []string
	var footerLines []string
	var codeHiddenCount int

	for _, line := range rawLines {
		// Detect "N: content" format from Read tool
		if idx := strings.Index(line, ": "); idx > 0 && idx <= 7 {
			numPart := line[:idx]
			if _, err := strconv.Atoi(strings.TrimSpace(numPart)); err == nil {
				codeLines = append(codeLines, line[idx+2:])
				continue
			}
		}
		// No line number — treat as footer/metadata (e.g., truncation notice)
		footerLines = append(footerLines, line)
	}

	// Apply maxCodeLines truncation
	totalCodeLines := len(codeLines)
	if totalCodeLines > maxCodeLines {
		codeHiddenCount = totalCodeLines - maxCodeLines
		codeLines = codeLines[:maxCodeLines]
	}

	// Clamp each line to the space actually available.
	//
	// herald renders the code block with a line-number gutter but does no
	// wrapping of its own, so without this a single long source line runs off
	// the right edge of the terminal — the emulator then wraps it and the
	// scroll list's height accounting is wrong for the rest of the session.
	// Truncation happens before syntax highlighting because cutting a string
	// that already contains ANSI escapes truncates the escapes too.
	numDigits := max(len(strconv.Itoa(offset+len(codeLines))), 3)
	// gutter: line-number field, its separating space, and the CodeBlock's
	// own PaddingLeft(1).
	gutterWidth := numDigits + 2
	codeWidth := max(width-style.ContentOffset-gutterWidth, style.MinContentWidth)
	for i, line := range codeLines {
		codeLines[i] = truncateLine(line, codeWidth)
	}

	// Build language hint from file extension
	lang := ""
	if fileName != "" {
		// Extract extension without the dot
		if ext := strings.TrimPrefix(filepath.Ext(fileName), "."); ext != "" {
			lang = ext
		}
	}

	// Create typography with line number offset and custom formatter
	// Match Write tool: GutterBg for line numbers, CodeBg for content
	codeContent := strings.Join(codeLines, "\n")
	theme := GetTheme()
	hty := herald.Theme{
		CodeBlock: lipgloss.NewStyle().
			Background(theme.CodeBg).
			PaddingLeft(1),
		CodeLineNumber: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(theme.GutterBg),
	}
	ty := herald.New(
		herald.WithTheme(hty),
		herald.WithCodeLineNumbers(true),
		herald.WithCodeLineNumberOffset(offset),
		herald.WithCodeFormatter(func(code, _ string) string {
			// Use our syntax highlighter with the filename for lexer detection
			return syntaxHighlight(code, fileName)
		}),
	)

	// Render the code block
	codeBlock := ty.CodeBlock(codeContent, lang)

	// Herald's codeBlockWithLineNumbers() hardcodes PaddingTop(1) and
	// PaddingBottom(1), adding invisible blank lines with background color
	// above and below the code. These interfere with mouse selection
	// (off-by-one) because the padding line looks blank but occupies a
	// line index in the rendered item. Strip them since the Compose
	// separator above and Figure caption below already provide spacing.
	codeBlock = stripCodeBlockPadding(codeBlock)

	// Parse total lines from footer if available (e.g., "[showing lines 1-100 of 407 total...]")
	totalLines := totalCodeLines
	for _, footer := range footerLines {
		if matches := totalLinesPattern.FindStringSubmatch(footer); len(matches) > 1 {
			if t, _ := strconv.Atoi(matches[1]); t > totalLines {
				totalLines = t
			}
		}
	}

	// Build caption with file metadata
	var captionParts []string
	if fileName != "" {
		captionParts = append(captionParts, filepath.Base(fileName))
	}
	if len(codeLines) > 0 {
		endLine := offset + len(codeLines) - 1
		captionParts = append(captionParts, fmt.Sprintf("lines %d-%d of %d", offset, endLine, totalLines))
	}
	if codeHiddenCount > 0 {
		nextOffset := offset + len(codeLines)
		captionParts = append(captionParts, fmt.Sprintf("offset=%d to continue", nextOffset))
	}

	caption := strings.Join(captionParts, " · ")

	// Use Figure with caption below content (default behavior)
	// Apply theme to ensure caption is positioned below
	figTheme := herald.Theme{
		FigureCaption:         lipgloss.NewStyle().Foreground(GetTheme().Muted),
		FigureCaptionPosition: herald.CaptionBottom,
	}
	tyFig := herald.New(herald.WithTheme(figTheme))
	result := tyFig.Figure(codeBlock, caption)

	// Indent entire block to match Write/Edit tools.
	return indentBlock(result, strings.Repeat(" ", style.ContentOffset))
}

// ---------------------------------------------------------------------------
// Write tool — green-tinted block with line numbers and "End of file" footer
// ---------------------------------------------------------------------------

// renderWriteBody extracts content from toolArgs and renders it as a green-tinted
// code block with line numbers and an "End of file" footer.
func renderWriteBody(toolArgs, toolResult string, width int) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
		return ""
	}

	content, _ := args["content"].(string)
	if content == "" {
		return "" // fall back to default
	}

	var fileName string
	if p, ok := args["path"].(string); ok {
		fileName = p
	}

	return renderWriteBlock(content, fileName, width)
}

// renderWriteBlock renders file content with green-tinted background, line numbers,
// and a footer showing the total line count.
func renderWriteBlock(content, fileName string, width int) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Truncate to maxWriteLines for display
	var hiddenCount int
	if totalLines > maxWriteLines {
		hiddenCount = totalLines - maxWriteLines
		lines = lines[:maxWriteLines]
	}

	// Line number width
	numDigits := max(len(fmt.Sprintf("%d", totalLines)), 3)

	// Syntax highlight
	displayContent := strings.Join(lines, "\n")
	highlighted := syntaxHighlight(displayContent, fileName)
	highlightedLines := strings.Split(highlighted, "\n")

	// Layout
	gutterWidth := numDigits + 2
	codeWidth := max(width-gutterWidth-style.ContentOffset, style.MinContentWidth)

	theme := GetTheme()
	gutterStyle := lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.GutterBg).PaddingRight(1)
	writeStyle := lipgloss.NewStyle().Background(theme.WriteBg).PaddingLeft(1)

	var result []string
	for i, line := range lines {
		numStr := fmt.Sprintf("%*d", numDigits, i+1)
		gutter := gutterStyle.Width(gutterWidth).Render(numStr)

		var codePart string
		if i < len(highlightedLines) {
			codePart = highlightedLines[i]
		} else {
			codePart = line
		}
		// Truncate the (possibly ANSI-highlighted) line to fit within
		// the code column, preventing lipgloss from wrapping it.
		codePart = truncateLine(codePart, codeWidth-1) // account for PaddingLeft(1)
		styledCode := writeStyle.Width(codeWidth).Render(codePart)

		result = append(result, toolIndent()+lipgloss.JoinHorizontal(lipgloss.Top, gutter, styledCode))
	}

	// Footer
	var footer string
	if hiddenCount > 0 {
		footer = fmt.Sprintf("...(%d more lines, %d total)", hiddenCount, totalLines)
	} else {
		footer = fmt.Sprintf("(End of file \u2014 total %d lines)", totalLines)
	}

	emptyGutter := gutterStyle.Width(gutterWidth).Render("")
	footerContent := writeStyle.Width(codeWidth).
		Foreground(theme.Muted).
		Italic(true).
		Render(footer)
	result = append(result, toolIndent()+lipgloss.JoinHorizontal(lipgloss.Top, emptyGutter, footerContent))

	return strings.Join(result, "\n")
}

// ---------------------------------------------------------------------------
// Bash tool — output with background styling
// ---------------------------------------------------------------------------

// renderBashBody renders bash output with per-line background and stderr
// in error color.
func renderBashBody(toolArgs, toolResult string, width int) string {
	if strings.TrimSpace(toolResult) == "" {
		return ""
	}

	theme := GetTheme()

	// Strip <stdout>/<stderr> tags if present. Kit's builtin bash tool emits
	// the STDERR:/Exit code: form handled below, but a third-party MCP tool
	// named "bash" may emit the tagged form, which the error path already
	// strips via parseBashOutput. Handling it here keeps the two paths
	// agreeing instead of showing raw markup on the success path only.
	result := toolResult
	if strings.Contains(result, "<stdout>") || strings.Contains(result, "<stderr>") {
		result = parseBashOutput(result, theme)
	}

	// Truncate to maxBashLines for display
	lines := strings.Split(result, "\n")
	var hiddenCount int
	if len(lines) > maxBashLines {
		hiddenCount = len(lines) - maxBashLines
		lines = lines[:maxBashLines]
	}

	panel := newToolPanel(width)

	var rendered []string
	exitCode := -1 // -1 means not found
	inStderr := false
	for _, line := range lines {
		// Detect the STDERR: label that Kit's bash tool emits
		if strings.TrimSpace(line) == "STDERR:" {
			inStderr = true
			continue
		}
		// Exit code line - extract it for caption
		if strings.HasPrefix(line, "Exit code:") {
			_, _ = fmt.Sscanf(line, "Exit code: %d", &exitCode)
			continue // Don't render exit code inline, it goes in caption
		}

		rendered = append(rendered, panel.line(line, inStderr))
	}

	// Build caption with status info
	var captionParts []string
	if hiddenCount > 0 {
		captionParts = append(captionParts, fmt.Sprintf("%d more lines", hiddenCount))
	}
	if exitCode >= 0 {
		captionParts = append(captionParts, fmt.Sprintf("exit code %d", exitCode))
	}

	content := strings.Join(rendered, "\n")
	if len(captionParts) > 0 {
		caption := strings.Join(captionParts, " · ")
		result := captionTypography().Figure(content, caption)

		// Indent entire block (content + caption) to match other tools
		return indentBlock(result, toolIndent())
	}

	// No caption - just return indented content
	return indentBlock(content, toolIndent())
}

// ---------------------------------------------------------------------------
// Syntax highlighting via Chroma
// ---------------------------------------------------------------------------

// syntaxHighlight applies syntax highlighting to source code using chroma.
// Uses the catppuccin-mocha style for dark terminals, catppuccin-latte for light.
// Returns the source unchanged if highlighting fails.
func syntaxHighlight(source, fileName string) string {
	if source == "" {
		return source
	}

	// Detect lexer from filename
	lexer := lexers.Match(fileName)
	if lexer == nil {
		// Try content-based detection
		lexer = lexers.Analyse(source)
	}
	if lexer == nil {
		return source // no highlighting
	}

	// Use true-color formatter
	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		formatter = formatters.Get("terminal256")
	}
	if formatter == nil {
		return source
	}

	// Build the highlighting palette from the active theme so code sits in
	// the same color family as the UI around it. Token backgrounds are unset
	// there, letting the containing lipgloss style own the fill.
	chromaStyle := SyntaxStyle()

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return source
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, chromaStyle, iterator); err != nil {
		return source
	}

	// Replace full ANSI resets with fg-only resets so they don't clear
	// the background set by lipgloss.
	result := strings.ReplaceAll(buf.String(), "\x1b[0m", "\x1b[39;22;23;24m")
	return strings.TrimRight(result, "\n")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// indentBlock prefixes every line of s (including empty lines) with indent.
// Used to shift a fully rendered tool body right so it aligns with other tools.
func indentBlock(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

// padRight pads s with spaces to exactly width visual characters.
// This is ANSI-aware: it measures the visual width of s (ignoring escape
// codes and accounting for wide characters) before padding or truncating.
func padRight(s string, width int) string {
	w := xansi.StringWidth(s)
	if w >= width {
		return xansi.Truncate(s, width, "")
	}
	return s + strings.Repeat(" ", width-w)
}

// stripCodeBlockPadding removes the top and bottom padding lines that herald's
// codeBlockWithLineNumbers() hardcodes via PaddingTop(1)/PaddingBottom(1).
// These padding lines are blank lines with background color that look invisible
// but occupy line indices, causing mouse selection to be off by one row.
func stripCodeBlockPadding(block string) string {
	lines := strings.Split(block, "\n")
	if len(lines) < 3 {
		return block
	}
	// The first and last lines are padding (blank with bg color).
	// Strip them only if they contain no visible text.
	first := xansi.Strip(lines[0])
	last := xansi.Strip(lines[len(lines)-1])
	if strings.TrimSpace(first) == "" && strings.TrimSpace(last) == "" {
		return strings.Join(lines[1:len(lines)-1], "\n")
	}
	return block
}

// truncateLine truncates a line to maxWidth visual characters, adding "…"
// if truncated. This is ANSI-aware: escape codes are preserved and wide
// characters are measured correctly.
func truncateLine(s string, maxWidth int) string {
	if xansi.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth < 2 {
		return xansi.Truncate(s, maxWidth, "")
	}
	return xansi.Truncate(s, maxWidth, "…")
}

// renderSubagentBody renders a clean summary of subagent results with bash-style
// background styling for consistency with other tools.
func renderSubagentBody(toolResult string, width int) string {
	result := strings.TrimSpace(toolResult)
	if result == "" {
		return ""
	}

	// Parse the subagent result format:
	// "Subagent completed successfully in Xs. (tokens: N in / M out)\n\nResult:\n..."
	// or "Subagent failed (exit code X) after Ys.\n\nError: ...\n\nPartial output:\n..."

	lines := strings.Split(result, "\n")
	if len(lines) == 0 {
		return ""
	}

	// First line is always the status summary
	statusLine := lines[0]

	// Build content lines for display with bash-style background
	panel := newToolPanel(width)

	var contentLines []string

	// Add status line
	contentLines = append(contentLines, panel.line(statusLine, false))

	// For successful results, extract a brief preview of the actual result
	if strings.Contains(statusLine, "successfully") {
		// Find where "Result:" starts and extract a preview
		if _, resultContent, found := strings.Cut(result, "Result:\n"); found {
			resultContent = strings.TrimSpace(resultContent)
			if resultContent != "" {
				// Show first few meaningful lines as preview
				previewLines := extractSubagentPreviewLines(resultContent, 5, panel.textWidth)
				if len(previewLines) > 0 {
					// Add blank separator line
					contentLines = append(contentLines, panel.blank())
					for _, line := range previewLines {
						contentLines = append(contentLines, panel.line(line, false))
					}
				}
			}
		}
	} else {
		// For failed results, show error info
		if _, errorContent, found := strings.Cut(result, "Error:\n"); found {
			errorContent = strings.TrimSpace(errorContent)
			if errorContent != "" {
				previewLines := extractSubagentPreviewLines(errorContent, 3, panel.textWidth)
				if len(previewLines) > 0 {
					contentLines = append(contentLines, panel.blank())
					for _, line := range previewLines {
						contentLines = append(contentLines, panel.line(line, true))
					}
				}
			}
		}
	}

	return indentBlock(strings.Join(contentLines, "\n"), toolIndent())
}

// extractSubagentPreviewLines extracts the first N non-empty lines from content,
// truncating each line to maxWidth. Returns as a slice of strings.
func extractSubagentPreviewLines(content string, maxLines, maxWidth int) []string {
	lines := strings.Split(content, "\n")
	var preview []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Truncate long lines (ANSI-aware)
		trimmed = truncateLine(trimmed, maxWidth)
		preview = append(preview, trimmed)

		if len(preview) >= maxLines {
			break
		}
	}

	// Count remaining lines for "more" indicator
	totalLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			totalLines++
		}
	}
	if totalLines > maxLines {
		preview = append(preview, fmt.Sprintf("...(%d more lines)", totalLines-maxLines))
	}

	return preview
}
