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

type grepArgs struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Glob       string `json:"glob,omitempty"`
	IgnoreCase bool   `json:"ignore_case,omitempty"`
	Literal    bool   `json:"literal,omitempty"`
	Context    int    `json:"context,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// NewGrepTool creates the grep core tool.
func NewGrepTool() fantasy.AgentTool {
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "grep",
			Description: "Search file contents for a pattern. Returns matching lines with file paths and line numbers. Respects .gitignore. Output is truncated to 100 matches or 50KB. Long lines are truncated to 500 chars.",
			Parameters: map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Search pattern (regex or literal string)",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory or file to search (default: current directory)",
				},
				"glob": map[string]any{
					"type":        "string",
					"description": "Filter files by glob pattern, e.g. '*.ts' or '**/*.spec.ts'",
				},
				"ignore_case": map[string]any{
					"type":        "boolean",
					"description": "Case-insensitive search (default: false)",
				},
				"literal": map[string]any{
					"type":        "boolean",
					"description": "Treat pattern as literal string instead of regex (default: false)",
				},
				"context": map[string]any{
					"type":        "number",
					"description": "Number of context lines before and after each match (default: 0)",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of matches to return (default: 100)",
				},
			},
			Required: []string{"pattern"},
		},
		handler: executeGrep,
	}
}

func executeGrep(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var args grepArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("pattern parameter is required"), nil
	}
	if args.Pattern == "" {
		return fantasy.NewTextErrorResponse("pattern parameter is required"), nil
	}

	limit := 100
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

	// Build ripgrep command
	rgArgs := []string{
		"--line-number",
		"--no-heading",
		"--color=never",
		"--max-count=" + strconv.Itoa(limit),
	}

	if args.IgnoreCase {
		rgArgs = append(rgArgs, "--ignore-case")
	}
	if args.Literal {
		rgArgs = append(rgArgs, "--fixed-strings")
	}
	if args.Context > 0 {
		rgArgs = append(rgArgs, fmt.Sprintf("--context=%d", args.Context))
	}
	if args.Glob != "" {
		rgArgs = append(rgArgs, "--glob="+args.Glob)
	}

	rgArgs = append(rgArgs, args.Pattern, searchPath)

	cmd := exec.CommandContext(ctx, "rg", rgArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// rg exits with 1 when no matches found (not an error)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return fantasy.NewTextResponse("No matches found."), nil
			}
			if exitErr.ExitCode() == 2 {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("grep error: %s", stderr.String())), nil
			}
		}
		// rg not found — fall back to grep
		return grepFallback(ctx, args, searchPath, limit)
	}

	output := stdout.String()
	if output == "" {
		return fantasy.NewTextResponse("No matches found."), nil
	}

	// Truncate long lines
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		lines[i] = truncateLine(line, grepMaxLineLen)
	}
	output = strings.Join(lines, "\n")

	tr := truncateHead(output, limit, defaultMaxBytes)
	return fantasy.NewTextResponse(tr.Content), nil
}

// grepFallback uses standard grep when rg is not available.
func grepFallback(ctx context.Context, args grepArgs, searchPath string, limit int) (fantasy.ToolResponse, error) {
	grepArgs := []string{"-rn", "--color=never"}

	if args.IgnoreCase {
		grepArgs = append(grepArgs, "-i")
	}
	if args.Literal {
		grepArgs = append(grepArgs, "-F")
	}
	if args.Context > 0 {
		grepArgs = append(grepArgs, fmt.Sprintf("-C%d", args.Context))
	}
	if args.Glob != "" {
		grepArgs = append(grepArgs, "--include="+args.Glob)
	}

	grepArgs = append(grepArgs, args.Pattern, searchPath)

	cmd := exec.CommandContext(ctx, "grep", grepArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_ = cmd.Run()

	output := stdout.String()
	if output == "" {
		return fantasy.NewTextResponse("No matches found."), nil
	}

	tr := truncateHead(output, limit, defaultMaxBytes)
	return fantasy.NewTextResponse(tr.Content), nil
}
