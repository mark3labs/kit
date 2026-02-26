package extensions

import (
	"context"
	"fmt"

	"charm.land/fantasy"
)

// WrapToolsWithExtensions wraps each tool so that ToolCall and ToolResult
// events are emitted through the extension runner before and after execution.
// This is the Go equivalent of Pi's wrapper.ts pattern.
//
// If the runner has no relevant handlers the original tools are returned
// unchanged (zero overhead).
func WrapToolsWithExtensions(tools []fantasy.AgentTool, runner *Runner) []fantasy.AgentTool {
	if runner == nil {
		return tools
	}
	if !runner.HasHandlers(ToolCall) && !runner.HasHandlers(ToolResult) &&
		!runner.HasHandlers(ToolExecutionStart) && !runner.HasHandlers(ToolExecutionEnd) {
		return tools
	}

	wrapped := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		wrapped[i] = &wrappedTool{inner: tool, runner: runner}
	}
	return wrapped
}

// ExtensionToolsAsFantasy converts ToolDef values registered by extensions
// into fantasy.AgentTool implementations so the LLM can invoke them.
func ExtensionToolsAsFantasy(defs []ToolDef) []fantasy.AgentTool {
	tools := make([]fantasy.AgentTool, 0, len(defs))
	for _, def := range defs {
		tools = append(tools, &extensionTool{def: def})
	}
	return tools
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

	// 1. Emit ToolCall — extensions can block execution.
	if w.runner.HasHandlers(ToolCall) {
		result, _ := w.runner.Emit(ToolCallEvent{
			ToolName:   toolName,
			ToolCallID: call.ID,
			Input:      call.Input,
		})
		if r, ok := result.(ToolCallResult); ok && r.Block {
			reason := r.Reason
			if reason == "" {
				reason = "blocked by extension"
			}
			return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %s", reason)),
				fmt.Errorf("tool blocked by extension: %s", reason)
		}
	}

	// 2. Emit ToolExecutionStart.
	if w.runner.HasHandlers(ToolExecutionStart) {
		_, _ = w.runner.Emit(ToolExecutionStartEvent{ToolName: toolName})
	}

	// 3. Execute the actual tool.
	resp, err := w.inner.Run(ctx, call)

	// 4. Emit ToolExecutionEnd.
	if w.runner.HasHandlers(ToolExecutionEnd) {
		_, _ = w.runner.Emit(ToolExecutionEndEvent{ToolName: toolName})
	}

	// 5. Emit ToolResult — extensions can modify output.
	if w.runner.HasHandlers(ToolResult) {
		result, _ := w.runner.Emit(ToolResultEvent{
			ToolName: toolName,
			Input:    call.Input,
			Content:  resp.Content,
			IsError:  err != nil || resp.IsError,
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
// extensionTool — wraps a ToolDef into a fantasy.AgentTool
// ---------------------------------------------------------------------------

type extensionTool struct {
	def             ToolDef
	providerOptions fantasy.ProviderOptions
}

func (t *extensionTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{
		Name:        t.def.Name,
		Description: t.def.Description,
	}
}

func (t *extensionTool) ProviderOptions() fantasy.ProviderOptions     { return t.providerOptions }
func (t *extensionTool) SetProviderOptions(o fantasy.ProviderOptions) { t.providerOptions = o }

func (t *extensionTool) Run(_ context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	result, err := t.def.Execute(call.Input)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), err
	}
	return fantasy.NewTextResponse(result), nil
}
