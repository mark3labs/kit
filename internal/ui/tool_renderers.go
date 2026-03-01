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
)

// Maximum visible lines per tool type before truncation.
const (
	maxDiffLines  = 20 // side-by-side rows for Edit
	maxCodeLines  = 20 // lines for Read / code blocks
	maxWriteLines = 10 // lines for Write blocks
	maxBashLines  = 20 // lines for Bash output (matches Read)
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
	}
	return "" // fall back to default
}

// ---------------------------------------------------------------------------
// Edit tool — side-by-side diff
// ---------------------------------------------------------------------------

// renderEditBody renders a side-by-side diff from old_text/new_text in toolArgs.
func renderEditBody(toolArgs, toolResult string, width int) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
		return ""
	}

	oldText, _ := args["old_text"].(string)
	newText, _ := args["new_text"].(string)
	if oldText == "" && newText == "" {
		return ""
	}

	// Try to extract the starting line number from the unified diff in the result
	startLine := extractDiffStartLine(toolResult)

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
	contentDelete := lipgloss.NewStyle().Background(theme.DiffDeleteBg).Strikethrough(true)
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

	const indent = "  "
	codeWidth := max(width-len(indent), 20)

	theme := getTheme()
	codeStyle := lipgloss.NewStyle().Background(theme.CodeBg).PaddingLeft(1)

	var result []string
	for _, line := range lines {
		styled := codeStyle.Width(codeWidth).Render(line)
		result = append(result, indent+styled)
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
			footer := codeStyle.Width(codeWidth).Render(p.code)
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
	var rendered []string
	inStderr := false
	for _, line := range lines {
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

// padRight pads s with spaces to exactly width characters.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// truncateLine truncates a line to maxWidth, adding "…" if truncated.
func truncateLine(s string, maxWidth int) string {
	if len(s) <= maxWidth {
		return s
	}
	if maxWidth < 2 {
		return s[:maxWidth]
	}
	return s[:maxWidth-1] + "…"
}
