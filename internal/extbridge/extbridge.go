// Package extbridge wires the public Kit SDK to the internal extensions
// package. It exists so that cmd/ and internal/acpserver/ don't both
// reimplement the same SDK→extension event/subagent conversions.
package extbridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"

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
// the result/events back into the extension-facing types.
//
// When cfg.Blocking is true, the call waits for completion and returns
// (nil, result, err); cfg.OnComplete (if set) is invoked before returning.
// When cfg.Blocking is false (default), the subagent runs in a background
// goroutine and the call returns immediately with (handle, nil, nil). The
// handle supports Wait/Done/Kill; cfg.OnComplete and cfg.OnEvent are
// invoked asynchronously from the subagent's goroutine.
//
// This function consolidates the previously-duplicated wiring in
// cmd/root.go (interactive + runtime contexts) and
// internal/acpserver/session.go.
func SpawnSubagent(ctx context.Context, k *kit.Kit, cfg extensions.SubagentConfig) (*extensions.SubagentHandle, *extensions.SubagentResult, error) {
	run := func(runCtx context.Context) (*extensions.SubagentResult, error) {
		return runSubagent(runCtx, k, cfg)
	}
	return dispatchSubagent(ctx, run, cfg)
}

// dispatchSubagent implements the blocking/background contract of
// SpawnSubagent on top of an injectable run function. Split out from
// SpawnSubagent so the dispatch logic is testable without a real Kit
// instance.
func dispatchSubagent(ctx context.Context, run func(context.Context) (*extensions.SubagentResult, error), cfg extensions.SubagentConfig) (*extensions.SubagentHandle, *extensions.SubagentResult, error) {
	if cfg.Blocking {
		result, err := run(ctx)
		if cfg.OnComplete != nil {
			cfg.OnComplete(completionResult(result, err))
		}
		return nil, result, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	handle := extensions.NewSubagentHandle(newSubagentID(), cancel)
	go func() {
		defer cancel()
		final := completionResult(run(runCtx))
		handle.Complete(final)
		if cfg.OnComplete != nil {
			cfg.OnComplete(final)
		}
	}()
	return handle, nil, nil
}

// completionResult normalizes a run outcome for delivery to
// SubagentHandle.Complete and cfg.OnComplete: a nil result (defensive —
// runSubagent always returns non-nil) becomes a failure result carrying
// err, so the error is never silently dropped.
func completionResult(result *extensions.SubagentResult, err error) extensions.SubagentResult {
	if result == nil {
		return extensions.SubagentResult{Error: err, ExitCode: 1}
	}
	return *result
}

// runSubagent executes one subagent run synchronously via the Kit SDK and
// converts the outcome to the extension-facing result type. The returned
// result is always non-nil; on failure result.Error and result.ExitCode
// are set in addition to the returned error.
func runSubagent(ctx context.Context, k *kit.Kit, cfg extensions.SubagentConfig) (*extensions.SubagentResult, error) {
	sdkCfg := kit.SubagentConfig{
		Prompt:       cfg.Prompt,
		Model:        cfg.Model,
		SystemPrompt: cfg.SystemPrompt,
		Timeout:      cfg.Timeout,
		NoSession:    cfg.NoSession,
		Tools:        k.GetToolsForSubagent(),
	}
	if cfg.OnEvent != nil || cfg.OnOutput != nil {
		sdkCfg.OnEvent = func(e kit.Event) {
			se := SDKEventToSubagentEvent(e)
			if se.Type == "" {
				return
			}
			if cfg.OnEvent != nil {
				cfg.OnEvent(se)
			}
			if cfg.OnOutput != nil && se.Type == "text" && se.Content != "" {
				cfg.OnOutput(se.Content)
			}
		}
	}

	result, err := k.Subagent(ctx, sdkCfg)
	if result == nil {
		return &extensions.SubagentResult{Error: err, ExitCode: 1}, err
	}

	extResult := &extensions.SubagentResult{
		Response:  result.Response,
		Error:     err,
		SessionID: result.SessionID,
		Elapsed:   result.Elapsed,
	}
	if err != nil {
		extResult.ExitCode = 1
	}
	if result.Usage != nil {
		extResult.Usage = &extensions.SubagentUsage{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
		}
	}
	return extResult, err
}

// newSubagentID returns a short random identifier for a background
// subagent handle.
func newSubagentID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "subagent-unknown"
	}
	return "subagent-" + hex.EncodeToString(b[:])
}
