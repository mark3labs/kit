package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// MessageType represents different categories of messages displayed in the UI,
// each with distinct visual styling and formatting rules.
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	ToolMessage
	ToolCallMessage // New type for showing tool calls in progress
	SystemMessage   // New type for KIT system messages (help, tools, etc.)
	ErrorMessage    // New type for error messages
)

// UIMessage encapsulates a fully rendered message ready for display in the UI,
// including its formatted content, display metrics, and metadata. Messages can
// be static or streaming (progressively updated).
type UIMessage struct {
	ID        string
	Type      MessageType
	Position  int
	Height    int
	Content   string
	Timestamp time.Time
	Streaming bool
}

// Helper functions to get theme colors
func getTheme() Theme {
	return GetTheme()
}

// toolDisplayNames maps raw tool names to human-friendly display names.
var toolDisplayNames = map[string]string{
	"bash":          "Bash",
	"read":          "Read",
	"write":         "Write",
	"edit":          "Edit",
	"grep":          "Grep",
	"find":          "Find",
	"ls":            "Ls",
	"run_shell_cmd": "Bash",
}

// toolDisplayName returns a human-friendly display name for a tool.
// Falls back to capitalizing the first letter of the raw name.
func toolDisplayName(rawName string) string {
	if display, ok := toolDisplayNames[rawName]; ok {
		return display
	}
	if rawName != "" {
		return strings.ToUpper(rawName[:1]) + rawName[1:]
	}
	return rawName
}

// formatToolParams formats tool input parameters for inline header display.
// Extracts the primary parameter (command/filePath) first, then shows
// remaining params as (key=val, ...). Truncates to maxWidth.
func formatToolParams(toolArgs string, maxWidth int) string {
	args := strings.TrimSpace(toolArgs)
	if args == "" || args == "{}" {
		return ""
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		// Fallback: strip braces and return raw content
		args = strings.TrimPrefix(args, "{")
		args = strings.TrimSuffix(args, "}")
		args = strings.TrimSpace(args)
		if len(args) > maxWidth && maxWidth > 3 {
			return args[:maxWidth-3] + "..."
		}
		return args
	}

	if len(params) == 0 {
		return ""
	}

	// Identify primary parameter by checking known keys in priority order
	primaryKeys := []string{"command", "filePath", "path", "pattern", "query", "url"}
	var primaryKey string
	var primaryVal string
	for _, key := range primaryKeys {
		if val, ok := params[key]; ok {
			primaryKey = key
			primaryVal = fmt.Sprintf("%v", val)
			break
		}
	}

	var result strings.Builder
	if primaryVal != "" {
		result.WriteString(primaryVal)
	}

	// Collect remaining parameters (skip large values like file content)
	var remaining []string
	for key, val := range params {
		if key == primaryKey {
			continue
		}
		valStr := fmt.Sprintf("%v", val)
		// Skip very large values (e.g., oldString, newString, content, todos)
		if len(valStr) > 100 {
			continue
		}
		remaining = append(remaining, fmt.Sprintf("%s=%s", key, valStr))
	}
	sort.Strings(remaining)

	if len(remaining) > 0 {
		if result.Len() > 0 {
			result.WriteString(" ")
		}
		result.WriteString("(")
		result.WriteString(strings.Join(remaining, ", "))
		result.WriteString(")")
	}

	str := result.String()
	if len(str) > maxWidth && maxWidth > 3 {
		return str[:maxWidth-3] + "..."
	}
	return str
}

// MessageRenderer handles the formatting and rendering of different message types
// with consistent styling, markdown support, and appropriate visual hierarchies
// for the standard (non-compact) display mode.
type MessageRenderer struct {
	width int
	debug bool

	// getToolRenderer returns extension-provided rendering overrides for a
	// specific tool. May be nil if no extensions are loaded. Used in
	// RenderToolMessage to check for custom header/body formatting before
	// falling back to builtin renderers.
	getToolRenderer func(toolName string) *ToolRendererData
}

