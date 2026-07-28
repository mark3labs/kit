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

	// Assistant prose carries no marker, so it is indented to the shared
	// content column rather than starting at the screen edge. Without this it
	// sits two columns left of every other block and the margin reads ragged.
	rendered := style.ToMarkdown(content, width-style.ContentOffset-1)
	rendered = style.Indent(rendered, style.ContentOffset)
	return styleMarginBottom(theme, rendered)
}

// ReasoningBlock renders a reasoning/thinking block with muted italic text.
// If duration > 0, shows "Thought for Xs" label. Otherwise shows just "Thought".
// The width parameter is the full terminal width and controls soft-wrapping so
// long reasoning lines don't get cut off.
func ReasoningBlock(content string, duration int64, width int, ty *herald.Typography, theme style.Theme) string {
	renderedContent := ReasoningContent(content, width, ty)
	if renderedContent == "" {
		return ""
	}
	return ReasoningBlockFromContent(renderedContent, duration, theme)
}

// ReasoningContent renders just the styled content portion of a reasoning
// block (muted italic, soft-wrapped, indented to the shared content column)
// without the duration label. This is the expensive part of ReasoningBlock;
// callers that render repeatedly (e.g. a streaming item with a live duration
// counter) can cache this and compose it with ReasoningBlockFromContent per
// frame.
//
// width is the full terminal width; the content column is derived from it.
func ReasoningContent(content string, width int, ty *herald.Typography) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	// Match live streaming styling: muted italic text.
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	contentStr := strings.TrimLeft(strings.Join(lines, "\n"), " \t\n")
	contentStr = wrapText(contentStr, style.ContentWidth(width))
	// Reasoning carries no marker, so like assistant prose it is indented to
	// the shared content column. Left at column 0 it sits two columns outside
	// every other block and the left margin reads ragged.
	contentStr = style.Indent(contentStr, style.ContentOffset)
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
	// The label is part of the block, so it sits in the same column as the
	// content above it.
	label = style.Indent(label, style.ContentOffset)

	return styleMarginBottom(theme, renderedContent+"\n"+label)
}

// AlertBody prepares content for a herald alert.
//
// herald prefixes every line of an alert with a two-column gutter bar but does
// not wrap: given a long line it emits a long line, which then wraps in the
// terminal emulator instead of in lipgloss. That corrupts the scroll list's
// height accounting, so alert content is wrapped to the content column first.
func AlertBody(content string, width int) string {
	return wrapText(content, style.ContentWidth(width))
}

// SystemBlock renders a system message with herald Note styling.
func SystemBlock(content string, width int, ty *herald.Typography, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		content = "No content available"
	}

	rendered := ty.Note(AlertBody(content, width))
	return styleMarginBottom(theme, rendered)
}

// CustomBlock renders a message with herald Note styling and a custom label.
// Content is rendered as markdown before being wrapped in the alert.
func CustomBlock(content, label string, width int, theme style.Theme) string {
	if strings.TrimSpace(content) == "" {
		content = "No content available"
	}

	// Render markdown first, at the width the alert body will occupy.
	rendered := style.ToMarkdown(content, style.ContentWidth(width))

	ty := style.NewNoteTypography(label)
	return styleMarginBottom(theme, ty.Note(AlertBody(rendered, width)))
}

// ErrorBlock renders an error message with herald Caution styling.
func ErrorBlock(errorMsg string, width int, ty *herald.Typography, theme style.Theme) string {
	rendered := ty.Caution(AlertBody(errorMsg, width))
	return styleMarginBottom(theme, rendered)
}

// styleMarginBottom applies a 1-line margin bottom using the theme.
func styleMarginBottom(theme style.Theme, content string) string {
	return style.GetCachedStyles().MarginBottom1.Render(content)
}

// wrapText hard-wraps a string to the given width using lipgloss, which is
// ANSI-aware and preserves escape sequences across line breaks. Unlike word
// wrapping this also breaks runs with no spaces in them, which is what keeps a
// long path or a base64 blob inside the terminal.
func wrapText(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}
