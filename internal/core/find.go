package core

import (
	"bufio"
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
func NewFindTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
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
			Parallel: true,
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeFind(ctx, call, cfg.WorkDir)
		},
	}
}

func executeFind(ctx context.Context, call fantasy.ToolCall, workDir string) (fantasy.ToolResponse, error) {
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
		resolved, err := resolvePathWithWorkDir(args.Path, workDir)
		if err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid path: %v", err)), nil
		}
		searchPath = resolved
	} else if workDir != "" {
		searchPath = workDir
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
		"-L", // follow symlinks
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
	findArgs := []string{"-L", searchPath, "-name", pattern, "-type", "f"}

	cmd := exec.CommandContext(ctx, "find", findArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("find failed to start: %w", err)
	}

	var result strings.Builder
	scanner := bufio.NewScanner(stdout)
	count := 0
	for scanner.Scan() {
		if count >= limit {
			fmt.Fprintf(&result, "\n[truncated: showing %d of more results]", limit)
			break
		}
		result.WriteString(scanner.Text() + "\n")
		count++
	}

	// Terminate process early if we reached our limit to prevent unbounded traversal.
	if count >= limit && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	_ = cmd.Wait()

	if count == 0 {
		if stderr.Len() > 0 {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("find error: %s", strings.TrimSpace(stderr.String()))), nil
		}
		return fantasy.NewTextResponse("No files found."), nil
	}

	output := strings.TrimSpace(result.String())
	tr := truncateHead(output, limit, defaultMaxBytes)
	return fantasy.NewTextResponse(tr.Content), nil
}
