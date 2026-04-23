package extensions

import (
	"context"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
)

// WrapToolsWithExtensions wraps each tool so that ToolCall and ToolResult
// events are emitted through the extension runner before and after execution.

// If the runner has no relevant handlers the original tools are returned
// unchanged (zero overhead).
func WrapToolsWithExtensions(tools []fantasy.AgentTool, runner *Runner) []fantasy.AgentTool {
	if runner == nil {
		return tools
	}
	// Always wrap tools through the runner so that SetActiveTools
	// (disabled-tool checking) and event handlers both work. The
	// overhead for disabled-tool checking is a single map lookup
	// per tool call, which is negligible.
	wrapped := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		wrapped[i] = &wrappedTool{inner: tool, runner: runner}
	}
	return wrapped
}

// ExtensionToolsAsLLMTools converts ToolDef values registered by extensions
// into LLM agent tool implementations so the LLM can invoke them.
// The runner is optional; if provided, ToolContext.OnProgress routes
// progress messages through the runner's Print function.
func ExtensionToolsAsLLMTools(defs []ToolDef, runner *Runner) []fantasy.AgentTool {
	tools := make([]fantasy.AgentTool, 0, len(defs))
	for _, def := range defs {
		tools = append(tools, &extensionTool{def: def, runner: runner})
	}
	return tools
}

// coreToolKinds maps built-in tool names to their kind classification.
var coreToolKinds = map[string]string{
	"bash":     "execute",
	"edit":     "edit",
	"write":    "edit",
	"read":     "read",
	"ls":       "read",
	"grep":     "search",
	"find":     "search",
	"subagent": "agent",
}

// toolKindFor returns the ToolKind for a given tool name, defaulting to
// "execute" for unknown tools (including MCP tools).
func toolKindFor(toolName string) string {
	if kind, ok := coreToolKinds[toolName]; ok {
		return kind
	}
	return "execute"
}

// parseToolArgsJSON attempts to parse JSON-encoded tool args into a map.
// Returns nil on failure (non-fatal convenience parsing).
func parseToolArgsJSON(input string) map[string]any {
	var parsed map[string]any
	if json.Unmarshal([]byte(input), &parsed) == nil {
		return parsed
	}
	return nil
}

// ---------------------------------------------------------------------------
// wrappedTool — intercepts tool calls through the extension runner
// ---------------------------------------------------------------------------

type wrappedTool struct {
	inner  fantasy.AgentTool
	runner *Runner
}

func (w *wrappedTool) Info() fantasy.ToolInfo                       { return w.inner.Info() }
func (w *wrappedTool) ProviderOptions() fantasy.ProviderOptions     { return w.inner.ProviderOptions() }
func (w *wrappedTool) SetProviderOptions(o fantasy.ProviderOptions) { w.inner.SetProviderOptions(o) }

func (w *wrappedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	toolName := w.inner.Info().Name

	// 0. Check if tool is disabled via SetActiveTools.
	if w.runner.IsToolDisabled(toolName) {
		return fantasy.NewTextErrorResponse(
			fmt.Sprintf("Error: tool %q is currently disabled", toolName)), nil
	}

	kind := toolKindFor(toolName)

	// 1. Emit ToolCall — extensions can block execution.
	if w.runner.HasHandlers(ToolCall) {
		result, _ := w.runner.Emit(ToolCallEvent{
			ToolName:   toolName,
			ToolCallID: call.ID,
			ToolKind:   kind,
			Input:      call.Input,
			ParsedArgs: parseToolArgsJSON(call.Input),
			Source:     "llm",
		})
		if r, ok := result.(ToolCallResult); ok && r.Block {
			reason := r.Reason
			if reason == "" {
				reason = "blocked by extension"
			}
			return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %s", reason)), nil
		}
	}

	// 2. Emit ToolExecutionStart.
	if w.runner.HasHandlers(ToolExecutionStart) {
		_, _ = w.runner.Emit(ToolExecutionStartEvent{ToolCallID: call.ID, ToolName: toolName, ToolKind: kind})
	}

	// 3. Execute the actual tool.
	resp, err := w.inner.Run(ctx, call)

	// 4. Emit ToolExecutionEnd.
	if w.runner.HasHandlers(ToolExecutionEnd) {
		_, _ = w.runner.Emit(ToolExecutionEndEvent{ToolCallID: call.ID, ToolName: toolName, ToolKind: kind})
	}

	// 5. Emit ToolResult — extensions can modify output.
	if w.runner.HasHandlers(ToolResult) {
		result, _ := w.runner.Emit(ToolResultEvent{
			ToolCallID: call.ID,
			ToolName:   toolName,
			ToolKind:   kind,
			Input:      call.Input,
			Content:    resp.Content,
			IsError:    err != nil || resp.IsError,
			Metadata:   resp.Metadata,
		})
		if r, ok := result.(ToolResultResult); ok {
			if r.Content != nil {
				resp.Content = *r.Content
			}
			if r.IsError != nil {
				resp.IsError = *r.IsError
			}
		}
	}

	return resp, err
}

// ---------------------------------------------------------------------------
// extensionTool — wraps a ToolDef into an LLM agent tool
// ---------------------------------------------------------------------------

type extensionTool struct {
	def             ToolDef
	runner          *Runner // optional; enables ToolContext.OnProgress
	providerOptions fantasy.ProviderOptions
}

func (t *extensionTool) Info() fantasy.ToolInfo {
	info := fantasy.ToolInfo{
		Name:        t.def.Name,
		Description: t.def.Description,
	}

	// Parse the extension's JSON Schema and extract the properties map.
	// Fantasy expects Parameters to contain property definitions directly
	// (e.g. {"command": {"type":"string"}}) and wraps them into a full
	// JSON Schema object internally. If the extension provides a full
	// schema with "type":"object" and "properties", we extract just the
	// properties. Required fields are also extracted if present.
	if t.def.Parameters != "" {
		var schema map[string]any
		if err := json.Unmarshal([]byte(t.def.Parameters), &schema); err == nil {
			if props, ok := schema["properties"].(map[string]any); ok {
				info.Parameters = props
			} else {
				// Schema doesn't have "properties" — use as-is (may be
				// a flat property map already matching the expected format).
				info.Parameters = schema
			}
			// Extract required fields if present.
			if req, ok := schema["required"].([]any); ok {
				for _, r := range req {
					if s, ok := r.(string); ok {
						info.Required = append(info.Required, s)
					}
				}
			}
		}
	}

	// Ensure Parameters and Required are never nil — the OpenAI Responses API
	// rejects tools where these fields serialize to JSON null instead of
	// empty object/array.
	if info.Parameters == nil {
		info.Parameters = map[string]any{}
	}
	if info.Required == nil {
		info.Required = []string{}
	}

	return info
}

func (t *extensionTool) ProviderOptions() fantasy.ProviderOptions     { return t.providerOptions }
func (t *extensionTool) SetProviderOptions(o fantasy.ProviderOptions) { t.providerOptions = o }

func (t *extensionTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var result string
	var err error

	if t.def.ExecuteWithContext != nil {
		tc := ToolContext{
			IsCancelled: func() bool {
				return ctx.Err() != nil
			},
			OnProgress: func(text string) {
				if t.runner != nil {
					t.runner.mu.RLock()
					printFn := t.runner.ctx.Print
					t.runner.mu.RUnlock()
					if printFn != nil {
						printFn(text)
					}
				}
			},
		}
		result, err = t.def.ExecuteWithContext(call.Input, tc)
	} else {
		result, err = t.def.Execute(call.Input)
	}

	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	return fantasy.NewTextResponse(result), nil
}
