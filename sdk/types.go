package sdk

import (
	"charm.land/fantasy"
	"github.com/mark3labs/mcphost/internal/session"
)

// Message is an alias for session.Message providing SDK users with access
// to message structures for conversation history and tool interactions.
type Message = session.Message

// ToolCall is an alias for session.ToolCall representing a tool invocation
// with its name, arguments, and result within a conversation.
type ToolCall = session.ToolCall

// ConvertToFantasyMessage converts an SDK message to the underlying fantasy message
// format used by the agent for LLM interactions.
func ConvertToFantasyMessage(msg *Message) fantasy.Message {
	return msg.ConvertToFantasyMessage()
}

// ConvertFromFantasyMessage converts a fantasy message from the agent to an SDK
// message format for use in the SDK API.
func ConvertFromFantasyMessage(msg fantasy.Message) Message {
	return session.ConvertFromFantasyMessage(msg)
}
