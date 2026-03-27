package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	udiff "github.com/aymanbagabas/go-udiff"
	xansi "github.com/charmbracelet/x/ansi"
)

// Maximum visible lines per tool type before truncation.
const (
	maxDiffLines  = 20 // side-by-side rows for Edit
	maxCodeLines  = 20 // lines for Read / code blocks
	maxWriteLines = 10 // lines for Write blocks
	maxBashLines  = 20 // lines for Bash output (matches Read)
	maxLsLines    = 20 // lines for Ls directory listings
)

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
	case toolName == "bash" || toolName == "run_shell_cmd" ||
		strings.Contains(toolName, "shell") || strings.Contains(toolName, "command"):
		if body := renderBashBody(toolResult, width); body != "" {
			return body
		}
	case toolName == "spawn_subagent":
		if body := renderSubagentBody(toolResult, width); body != "" {
			return body
		}
	}
	return "" // fall back to default
}

// ---------------------------------------------------------------------------
// Edit tool — side-by-side diff
// ---------------------------------------------------------------------------

// renderEditBody renders a side-by-side diff from old_text/new_text in toolArgs.
// Supports both single-edit mode and multi-edit mode (edits array).
func renderEditBody(toolArgs, toolResult string, width int) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
		return ""
	}

	// Try to extract the starting line number from the unified diff in the result
	startLine := extractDiffStartLine(toolResult)

	// Check for multi-edit mode (edits array)
	if editsArr, ok := args["edits"].([]any); ok && len(editsArr) > 0 {
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

	// Single-edit mode (legacy)
	oldText, _ := args["old_text"].(string)
	newText, _ := args["new_text"].(string)
	if oldText == "" && newText == "" {
		return ""
	}

	return renderDiffBlock(oldText, newText, startLine, width)
}

// extractDiffStartLine parses the first @@ hunk header from a unified diff
// result to find the starting line number. Returns 1 if not found.
func extractDiffStartLine(result string) int {
	re := regexp.MustCompile(`@@ -(\d+)`)
	matches := re.FindStringSubmatch(result)
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
	const indent = "  "
	availableWidth := width - len(indent)
	panelWidth := max((availableWidth-3)/2, 20) // " │ " divider

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
	contentWidth := max(panelWidth-gutterWidth-4, 10) // gutter + " - " or " + "

	theme := getTheme()

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
			sep := indent +
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

		row := indent + left + " " + dividerStyle.Render("│") + " " + right
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
		hintRow := indent + hintStyle.Width(fullWidth).Render(hint)
		result = append(result, hintRow)
	}

	return strings.Join(result, "\n")
}

// ---------------------------------------------------------------------------
// Ls tool — simple list without gutter
// ---------------------------------------------------------------------------

// renderLsBody renders ls output as a plain list with code background and no
// line-number gutter.
func renderLsBody(toolResult string, width int) string {
	content := strings.TrimSpace(toolResult)
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")

	// Truncate to maxLsLines for display
	var hiddenCount int
	if len(lines) > maxLsLines {
		hiddenCount = len(lines) - maxLsLines
		lines = lines[:maxLsLines]
	}

	const indent = "  "
	codeWidth := max(width-len(indent), 20)

	theme := getTheme()
	codeStyle := lipgloss.NewStyle().Background(theme.CodeBg).PaddingLeft(1)

	var result []string
	for _, line := range lines {
		// Truncate before styling to prevent wrapping.
		line = truncateLine(line, codeWidth-1) // account for PaddingLeft(1)
		styled := codeStyle.Width(codeWidth).Render(line)
		result = append(result, indent+styled)
	}

	if hiddenCount > 0 {
		hint := fmt.Sprintf("...(%d more entries)", hiddenCount)
		hintContent := codeStyle.Width(codeWidth).
			Foreground(theme.Muted).Italic(true).Render(hint)
		result = append(result, indent+hintContent)
	}

	return strings.Join(result, "\n")
}

