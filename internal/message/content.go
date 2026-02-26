package message

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"charm.land/fantasy"
)

// ContentPart is the marker interface for all message content block types.
// A message contains a heterogeneous slice of ContentPart values, enabling
// rich structured messages that carry text, reasoning, tool calls, tool
// results, and finish markers in a single message.
type ContentPart interface {
	isPart() // marker — prevents external implementations
}

// --- Concrete content block types ---

// TextContent holds plain text content within a message.
type TextContent struct {
	Text string `json:"text"`
}

func (TextContent) isPart() {}

// ReasoningContent holds extended thinking / reasoning output from the LLM.
// Provider-specific metadata (signatures, etc.) is preserved for round-trip
// fidelity when the conversation is sent back to the provider.
type ReasoningContent struct {
	Thinking  string `json:"thinking"`
	Signature string `json:"signature,omitempty"` // Anthropic
}

func (ReasoningContent) isPart() {}

// ToolCall represents a tool invocation initiated by the LLM. It is stored
// as a content part within an assistant message, not as a separate message.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input"` // JSON string of arguments
	Finished bool   `json:"finished"`
}

func (ToolCall) isPart() {}

// ToolResult represents the result of executing a tool. It is stored as a
// content part within a tool-role message, linked to a ToolCall by ID.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

func (ToolResult) isPart() {}

// Finish marks the end of an assistant turn, carrying the stop reason.
type Finish struct {
	Reason string `json:"reason"` // "end_turn", "tool_use", "max_tokens", etc.
}

func (Finish) isPart() {}

// --- Message container ---

// MessageRole identifies the sender of a message.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
	RoleSystem    MessageRole = "system"
)

