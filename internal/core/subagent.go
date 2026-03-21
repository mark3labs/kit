package core

import (
	"context"
	"fmt"
	"time"

	"charm.land/fantasy"
)

const defaultSubagentTimeout = 5 * time.Minute
const maxSubagentTimeout = 30 * time.Minute

// ---------------------------------------------------------------------------
// Context-based subagent spawner
// ---------------------------------------------------------------------------

// SubagentSpawnResult carries the outcome of an in-process subagent spawn.
type SubagentSpawnResult struct {
	Response     string
	Error        error
	SessionID    string
	InputTokens  int64
	OutputTokens int64
	Elapsed      time.Duration
}

// SubagentSpawnFunc is a callback that spawns an in-process subagent. The
// parent Kit instance injects this into the context so the core tool can
// call back without importing pkg/kit (which would create a cycle).
// The toolCallID parameter is the LLM-assigned ID of the spawn_subagent
// tool call, enabling the parent to correlate subagent events.
type SubagentSpawnFunc func(ctx context.Context, toolCallID, prompt, model, systemPrompt string, timeout time.Duration) (*SubagentSpawnResult, error)

type subagentCtxKey struct{}

// WithSubagentSpawner stores a spawn function in the context so that the
// spawn_subagent core tool can create in-process subagents.
func WithSubagentSpawner(ctx context.Context, fn SubagentSpawnFunc) context.Context {
	return context.WithValue(ctx, subagentCtxKey{}, fn)
}

// getSubagentSpawner retrieves the spawn function from the context.
func getSubagentSpawner(ctx context.Context) SubagentSpawnFunc {
	if fn, ok := ctx.Value(subagentCtxKey{}).(SubagentSpawnFunc); ok {
		return fn
	}
	return nil
}

// ---------------------------------------------------------------------------
// spawn_subagent tool
// ---------------------------------------------------------------------------

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
			Description: `Spawn a subagent to perform a task autonomously.

The subagent runs as a separate in-process Kit instance with full tool access
(except spawning further subagents). Use this to:
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

	// Determine timeout.
	timeout := defaultSubagentTimeout
	if args.TimeoutSeconds > 0 {
		timeout = min(time.Duration(args.TimeoutSeconds)*time.Second, maxSubagentTimeout)
	}

	// Retrieve in-process spawner from context.
	spawner := getSubagentSpawner(ctx)
	if spawner == nil {
		return fantasy.NewTextErrorResponse(
			"Error: subagent spawner not available. " +
				"Ensure Kit is initialized with subagent support.",
		), fmt.Errorf("no subagent spawner in context")
	}

	// Spawn in-process subagent.
	result, err := spawner(ctx, call.ID, args.Task, args.Model, args.SystemPrompt, timeout)
	if err != nil || result.Error != nil {
		spawnErr := err
		if spawnErr == nil {
			spawnErr = result.Error
		}
		response := fmt.Sprintf("Subagent failed after %ds.\n\nError: %v",
			int(result.Elapsed.Seconds()), spawnErr)
		if result.Response != "" {
			response += fmt.Sprintf("\n\nPartial output:\n%s", truncateResponse(result.Response, 8000))
		}
		return fantasy.NewTextErrorResponse(response), nil
	}

	// Build successful response.
	response := fmt.Sprintf("Subagent completed successfully in %ds.", int(result.Elapsed.Seconds()))
	if result.InputTokens > 0 || result.OutputTokens > 0 {
		response += fmt.Sprintf(" (tokens: %d in / %d out)", result.InputTokens, result.OutputTokens)
	}
	response += fmt.Sprintf("\n\nResult:\n%s", truncateResponse(result.Response, 12000))

	resp := fantasy.NewTextResponse(response)

	// Attach subagent session ID as metadata when available.
	if result.SessionID != "" {
		resp = fantasy.WithResponseMetadata(resp, map[string]any{
			"subagent_session_id": result.SessionID,
		})
	}

	return resp, nil
}

// truncateResponse limits the response length to avoid overwhelming context windows.
func truncateResponse(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n\n... [truncated — " + fmt.Sprintf("%d", len(s)-maxLen) + " bytes omitted]"
}
