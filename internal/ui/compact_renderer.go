package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// CompactRenderer handles rendering messages in a space-efficient compact format,
// optimized for terminals with limited vertical space. It displays messages with
// minimal decorations while maintaining readability and essential information.
type CompactRenderer struct {
	width int
	debug bool

	// getToolRenderer returns extension-provided rendering overrides for a
	// specific tool. May be nil if no extensions are loaded. Used in
	// RenderToolMessage to check for custom header/body formatting before
	// falling back to builtin renderers.
	getToolRenderer func(toolName string) *ToolRendererData
}

// NewCompactRenderer creates and initializes a new CompactRenderer with the specified
// terminal width and debug mode setting. The width parameter determines line wrapping,
// while debug enables additional diagnostic output in rendered messages.
func NewCompactRenderer(width int, debug bool) *CompactRenderer {
	return &CompactRenderer{
		width: width,
		debug: debug,
	}
}

// SetWidth updates the terminal width for the renderer, affecting how content
// is wrapped and formatted in subsequent render operations.
func (r *CompactRenderer) SetWidth(width int) {
	r.width = width
}

// RenderUserMessage renders a user's input message in compact format with a
// distinctive symbol (>) and label. The content is formatted to preserve structure
// while minimizing vertical space usage. Returns a UIMessage with formatted content
// and metadata.
func (r *CompactRenderer) RenderUserMessage(content string, timestamp time.Time) UIMessage {
	theme := GetTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Info).Render(">")
	label := lipgloss.NewStyle().Foreground(theme.Info).Bold(true).Render("User")

	// Only run markdown rendering when the message contains code spans or
	// fenced code blocks. Plain text is rendered directly so that newlines
	// are preserved without the extra paragraph spacing glamour adds.
	var compactContent string
	if strings.Contains(content, "`") {
		mdContent := strings.ReplaceAll(content, "\n", "\n\n")
		compactContent = r.formatUserAssistantContent(mdContent)
		compactContent = removeBlankLines(compactContent)
	} else {
		compactContent = content
	}

	// Handle multi-line content
	lines := strings.Split(compactContent, "\n")
	var formattedLines []string

	for i, line := range lines {
		if i == 0 {
			// First line includes symbol and label
			formattedLines = append(formattedLines, fmt.Sprintf("%s  %s %s", symbol, label, line))
		} else {
			// Subsequent lines without indentation for compact mode
			formattedLines = append(formattedLines, line)
		}
	}

	return UIMessage{
		Type:      UserMessage,
		Content:   strings.Join(formattedLines, "\n"),
		Height:    len(formattedLines),
		Timestamp: timestamp,
	}
}

// RenderAssistantMessage renders an AI assistant's response in compact format with
// a distinctive symbol (<) and the model name as label. Empty content is ignored
// and returns an empty message. Returns a UIMessage with formatted content and metadata.
func (r *CompactRenderer) RenderAssistantMessage(content string, timestamp time.Time, modelName string) UIMessage {
	// Ignore empty responses - don't render anything
	compactContent := r.formatUserAssistantContent(content)
	if compactContent == "" {
		return UIMessage{
			Type:      AssistantMessage,
			Content:   "",
			Height:    0,
			Timestamp: timestamp,
		}
	}

	theme := GetTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Primary).Render("<")

	// Use the full model name, fallback to "Assistant" if empty
	if modelName == "" {
		modelName = "Assistant"
	}
	label := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(modelName)

	// Handle multi-line content
	lines := strings.Split(compactContent, "\n")
	var formattedLines []string

	for i, line := range lines {
		if i == 0 {
			// First line includes symbol and label
			formattedLines = append(formattedLines, fmt.Sprintf("%s  %s %s", symbol, label, line))
		} else {
			// Subsequent lines without indentation for compact mode
			formattedLines = append(formattedLines, line)
		}
	}

	return UIMessage{
		Type:      AssistantMessage,
		Content:   strings.Join(formattedLines, "\n"),
		Height:    len(formattedLines),
		Timestamp: timestamp,
	}
}