// getSystemUsername returns the current system username, fallback to "User"
func getSystemUsername() string {
	if currentUser, err := user.Current(); err == nil && currentUser.Username != "" {
		return currentUser.Username
	}
	// Fallback to environment variable
	if username := os.Getenv("USER"); username != "" {
		return username
	}
	if username := os.Getenv("USERNAME"); username != "" {
		return username
	}
	return "User"
}

// NewMessageRenderer creates and initializes a new MessageRenderer with the specified
// terminal width and debug mode setting. The width parameter determines line wrapping
// and layout calculations.
func NewMessageRenderer(width int, debug bool) *MessageRenderer {
	return &MessageRenderer{
		width: width,
		debug: debug,
	}
}

// SetWidth updates the terminal width for the renderer, affecting how content
// is wrapped and formatted in subsequent render operations.
func (r *MessageRenderer) SetWidth(width int) {
	r.width = width
}

// RenderUserMessage renders a user's input message with distinctive right-aligned
// formatting, including the system username, timestamp, and markdown-rendered content.
// The message is displayed with a colored right border for visual distinction.
func (r *MessageRenderer) RenderUserMessage(content string, timestamp time.Time) UIMessage {
	// Format timestamp and username
	timeStr := timestamp.Local().Format("15:04")
	username := getSystemUsername()

	// Convert single newlines to paragraph breaks so they survive glamour's
	// markdown rendering (glamour treats single \n as a soft break).
	content = strings.ReplaceAll(content, "\n", "\n\n")

	theme := getTheme()

	messageContent := r.renderMarkdown(content, r.width-8) // Account for padding and borders

	// Create info line
	info := fmt.Sprintf(" %s (%s)", username, timeStr)

	// Combine content and info
	fullContent := strings.TrimSuffix(messageContent, "\n") + "\n" +
		lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)

	// Use the block renderer — left border with Primary color, no background.
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.Primary),
		WithMarginBottom(1),
	)

	return UIMessage{
		Type:      UserMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderAssistantMessage renders an AI assistant's response with left-aligned formatting,
// including the model name, timestamp, and markdown-rendered content. Empty responses
// are displayed with a special "Finished without output" message. The message features
// a colored left border for visual distinction.
func (r *MessageRenderer) RenderAssistantMessage(content string, timestamp time.Time, modelName string) UIMessage {
	// Format timestamp and model info with better defaults
	timeStr := timestamp.Local().Format("15:04")
	if modelName == "" {
		modelName = "Assistant"
	}

	// Handle empty content with better styling
	theme := getTheme()
	var messageContent string
	if strings.TrimSpace(content) == "" {
		messageContent = lipgloss.NewStyle().
			Italic(true).
			Foreground(theme.Muted).
			Align(lipgloss.Center).
			Render("Finished without output")
	} else {
		messageContent = r.renderMarkdown(content, r.width-8) // Account for padding and borders
	}

	// Create info line
	info := fmt.Sprintf(" %s (%s)", modelName, timeStr)

	// Combine content and info
	fullContent := strings.TrimSuffix(messageContent, "\n") + "\n" +
		lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)

	// Use the new block renderer — no borders for agent messages.
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithNoBorder(),
		WithMarginBottom(1),
	)

	return UIMessage{
		Type:      AssistantMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderSystemMessage renders KIT system messages such as help text, command outputs,
// and informational notifications. These messages are displayed with a distinctive system
// color border and "KIT System" label to differentiate them from user and AI content.
func (r *MessageRenderer) RenderSystemMessage(content string, timestamp time.Time) UIMessage {
	// Format timestamp
	timeStr := timestamp.Local().Format("15:04")

	// Handle empty content with better styling
	theme := getTheme()
	var messageContent string
	if strings.TrimSpace(content) == "" {
		messageContent = lipgloss.NewStyle().
			Italic(true).
			Foreground(theme.Muted).
			Align(lipgloss.Center).
			Render("No content available")
	} else {
		messageContent = r.renderMarkdown(content, r.width-8) // Account for padding and borders
	}

	// Create info line
	info := fmt.Sprintf(" KIT System (%s)", timeStr)

	// Combine content and info
	fullContent := strings.TrimSuffix(messageContent, "\n") + "\n" +
		lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)

	// Use the new block renderer
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.System),
		WithMarginBottom(1),
	)

	return UIMessage{
		Type:      SystemMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderDebugMessage renders diagnostic and debugging information with special formatting
// including a debug icon, colored border, and structured layout. Debug messages are only
// displayed when debug mode is enabled and help developers troubleshoot issues.
func (r *MessageRenderer) RenderDebugMessage(message string, timestamp time.Time) UIMessage {
	baseStyle := lipgloss.NewStyle()

	// Create the main message style with border using tool color
	theme := getTheme()
	style := baseStyle.
		Width(r.width - 3). // Account for left margin
		BorderLeft(true).
		Foreground(theme.Muted).
		BorderForeground(theme.Tool).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1).
		MarginLeft(2).  // Add left margin like other messages
		MarginBottom(1) // Add bottom margin

	// Format timestamp
	timeStr := timestamp.Local().Format("02 Jan 2006 03:04 PM")

	// Create header with debug icon
	header := baseStyle.
		Foreground(theme.Tool).
		Bold(true).
		Render("🔍 Debug Output")

	// Process and format the message content
	// Split into lines and format each one
	lines := strings.Split(message, "\n")
	var formattedLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			formattedLines = append(formattedLines, "  "+line)
		}
	}

	content := baseStyle.
		Foreground(theme.Muted).
		Render(strings.Join(formattedLines, "\n"))

	// Create info line
	info := baseStyle.
		Width(r.width - 5). // Account for margins and padding
		Foreground(theme.Muted).
		Render(fmt.Sprintf(" KIT (%s)", timeStr))

	// Combine all parts
	fullContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		content,
		info,
	)

	return UIMessage{
		Content: style.Render(fullContent),
		Height:  lipgloss.Height(style.Render(fullContent)),
	}
}