// ---------------------------------------------------------------------------
// Read tool — code block with line numbers + syntax highlighting
// ---------------------------------------------------------------------------

// renderReadBody renders Read tool output with styled line numbers and optional
// syntax highlighting based on file extension.
func renderReadBody(toolArgs, toolResult string, width int) string {
	if strings.TrimSpace(toolResult) == "" {
		return ""
	}

	// Extract file path for syntax highlighting
	var fileName string
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err == nil {
		if p, ok := args["path"].(string); ok {
			fileName = p
		}
	}

	return renderCodeBlock(toolResult, fileName, width)
}

// codeLine holds a parsed line with optional line number.
type codeLine struct {
	lineNum string
	code    string
}

// renderCodeBlock renders content with a styled gutter (line numbers) and
// optional syntax highlighting.
func renderCodeBlock(content, fileName string, width int) string {
	rawLines := strings.Split(content, "\n")

	// Parse lines: detect "N: content" format from Read tool
	var parsed []codeLine
	maxNumWidth := 0
	var codeOnly []string

	for _, line := range rawLines {
		if idx := strings.Index(line, ": "); idx > 0 && idx <= 7 {
			numPart := line[:idx]
			if _, err := strconv.Atoi(strings.TrimSpace(numPart)); err == nil {
				parsed = append(parsed, codeLine{lineNum: numPart, code: line[idx+2:]})
				if len(numPart) > maxNumWidth {
					maxNumWidth = len(numPart)
				}
				codeOnly = append(codeOnly, line[idx+2:])
				continue
			}
		}
		// No line number — treat as metadata/footer
		parsed = append(parsed, codeLine{code: line})
		codeOnly = append(codeOnly, line)
	}

	if len(parsed) == 0 {
		return ""
	}

	// Truncate to maxCodeLines visible lines (preserve footer/metadata lines)
	var codeHiddenCount int
	totalParsed := len(parsed)
	if totalParsed > maxCodeLines {
		// Check if last line is a footer (no line number) — keep it
		var footerLines []codeLine
		for totalParsed > 0 && parsed[totalParsed-1].lineNum == "" {
			footerLines = append([]codeLine{parsed[totalParsed-1]}, footerLines...)
			totalParsed--
		}
		if totalParsed > maxCodeLines {
			codeHiddenCount = totalParsed - maxCodeLines
			parsed = append(parsed[:maxCodeLines], footerLines...)
			codeOnly = codeOnly[:maxCodeLines]
			for _, fl := range footerLines {
				codeOnly = append(codeOnly, fl.code)
			}
		} else {
			// Restore — footer trimming was enough
			parsed = parsed[:totalParsed]
			parsed = append(parsed, footerLines...)
		}
	}

	// Syntax highlight the code portion
	highlighted := syntaxHighlight(strings.Join(codeOnly, "\n"), fileName)
	highlightedLines := strings.Split(highlighted, "\n")

	// Layout
	const codeIndent = "  "
	gutterWidth := max(maxNumWidth+2, 5)
	codeWidth := max(width-gutterWidth-len(codeIndent), 20)

	theme := getTheme()
	gutterStyle := lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.GutterBg).PaddingRight(1)
	codeStyle := lipgloss.NewStyle().Background(theme.CodeBg).PaddingLeft(1)

	var result []string
	for i, p := range parsed {
		// If this line has no line number, it's a metadata/footer line (e.g. truncation notice).
		if p.lineNum == "" {
			// Render footer lines with code background but no gutter
			truncatedFooter := truncateLine(p.code, codeWidth-1) // account for PaddingLeft(1)
			footer := codeStyle.Width(codeWidth).Render(truncatedFooter)
			emptyGutter := gutterStyle.Width(gutterWidth).Render("")
			result = append(result, codeIndent+lipgloss.JoinHorizontal(lipgloss.Top, emptyGutter, footer))
			continue
		}

		gutter := gutterStyle.Width(gutterWidth).Render(p.lineNum)

		var codePart string
		if i < len(highlightedLines) {
			codePart = highlightedLines[i]
		} else {
			codePart = p.code
		}
		// Truncate the (possibly ANSI-highlighted) line to fit within
		// the code column, preventing lipgloss from wrapping it.
		codePart = truncateLine(codePart, codeWidth-1) // account for PaddingLeft(1)
		styledCode := codeStyle.Width(codeWidth).Render(codePart)

		result = append(result, codeIndent+lipgloss.JoinHorizontal(lipgloss.Top, gutter, styledCode))
	}

	// Truncation hint
	if codeHiddenCount > 0 {
		hint := fmt.Sprintf("...(%d more lines)", codeHiddenCount)
		emptyGutter := gutterStyle.Width(gutterWidth).Render("")
		hintContent := codeStyle.Width(codeWidth).
			Foreground(theme.Muted).Italic(true).Render(hint)
		result = append(result, codeIndent+lipgloss.JoinHorizontal(lipgloss.Top, emptyGutter, hintContent))
	}

	return strings.Join(result, "\n")
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
	const codeIndent = "  "
	gutterWidth := numDigits + 2
	codeWidth := max(width-gutterWidth-len(codeIndent), 20)

	theme := getTheme()
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

		result = append(result, codeIndent+lipgloss.JoinHorizontal(lipgloss.Top, gutter, styledCode))
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
	result = append(result, codeIndent+lipgloss.JoinHorizontal(lipgloss.Top, emptyGutter, footerContent))

	return strings.Join(result, "\n")
}