// RenderToolMessage renders a unified tool block in compact format, combining
// the tool invocation header (icon + display name + params) with the execution
// result body. Status is indicated by icon: checkmark for success, cross for error.
func (r *CompactRenderer) RenderToolMessage(toolName, toolArgs, toolResult string, isError bool) UIMessage {
	theme := GetTheme()

	// Resolve extension renderer once for all overrides.
	var extRd *ToolRendererData
	if r.getToolRenderer != nil {
		extRd = r.getToolRenderer(toolName)
	}

	// Status icon
	var icon string
	iconColor := theme.Success
	if isError {
		icon = "×"
		iconColor = theme.Error
	} else {
		icon = "✓"
	}

	iconStr := lipgloss.NewStyle().Foreground(iconColor).Bold(true).Render(icon)

	// Extension can override display name.
	displayName := toolDisplayName(toolName)
	if extRd != nil && extRd.DisplayName != "" {
		displayName = extRd.DisplayName
	}
	nameStr := lipgloss.NewStyle().Foreground(theme.Info).Bold(true).Render(displayName)

	// Format params — check extension renderer first.
	paramBudget := max(r.width-10-len(displayName), 20)
	var params string
	if extRd != nil && extRd.RenderHeader != nil {
		params = extRd.RenderHeader(toolArgs, paramBudget)
	}
	if params == "" {
		params = formatToolParams(toolArgs, paramBudget)
	}

	// Build header line
	header := iconStr + " " + nameStr
	if params != "" {
		header += " " + lipgloss.NewStyle().Foreground(theme.Muted).Render(params)
	}

	// Format body: check extension renderer first, then compact builtin, then default.
	var body string
	if extRd != nil && extRd.RenderBody != nil {
		body = extRd.RenderBody(toolResult, isError, r.width-4)
		// Apply markdown rendering if requested and body is non-empty.
		if body != "" && extRd.BodyMarkdown {
			body = strings.TrimSuffix(toMarkdown(body, r.width-4), "\n")
		}
	}
	if body == "" {
		if isError {
			body = lipgloss.NewStyle().Foreground(theme.Error).Render(r.formatToolResult(toolResult))
		} else {
			// Use compact summary renderers instead of full tool body renderers.
			body = renderToolBodyCompact(toolName, toolArgs, toolResult, r.width-4)
			if body == "" {
				formatted := r.formatToolResult(toolResult)
				if formatted == "" {
					body = lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render("(no output)")
				} else {
					body = lipgloss.NewStyle().Foreground(theme.Muted).Render(formatted)
				}
			}
		}
	}

	// Combine header + indented body
	var lines []string
	lines = append(lines, header)
	if body != "" {
		for line := range strings.SplitSeq(body, "\n") {
			lines = append(lines, "  "+line)
		}
	}

	return UIMessage{
		Type:    ToolMessage,
		Content: strings.Join(lines, "\n"),
		Height:  len(lines),
	}
}

