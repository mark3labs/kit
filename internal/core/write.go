package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
)

type writeArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// NewWriteTool creates the write core tool.
func NewWriteTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
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
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeWrite(ctx, call, cfg.WorkDir)
		},
	}
}

func executeWrite(ctx context.Context, call fantasy.ToolCall, workDir string) (fantasy.ToolResponse, error) {
	var args writeArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("path and content parameters are required"), nil
	}
	if args.Path == "" {
		return fantasy.NewTextErrorResponse("path parameter is required"), nil
	}

	absPath, err := resolvePathWithWorkDir(args.Path, workDir)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid path: %v", err)), nil
	}

	// Read existing content before writing (for diff metadata).
	var beforeContent string
	isNew := true
	if existing, readErr := os.ReadFile(absPath); readErr == nil {
		beforeContent = string(existing)
		isNew = false
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to create directories: %v", err)), nil
	}

	if err := os.WriteFile(absPath, []byte(args.Content), 0644); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to write file: %v", err)), nil
	}

	resp := fantasy.NewTextResponse(fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), args.Path))
	return fantasy.WithResponseMetadata(resp, writeDiffMeta(absPath, beforeContent, args.Content, isNew)), nil
}

// writeDiffMeta builds the structured metadata attached to write tool responses.
func writeDiffMeta(path, beforeContent, afterContent string, isNew bool) map[string]any {
	additions := strings.Count(afterContent, "\n") + 1
	deletions := 0
	if !isNew {
		deletions = strings.Count(beforeContent, "\n") + 1
	}
	return map[string]any{
		"file_diffs": []map[string]any{{
			"path":      path,
			"additions": additions,
			"deletions": deletions,
			"is_new":    isNew,
			"diff_blocks": []map[string]any{{
				"old_text": beforeContent,
				"new_text": afterContent,
			}},
		}},
	}
}