// ---------------------------------------------------------------------------
// Bash tool — output with background styling
// ---------------------------------------------------------------------------

// renderBashBody renders bash output with per-line background and stderr
// in error color.
func renderBashBody(toolResult string, width int) string {
	if strings.TrimSpace(toolResult) == "" {
		return ""
	}

	theme := getTheme()
	outputStyle := lipgloss.NewStyle().Background(theme.CodeBg).PaddingLeft(1)
	stderrStyle := lipgloss.NewStyle().Foreground(theme.Error).Background(theme.CodeBg).PaddingLeft(1)

	// Parse stdout/stderr sections (if tagged) or STDERR: label
	result := toolResult

	// Truncate to maxBashLines for display
	lines := strings.Split(result, "\n")
	var hiddenCount int
	if len(lines) > maxBashLines {
		hiddenCount = len(lines) - maxBashLines
		lines = lines[:maxBashLines]
	}

	const lineIndent = "  "
	// Truncate individual lines to the available width so they never wrap.
	// This mirrors Crush's approach: truncate, don't wrap.
	lineWidth := max(width-len(lineIndent), 20)
	// Account for PaddingLeft(1) on the output/stderr styles
	maxLineChars := lineWidth - 1

	var rendered []string
	inStderr := false
	for _, line := range lines {
		line = truncateLine(line, maxLineChars)
		// Detect the STDERR: label that Kit's bash tool emits
		if strings.TrimSpace(line) == "STDERR:" {
			inStderr = true
			continue
		}
		// Exit code line
		if strings.HasPrefix(line, "Exit code:") {
			styled := stderrStyle.Width(width - len(lineIndent)).Render(line)
			rendered = append(rendered, lineIndent+styled)
			continue
		}

		if inStderr {
			styled := stderrStyle.Width(width - len(lineIndent)).Render(line)
			rendered = append(rendered, lineIndent+styled)
		} else {
			styled := outputStyle.Width(width - len(lineIndent)).Render(line)
			rendered = append(rendered, lineIndent+styled)
		}
	}

	if hiddenCount > 0 {
		truncMsg := fmt.Sprintf("...(%d more lines)", hiddenCount)
		hint := outputStyle.Width(width - len(lineIndent)).
			Foreground(theme.Muted).Italic(true).Render(truncMsg)
		rendered = append(rendered, lineIndent+hint)
	}

	return strings.Join(rendered, "\n")
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

	// Pick style matching our UI theme
	styleName := "catppuccin-mocha"
	if !IsDarkBackground() {
		styleName = "catppuccin-latte"
	}
	baseStyle := styles.Get(styleName)
	if baseStyle == nil {
		baseStyle = styles.Fallback
	}

	// Clear token backgrounds so the containing lipgloss style controls bg.
	style, err := baseStyle.Builder().Transform(func(entry chroma.StyleEntry) chroma.StyleEntry {
		entry.Background = 0
		return entry
	}).Build()
	if err != nil {
		style = baseStyle
	}

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return source
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
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

// ---------------------------------------------------------------------------
// Compact tool body renderers — one-line summaries for compact mode
// ---------------------------------------------------------------------------

// renderToolBodyCompact returns a brief summary string for tool results in
// compact display mode. Returns empty string to fall back to default.
func renderToolBodyCompact(toolName, toolArgs, toolResult string, width int) string {
	switch {
	case toolName == "edit":
		return renderEditCompact(toolArgs, toolResult)
	case toolName == "ls":
		return renderLsCompact(toolResult)
	case toolName == "read":
		return renderReadCompact(toolResult)
	case toolName == "write":
		return renderWriteCompact(toolArgs)
	case toolName == "bash" || toolName == "run_shell_cmd" ||
		strings.Contains(toolName, "shell") || strings.Contains(toolName, "command"):
		return renderBashCompact(toolResult, width)
	case toolName == "spawn_subagent":
		return renderSubagentCompact(toolResult)
	}
	return ""
}

// renderReadCompact returns a line-count summary for Read tool output.
func renderReadCompact(toolResult string) string {
	content := strings.TrimSpace(toolResult)
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")

	// Count actual code lines (those with "N: " line-number prefix)
	codeLines := 0
	for _, line := range lines {
		if idx := strings.Index(line, ": "); idx > 0 && idx <= 7 {
			numPart := line[:idx]
			if _, err := strconv.Atoi(strings.TrimSpace(numPart)); err == nil {
				codeLines++
			}
		}
	}
	if codeLines == 0 {
		return ""
	}

	theme := getTheme()
	summary := fmt.Sprintf("%d lines", codeLines)
	return lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render(summary)
}

// renderEditCompact returns a change-count summary for Edit tool output.
func renderEditCompact(toolArgs, toolResult string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
		return ""
	}

	oldText, _ := args["old_text"].(string)
	newText, _ := args["new_text"].(string)
	if oldText == "" && newText == "" {
		return ""
	}

	oldCount := len(strings.Split(oldText, "\n"))
	newCount := len(strings.Split(newText, "\n"))

	theme := getTheme()
	var summary string
	if oldCount == newCount {
		summary = fmt.Sprintf("%d lines modified", oldCount)
	} else {
		summary = fmt.Sprintf("-%d/+%d lines", oldCount, newCount)
	}
	return lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render(summary)
}