// RenderSystemMessage renders a system notification or informational message in
// compact format with a distinctive symbol (*) and "System" label. Content is
// formatted to fit on a single line for minimal space usage.
func (r *CompactRenderer) RenderSystemMessage(content string, timestamp time.Time) UIMessage {
	theme := GetTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Muted).Render("◇")
	label := lipgloss.NewStyle().Foreground(theme.Muted).Bold(true).Render("System")

	compactContent := r.formatCompactContent(content)

	line := fmt.Sprintf("%s  %-8s %s", symbol, label, compactContent)

	return UIMessage{
		Type:      SystemMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// RenderErrorMessage renders an error notification in compact format with a
// distinctive error symbol (!) and styling to ensure visibility. The error
// content is displayed in a single line with appropriate color highlighting.
func (r *CompactRenderer) RenderErrorMessage(errorMsg string, timestamp time.Time) UIMessage {
	theme := GetTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Error).Render("!")
	label := lipgloss.NewStyle().Foreground(theme.Error).Bold(true).Render("Error")

	compactContent := lipgloss.NewStyle().Foreground(theme.Error).Render(r.formatCompactContent(errorMsg))

	line := fmt.Sprintf("%s  %-8s %s", symbol, label, compactContent)

	return UIMessage{
		Type:      ErrorMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// RenderDebugMessage renders diagnostic information in compact format when debug
// mode is enabled. Messages are truncated if they exceed the available width to
// maintain single-line display.
func (r *CompactRenderer) RenderDebugMessage(message string, timestamp time.Time) UIMessage {
	theme := GetTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Tool).Render("*")
	label := lipgloss.NewStyle().Foreground(theme.Tool).Bold(true).Render("Debug")

	// Truncate message if too long
	content := message
	if len(content) > r.width-20 {
		content = content[:r.width-23] + "..."
	}

	line := fmt.Sprintf("%s  %-8s %s", symbol, label, content)

	return UIMessage{
		Type:      SystemMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// RenderDebugConfigMessage renders configuration settings in compact format for
// debugging purposes. Config entries are displayed as key=value pairs separated
// by commas, truncated if necessary to fit on a single line.
func (r *CompactRenderer) RenderDebugConfigMessage(config map[string]any, timestamp time.Time) UIMessage {
	theme := GetTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Tool).Render("*")
	label := lipgloss.NewStyle().Foreground(theme.Tool).Bold(true).Render("Debug")

	// Format config as compact key=value pairs
	var configPairs []string
	for key, value := range config {
		if value != nil {
			configPairs = append(configPairs, fmt.Sprintf("%s=%v", key, value))
		}
	}

	content := strings.Join(configPairs, ", ")
	if len(content) > r.width-20 {
		content = content[:r.width-23] + "..."
	}

	line := fmt.Sprintf("%s  %-8s %s", symbol, label, content)

	return UIMessage{
		Type:      SystemMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// formatCompactContent formats content for compact single-line display
func (r *CompactRenderer) formatCompactContent(content string) string {
	if content == "" {
		return ""
	}

	// Remove markdown formatting for compact display
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\t", " ")

	// Collapse multiple spaces
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}

	content = strings.TrimSpace(content)

	// Truncate if too long (unless in debug mode)
	maxLen := max(
		// Reserve space for symbol and label more conservatively
		r.width-28,
		// Minimum width for readability
		40)
	if !r.debug && len(content) > maxLen {
		content = content[:maxLen-3] + "..."
	}

	return content
}

// formatUserAssistantContent formats user and assistant content using glamour markdown rendering
func (r *CompactRenderer) formatUserAssistantContent(content string) string {
	if content == "" {
		return ""
	}

	// Calculate available width more conservatively
	// Account for: symbol (1) + spaces (2) + label (up to 20 chars) + space (1) + margin (4)
	availableWidth := max(r.width-28,
		// Minimum width for readability
		40)

	// Use glamour to render markdown content with proper width
	rendered := toMarkdown(content, availableWidth)
	return strings.TrimSuffix(rendered, "\n")
}

// wrapText wraps text to the specified width, preserving existing line breaks
func (r *CompactRenderer) wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	lines := strings.Split(text, "\n")
	var wrappedLines []string

	for _, line := range lines {
		if len(line) <= width {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		// Wrap long lines
		words := strings.Fields(line)
		if len(words) == 0 {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		currentLine := ""
		for _, word := range words {
			// If adding this word would exceed the width, start a new line
			if len(currentLine)+len(word)+1 > width && currentLine != "" {
				wrappedLines = append(wrappedLines, currentLine)
				currentLine = word
			} else {
				if currentLine == "" {
					currentLine = word
				} else {
					currentLine += " " + word
				}
			}
		}
		if currentLine != "" {
			wrappedLines = append(wrappedLines, currentLine)
		}
	}

	return strings.Join(wrappedLines, "\n")
}

// formatToolResult formats tool results preserving formatting but limiting to 5 lines
func (r *CompactRenderer) formatToolResult(result string) string {
	if result == "" {
		return ""
	}

	// Check if this is bash output with stdout/stderr tags
	if strings.Contains(result, "<stdout>") || strings.Contains(result, "<stderr>") {
		result = r.formatBashOutput(result)
	}

	// Calculate available width more conservatively
	availableWidth := max(r.width-28,
		// Minimum width for readability
		40)

	// First wrap the text to prevent long lines (tool results are usually plain text, not markdown)
	wrappedResult := r.wrapText(result, availableWidth)

	// Then limit to 5 lines
	lines := strings.Split(wrappedResult, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
		// Add truncation indicator
		if len(lines) == 5 && lines[4] != "" {
			lines[4] = lines[4] + "..."
		} else {
			lines = append(lines, "...")
		}
	}

	return strings.Join(lines, "\n")
}

// formatBashOutput formats bash command output by removing stdout/stderr tags
// and styling appropriately. Delegates tag parsing to the shared parseBashOutput
// helper.
func (r *CompactRenderer) formatBashOutput(result string) string {
	return parseBashOutput(result, GetTheme())
}
