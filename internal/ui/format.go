package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Renderer is the interface satisfied by both MessageRenderer and
// CompactRenderer. It allows model.go and cli.go to call rendering methods
// without branching on compact mode.
type Renderer interface {
	RenderUserMessage(content string, timestamp time.Time) UIMessage
	RenderAssistantMessage(content string, timestamp time.Time, modelName string) UIMessage
	RenderToolMessage(toolName, toolArgs, toolResult string, isError bool) UIMessage
	RenderSystemMessage(content string, timestamp time.Time) UIMessage
	RenderErrorMessage(errorMsg string, timestamp time.Time) UIMessage
	RenderDebugMessage(message string, timestamp time.Time) UIMessage
	RenderDebugConfigMessage(config map[string]any, timestamp time.Time) UIMessage
	SetWidth(width int)
}

// Compile-time checks that both renderers satisfy the Renderer interface.
var _ Renderer = (*MessageRenderer)(nil)
var _ Renderer = (*CompactRenderer)(nil)

// parseBashOutput parses <stdout>/<stderr> tagged output from bash tool
// results, styling stderr with the theme's error color. Returns the
// combined, styled output string with tags stripped.
//
// Shared by both MessageRenderer and CompactRenderer.
func parseBashOutput(result string, theme Theme) string {
	var formattedResult strings.Builder
	remaining := result

	for {
		// Find stderr tags
		stderrStart := strings.Index(remaining, "<stderr>")
		stderrEnd := strings.Index(remaining, "</stderr>")

		// Find stdout tags
		stdoutStart := strings.Index(remaining, "<stdout>")
		stdoutEnd := strings.Index(remaining, "</stdout>")

		// Process whichever comes first
		if stderrStart != -1 && stderrEnd != -1 && stderrEnd > stderrStart &&
			(stdoutStart == -1 || stderrStart < stdoutStart) {
			// Process stderr
			if stderrStart > 0 {
				formattedResult.WriteString(remaining[:stderrStart])
			}
			stderrContent := remaining[stderrStart+8 : stderrEnd]
			stderrContent = strings.Trim(stderrContent, "\n")
			if len(stderrContent) > 0 {
				styledContent := lipgloss.NewStyle().Foreground(theme.Error).Render(stderrContent)
				formattedResult.WriteString(styledContent)
			}
			remaining = remaining[stderrEnd+9:] // Skip past </stderr>

		} else if stdoutStart != -1 && stdoutEnd != -1 && stdoutEnd > stdoutStart {
			// Process stdout
			if stdoutStart > 0 {
				formattedResult.WriteString(remaining[:stdoutStart])
			}
			stdoutContent := remaining[stdoutStart+8 : stdoutEnd]
			stdoutContent = strings.Trim(stdoutContent, "\n")
			if len(stdoutContent) > 0 {
				formattedResult.WriteString(stdoutContent)
			}
			remaining = remaining[stdoutEnd+9:] // Skip past </stdout>

		} else {
			// No more tags, add remaining content
			formattedResult.WriteString(remaining)
			break
		}
	}

	return strings.TrimSpace(formattedResult.String())
}