// renderWriteCompact returns a line-count summary for Write tool output.
func renderWriteCompact(toolArgs string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
		return ""
	}

	content, _ := args["content"].(string)
	if content == "" {
		return ""
	}

	count := len(strings.Split(content, "\n"))
	theme := getTheme()
	summary := fmt.Sprintf("%d lines written", count)
	return lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render(summary)
}

// renderLsCompact returns an entry-count summary for Ls tool output.
func renderLsCompact(toolResult string) string {
	content := strings.TrimSpace(toolResult)
	if content == "" {
		return ""
	}

	entries := strings.Split(content, "\n")
	theme := getTheme()
	summary := fmt.Sprintf("%d entries", len(entries))
	return lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render(summary)
}

// renderBashCompact returns the first few lines of bash output as a compact
// summary. Shows up to 3 meaningful output lines.
func renderBashCompact(toolResult string, width int) string {
	result := strings.TrimSpace(toolResult)
	if result == "" {
		return ""
	}

	lines := strings.Split(result, "\n")

	// Filter to meaningful output lines (skip STDERR: label, keep exit codes separate)
	var outputLines []string
	var exitCode string
	inStderr := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "STDERR:" {
			inStderr = true
			continue
		}
		if strings.HasPrefix(trimmed, "Exit code:") {
			exitCode = trimmed
			continue
		}
		if trimmed == "" {
			continue
		}
		outputLines = append(outputLines, line)
		_ = inStderr // stderr lines are included in output
	}

	if len(outputLines) == 0 {
		if exitCode != "" {
			theme := getTheme()
			return lipgloss.NewStyle().Foreground(theme.Error).Render(exitCode)
		}
		return ""
	}

	const maxLines = 3
	theme := getTheme()

	display := outputLines
	if len(display) > maxLines {
		display = display[:maxLines]
	}

	// Truncate each line to available width (ANSI-aware)
	lineMax := max(width-4, 20)
	for i, line := range display {
		display[i] = truncateLine(line, lineMax)
	}

	summary := strings.Join(display, "\n")
	if len(outputLines) > maxLines {
		summary += fmt.Sprintf("\n...(%d more lines)", len(outputLines)-maxLines)
	}
	if exitCode != "" {
		summary += "\n" + lipgloss.NewStyle().Foreground(theme.Error).Render(exitCode)
	}

	return lipgloss.NewStyle().Foreground(theme.Muted).Render(summary)
}