// RenderDebugConfigMessage renders configuration settings in a formatted debug display
// with key-value pairs shown in a structured layout. Used to display runtime configuration
// for debugging purposes with a distinctive icon and border styling.
func (r *MessageRenderer) RenderDebugConfigMessage(config map[string]any, timestamp time.Time) UIMessage {
	baseStyle := lipgloss.NewStyle()

	// Create the main message style with border using tool color
	theme := getTheme()
	style := baseStyle.
		Width(r.width - 1).
		BorderLeft(true).
		Foreground(theme.Muted).
		BorderForeground(theme.Tool).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1)

	// Format timestamp
	timeStr := timestamp.Local().Format("02 Jan 2006 03:04 PM")

	// Create header with debug icon
	header := baseStyle.
		Foreground(theme.Tool).
		Bold(true).
		Render("🔧 Debug Configuration")

	// Format configuration settings
	var configLines []string
	for key, value := range config {
		if value != nil {
			configLines = append(configLines, fmt.Sprintf("  %s: %v", key, value))
		}
	}

	configContent := baseStyle.
		Foreground(theme.Muted).
		Render(strings.Join(configLines, "\n"))

	// Create info line
	info := baseStyle.
		Width(r.width - 1).
		Foreground(theme.Muted).
		Render(fmt.Sprintf(" KIT (%s)", timeStr))

	// Combine parts
	parts := []string{header}
	if len(configLines) > 0 {
		parts = append(parts, configContent)
	}
	parts = append(parts, info)

	rendered := style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)

	return UIMessage{
		Type:      SystemMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderErrorMessage renders error notifications with distinctive red coloring and
// bold text to ensure visibility. Error messages include timestamp information and
// are displayed with an error-colored border for immediate recognition.
func (r *MessageRenderer) RenderErrorMessage(errorMsg string, timestamp time.Time) UIMessage {
	// Format timestamp
	timeStr := timestamp.Local().Format("15:04")

	// Format error content
	theme := getTheme()
	errorContent := lipgloss.NewStyle().
		Foreground(theme.Error).
		Bold(true).
		Render(errorMsg)

	// Create info line
	info := fmt.Sprintf(" Error (%s)", timeStr)

	// Combine content and info
	fullContent := errorContent + "\n" +
		lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)

	// Use the new block renderer
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.Error),
		WithMarginBottom(1),
	)

	return UIMessage{
		Type:      ErrorMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderToolCallMessage renders a notification that a tool is being executed, showing
// the tool name, formatted arguments (if any), and execution timestamp. The message
// uses tool-specific coloring to distinguish it from regular conversation messages.
func (r *MessageRenderer) RenderToolCallMessage(toolName, toolArgs string, timestamp time.Time) UIMessage {
	// Format timestamp
	timeStr := timestamp.Local().Format("15:04")

	// Format arguments with better presentation
	theme := getTheme()
	var argsContent string
	if toolArgs != "" && toolArgs != "{}" {
		argsContent = lipgloss.NewStyle().
			Foreground(theme.Muted).
			Italic(true).
			Render(fmt.Sprintf("Arguments: %s", r.formatToolArgs(toolArgs)))
	}

	// Create info line
	info := fmt.Sprintf(" Executing %s (%s)", toolName, timeStr)

	// Combine parts
	var fullContent string
	if argsContent != "" {
		fullContent = argsContent + "\n" +
			lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)
	} else {
		fullContent = lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)
	}

	// Use the new block renderer
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.Tool),
		WithMarginBottom(1),
	)

	return UIMessage{
		Type:      ToolCallMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderToolMessage renders a unified tool block combining the tool invocation
// header (icon + display name + params) with the execution result body. The
// border color indicates status: green for success, red for error. This replaces
// the previous two-block approach (separate call + result blocks).
func (r *MessageRenderer) RenderToolMessage(toolName, toolArgs, toolResult string, isError bool) UIMessage {
	theme := getTheme()

	// Resolve extension renderer once for all overrides.
	var extRd *ToolRendererData
	if r.getToolRenderer != nil {
		extRd = r.getToolRenderer(toolName)
	}

	// --- Header: [icon] [name] [params] ---
	var icon string
	borderColor := theme.Success
	iconColor := theme.Success
	if isError {
		icon = "×"
		borderColor = theme.Error
		iconColor = theme.Error
	} else {
		icon = "✓"
	}

	// Extension can override border color (applies to both success and error).
	if extRd != nil && extRd.BorderColor != "" {
		borderColor = lipgloss.Color(extRd.BorderColor)
	}

	iconStr := lipgloss.NewStyle().Foreground(iconColor).Bold(true).Render(icon)

	// Extension can override display name.
	displayName := toolDisplayName(toolName)
	if extRd != nil && extRd.DisplayName != "" {
		displayName = extRd.DisplayName
	}
	nameStr := lipgloss.NewStyle().Foreground(theme.Tool).Bold(true).Render(displayName)

	// Format params with width budget for the header line.
	// Check extension renderer for custom header params first.
	paramBudget := max(r.width-10-len(displayName), 20)
	var params string
	if extRd != nil && extRd.RenderHeader != nil {
		params = extRd.RenderHeader(toolArgs, paramBudget)
	}
	if params == "" {
		params = formatToolParams(toolArgs, paramBudget)
	}

	header := iconStr + " " + nameStr
	if params != "" {
		header += " " + lipgloss.NewStyle().Foreground(theme.Muted).Render(params)
	}

	// --- Body: check extension renderer first, then builtin, then default ---
	var body string
	if extRd != nil && extRd.RenderBody != nil {
		body = extRd.RenderBody(toolResult, isError, r.width-8)
		// Apply markdown rendering if requested and body is non-empty.
		if body != "" && extRd.BodyMarkdown {
			body = strings.TrimSuffix(toMarkdown(body, r.width-8), "\n")
		}
	}
	if body == "" {
		if isError {
			body = lipgloss.NewStyle().
				Foreground(theme.Error).
				Render(toolResult)
		} else {
			body = renderToolBody(toolName, toolArgs, toolResult, r.width-8)
			if body == "" {
				body = r.formatToolResult(toolName, toolResult, r.width-8)
			}
		}
	}

	if strings.TrimSpace(body) == "" {
		body = lipgloss.NewStyle().
			Italic(true).
			Foreground(theme.Muted).
			Render("(no output)")
	}

	// Combine header + body into a single block.
	fullContent := header + "\n\n" + strings.TrimSuffix(body, "\n")

	// Build rendering options; extension can override background.
	blockOpts := []renderingOption{
		WithAlign(lipgloss.Left),
		WithBorderColor(borderColor),
		WithMarginBottom(1),
	}
	if extRd != nil && extRd.Background != "" {
		blockOpts = append(blockOpts, WithBackground(lipgloss.Color(extRd.Background)))
	}

	rendered := renderContentBlock(
		fullContent,
		r.width,
		blockOpts...,
	)

	return UIMessage{
		Type:    ToolMessage,
		Content: rendered,
		Height:  lipgloss.Height(rendered),
	}
}

// formatToolArgs formats tool arguments for display
func (r *MessageRenderer) formatToolArgs(args string) string {
	// Remove outer braces and clean up JSON formatting
	args = strings.TrimSpace(args)
	if strings.HasPrefix(args, "{") && strings.HasSuffix(args, "}") {
		args = strings.TrimPrefix(args, "{")
		args = strings.TrimSuffix(args, "}")
		args = strings.TrimSpace(args)
	}

	// If it's empty after cleanup, return a placeholder
	if args == "" {
		return "(no arguments)"
	}

	// Truncate if too long, but skip truncation in debug mode
	if !r.debug {
		maxLen := 100
		if len(args) > maxLen {
			return args[:maxLen] + "..."
		}
	}

	return args
}

// formatToolResult formats tool results based on tool type
func (r *MessageRenderer) formatToolResult(toolName, result string, width int) string {
	baseStyle := lipgloss.NewStyle()

	// Truncate very long results only if not in debug mode
	if !r.debug {
		maxLines := 10
		lines := strings.Split(result, "\n")
		if len(lines) > maxLines {
			result = strings.Join(lines[:maxLines], "\n") + "\n... (truncated)"
		}
	}

	// Format bash/command output with better formatting
	if strings.Contains(toolName, "bash") || strings.Contains(toolName, "command") || strings.Contains(toolName, "shell") || toolName == "run_shell_cmd" {
		theme := getTheme()

		// Split result into sections if it contains both stdout and stderr
		if strings.Contains(result, "<stdout>") || strings.Contains(result, "<stderr>") {
			return r.formatBashOutput(result, width, theme)
		}

		// For simple output, just render as monospace text with proper line breaks
		return baseStyle.
			Width(width).
			Foreground(theme.Muted).
			Render(result)
	}

	// For other tools, render as muted text
	theme := getTheme()
	return baseStyle.
		Width(width).
		Foreground(theme.Muted).
		Render(result)
}

// formatBashOutput formats bash command output with proper section handling.
// Delegates tag parsing to the shared parseBashOutput helper.
func (r *MessageRenderer) formatBashOutput(result string, width int, theme Theme) string {
	parsed := parseBashOutput(result, theme)
	return lipgloss.NewStyle().
		Width(width).
		Foreground(theme.Muted).
		Render(parsed)
}

// renderMarkdown renders markdown content using glamour
func (r *MessageRenderer) renderMarkdown(content string, width int) string {
	rendered := toMarkdown(content, width)
	return strings.TrimSuffix(rendered, "\n")
}
