package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"charm.land/fantasy"
)

// Session represents a complete conversation session with metadata.
// It stores all messages exchanged during a conversation along with
// contextual information about the session such as the provider, model,
// and timestamps. Sessions can be saved to and loaded from JSON files
// for persistence across program runs.
type Session struct {
	// Version indicates the session format version for compatibility
	Version string `json:"version"`
	// CreatedAt is the timestamp when the session was first created
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the session was last modified
	UpdatedAt time.Time `json:"updated_at"`
	// Metadata contains contextual information about the session
	Metadata Metadata `json:"metadata"`
	// Messages is the ordered list of all messages in this session
	Messages []Message `json:"messages"`
}

// Metadata contains session metadata that provides context about the
// environment and configuration used during the conversation.
type Metadata struct {
	// MCPHostVersion is the version of MCPHost used for this session
	MCPHostVersion string `json:"mcphost_version"`
	// Provider is the LLM provider used (e.g., "anthropic", "openai", "gemini")
	Provider string `json:"provider"`
	// Model is the specific model identifier used for the conversation
	Model string `json:"model"`
}

// Message represents a single message in the conversation session.
// Messages can be from different roles (user, assistant, tool) and may
// include tool calls for assistant messages or tool results for tool messages.
type Message struct {
	// ID is a unique identifier for this message, auto-generated if not provided
	ID string `json:"id"`
	// Role indicates who sent the message ("user", "assistant", "tool", or "system")
	Role string `json:"role"`
	// Content is the text content of the message
	Content string `json:"content"`
	// Timestamp is when the message was created
	Timestamp time.Time `json:"timestamp"`
	// ToolCalls contains any tool invocations made by the assistant in this message
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID links a tool result message to its corresponding tool call
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool invocation within an assistant message.
type ToolCall struct {
	// ID is a unique identifier for this tool call, used to link results
	ID string `json:"id"`
	// Name is the name of the tool being invoked
	Name string `json:"name"`
	// Arguments contains the parameters passed to the tool, typically as JSON
	Arguments any `json:"arguments"`
}

// NewSession creates a new session with default values.
func NewSession() *Session {
	return &Session{
		Version:   "1.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []Message{},
		Metadata:  Metadata{},
	}
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(msg Message) {
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// SetMetadata sets the session metadata.
func (s *Session) SetMetadata(metadata Metadata) {
	s.Metadata = metadata
	s.UpdatedAt = time.Now()
}

// SaveToFile saves the session to a JSON file.
func (s *Session) SaveToFile(filePath string) error {
	s.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %v", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadFromFile loads a session from a JSON file.
func LoadFromFile(filePath string) (*Session, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %v", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %v", err)
	}

	return &session, nil
}

// ConvertFromFantasyMessage converts a fantasy.Message to a session Message.
// This function bridges between the fantasy message format and the
// session's internal message format for JSON persistence.
func ConvertFromFantasyMessage(msg fantasy.Message) Message {
	sessionMsg := Message{
		Role:      string(msg.Role),
		Timestamp: time.Now(),
	}

	// Extract text content and tool calls from message parts
	var textParts []string
	for _, part := range msg.Content {
		switch p := part.(type) {
		case fantasy.TextPart:
			textParts = append(textParts, p.Text)
		case fantasy.ToolCallPart:
			sessionMsg.ToolCalls = append(sessionMsg.ToolCalls, ToolCall{
				ID:        p.ToolCallID,
				Name:      p.ToolName,
				Arguments: p.Input,
			})
		case fantasy.ToolResultPart:
			// Tool result messages — store the tool call ID
			sessionMsg.ToolCallID = p.ToolCallID
			// Marshal result for storage
			if p.Output != nil {
				if resultBytes, err := json.Marshal(p.Output); err == nil {
					textParts = append(textParts, string(resultBytes))
				}
			}
		}
	}

	// Join all text parts
	for i, t := range textParts {
		if i > 0 {
			sessionMsg.Content += "\n"
		}
		sessionMsg.Content += t
	}

	return sessionMsg
}

// ConvertToFantasyMessage converts a session Message to a fantasy.Message.
// This method bridges between the session's internal message format and
// the fantasy message format used by the LLM providers.
func (m *Message) ConvertToFantasyMessage() fantasy.Message {
	msg := fantasy.Message{
		Role: fantasy.MessageRole(m.Role),
	}

	// Build content parts based on role
	switch m.Role {
	case "assistant":
		// Add text content if present
		if m.Content != "" {
			msg.Content = append(msg.Content, fantasy.TextPart{Text: m.Content})
		}
		// Add tool calls if present
		for _, tc := range m.ToolCalls {
			var inputStr string
			if str, ok := tc.Arguments.(string); ok {
				inputStr = str
			} else if argBytes, err := json.Marshal(tc.Arguments); err == nil {
				inputStr = string(argBytes)
			}

			msg.Content = append(msg.Content, fantasy.ToolCallPart{
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				Input:      inputStr,
			})
		}
	case "tool":
		// Tool result message
		msg.Role = fantasy.MessageRoleTool
		var resultContent fantasy.ToolResultOutputContent
		resultContent = fantasy.ToolResultOutputContentText{Text: m.Content}

		msg.Content = append(msg.Content, fantasy.ToolResultPart{
			ToolCallID: m.ToolCallID,
			Output:     resultContent,
		})
	case "user":
		msg.Content = append(msg.Content, fantasy.TextPart{Text: m.Content})
	case "system":
		msg.Content = append(msg.Content, fantasy.TextPart{Text: m.Content})
	default:
		msg.Content = append(msg.Content, fantasy.TextPart{Text: m.Content})
	}

	return msg
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return "msg_" + hex.EncodeToString(bytes)
}
