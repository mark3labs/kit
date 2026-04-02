// Package render provides pure rendering functions for message blocks.
// These functions are stateless and can be used by both streaming and
// historical message rendering paths, eliminating code duplication.
package render

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/indaco/herald"

	"github.com/mark3labs/kit/internal/ui/style"
)

// UserBlock renders a user message with herald Tip styling.
func UserBlock(content string, ty *herald.Typography, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		content = "(empty message)"
	}

	rendered := ty.Tip(content)
	return styleMarginBottom(theme, rendered)
}

// AssistantBlock renders an assistant message with markdown styling.
func AssistantBlock(content string, width int, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	rendered := style.ToMarkdown(content, width-4)
	return styleMarginBottom(theme, rendered)
}

// ReasoningBlock renders a reasoning/thinking block with muted italic text.
// If duration > 0, shows "Thought for Xs" label. Otherwise shows just "Thought".
func ReasoningBlock(content string, duration int64, ty *herald.Typography, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	// Match live streaming styling: muted italic text
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	contentStr := strings.TrimLeft(strings.Join(lines, "\n"), " \t\n")
	mutedStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	contentRendered := mutedStyle.Render(ty.Italic(contentStr))

	// Build label based on duration
	if duration > 0 {
		var durationStr string
		if duration < 1000 {
			durationStr = fmt.Sprintf("%dms", duration)
		} else {
			durationStr = fmt.Sprintf("%.1fs", float64(duration)/1000)
		}
		labelPart := lipgloss.NewStyle().Foreground(theme.VeryMuted).Render("Thought for ")
		durationPart := lipgloss.NewStyle().Foreground(theme.Accent).Render(durationStr)
		label := labelPart + durationPart
		rendered := contentRendered + "\n" + label
		return styleMarginBottom(theme, rendered)
	}

	label := lipgloss.NewStyle().Foreground(theme.VeryMuted).Render("Thought")
	rendered := contentRendered + "\n" + label

	return styleMarginBottom(theme, rendered)
}

// SystemBlock renders a system message with herald Note styling.
func SystemBlock(content string, ty *herald.Typography, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		content = "No content available"
	}

	rendered := ty.Note(content)
	return styleMarginBottom(theme, rendered)
}

// ErrorBlock renders an error message with herald Caution styling.
func ErrorBlock(errorMsg string, ty *herald.Typography, theme style.Theme) string {
	rendered := ty.Caution(errorMsg)
	return styleMarginBottom(theme, rendered)
}

// ToolBlock renders a tool execution result with header and body.
func ToolBlock(displayName, params, body string, isError bool, width int, ty *herald.Typography, theme style.Theme) string {
	var icon string
	iconColor := theme.Success
	if isError {
		icon = "×"
		iconColor = theme.Error
	} else {
		icon = "✓"
	}

	// Style the tool name with color
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

	if strings.TrimSpace(body) == "" {
		body = ty.Italic("(no output)")
	}

	// Compose: icon + name + params, then body
	fullContent := ty.Compose(
		headerLine,
		"",
		body,
	)
	return styleMarginBottom(theme, fullContent)
}

// styleMarginBottom applies a 1-line margin bottom using the theme.
func styleMarginBottom(theme style.Theme, content string) string {
	return lipgloss.NewStyle().MarginBottom(1).Render(content)
}
