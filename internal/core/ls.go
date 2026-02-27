package core

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"charm.land/fantasy"
)

type lsArgs struct {
	Path  string `json:"path,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// NewLsTool creates the ls core tool.
func NewLsTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "ls",
			Description: "List directory contents. Returns entries sorted alphabetically, with '/' suffix for directories. Includes dotfiles. Output is truncated to 500 entries or 50KB.",
			Parameters: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to list (default: current directory)",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of entries to return (default: 500)",
				},
			},
			Required: []string{},
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeLs(ctx, call, cfg.WorkDir)
		},
	}
}

func executeLs(ctx context.Context, call fantasy.ToolCall, workDir string) (fantasy.ToolResponse, error) {
	var args lsArgs
	_ = parseArgs(call.Input, &args) // optional args

	limit := 500
	if args.Limit > 0 {
		limit = args.Limit
	}

	dirPath := "."
	if args.Path != "" {
		resolved, err := resolvePathWithWorkDir(args.Path, workDir)
		if err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid path: %v", err)), nil
		}
		dirPath = resolved
	} else if workDir != "" {
		dirPath = workDir
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("cannot access '%s': %v", args.Path, err)), nil
	}
	if !info.IsDir() {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("'%s' is not a directory", args.Path)), nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to read directory: %v", err)), nil
	}

	// Sort alphabetically (case-insensitive)
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	var result strings.Builder
	count := 0
	for _, entry := range entries {
		if count >= limit {
			result.WriteString(fmt.Sprintf("\n[truncated: showing %d of %d entries]", limit, len(entries)))
			break
		}
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		result.WriteString(name + "\n")
		count++
	}

	output := result.String()
	if output == "" {
		return fantasy.NewTextResponse("(empty directory)"), nil
	}

	return fantasy.NewTextResponse(strings.TrimRight(output, "\n")), nil
}
