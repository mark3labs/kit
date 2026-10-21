//go:build ignore

package main

import (
	"fmt"
	"strings"

	"kit/ext"
)

// toolLines is how many lines of each tool result the transcript shows. The
// message inspector still shows the full result.
const toolLines = 6

// mcpQuiet is true when a successful MCP tool result is shown as one line.
// The option presentation-mcp-output sets it at session start.
var mcpQuiet = true

// Init sets up Kit for a talk or a demo on a projector. It hides the
// wordmark, the status bar and the count of queued messages, but keeps the
// model line. Every tool result shows at most toolLines lines, and a
// successful MCP tool result shows one line instead.
//
// The option presentation-mcp-output selects how MCP tool results appear.
// With "quiet", the default, a result shows how many lines are hidden. With
// "full", it shows like the result of any other tool. Set the option in the
// options block of the config file, or as KIT_OPT_PRESENTATION_MCP_OUTPUT.
//
// Usage: kit -e examples/extensions/presentation.go
func Init(api ext.API) {
	api.RegisterOption(ext.OptionDef{
		Name:        "presentation-mcp-output",
		Description: "How MCP tool results appear: quiet (one line) or full",
		Default:     "quiet",
	})

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		mcpQuiet = ctx.GetOption("presentation-mcp-output") != "full"
		ctx.SetUIVisibility(ext.UIVisibility{
			HideLogo:      true,
			HideStatusBar: true,
			HideSeparator: true,
		})
	})

	// Every tool: Kit renders the result as usual, with at most toolLines
	// lines.
	api.RegisterToolRenderer(ext.ToolRenderConfig{
		ToolName:     "*",
		MaxBodyLines: toolLines,
	})

	// MCP tools are named server__tool. This renderer is registered after
	// the one above, so it wins for them. An error and an empty result fall
	// back to Kit's own rendering, again with at most toolLines lines. With
	// the option set to full, every result does.
	api.RegisterToolRenderer(ext.ToolRenderConfig{
		ToolName:     "*__*",
		MaxBodyLines: toolLines,
		RenderBody: func(result string, isError bool, width int) string {
			if !mcpQuiet || isError || strings.TrimSpace(result) == "" {
				return ""
			}
			n := strings.Count(strings.TrimRight(result, "\n"), "\n") + 1
			if n == 1 {
				return "(1 line of output hidden)"
			}
			return fmt.Sprintf("(%d lines of output hidden)", n)
		},
	})
}
