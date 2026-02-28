//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"kit/ext"
)

// Init demonstrates the custom tool rendering system. It registers
// renderers that override how specific tools display their headers,
// result bodies, display names, border colors, and backgrounds.
//
// Usage:
//
//	kit -e examples/extensions/tool-renderer-demo.go
//
// Then ask the agent to read a file or run a bash command to see
// the custom rendering in action.
func Init(api ext.API) {
	// Custom renderer for the "read" tool: custom display name,
	// blue border, compact filename-only header.
	api.RegisterToolRenderer(ext.ToolRenderConfig{
		ToolName:    "read",
		DisplayName: "File",
		BorderColor: "#89b4fa", // Catppuccin blue
		RenderHeader: func(toolArgs string, width int) string {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
				return ""
			}
			path, _ := args["path"].(string)
			if path == "" {
				return "" // fall back to default
			}

			// Show just the filename, not the full path.
			parts := strings.Split(path, "/")
			name := parts[len(parts)-1]

			// Include offset/limit if present.
			var extras []string
			if offset, ok := args["offset"]; ok {
				extras = append(extras, fmt.Sprintf("from line %v", offset))
			}
			if limit, ok := args["limit"]; ok {
				extras = append(extras, fmt.Sprintf("max %v lines", limit))
			}

			result := name
			if len(extras) > 0 {
				result += " (" + strings.Join(extras, ", ") + ")"
			}

			if len(result) > width {
				return result[:width-3] + "..."
			}
			return result
		},
		// RenderBody is nil — fall back to the builtin read renderer
		// which already provides syntax-highlighted code blocks.
	})

	// Custom renderer for the "bash" tool: renamed to "Shell",
	// dark background, custom header with $ prefix.
	api.RegisterToolRenderer(ext.ToolRenderConfig{
		ToolName:    "bash",
		DisplayName: "Shell",
		Background:  "#1e1e2e", // Dark background
		BorderColor: "#a6e3a1", // Catppuccin green
		RenderHeader: func(toolArgs string, width int) string {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
				return ""
			}
			cmd, _ := args["command"].(string)
			if cmd == "" {
				return "" // fall back to default
			}

			// Show first line of command with a $ prefix.
			lines := strings.SplitN(cmd, "\n", 2)
			display := "$ " + lines[0]
			if len(lines) > 1 {
				display += " ..."
			}

			if len(display) > width {
				return display[:width-3] + "..."
			}
			return display
		},
		RenderBody: func(toolResult string, isError bool, width int) string {
			if isError {
				return "" // fall back to default error rendering
			}

			// Count lines and show a summary at the end.
			lines := strings.Split(toolResult, "\n")
			lineCount := len(lines)

			// Show the first few lines of output.
			maxShow := 10
			if lineCount <= maxShow {
				return toolResult
			}

			shown := strings.Join(lines[:maxShow], "\n")
			return fmt.Sprintf("%s\n\n[%d lines total, showing first %d]",
				shown, lineCount, maxShow)
		},
	})
}
