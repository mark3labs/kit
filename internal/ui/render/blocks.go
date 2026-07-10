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

// UserBlock-related rendering helpers and herald typography.

// HighlightFileTokens wraps @file tokens in the given text with the theme
// accent color so they stand out visually in rendered user messages.
func HighlightFileTokens(text string, theme style.Theme) string {
	accentStyle := style.GetCachedStyles().FileTokenAccent
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
	renderedContent := ReasoningContent(content, width, ty)
	if renderedContent == "" {
		return ""
	}
	return ReasoningBlockFromContent(renderedContent, duration, theme)
}

// ReasoningContent renders just the styled content portion of a reasoning
// block (muted italic, soft-wrapped) without the duration label. This is
// the expensive part of ReasoningBlock; callers that render repeatedly
// (e.g. a streaming item with a live duration counter) can cache this and
// compose it with ReasoningBlockFromContent per frame.
func ReasoningContent(content string, width int, ty *herald.Typography) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	// Match live streaming styling: muted italic text.
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	contentStr := strings.TrimLeft(strings.Join(lines, "\n"), " \t\n")
	if width > 4 {
		contentStr = wrapText(contentStr, width-4)
	}
	return style.GetCachedStyles().Muted.Render(ty.Italic(contentStr))
}

// ReasoningBlockFromContent composes a pre-rendered reasoning content block
// (from ReasoningContent) with the duration label and bottom margin. This is
// cheap relative to ReasoningContent and safe to call per frame.
func ReasoningBlockFromContent(renderedContent string, duration int64, theme style.Theme) string {
	if renderedContent == "" {
		return ""
	}
	cs := style.GetCachedStyles()

	// Build label based on duration
	var label string
	if duration > 0 {
		var durationStr string
		if duration < 1000 {
			durationStr = fmt.Sprintf("%dms", duration)
		} else {
			durationStr = fmt.Sprintf("%.1fs", float64(duration)/1000)
		}
		label = cs.VeryMuted.Render("Thought for ") + cs.Accent.Render(durationStr)
	} else {
		label = cs.VeryMuted.Render("Thought")
	}

	return styleMarginBottom(theme, renderedContent+"\n"+label)
}

// SystemBlock renders a system message with herald Note styling.
func SystemBlock(content string, ty *herald.Typography, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		content = "No content available"
	}

	rendered := ty.Note(content)
	return styleMarginBottom(theme, rendered)
}

// CustomBlock renders a message with herald Note styling and a custom label.
// Content is rendered as markdown before being wrapped in the alert. This
// creates a one-off Typography instance with the given label so callers
// can use any title (e.g. "Help", "Warning") without changing the shared
// typography's default "Info" label.
func CustomBlock(content, label string, width int, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		content = "No content available"
	}

	// Render markdown first — subtract 4 for the alert bar prefix ("│ ").
	mdWidth := max(width-4, 10)
	rendered := style.ToMarkdown(content, mdWidth)

	ty := herald.New(
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
		herald.WithAlertLabel(herald.AlertNote, label),
	)
	alertRendered := ty.Note(rendered)
	return styleMarginBottom(theme, alertRendered)
}

// ErrorBlock renders an error message with herald Caution styling.
func ErrorBlock(errorMsg string, ty *herald.Typography, theme style.Theme) string {
	rendered := ty.Caution(errorMsg)
	return styleMarginBottom(theme, rendered)
}

// styleMarginBottom applies a 1-line margin bottom using the theme.
func styleMarginBottom(theme style.Theme, content string) string {
	return style.GetCachedStyles().MarginBottom1.Render(content)
}

// wrapText soft-wraps a string to the given width using lipgloss, which is
// ANSI-aware and preserves escape sequences across line breaks.
func wrapText(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}
