package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/fantasy"
)

type writeArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// NewWriteTool creates the write core tool.
func NewWriteTool() fantasy.AgentTool {
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "write",
			Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Automatically creates parent directories.",
			Parameters: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to write (relative or absolute)",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			Required: []string{"path", "content"},
		},
		handler: executeWrite,
	}
}

func executeWrite(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var args writeArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("path and content parameters are required"), nil
	}
	if args.Path == "" {
		return fantasy.NewTextErrorResponse("path parameter is required"), nil
	}

	absPath, err := resolvePath(args.Path)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid path: %v", err)), nil
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to create directories: %v", err)), nil
	}

	if err := os.WriteFile(absPath, []byte(args.Content), 0644); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to write file: %v", err)), nil
	}

	return fantasy.NewTextResponse(fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), args.Path)), nil
}