// Message is a single conversation message containing a heterogeneous slice
// of ContentPart blocks. This design (borrowed from crush) enables a single
// assistant message to carry text, reasoning, and multiple tool calls as
// discrete, typed blocks rather than flattening everything into strings.
type Message struct {
	ID        string        `json:"id"`
	Role      MessageRole   `json:"role"`
	Parts     []ContentPart `json:"parts"`
	Model     string        `json:"model,omitempty"`
	Provider  string        `json:"provider,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// --- Typed accessors ---

// Content returns the concatenated text from all TextContent parts.
func (m *Message) Content() string {
	var text string
	for _, part := range m.Parts {
		if c, ok := part.(TextContent); ok {
			if text != "" {
				text += "\n"
			}
			text += c.Text
		}
	}
	return text
}

// ToolCalls returns all ToolCall parts from this message.
func (m *Message) ToolCalls() []ToolCall {
	var calls []ToolCall
	for _, part := range m.Parts {
		if c, ok := part.(ToolCall); ok {
			calls = append(calls, c)
		}
	}
	return calls
}

// ToolResults returns all ToolResult parts from this message.
func (m *Message) ToolResults() []ToolResult {
	var results []ToolResult
	for _, part := range m.Parts {
		if r, ok := part.(ToolResult); ok {
			results = append(results, r)
		}
	}
	return results
}

// Reasoning returns the ReasoningContent if present, or a zero value.
func (m *Message) Reasoning() ReasoningContent {
	for _, part := range m.Parts {
		if r, ok := part.(ReasoningContent); ok {
			return r
		}
	}
	return ReasoningContent{}
}

// AddPart appends a content part and updates the timestamp.
func (m *Message) AddPart(part ContentPart) {
	m.Parts = append(m.Parts, part)
	m.UpdatedAt = time.Now()
}

// AddToolCall appends or updates a ToolCall part. If a call with the same
// ID already exists, it is replaced (supports streaming where partial calls
// arrive before the final version).
func (m *Message) AddToolCall(tc ToolCall) {
	for i, part := range m.Parts {
		if existing, ok := part.(ToolCall); ok && existing.ID == tc.ID {
			m.Parts[i] = tc
			m.UpdatedAt = time.Now()
			return
		}
	}
	m.Parts = append(m.Parts, tc)
	m.UpdatedAt = time.Now()
}

// --- Type-tagged JSON serialization ---

type partType string

const (
	textType       partType = "text"
	reasoningType  partType = "reasoning"
	toolCallType   partType = "tool_call"
	toolResultType partType = "tool_result"
	finishType     partType = "finish"
)

type partWrapper struct {
	Type partType        `json:"type"`
	Data json.RawMessage `json:"data"`
}

// MarshalParts serializes a slice of ContentPart to JSON using type-tagged
// wrappers. Each part becomes {"type":"...", "data":{...}}.
func MarshalParts(parts []ContentPart) ([]byte, error) {
	wrappers := make([]partWrapper, 0, len(parts))
	for _, part := range parts {
		var pt partType
		switch part.(type) {
		case TextContent:
			pt = textType
		case ReasoningContent:
			pt = reasoningType
		case ToolCall:
			pt = toolCallType
		case ToolResult:
			pt = toolResultType
		case Finish:
			pt = finishType
		default:
			return nil, fmt.Errorf("unknown content part type: %T", part)
		}
		data, err := json.Marshal(part)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal %s part: %w", pt, err)
		}
		wrappers = append(wrappers, partWrapper{Type: pt, Data: data})
	}
	return json.Marshal(wrappers)
}

// UnmarshalParts deserializes type-tagged JSON back into a slice of ContentPart.
func UnmarshalParts(data []byte) ([]ContentPart, error) {
	var wrappers []partWrapper
	if err := json.Unmarshal(data, &wrappers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parts array: %w", err)
	}

	parts := make([]ContentPart, 0, len(wrappers))
	for _, w := range wrappers {
		var part ContentPart
		switch w.Type {
		case textType:
			var p TextContent
			if err := json.Unmarshal(w.Data, &p); err != nil {
				return nil, fmt.Errorf("failed to unmarshal text part: %w", err)
			}
			part = p
		case reasoningType:
			var p ReasoningContent
			if err := json.Unmarshal(w.Data, &p); err != nil {
				return nil, fmt.Errorf("failed to unmarshal reasoning part: %w", err)
			}
			part = p
		case toolCallType:
			var p ToolCall
			if err := json.Unmarshal(w.Data, &p); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool_call part: %w", err)
			}
			part = p
		case toolResultType:
			var p ToolResult
			if err := json.Unmarshal(w.Data, &p); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool_result part: %w", err)
			}
			part = p
		case finishType:
			var p Finish
			if err := json.Unmarshal(w.Data, &p); err != nil {
				return nil, fmt.Errorf("failed to unmarshal finish part: %w", err)
			}
			part = p
		default:
			return nil, fmt.Errorf("unknown part type: %s", w.Type)
		}
		parts = append(parts, part)
	}
	return parts, nil
}

// --- Fantasy bridge ---

// ToFantasyMessages converts a Message to one or more fantasy.Message values.
// An assistant message with tool calls produces a single fantasy message with
// mixed TextPart and ToolCallPart content. Tool-role messages produce
// ToolResultPart entries.
func (m *Message) ToFantasyMessages() []fantasy.Message {
	switch m.Role {
	case RoleAssistant:
		var parts []fantasy.MessagePart

		// Add reasoning if present
		reasoning := m.Reasoning()
		if reasoning.Thinking != "" {
			parts = append(parts, fantasy.ReasoningPart{
				Text: reasoning.Thinking,
			})
		}

		// Add text content
		if text := m.Content(); text != "" {
			parts = append(parts, fantasy.TextPart{Text: text})
		}

		// Add tool calls
		for _, tc := range m.ToolCalls() {
			parts = append(parts, fantasy.ToolCallPart{
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				Input:      tc.Input,
			})
		}

		if len(parts) == 0 {
			return nil
		}
		return []fantasy.Message{{
			Role:    fantasy.MessageRoleAssistant,
			Content: parts,
		}}

	case RoleTool:
		var parts []fantasy.MessagePart
		for _, result := range m.ToolResults() {
			var output fantasy.ToolResultOutputContent
			if result.IsError {
				output = fantasy.ToolResultOutputContentError{
					Error: errors.New(result.Content),
				}
			} else {
				output = fantasy.ToolResultOutputContentText{
					Text: result.Content,
				}
			}
			parts = append(parts, fantasy.ToolResultPart{
				ToolCallID: result.ToolCallID,
				Output:     output,
			})
		}
		if len(parts) == 0 {
			return nil
		}
		return []fantasy.Message{{
			Role:    fantasy.MessageRoleTool,
			Content: parts,
		}}

	case RoleUser:
		text := m.Content()
		if text == "" {
			return nil
		}
		return []fantasy.Message{{
			Role:    fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: text}},
		}}

	case RoleSystem:
		text := m.Content()
		if text == "" {
			return nil
		}
		return []fantasy.Message{{
			Role:    fantasy.MessageRoleSystem,
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: text}},
		}}

	default:
		return nil
	}
}

// FromFantasyMessage converts a fantasy.Message into our Message type,
// extracting all content parts into the appropriate block types.
func FromFantasyMessage(msg fantasy.Message) Message {
	m := Message{
		Role:      MessageRole(msg.Role),
		Parts:     make([]ContentPart, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for _, part := range msg.Content {
		switch p := part.(type) {
		case fantasy.TextPart:
			if p.Text != "" {
				m.Parts = append(m.Parts, TextContent{Text: p.Text})
			}
		case fantasy.ToolCallPart:
			m.Parts = append(m.Parts, ToolCall{
				ID:       p.ToolCallID,
				Name:     p.ToolName,
				Input:    p.Input,
				Finished: true,
			})
		case fantasy.ToolResultPart:
			result := ToolResult{
				ToolCallID: p.ToolCallID,
			}
			switch r := p.Output.(type) {
			case fantasy.ToolResultOutputContentText:
				result.Content = r.Text
			case fantasy.ToolResultOutputContentError:
				result.Content = r.Error.Error()
				result.IsError = true
			}
			m.Parts = append(m.Parts, result)
		case fantasy.ReasoningPart:
			if p.Text != "" {
				m.Parts = append(m.Parts, ReasoningContent{
					Thinking: p.Text,
				})
			}
		}
	}

	return m
}
