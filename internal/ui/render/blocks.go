// Package render provides pure rendering functions for message blocks.
// These functions are stateless and can be used by both streaming and
// historical message rendering paths, eliminating code duplication.
package render

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/indaco/herald"

	"github.com/mark3labs/kit/internal/ui/style"
)

// fileTokenPattern matches @file references in user text. Supports:
//   - @"path with spaces.txt" (quoted)
//   - @path/to/file.txt      (unquoted, no spaces)
var fileTokenPattern = regexp.MustCompile(`@"[^"]+"|@[^\s]+`)

// UserBlock renders a user message with herald Tip styling.
// The width parameter controls line wrapping so long messages don't overflow.
// Any @file tokens in the content are highlighted with the theme accent color.
func UserBlock(content string, width int, ty *herald.Typography, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		content = "(empty message)"
	}

	// Wrap content before passing to herald Alert so long lines break
	// inside the alert box. Subtract 4 to account for the alert bar
	// prefix ("│ ") and a small margin.
	if width > 4 {
		content = lipgloss.Wrap(content, width-4, "")
	}

	// Highlight @file tokens with accent color so file references are
	// visually distinct from surrounding prompt text.
	content = highlightFileTokens(content, theme)

	rendered := ty.Tip(content)
	return styleMarginBottom(theme, rendered)
}

// highlightFileTokens wraps @file tokens in the given text with the theme
// accent color so they stand out visually in rendered user messages.
func highlightFileTokens(text string, theme style.Theme) string {
	accentStyle := lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	return fileTokenPattern.ReplaceAllStringFunc(text, func(token string) string {
		return accentStyle.Render(token)
	})
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
// The width parameter controls soft-wrapping so long reasoning lines don't get cut off.
func ReasoningBlock(content string, duration int64, width int, ty *herald.Typography, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	// Match live streaming styling: muted italic text. Wrap before styling so
	// ANSI sequences from italics don't interfere with width calculations.
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	contentStr := strings.TrimLeft(strings.Join(lines, "\n"), " \t\n")
	if width > 4 { // mirror other blocks (User/Assistant) which subtract 4
		contentStr = lipgloss.Wrap(contentStr, width-4, "")
	}
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