// ---------------------------------------------------------------------------
// Subagent tool renderers — show only summary, not full output
// ---------------------------------------------------------------------------

// renderSubagentBody renders a clean summary of subagent results.
// Extracts timing/token info and shows only a brief summary instead of raw output.
func renderSubagentBody(toolResult string, width int) string {
	theme := getTheme()
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

	// Build a clean summary
	var summary strings.Builder
	summary.WriteString(lipgloss.NewStyle().Foreground(theme.Muted).Render(statusLine))

	// For successful results, extract a brief preview of the actual result
	if strings.Contains(statusLine, "successfully") {
		// Find where "Result:" starts and extract a preview
		if _, resultContent, found := strings.Cut(result, "Result:\n"); found {
			resultContent = strings.TrimSpace(resultContent)
			if resultContent != "" {
				// Show first 3 meaningful lines as preview
				preview := extractSubagentPreview(resultContent, 3, width-4)
				if preview != "" {
					summary.WriteString("\n\n")
					summary.WriteString(lipgloss.NewStyle().
						Foreground(theme.Muted).
						Italic(true).
						Render(preview))
				}
			}
		}
	}

	return summary.String()
}

// extractSubagentPreview extracts the first N non-empty lines from content,
// truncating each line to maxWidth.
func extractSubagentPreview(content string, maxLines, maxWidth int) string {
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

	if len(preview) == 0 {
		return ""
	}

	result := strings.Join(preview, "\n")

	// Count remaining lines for "more" indicator
	totalLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			totalLines++
		}
	}
	if totalLines > maxLines {
		result += fmt.Sprintf("\n...(%d more lines)", totalLines-maxLines)
	}

	return result
}

// renderSubagentCompact returns a brief one-line summary for subagent results.
func renderSubagentCompact(toolResult string) string {
	result := strings.TrimSpace(toolResult)
	if result == "" {
		return ""
	}

	theme := getTheme()

	// Extract just the first line which contains the status
	lines := strings.Split(result, "\n")
	if len(lines) == 0 {
		return ""
	}

	statusLine := lines[0]

	// Make it more compact by removing redundant words
	statusLine = strings.Replace(statusLine, "Subagent completed successfully in ", "Completed in ", 1)
	statusLine = strings.Replace(statusLine, "Subagent failed", "Failed", 1)

	return lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render(statusLine)
}
