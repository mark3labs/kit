package kit

import (
	"strings"
	"sync"

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

	// --- Subagent lifecycle events ---
	// When an extension registers OnSubagentStart/Chunk/End handlers, bridge
	// the SDK's per-subagent event stream (SubscribeSubagent) into the
	// extension runner.
	//
	// Flow:
	//   ToolExecutionStartEvent(spawn_subagent) → emit SubagentStartEvent
	//                                           → SubscribeSubagent → emit SubagentChunkEvents
	//   ToolResultEvent(spawn_subagent)         → emit SubagentEndEvent
	//
	// We use ToolExecutionStart (not ToolCall) for SubagentStart because that
	// is when the subagent actually begins running. We use ToolResult for
	// SubagentEnd because that carries the final response text.
	wantsSubagent := runner.HasHandlers(extensions.SubagentStart) ||
		runner.HasHandlers(extensions.SubagentChunk) ||
		runner.HasHandlers(extensions.SubagentEnd)

	if wantsSubagent {
		// taskByCallID tracks the task description extracted from ToolCall input,
		// keyed by toolCallID. Populated on ToolCall, consumed on ToolResult.
		taskByCallID := make(map[string]string)
		var taskMu = &taskMutex{}

		// Intercept ToolCall to capture the task and subscribe to child events.
		m.Subscribe(func(e Event) {
			ev, ok := e.(ToolCallEvent)
			if !ok || ev.ToolName != "spawn_subagent" {
				return
			}

			// Extract task from parsed args.
			task := ""
			if ev.ParsedArgs != nil {
				if t, ok := ev.ParsedArgs["task"].(string); ok {
					task = t
				}
			}
			taskMu.set(taskByCallID, ev.ToolCallID, task)

			// Subscribe to child events so we can forward them as SubagentChunkEvents.
			if runner.HasHandlers(extensions.SubagentChunk) {
				m.SubscribeSubagent(ev.ToolCallID, func(childEvent Event) {
					chunk := extensions.SubagentChunkEvent{
						ToolCallID: ev.ToolCallID,
						Task:       task,
					}
					switch ce := childEvent.(type) {
					case MessageUpdateEvent:
						chunk.ChunkType = "text"
						chunk.Content = ce.Chunk
					case TurnStartEvent:
						chunk.ChunkType = "turn_start"
					case TurnEndEvent:
						chunk.ChunkType = "turn_end"
					case ToolCallEvent:
						chunk.ChunkType = "tool_call"
						chunk.ToolName = ce.ToolName
						chunk.ToolArgs = ce.ToolArgs
					case ToolExecutionStartEvent:
						chunk.ChunkType = "tool_execution_start"
						chunk.ToolName = ce.ToolName
					case ToolExecutionEndEvent:
						chunk.ChunkType = "tool_execution_end"
						chunk.ToolName = ce.ToolName
					case ToolResultEvent:
						chunk.ChunkType = "tool_result"
						chunk.ToolName = ce.ToolName
						chunk.ToolResult = ce.Result
						chunk.IsError = ce.IsError
					default:
						return // skip unknown event types
					}
					_, _ = runner.Emit(chunk)
				})
			}
		})

		// Emit SubagentStartEvent when execution begins.
		if runner.HasHandlers(extensions.SubagentStart) {
			m.Subscribe(func(e Event) {
				ev, ok := e.(ToolExecutionStartEvent)
				if !ok || ev.ToolName != "spawn_subagent" {
					return
				}
				task := taskMu.get(taskByCallID, ev.ToolCallID)
				_, _ = runner.Emit(extensions.SubagentStartEvent{
					ToolCallID: ev.ToolCallID,
					Task:       task,
				})
			})
		}

		// Emit SubagentEndEvent when the tool result arrives.
		if runner.HasHandlers(extensions.SubagentEnd) {
			m.Subscribe(func(e Event) {
				ev, ok := e.(ToolResultEvent)
				if !ok || ev.ToolName != "spawn_subagent" {
					return
				}
				task := taskMu.get(taskByCallID, ev.ToolCallID)
				taskMu.del(taskByCallID, ev.ToolCallID)
				errMsg := ""
				if ev.IsError {
					errMsg = ev.Result
				}
				response := ""
				if !ev.IsError {
					response = ev.Result
				}
				_, _ = runner.Emit(extensions.SubagentEndEvent{
					ToolCallID: ev.ToolCallID,
					Task:       task,
					Response:   response,
					ErrorMsg:   errMsg,
				})
			})
		}
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

// taskMutex is a simple mutex-protected map helper used by bridgeExtensions.
// It lives in this file to avoid polluting the kit package with unexported types.
type taskMutex struct {
	mu sync.Mutex
}

func (t *taskMutex) set(m map[string]string, key, val string) {
	t.mu.Lock()
	m[key] = val
	t.mu.Unlock()
}

func (t *taskMutex) get(m map[string]string, key string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return m[key]
}

func (t *taskMutex) del(m map[string]string, key string) {
	t.mu.Lock()
	delete(m, key)
	t.mu.Unlock()
}
