package kit

import (
	"strings"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/extensions"
)

// bridgeExtensions registers extension event handlers as SDK hooks and
// subscribes to SDK observation events to forward them to the extension runner.
//
// Interception hooks (Input, BeforeAgentStart) were bridged in Plan 09.
// Observation events (AgentStart/End, MessageStart/Update/End) are bridged here
// so extensions see them regardless of whether the app layer or SDK drives
// the generation loop.
//
// Tool-level events (ToolCall, ToolResult) are handled by the extension tool
// wrapper (internal/extensions/wrapper.go) which composes underneath the SDK
// hook wrapper.
func (m *Kit) bridgeExtensions(runner *extensions.Runner) {
	// --- Interception hooks ---

	// Extension Input → BeforeTurn hook (high priority, runs first).
	// An Input handler with Action="transform" replaces the prompt text.
	if runner.HasHandlers(extensions.Input) {
		m.OnBeforeTurn(HookPriorityHigh, func(h BeforeTurnHook) *BeforeTurnResult {
			result, _ := runner.Emit(extensions.InputEvent{Text: h.Prompt})
			if r, ok := result.(extensions.InputResult); ok {
				if r.Action == "transform" {
					return &BeforeTurnResult{Prompt: &r.Text}
				}
			}
			return nil
		})
	}

	// Extension BeforeAgentStart → BeforeTurn hook (normal priority).
	// Can inject a system prompt prefix and/or context text.
	if runner.HasHandlers(extensions.BeforeAgentStart) {
		m.OnBeforeTurn(HookPriorityNormal, func(h BeforeTurnHook) *BeforeTurnResult {
			result, _ := runner.Emit(extensions.BeforeAgentStartEvent{Prompt: h.Prompt})
			if r, ok := result.(extensions.BeforeAgentStartResult); ok {
				return &BeforeTurnResult{
					SystemPrompt: r.SystemPrompt,
					InjectText:   r.InjectText,
				}
			}
			return nil
		})
	}

	// --- Observation event forwarding ---
	// Subscribe to SDK events and forward to extension runner so extensions
	// see lifecycle events from the SDK's runTurn()/generate() path.

	if runner.HasHandlers(extensions.AgentStart) {
		m.Subscribe(func(e Event) {
			if ev, ok := e.(TurnStartEvent); ok {
				_, _ = runner.Emit(extensions.AgentStartEvent{Prompt: ev.Prompt})
			}
		})
	}

	if runner.HasHandlers(extensions.MessageStart) {
		m.Subscribe(func(e Event) {
			if _, ok := e.(MessageStartEvent); ok {
				_, _ = runner.Emit(extensions.MessageStartEvent{})
			}
		})
	}

	if runner.HasHandlers(extensions.MessageUpdate) {
		m.Subscribe(func(e Event) {
			if ev, ok := e.(MessageUpdateEvent); ok {
				_, _ = runner.Emit(extensions.MessageUpdateEvent{Chunk: ev.Chunk})
			}
		})
	}

	if runner.HasHandlers(extensions.MessageEnd) {
		m.Subscribe(func(e Event) {
			if ev, ok := e.(MessageEndEvent); ok {
				_, _ = runner.Emit(extensions.MessageEndEvent{Content: ev.Content})
			}
		})
	}

	// Tool output streaming events (observation only).
	if runner.HasHandlers(extensions.ToolOutput) {
		m.Subscribe(func(e Event) {
			if ev, ok := e.(ToolOutputEvent); ok {
				_, _ = runner.Emit(extensions.ToolOutputEvent{
					ToolCallID: ev.ToolCallID,
					ToolName:   ev.ToolName,
					Chunk:      ev.Chunk,
					IsStderr:   ev.IsStderr,
				})
			}
		})
	}

	if runner.HasHandlers(extensions.AgentEnd) {
		m.Subscribe(func(e Event) {
			if ev, ok := e.(TurnEndEvent); ok {
				stopReason := ev.StopReason
				response := ev.Response
				if ev.Error != nil {
					stopReason = "error"
					response = ""
				} else if stopReason == "" {
					stopReason = "completed"
				}
				_, _ = runner.Emit(extensions.AgentEndEvent{
					Response:   response,
					StopReason: stopReason,
				})
			}
		})
	}

	// --- Context filtering hook ---
	// Extension ContextPrepare → SDK ContextPrepare hook.
	if runner.HasHandlers(extensions.ContextPrepare) {
		m.OnContextPrepare(HookPriorityNormal, func(h ContextPrepareHook) *ContextPrepareResult {
			// Convert fantasy.Message slice to extension ContextMessage slice.
			extMsgs := make([]extensions.ContextMessage, len(h.Messages))
			for i, msg := range h.Messages {
				// Extract text from content parts.
				var text strings.Builder
				for _, part := range msg.Content {
					if tp, ok := part.(fantasy.TextPart); ok {
						text.WriteString(tp.Text)
					}
				}
				extMsgs[i] = extensions.ContextMessage{
					Index:   i,
					Role:    string(msg.Role),
					Content: text.String(),
				}
			}

			result, _ := runner.Emit(extensions.ContextPrepareEvent{Messages: extMsgs})
			r, ok := result.(extensions.ContextPrepareResult)
			if !ok || r.Messages == nil {
				return nil
			}

			// Rebuild fantasy.Message slice from extension result.
			rebuilt := make([]fantasy.Message, 0, len(r.Messages))
			for _, cm := range r.Messages {
				if cm.Index >= 0 && cm.Index < len(h.Messages) {
					// Reuse original message (preserves tool calls, reasoning, etc.)
					rebuilt = append(rebuilt, h.Messages[cm.Index])
				} else {
					// New message injected by extension.
					role := fantasy.MessageRoleUser
					switch cm.Role {
					case "assistant":
						role = fantasy.MessageRoleAssistant
					case "system":
						role = fantasy.MessageRoleSystem
					case "tool":
						role = fantasy.MessageRoleTool
					}
					rebuilt = append(rebuilt, fantasy.Message{
						Role: role,
						Content: []fantasy.MessagePart{
							fantasy.TextPart{Text: cm.Content},
						},
					})
				}
			}

			return &ContextPrepareResult{Messages: rebuilt}
		})
	}

	// --- Compaction hook ---
	// Extension BeforeCompact → SDK BeforeCompact hook.
	if runner.HasHandlers(extensions.BeforeCompact) {
		m.OnBeforeCompact(HookPriorityNormal, func(h BeforeCompactHook) *BeforeCompactResult {
			result, _ := runner.Emit(extensions.BeforeCompactEvent{
				EstimatedTokens: h.EstimatedTokens,
				ContextLimit:    h.ContextLimit,
				UsagePercent:    h.UsagePercent,
				MessageCount:    h.MessageCount,
				IsAutomatic:     h.IsAutomatic,
			})
			if r, ok := result.(extensions.BeforeCompactResult); ok {
				if r.Cancel {
					return &BeforeCompactResult{
						Cancel: true,
						Reason: r.Reason,
					}
				}
				if r.Summary != "" {
					return &BeforeCompactResult{
						Summary: r.Summary,
					}
				}
			}
			return nil
		})
	}
}
