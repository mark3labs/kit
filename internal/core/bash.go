package core

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"charm.land/fantasy"
)

const defaultBashTimeout = 120 * time.Second
const maxBashTimeout = 600 * time.Second

var bannedCommands = []string{
	"alias ", "bg ", "bind ", "builtin ",
	"caller ", "command ", "compgen ",
	"complete ", "compopt ", "coproc ",
	"dirs ", "disown ", "enable ",
	"fc ", "fg ", "hash ", "help ",
	"history ", "jobs ", "kill ",
	"logout ", "mapfile ", "popd ",
	"pushd ", "readonly ", "select ",
	"set ", "shopt ", "source ",
	"suspend ", "times ", "trap ",
	"type ", "typeset ", "ulimit ",
	"umask ", "unalias ", "wait ",
}

type bashArgs struct {
	Command string  `json:"command"`
	Timeout float64 `json:"timeout,omitempty"`
}

// NewBashTool creates the bash core tool.
func NewBashTool() fantasy.AgentTool {
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "bash",
			Description: "Execute a bash command. Returns stdout and stderr. Output is truncated to the last 2000 lines or 50KB. Optionally provide a timeout in seconds.",
			Parameters: map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Bash command to execute",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "Timeout in seconds (optional, default 120s, max 600s)",
				},
			},
			Required: []string{"command"},
		},
		handler: executeBash,
	}
}

func executeBash(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var args bashArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("command parameter is required"), nil
	}
	if args.Command == "" {
		return fantasy.NewTextErrorResponse("command parameter is required"), nil
	}

	// Check for banned commands
	for _, banned := range bannedCommands {
		if strings.HasPrefix(args.Command, banned) {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("command '%s' is not allowed", args.Command)), nil
		}
	}

	// Determine timeout
	timeout := defaultBashTimeout
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Second
		if timeout > maxBashTimeout {
			timeout = maxBashTimeout
		}
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", args.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if cmdCtx.Err() == context.DeadlineExceeded {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("command timed out after %v", timeout)), nil
		}
	}

	// Build result
	var result strings.Builder
	if stdout.Len() > 0 {
		result.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString("STDERR:\n")
		result.WriteString(stderr.String())
	}
	if exitCode != 0 {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(fmt.Sprintf("Exit code: %d", exitCode))
	}

	output := result.String()
	if output == "" {
		output = "(no output)"
	}

	// Truncate from tail (keep last N lines, most relevant for bash)
	tr := truncateTail(output, defaultMaxLines, defaultMaxBytes)

	if exitCode != 0 {
		return fantasy.NewTextErrorResponse(tr.Content), nil
	}
	return fantasy.NewTextResponse(tr.Content), nil
}
