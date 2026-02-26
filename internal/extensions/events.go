// Package extensions implements a Pi-style in-process extension system for KIT.
// Extensions are plain Go files loaded at runtime via Yaegi (a Go interpreter).
// They register event handlers using an API object, enabling tool interception,
// input transformation, and lifecycle observation — all without recompilation.
package extensions

// EventType identifies a point in KIT's lifecycle where extensions can hook in.
type EventType string

const (
	// ToolCall fires before a tool executes. Handlers can block execution.
	ToolCall EventType = "tool_call"

	// ToolExecutionStart fires when a tool begins executing.
	ToolExecutionStart EventType = "tool_execution_start"

	// ToolExecutionEnd fires when a tool finishes executing.
	ToolExecutionEnd EventType = "tool_execution_end"

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
)

// AllEventTypes returns every supported event type.
func AllEventTypes() []EventType {
	return []EventType{
		ToolCall, ToolExecutionStart, ToolExecutionEnd, ToolResult,
		Input, BeforeAgentStart, AgentStart, AgentEnd,
		MessageStart, MessageUpdate, MessageEnd,
		SessionStart, SessionShutdown,
	}
}

// IsValid returns true if the event type is a recognised lifecycle event.
func (e EventType) IsValid() bool {
	for _, valid := range AllEventTypes() {
		if e == valid {
			return true
		}
	}
	return false
}
