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
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	udiff "github.com/aymanbagabas/go-udiff"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/indaco/herald"
)

// Maximum visible lines per tool type before truncation.
const (
	maxDiffLines  = 20 // side-by-side rows for Edit
	maxCodeLines  = 20 // lines for Read / code blocks
	maxWriteLines = 10 // lines for Write blocks
	maxBashLines  = 20 // lines for Bash output (matches Read)
	maxLsLines    = 20 // lines for Ls directory listings
)

// isShellTool reports if the tool name matches a shell-like tool (bash or
// tools with "shell"/"command" in the name). Used by renderToolBody.
func isShellTool(toolName string) bool {
	return toolName == "bash" ||
		strings.Contains(toolName, "shell") || strings.Contains(toolName, "command")
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

// renderFindBody renders find output as a plain list with code background.
// Similar to ls but with results-specific caption.
func renderFindBody(toolResult string, width int) string {
	content := strings.TrimSpace(toolResult)
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	totalResults := len(lines)

	// Truncate to maxLsLines for display
	var hiddenCount int
	if len(lines) > maxLsLines {
		hiddenCount = len(lines) - maxLsLines
		lines = lines[:maxLsLines]
	}

	const lineIndent = "  "
	codeWidth := max(width-len(lineIndent), 20)

	theme := GetTheme()
	codeStyle := lipgloss.NewStyle().Background(theme.CodeBg).PaddingLeft(1)

	var rendered []string
	for _, line := range lines {
		// Truncate before styling to prevent wrapping.
		line = truncateLine(line, codeWidth-1) // account for PaddingLeft(1)
		styled := codeStyle.Width(codeWidth).Render(line)
		rendered = append(rendered, styled)
	}

	content = strings.Join(rendered, "\n")

	// Build caption with results info
	var captionParts []string
	if totalResults == 1 {
		captionParts = append(captionParts, "1 result")
	} else {
		captionParts = append(captionParts, fmt.Sprintf("%d results", totalResults))
	}
	if hiddenCount > 0 {
		captionParts = append(captionParts, fmt.Sprintf("%d more", hiddenCount))
	}

	if len(captionParts) > 1 || hiddenCount > 0 {
		ty := herald.New(herald.WithTheme(herald.Theme{
			FigureCaption:         lipgloss.NewStyle().Foreground(theme.Muted),
			FigureCaptionPosition: herald.CaptionBottom,
		}))
		caption := strings.Join(captionParts, " • ")
		result := ty.Figure(content, caption)

		// Indent entire block (content + caption) to match other tools
		const blockIndent = "  "
		resultLines := strings.Split(result, "\n")
		for i, line := range resultLines {
			resultLines[i] = blockIndent + line
		}
		return strings.Join(resultLines, "\n")
	}

	// Single result with no truncation - just return indented content
	const blockIndent = "  "
	contentLines := strings.Split(content, "\n")
	for i, line := range contentLines {
		contentLines[i] = blockIndent + line
	}
	return strings.Join(contentLines, "\n")
}

// renderGrepBody renders grep output as a plain list with code background.
// Similar to find but with match-specific caption terminology.
func renderGrepBody(toolResult string, width int) string {
	content := strings.TrimSpace(toolResult)
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	totalMatches := len(lines)

	// Truncate to maxLsLines for display
	var hiddenCount int
	if len(lines) > maxLsLines {
		hiddenCount = len(lines) - maxLsLines
		lines = lines[:maxLsLines]
	}

	const lineIndent = "  "
	codeWidth := max(width-len(lineIndent), 20)

	theme := GetTheme()
	codeStyle := lipgloss.NewStyle().Background(theme.CodeBg).PaddingLeft(1)

	var rendered []string
	for _, line := range lines {
		// Truncate before styling to prevent wrapping.
		line = truncateLine(line, codeWidth-1) // account for PaddingLeft(1)
		styled := codeStyle.Width(codeWidth).Render(line)
		rendered = append(rendered, styled)
	}

	content = strings.Join(rendered, "\n")

	// Build caption with match info
	var captionParts []string
	if totalMatches == 1 {
		captionParts = append(captionParts, "1 match")
	} else {
		captionParts = append(captionParts, fmt.Sprintf("%d matches", totalMatches))
	}
	if hiddenCount > 0 {
		captionParts = append(captionParts, fmt.Sprintf("%d more", hiddenCount))
	}

	if len(captionParts) > 1 || hiddenCount > 0 {
		ty := herald.New(herald.WithTheme(herald.Theme{
			FigureCaption:         lipgloss.NewStyle().Foreground(theme.Muted),
			FigureCaptionPosition: herald.CaptionBottom,
		}))
		caption := strings.Join(captionParts, " • ")
		result := ty.Figure(content, caption)

		// Indent entire block (content + caption) to match other tools
		const blockIndent = "  "
		resultLines := strings.Split(result, "\n")
		for i, line := range resultLines {
			resultLines[i] = blockIndent + line
		}
		return strings.Join(resultLines, "\n")
	}

	// Single match with no truncation - just return indented content
	const blockIndent = "  "
	contentLines := strings.Split(content, "\n")
	for i, line := range contentLines {
		contentLines[i] = blockIndent + line
	}
	return strings.Join(contentLines, "\n")
}

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

	const lineIndent = "  "
	codeWidth := max(width-len(lineIndent), 20)

	theme := GetTheme()
	codeStyle := lipgloss.NewStyle().Background(theme.CodeBg).PaddingLeft(1)

	var rendered []string
	for _, line := range lines {
		// Truncate before styling to prevent wrapping.
		line = truncateLine(line, codeWidth-1) // account for PaddingLeft(1)
		styled := codeStyle.Width(codeWidth).Render(line)
		rendered = append(rendered, styled)
	}

	content = strings.Join(rendered, "\n")

	// Build caption with hidden entries info
	if hiddenCount > 0 {
		ty := herald.New(herald.WithTheme(herald.Theme{
			FigureCaption:         lipgloss.NewStyle().Foreground(theme.Muted),
			FigureCaptionPosition: herald.CaptionBottom,
		}))
		caption := fmt.Sprintf("%d more entries", hiddenCount)
		result := ty.Figure(content, caption)

		// Indent entire block (content + caption) to match other tools
		const blockIndent = "  "
		resultLines := strings.Split(result, "\n")
		for i, line := range resultLines {
			resultLines[i] = blockIndent + line
		}
		return strings.Join(resultLines, "\n")
	}

	// No caption - just return indented content
	const blockIndent = "  "
	contentLines := strings.Split(content, "\n")
	for i, line := range contentLines {
		contentLines[i] = blockIndent + line
	}
	return strings.Join(contentLines, "\n")
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
		if matches := regexp.MustCompile(`of (\d+) total`).FindStringSubmatch(footer); len(matches) > 1 {
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

	caption := strings.Join(captionParts, " • ")

	// Use Figure with caption below content (default behavior)
	// Apply theme to ensure caption is positioned below
	figTheme := herald.Theme{
		FigureCaption:         lipgloss.NewStyle().Foreground(GetTheme().Muted),
		FigureCaptionPosition: herald.CaptionBottom,
	}
	tyFig := herald.New(herald.WithTheme(figTheme))
	result := tyFig.Figure(codeBlock, caption)

	// Indent entire block to match Write/Edit tools (2 spaces)
	const blockIndent = "  "
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		lines[i] = blockIndent + line
	}
	return strings.Join(lines, "\n")
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
func renderBashBody(toolArgs, toolResult string, width int) string {
	if strings.TrimSpace(toolResult) == "" {
		return ""
	}

	theme := GetTheme()
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
	lineWidth := max(width-len(lineIndent), 20)
	// Account for PaddingLeft(1) on the output/stderr styles
	maxLineChars := lineWidth - 1

	var rendered []string
	exitCode := -1 // -1 means not found
	inStderr := false
	for _, line := range lines {
		line = truncateLine(line, maxLineChars)
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

		if inStderr {
			styled := stderrStyle.Width(width - len(lineIndent)).Render(line)
			rendered = append(rendered, styled)
		} else {
			styled := outputStyle.Width(width - len(lineIndent)).Render(line)
			rendered = append(rendered, styled)
		}
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
		ty := herald.New(herald.WithTheme(herald.Theme{
			FigureCaption:         lipgloss.NewStyle().Foreground(theme.Muted),
			FigureCaptionPosition: herald.CaptionBottom,
		}))
		caption := strings.Join(captionParts, " • ")
		result := ty.Figure(content, caption)

		// Indent entire block (content + caption) to match other tools
		const blockIndent = "  "
		lines := strings.Split(result, "\n")
		for i, line := range lines {
			lines[i] = blockIndent + line
		}
		return strings.Join(lines, "\n")
	}

	// No caption - just return indented content
	const blockIndent = "  "
	contentLines := strings.Split(content, "\n")
	for i, line := range contentLines {
		contentLines[i] = blockIndent + line
	}
	return strings.Join(contentLines, "\n")
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
	theme := GetTheme()
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
	outputStyle := lipgloss.NewStyle().Background(theme.CodeBg).PaddingLeft(1)
	errorStyle := lipgloss.NewStyle().Foreground(theme.Error).Background(theme.CodeBg).PaddingLeft(1)

	const lineIndent = "  "
	lineWidth := max(width-len(lineIndent), 20)
	maxLineChars := lineWidth - 1 // account for PaddingLeft(1)

	var contentLines []string

	// Add status line
	styledStatus := outputStyle.Width(lineWidth).Render(truncateLine(statusLine, maxLineChars))
	contentLines = append(contentLines, lineIndent+styledStatus)

	// For successful results, extract a brief preview of the actual result
	if strings.Contains(statusLine, "successfully") {
		// Find where "Result:" starts and extract a preview
		if _, resultContent, found := strings.Cut(result, "Result:\n"); found {
			resultContent = strings.TrimSpace(resultContent)
			if resultContent != "" {
				// Show first few meaningful lines as preview
				previewLines := extractSubagentPreviewLines(resultContent, 5, maxLineChars)
				if len(previewLines) > 0 {
					// Add blank separator line
					blankLine := outputStyle.Width(lineWidth).Render("")
					contentLines = append(contentLines, lineIndent+blankLine)

					for _, line := range previewLines {
						styled := outputStyle.Width(lineWidth).Render(line)
						contentLines = append(contentLines, lineIndent+styled)
					}
				}
			}
		}
	} else {
		// For failed results, show error info
		if _, errorContent, found := strings.Cut(result, "Error:\n"); found {
			errorContent = strings.TrimSpace(errorContent)
			if errorContent != "" {
				previewLines := extractSubagentPreviewLines(errorContent, 3, maxLineChars)
				if len(previewLines) > 0 {
					blankLine := outputStyle.Width(lineWidth).Render("")
					contentLines = append(contentLines, lineIndent+blankLine)

					for _, line := range previewLines {
						styled := errorStyle.Width(lineWidth).Render(line)
						contentLines = append(contentLines, lineIndent+styled)
					}
				}
			}
		}
	}

	return strings.Join(contentLines, "\n")
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
