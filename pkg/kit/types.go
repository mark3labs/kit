package kit

import (
	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/session"
)

// Message is an alias for message.Message providing SDK users with access
// to message structures for conversation history and tool interactions.
type Message = message.Message

// ToolCall is an alias for message.ToolCall representing a tool invocation
// with its name, arguments, and result within a conversation.
type ToolCall = message.ToolCall

// ToolResult is an alias for message.ToolResult representing the result
// of executing a tool.
type ToolResult = message.ToolResult

// ConvertToFantasyMessages converts an SDK message to the underlying fantasy
// messages used by the agent for LLM interactions.
func ConvertToFantasyMessages(msg *Message) []fantasy.Message {
	return msg.ToFantasyMessages()
}

// ConvertFromFantasyMessage converts a fantasy message from the agent to an SDK
// message format for use in the SDK API.
func ConvertFromFantasyMessage(msg fantasy.Message) Message {
	return session.ConvertFromFantasyMessage(msg)
}
