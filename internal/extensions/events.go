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

	// ToolCallInputStart fires when the LLM begins generating tool call
	// arguments. The tool name is known but the full argument JSON is still
	// being streamed.
	ToolCallInputStart EventType = "tool_call_input_start"

	// ToolCallInputDelta fires for each streamed fragment of tool call
	// arguments as they arrive from the LLM.
	ToolCallInputDelta EventType = "tool_call_input_delta"

	// ToolCallInputEnd fires when tool argument streaming is complete,
	// before the tool call is parsed and execution begins.
	ToolCallInputEnd EventType = "tool_call_input_end"

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

	// SubagentStart fires when a subagent tool call begins executing.
	// Carries the tool call ID and the task description.
	SubagentStart EventType = "subagent_start"

	// SubagentChunk fires for each real-time event emitted by a running
	// subagent: text chunks, tool calls, tool results, etc.
	SubagentChunk EventType = "subagent_chunk"

	// SubagentEnd fires when a subagent tool call completes (success
	// or error). Carries the final response and any error message.
	SubagentEnd EventType = "subagent_end"

	// StepStart fires when a new LLM call begins within a multi-step
	// agent turn.
	StepStart EventType = "step_start"

	// StepFinish fires when a step completes, providing step number,
	// finish reason, and token usage.
	StepFinish EventType = "step_finish"

	// ReasoningStart fires when the LLM begins reasoning/thinking.
	ReasoningStart EventType = "reasoning_start"

	// Warnings fires when the LLM provider returns warnings.
	Warnings EventType = "warnings"

	// Source fires when the LLM references a source (e.g. web search).
	Source EventType = "source"

	// Error fires when an agent-level error occurs during streaming.
	Error EventType = "error"

	// Retry fires when the LLM provider request is retried after a
	// transient error.
	Retry EventType = "retry"

	// PrepareStep fires between steps within a multi-step agent turn,
	// after steering messages are injected and before messages are sent
	// to the LLM. Handlers can replace the context window for this step.
	PrepareStep EventType = "prepare_step"
)

// AllEventTypes returns every supported event type.
func AllEventTypes() []EventType {
	return []EventType{
		ToolCall, ToolCallInputStart, ToolCallInputDelta, ToolCallInputEnd,
		ToolExecutionStart, ToolExecutionEnd, ToolResult,
		Input, BeforeAgentStart, AgentStart, AgentEnd,
		MessageStart, MessageUpdate, MessageEnd,
		SessionStart, SessionShutdown,
		ModelChange, ContextPrepare,
		BeforeFork, BeforeSessionSwitch, BeforeCompact,
		SubagentStart, SubagentChunk, SubagentEnd,
		StepStart, StepFinish, ReasoningStart, Warnings, Source, Error, Retry,
		PrepareStep,
	}
}

// IsValid returns true if the event type is a recognised lifecycle event.
func (e EventType) IsValid() bool {
	return slices.Contains(AllEventTypes(), e)
}
