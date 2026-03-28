package ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/indaco/herald"
)

// ansiEscapeRe matches ANSI escape sequences used for terminal styling.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// MessageType represents different categories of messages displayed in the UI,
// each with distinct visual styling and formatting rules.
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	ToolMessage
	ToolCallMessage
	SystemMessage
	ErrorMessage
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

// toolDisplayNames maps raw tool names to human-friendly display names.
var toolDisplayNames = map[string]string{
	"bash":  "Bash",
	"read":  "Read",
	"write": "Write",
	"edit":  "Edit",
	"grep":  "Grep",
	"find":  "Find",
	"ls":    "Ls",
}

// getTheme returns the current theme (helper for compact_renderer.go)
func getTheme() Theme {
	return GetTheme()
}

// toolDisplayName returns a human-friendly display name for a tool.
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
func formatToolParams(toolArgs string, maxWidth int) string {
	args := strings.TrimSpace(toolArgs)
	if args == "" || args == "{}" {
		return ""
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(args), &params); err != nil {
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

	bodyKeys := map[string]bool{
		"content":  true,
		"old_text": true,
		"new_text": true,
		"oldText":  true,
		"newText":  true,
		"edits":    true,
		"todos":    true,
	}
	var remaining []string
	for key, val := range params {
		if key == primaryKey {
			continue
		}
		if bodyKeys[key] {
			continue
		}
		valStr := fmt.Sprintf("%v", val)
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
type MessageRenderer struct {
	width           int
	debug           bool
	ty              *herald.Typography
	getToolRenderer func(toolName string) *ToolRendererData
}

// newMessageRenderer creates and initializes a new MessageRenderer
func newMessageRenderer(width int, debug bool) *MessageRenderer {
	return &MessageRenderer{
		width: width,
		debug: debug,
		ty:    createTypography(GetTheme()),
	}
}

// SetWidth updates the terminal width for the renderer
func (r *MessageRenderer) SetWidth(width int) {
	r.width = width
}

// RenderUserMessage renders a user's input message using herald Tip alert
func (r *MessageRenderer) RenderUserMessage(content string, timestamp time.Time) UIMessage {
	if strings.TrimSpace(content) == "" {
		content = "(empty message)"
	}

	rendered := r.ty.Tip(content)
	rendered = lipgloss.NewStyle().MarginBottom(1).Render(rendered)

	return UIMessage{
		Type:      UserMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderAssistantMessage renders an AI assistant's response
func (r *MessageRenderer) RenderAssistantMessage(content string, timestamp time.Time, modelName string) UIMessage {
	if strings.TrimSpace(content) == "" {
		return UIMessage{
			Type:      AssistantMessage,
			Content:   "",
			Height:    0,
			Timestamp: timestamp,
		}
	}

	// Use markdown rendering with Chroma syntax highlighting
	rendered := toMarkdown(content, r.width-4)
	rendered = lipgloss.NewStyle().MarginBottom(1).Render(rendered)

	return UIMessage{
		Type:      AssistantMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderSystemMessage renders KIT system messages using herald Note alert
func (r *MessageRenderer) RenderSystemMessage(content string, timestamp time.Time) UIMessage {
	if strings.TrimSpace(content) == "" {
		content = "No content available"
	}

	rendered := r.ty.Note(content)
	rendered = lipgloss.NewStyle().MarginBottom(1).Render(rendered)

	return UIMessage{
		Type:      SystemMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderDebugMessage renders diagnostic and debugging information
func (r *MessageRenderer) RenderDebugMessage(message string, timestamp time.Time) UIMessage {
	header := r.ty.H6("🔍 Debug Output")

	lines := strings.Split(message, "\n")
	var formattedLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			formattedLines = append(formattedLines, "  "+line)
		}
	}

	content := r.ty.Compose(
		header,
		r.ty.P(strings.Join(formattedLines, "\n")),
	)
	content = lipgloss.NewStyle().MarginBottom(1).Render(content)

	return UIMessage{
		Content: content,
		Height:  lipgloss.Height(content),
	}
}

// RenderDebugConfigMessage renders configuration settings
func (r *MessageRenderer) RenderDebugConfigMessage(config map[string]any, timestamp time.Time) UIMessage {
	header := r.ty.H6("🔧 Debug Configuration")

	var configLines []string
	for key, value := range config {
		if value != nil {
			configLines = append(configLines, fmt.Sprintf("%s: %v", key, value))
		}
	}

	var content string
	if len(configLines) > 0 {
		content = r.ty.Compose(
			header,
			r.ty.P(strings.Join(configLines, "\n")),
		)
	} else {
		content = header
	}
	content = lipgloss.NewStyle().MarginBottom(1).Render(content)

	return UIMessage{
		Type:      SystemMessage,
		Content:   content,
		Height:    lipgloss.Height(content),
		Timestamp: timestamp,
	}
}

// RenderErrorMessage renders error notifications
func (r *MessageRenderer) RenderErrorMessage(errorMsg string, timestamp time.Time) UIMessage {
	rendered := r.ty.Caution(errorMsg)
	rendered = lipgloss.NewStyle().MarginBottom(1).Render(rendered)

	return UIMessage{
		Type:      ErrorMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderToolMessage renders a unified tool block
func (r *MessageRenderer) RenderToolMessage(toolName, toolArgs, toolResult string, isError bool) UIMessage {
	var extRd *ToolRendererData
	if r.getToolRenderer != nil {
		extRd = r.getToolRenderer(toolName)
	}

	displayName := toolDisplayName(toolName)
	if extRd != nil && extRd.DisplayName != "" {
		displayName = extRd.DisplayName
	}

	paramBudget := max(r.width-10-len(displayName), 20)
	var params string
	if extRd != nil && extRd.RenderHeader != nil {
		params = extRd.RenderHeader(toolArgs, paramBudget)
	}
	if params == "" {
		params = formatToolParams(toolArgs, paramBudget)
	}

	var icon string
	iconColor := GetTheme().Success
	if isError {
		icon = "×"
		iconColor = GetTheme().Error
	} else {
		icon = "✓"
	}

	// Style the tool name with color
	theme := GetTheme()
	nameColor := theme.Info
	if isError {
		nameColor = theme.Error
	}
	styledName := lipgloss.NewStyle().Foreground(nameColor).Bold(true).Render(displayName)
	styledIcon := lipgloss.NewStyle().Foreground(iconColor).Render(icon)

	// Build the content: icon + name + params on first line, then body
	headerLine := styledIcon + " " + styledName
	if params != "" {
		headerLine += " " + lipgloss.NewStyle().Foreground(theme.Muted).Render(params)
	}

	// Get body content
	var body string
	if extRd != nil && extRd.RenderBody != nil {
		body = extRd.RenderBody(toolResult, isError, r.width-8)
		if body != "" && extRd.BodyMarkdown {
			body = strings.TrimSuffix(toMarkdown(body, r.width-8), "\n")
		}
	}
	if body == "" {
		if isError {
			body = r.formatToolResult(toolName, toolResult)
		} else {
			body = renderToolBody(toolName, toolArgs, toolResult, r.width-8)
			if body == "" {
				body = r.formatToolResult(toolName, toolResult)
			}
		}
	}

	if strings.TrimSpace(body) == "" {
		body = r.ty.Italic("(no output)")
	}

	// Compose: icon + name + params, then body
	fullContent := r.ty.Compose(
		headerLine,
		"",
		body,
	)
	fullContent = lipgloss.NewStyle().MarginBottom(1).Render(fullContent)

	return UIMessage{
		Type:    ToolMessage,
		Content: fullContent,
		Height:  lipgloss.Height(fullContent),
	}
}

// formatToolArgs formats tool arguments for display
func (r *MessageRenderer) formatToolArgs(args string) string {
	args = strings.TrimSpace(args)
	if strings.HasPrefix(args, "{") && strings.HasSuffix(args, "}") {
		args = strings.TrimPrefix(args, "{")
		args = strings.TrimSuffix(args, "}")
		args = strings.TrimSpace(args)
	}

	if args == "" {
		return "(no arguments)"
	}

	if !r.debug {
		maxLen := 100
		if len(args) > maxLen {
			return args[:maxLen] + "..."
		}
	}

	return args
}

// formatToolResult formats tool results based on tool type
func (r *MessageRenderer) formatToolResult(toolName, result string) string {
	if !r.debug {
		maxLines := 10
		lines := strings.Split(result, "\n")
		if len(lines) > maxLines {
			result = strings.Join(lines[:maxLines], "\n") + "\n... (truncated)"
		}
	}

	if strings.Contains(toolName, "bash") || strings.Contains(toolName, "command") ||
		strings.Contains(toolName, "shell") {
		if strings.Contains(result, "<stdout>") || strings.Contains(result, "<stderr>") {
			return parseBashOutput(result, GetTheme())
		}
	}

	return result
}

// createTypography creates a typography instance from theme
func createTypography(theme Theme) *herald.Typography {
	return herald.New(
		herald.WithPalette(herald.ColorPalette{
			Primary:   theme.Primary,
			Secondary: theme.Secondary,
			Tertiary:  theme.Info,
			Accent:    theme.Accent,
			Highlight: theme.Highlight,
			Muted:     theme.Muted,
			Text:      theme.Text,
			Surface:   theme.Background,
			Base:      theme.CodeBg,
		}),
		herald.WithAlertPalette(herald.AlertPalette{
			Note:      theme.Info,
			Tip:       theme.Success,
			Important: theme.Accent,
			Warning:   theme.Warning,
			Caution:   theme.Error,
		}),
		herald.WithCodeLineNumbers(true),
		// Customize alert labels
		herald.WithAlertLabel(herald.AlertNote, "Info"),
		herald.WithAlertLabel(herald.AlertTip, "You"),
		herald.WithAlertLabel(herald.AlertWarning, "Working"),
		herald.WithAlertLabel(herald.AlertCaution, "Error"),
	)
}

// removeBlankLines removes lines that are visually blank from rendered output.
func removeBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if strings.TrimSpace(ansiEscapeRe.ReplaceAllString(line, "")) != "" {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}
