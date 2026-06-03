// Package extbridge wires the public Kit SDK to the internal extensions
// package. It exists so that cmd/ and internal/acpserver/ don't both
// reimplement the same SDK→extension event/subagent conversions.
package extbridge

import (
	"context"

	"github.com/mark3labs/kit/internal/extensions"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// SDKEventToSubagentEvent converts an SDK [kit.Event] into the
// extension-facing [extensions.SubagentEvent]. Returns a zero-value event
// (Type=="") for events that don't map to anything useful — callers should
// drop those.
func SDKEventToSubagentEvent(e kit.Event) extensions.SubagentEvent {
	switch ev := e.(type) {
	case kit.MessageUpdateEvent:
		return extensions.SubagentEvent{Type: "text", Content: ev.Chunk}
	case kit.ReasoningDeltaEvent:
		return extensions.SubagentEvent{Type: "reasoning", Content: ev.Delta}
	case kit.ToolCallEvent:
		return extensions.SubagentEvent{
			Type: "tool_call", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind, ToolArgs: ev.ToolArgs,
		}
	case kit.ToolExecutionStartEvent:
		return extensions.SubagentEvent{
			Type: "tool_execution_start", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind,
		}
	case kit.ToolExecutionEndEvent:
		return extensions.SubagentEvent{
			Type: "tool_execution_end", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind,
		}
	case kit.ToolResultEvent:
		return extensions.SubagentEvent{
			Type: "tool_result", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind,
			ToolResult: ev.Result, IsError: ev.IsError,
		}
	case kit.TurnStartEvent:
		return extensions.SubagentEvent{Type: "turn_start"}
	case kit.TurnEndEvent:
		return extensions.SubagentEvent{Type: "turn_end"}
	default:
		return extensions.SubagentEvent{}
	}
}

// SpawnSubagent runs a subagent in-process via the Kit SDK and translates
// the result/events back into the extension-facing types. The returned
// handle is always nil — the SDK path runs synchronously and does not
// expose a separate process handle. Callers that need non-blocking
// behaviour should run this in their own goroutine.
//
// This function consolidates the previously-duplicated wiring in
// cmd/root.go (interactive + runtime contexts) and
// internal/acpserver/session.go.
func SpawnSubagent(ctx context.Context, k *kit.Kit, cfg extensions.SubagentConfig) (*extensions.SubagentHandle, *extensions.SubagentResult, error) {
	sdkCfg := kit.SubagentConfig{
		Prompt:       cfg.Prompt,
		Model:        cfg.Model,
		SystemPrompt: cfg.SystemPrompt,
		Timeout:      cfg.Timeout,
		NoSession:    cfg.NoSession,
		Tools:        k.GetToolsForSubagent(),
	}
	if cfg.OnEvent != nil {
		sdkCfg.OnEvent = func(e kit.Event) {
			se := SDKEventToSubagentEvent(e)
			if se.Type != "" {
				cfg.OnEvent(se)
			}
		}
	}

	result, err := k.Subagent(ctx, sdkCfg)
	if result == nil {
		return nil, &extensions.SubagentResult{Error: err}, err
	}

	extResult := &extensions.SubagentResult{
		Response:  result.Response,
		Error:     err,
		SessionID: result.SessionID,
		Elapsed:   result.Elapsed,
	}
	if result.Usage != nil {
		extResult.Usage = &extensions.SubagentUsage{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
		}
	}
	return nil, extResult, err
}
