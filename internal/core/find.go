package core

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"charm.land/fantasy"
)

type findArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// NewFindTool creates the find core tool.
func NewFindTool() fantasy.AgentTool {
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "find",
			Description: "Search for files by glob pattern. Returns matching file paths relative to the search directory. Respects .gitignore. Output is truncated to 1000 results or 50KB.",
			Parameters: map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match files, e.g. '*.ts', '**/*.json', or 'src/**/*.spec.ts'",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to search in (default: current directory)",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of results (default: 1000)",
				},
			},
			Required: []string{"pattern"},
		},
		handler: executeFind,
	}
}

func executeFind(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var args findArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("pattern parameter is required"), nil
	}
	if args.Pattern == "" {
		return fantasy.NewTextErrorResponse("pattern parameter is required"), nil
	}

	limit := 1000
	if args.Limit > 0 {
		limit = args.Limit
	}

	searchPath := "."
	if args.Path != "" {
		resolved, err := resolvePath(args.Path)
		if err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid path: %v", err)), nil
		}
		searchPath = resolved
	}

	// Try fd first (faster, respects .gitignore by default)
	result, err := findWithFd(ctx, args.Pattern, searchPath, limit)
	if err == nil {
		return result, nil
	}

	// Fall back to find + globbing
	return findWithFind(ctx, args.Pattern, searchPath, limit)
}

func findWithFd(ctx context.Context, pattern, searchPath string, limit int) (fantasy.ToolResponse, error) {
	fdArgs := []string{
		"--glob", pattern,
		"--hidden",
		"--max-results", strconv.Itoa(limit),
		".", // search current or specified path
	}

	cmd := exec.CommandContext(ctx, "fd", fdArgs...)
	cmd.Dir = searchPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("fd failed: %w: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return fantasy.NewTextResponse("No files found."), nil
	}

	tr := truncateHead(output, limit, defaultMaxBytes)
	return fantasy.NewTextResponse(tr.Content), nil
}

func findWithFind(ctx context.Context, pattern, searchPath string, limit int) (fantasy.ToolResponse, error) {
	// Use find with -name for simple patterns
	findArgs := []string{searchPath, "-name", pattern, "-type", "f"}

	cmd := exec.CommandContext(ctx, "find", findArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_ = cmd.Run()

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return fantasy.NewTextResponse("No files found."), nil
	}

	// Apply limit
	lines := strings.Split(output, "\n")
	if len(lines) > limit {
		lines = lines[:limit]
		output = strings.Join(lines, "\n")
		output += fmt.Sprintf("\n[truncated: showing %d of more results]", limit)
	}

	tr := truncateHead(output, limit, defaultMaxBytes)
	return fantasy.NewTextResponse(tr.Content), nil
}
