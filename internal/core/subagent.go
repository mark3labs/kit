package core

import (
	"context"
	"fmt"
	"time"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/extensions"
)

const defaultSubagentTimeout = 5 * time.Minute
const maxSubagentTimeout = 30 * time.Minute

type subagentArgs struct {
	Task           string `json:"task"`
	Model          string `json:"model,omitempty"`
	SystemPrompt   string `json:"system_prompt,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// NewSubagentTool creates the spawn_subagent core tool.
func NewSubagentTool(opts ...ToolOption) fantasy.AgentTool {
	return &coreTool{
		info: fantasy.ToolInfo{
			Name: "spawn_subagent",
			Description: `Spawn a background subagent to perform a task autonomously.

The subagent runs as a separate Kit instance with full tool access. Use this to:
- Delegate independent subtasks that can run in parallel
- Perform research or analysis without blocking your main work
- Execute tasks that benefit from a fresh context window

The subagent result is returned when it completes. For long-running tasks,
consider breaking them into smaller focused subtasks.

Example use cases:
- "Research the authentication patterns in this codebase"
- "Write unit tests for the UserService class"
- "Analyze the performance bottlenecks in the database queries"`,
			Parameters: map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The complete task description for the subagent to perform",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Optional model override (e.g. 'anthropic/claude-haiku-3-5-20241022' for faster/cheaper tasks)",
				},
				"system_prompt": map[string]any{
					"type":        "string",
					"description": "Optional system prompt for domain-specific guidance",
				},
				"timeout_seconds": map[string]any{
					"type":        "number",
					"description": "Maximum execution time in seconds (default: 300, max: 1800)",
				},
			},
			Required: []string{"task"},
			Parallel: true,
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeSubagent(ctx, call)
		},
	}
}

func executeSubagent(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var args subagentArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("task parameter is required"), nil
	}
	if args.Task == "" {
		return fantasy.NewTextErrorResponse("task parameter is required"), nil
	}

	// Determine timeout
	timeout := defaultSubagentTimeout
	if args.TimeoutSeconds > 0 {
		timeout = min(time.Duration(args.TimeoutSeconds)*time.Second, maxSubagentTimeout)
	}

	// Spawn subagent in blocking mode
	_, result, err := extensions.SpawnSubagent(extensions.SubagentConfig{
		Prompt:       args.Task,
		Model:        args.Model,
		SystemPrompt: args.SystemPrompt,
		Timeout:      timeout,
		Blocking:     true,
	})
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to spawn subagent: %v", err)), nil
	}

	if result.Error != nil {
		// Subagent failed but we still have partial output
		response := fmt.Sprintf("Subagent failed (exit code %d) after %ds.\n\nError: %v",
			result.ExitCode, int(result.Elapsed.Seconds()), result.Error)
		if result.Response != "" {
			response += fmt.Sprintf("\n\nPartial output:\n%s", truncateResponse(result.Response, 8000))
		}
		return fantasy.NewTextErrorResponse(response), nil
	}

	// Build successful response
	response := fmt.Sprintf("Subagent completed successfully in %ds.", int(result.Elapsed.Seconds()))
	if result.Usage != nil {
		response += fmt.Sprintf(" (tokens: %d in / %d out)", result.Usage.InputTokens, result.Usage.OutputTokens)
	}
	response += fmt.Sprintf("\n\nResult:\n%s", truncateResponse(result.Response, 12000))

	return fantasy.NewTextResponse(response), nil
}

// truncateResponse limits the response length to avoid overwhelming context windows.
func truncateResponse(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n\n... [truncated — " + fmt.Sprintf("%d", len(s)-maxLen) + " bytes omitted]"
}
