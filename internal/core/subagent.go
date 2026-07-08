package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
)

// maxSubagentTimeout caps the timeout an LLM can request via
// timeout_seconds. The 5-minute default (applied downstream when no timeout
// is requested and no named agent preset supplies one) lives in
// pkg/kit's Kit.Subagent.
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

// SubagentSpawnRequest carries the parameters of an in-process subagent
// spawn from the subagent core tool to the parent Kit instance.
type SubagentSpawnRequest struct {
	// ToolCallID is the LLM-assigned ID of the subagent tool call,
	// enabling the parent to correlate subagent events.
	ToolCallID string
	// Prompt is the task for the subagent (required).
	Prompt string
	// Agent optionally names a discovered agent definition whose presets
	// (model, system prompt, tools, timeout) apply as defaults.
	Agent string
	// Model optionally overrides the model.
	Model string
	// SystemPrompt optionally overrides the system prompt.
	SystemPrompt string
	// Timeout bounds execution. Zero means "unset": the spawner applies
	// the named agent's timeout (if any) or the default.
	Timeout time.Duration
}

// SubagentSpawnFunc is a callback that spawns an in-process subagent. The
// parent Kit instance injects this into the context so the core tool can
// call back without importing pkg/kit (which would create a cycle).
type SubagentSpawnFunc func(ctx context.Context, req SubagentSpawnRequest) (*SubagentSpawnResult, error)

type subagentCtxKey struct{}

// WithSubagentSpawner stores a spawn function in the context so that the
// subagent core tool can create in-process subagents.
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
// subagent tool
// ---------------------------------------------------------------------------

type subagentArgs struct {
	Task           string `json:"task"`
	Agent          string `json:"agent,omitempty"`
	Model          string `json:"model,omitempty"`
	SystemPrompt   string `json:"system_prompt,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// NamedAgentSpec summarises a named agent definition for advertisement in
// the subagent tool description. It is intentionally minimal so the core
// package does not depend on the agent discovery machinery.
type NamedAgentSpec struct {
	// Name is the value the LLM passes in the "agent" parameter.
	Name string
	// Description summarises what the agent does.
	Description string
	// Tools lists the tool names the agent may use. Empty means the full
	// default subagent tool set.
	Tools []string
}

const subagentBaseDescription = `Spawn a subagent to perform a task autonomously.

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
- "Analyze the performance bottlenecks in the database queries"`

// subagentDescription composes the tool description, appending the list of
// available named agents (and their tool access) when any are configured.
func subagentDescription(agents []NamedAgentSpec) string {
	if len(agents) == 0 {
		return subagentBaseDescription
	}
	var b strings.Builder
	b.WriteString(subagentBaseDescription)
	b.WriteString("\n\nAvailable named agents (pass the \"agent\" parameter to use one) and the tools they have access to:\n")
	for _, a := range agents {
		tools := "all tools"
		if len(a.Tools) > 0 {
			tools = strings.Join(a.Tools, ", ")
		}
		fmt.Fprintf(&b, "- %s: %s (tools: %s)\n", a.Name, a.Description, tools)
	}
	return strings.TrimRight(b.String(), "\n")
}

// NewSubagentTool creates the subagent core tool. When named agents are
// configured via WithNamedAgents, they are advertised in the tool
// description so the LLM can delegate to the right specialist.
func NewSubagentTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "subagent",
			Description: subagentDescription(cfg.NamedAgents),
			Parameters: map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The complete task description for the subagent to perform",
				},
				"agent": map[string]any{
					"type":        "string",
					"description": "Optional named agent to run the task with. Named agents provide preset system prompts, models, and tool restrictions. Explicit model/system_prompt/timeout_seconds arguments override the agent's presets.",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Optional model override. Empty string uses the current model.",
				},
				"system_prompt": map[string]any{
					"type":        "string",
					"description": "Optional system prompt for domain-specific guidance",
				},
				"timeout_seconds": map[string]any{
					"type":        "number",
					"description": "Maximum execution time in seconds (default: 300, max: 1800, minimum recommended: 240)",
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

	// Determine timeout. Zero means "unset" so downstream resolution can
	// apply a named agent's preset timeout (or the 5-minute default).
	var timeout time.Duration
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

	// Build a clean context for the subagent that inherits values (e.g. the
	// spawner callback) but is completely detached from the parent's
	// deadline AND cancellation. The subagent gets its own independent
	// timeout (applied downstream in Kit.Subagent).
	//
	// Why full detachment instead of propagating parent cancellation?
	// The parent context may already be done (deadline exceeded or
	// cancelled) by the time this tool handler executes — for example when
	// the generation loop context carries a deadline, when the user
	// double-ESC cancels mid-turn, or when parallel tool execution
	// encounters a race between stream completion and tool dispatch. Using
	// context.WithoutCancel (Go 1.21+) ensures the subagent always starts
	// cleanly with a fresh timeout, following the pattern used by crush for
	// shutdown-resilient child work. The subagent's own timeout
	// (defaultSubagentTimeout / user-specified) provides the safety net.
	spawnCtx := context.WithoutCancel(valuesContext{parent: ctx})

	// Spawn in-process subagent.
	result, err := spawner(spawnCtx, SubagentSpawnRequest{
		ToolCallID:   call.ID,
		Prompt:       args.Task,
		Agent:        args.Agent,
		Model:        args.Model,
		SystemPrompt: args.SystemPrompt,
		Timeout:      timeout,
	})
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

// ---------------------------------------------------------------------------
// Context helpers
// ---------------------------------------------------------------------------

// valuesContext preserves a parent context's values (e.g. the subagent
// spawner callback) while stripping its deadline and cancellation. Combined
// with context.WithoutCancel() this gives the subagent a completely clean
// context that only inherits value-based dependencies.
type valuesContext struct {
	parent context.Context
}

func (v valuesContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (v valuesContext) Done() <-chan struct{}       { return nil }
func (v valuesContext) Err() error                  { return nil }
func (v valuesContext) Value(key any) any           { return v.parent.Value(key) }

// truncateResponse limits the response length to avoid overwhelming context windows.
func truncateResponse(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n\n... [truncated — " + fmt.Sprintf("%d", len(s)-maxLen) + " bytes omitted]"
}
