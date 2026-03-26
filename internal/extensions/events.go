// Package extensions implements an in-process extension system for KIT.
// Extensions are plain Go files loaded at runtime via Yaegi (a Go interpreter).
// They register event handlers using an API object, enabling tool interception,
// input transformation, and lifecycle observation — all without recompilation.
package extensions

import "slices"

// EventType identifies a point in KIT's lifecycle where extensions can hook in.
type EventType string

const (
	// ToolCall fires before a tool executes. Handlers can block execution.
	ToolCall EventType = "tool_call"

	// ToolExecutionStart fires when a tool begins executing.
	ToolExecutionStart EventType = "tool_execution_start"

	// ToolExecutionEnd fires when a tool finishes executing.
	ToolExecutionEnd EventType = "tool_execution_end"

	// ToolOutput fires when a tool produces streaming output chunks.
	ToolOutput EventType = "tool_output"

	// ToolResult fires after a tool executes. Handlers can modify the result.
	ToolResult EventType = "tool_result"

	// Input fires when user input is received. Handlers can transform or handle it.
	Input EventType = "input"

	// BeforeAgentStart fires before the agent loop begins for a prompt.
	BeforeAgentStart EventType = "before_agent_start"

	// AgentStart fires when the agent loop begins processing.
	AgentStart EventType = "agent_start"

	// AgentEnd fires when the agent finishes responding.
	AgentEnd EventType = "agent_end"

	// MessageStart fires when a new assistant message begins.
	MessageStart EventType = "message_start"

	// MessageUpdate fires for each streaming text chunk.
	MessageUpdate EventType = "message_update"

	// MessageEnd fires when the assistant message is complete.
	MessageEnd EventType = "message_end"

	// SessionStart fires when a session is loaded or created.
	SessionStart EventType = "session_start"

	// SessionShutdown fires when the application is closing.
	SessionShutdown EventType = "session_shutdown"

	// ModelChange fires after the active model is changed via ctx.SetModel().
	ModelChange EventType = "model_change"

	// ContextPrepare fires after context is built from the session tree and
	// before the messages are sent to the LLM. Handlers can filter, reorder,
	// or inject messages into the context window.
	ContextPrepare EventType = "context_prepare"

	// BeforeFork fires before the session tree is branched to a different
	// entry point. Handlers can cancel the fork by returning Cancel=true.
	BeforeFork EventType = "before_fork"

	// BeforeSessionSwitch fires before the session is switched to a new
	// branch (e.g. /new command). Handlers can cancel by returning Cancel=true.
	BeforeSessionSwitch EventType = "before_session_switch"

	// BeforeCompact fires before context compaction runs. Handlers can
	// cancel compaction by returning Cancel=true.
	BeforeCompact EventType = "before_compact"

	// SubagentStart fires when a spawn_subagent tool call begins executing.
	// Carries the tool call ID and the task description.
	SubagentStart EventType = "subagent_start"

	// SubagentChunk fires for each real-time event emitted by a running
	// subagent: text chunks, tool calls, tool results, etc.
	SubagentChunk EventType = "subagent_chunk"

	// SubagentEnd fires when a spawn_subagent tool call completes (success
	// or error). Carries the final response and any error message.
	SubagentEnd EventType = "subagent_end"
)

// AllEventTypes returns every supported event type.
func AllEventTypes() []EventType {
	return []EventType{
		ToolCall, ToolExecutionStart, ToolExecutionEnd, ToolResult,
		Input, BeforeAgentStart, AgentStart, AgentEnd,
		MessageStart, MessageUpdate, MessageEnd,
		SessionStart, SessionShutdown,
		ModelChange, ContextPrepare,
		BeforeFork, BeforeSessionSwitch, BeforeCompact,
		SubagentStart, SubagentChunk, SubagentEnd,
	}
}

// IsValid returns true if the event type is a recognised lifecycle event.
func (e EventType) IsValid() bool {
	return slices.Contains(AllEventTypes(), e)
}
