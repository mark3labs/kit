package kit

import (
	"context"
	"strings"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/compaction"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/session"
)

// ==== Message Types (internal/message/content.go) ====

// Message is a single conversation message containing heterogeneous content
// parts (text, reasoning, tool calls, tool results, finish markers).
type Message = message.Message

// MessageRole identifies the sender of a message (user, assistant, tool, system).
type MessageRole = message.MessageRole

const (
	RoleUser      = message.RoleUser
	RoleAssistant = message.RoleAssistant
	RoleTool      = message.RoleTool
	RoleSystem    = message.RoleSystem
)

// ContentPart is the marker interface for all message content block types.
type ContentPart = message.ContentPart

// TextContent holds plain text content within a message.
type TextContent = message.TextContent

// ReasoningContent holds extended thinking / reasoning output from the LLM.
type ReasoningContent = message.ReasoningContent

// ToolCall represents a tool invocation initiated by the LLM.
type ToolCall = message.ToolCall

// ToolResult represents the result of executing a tool.
type ToolResult = message.ToolResult

// Finish marks the end of an assistant turn, carrying the stop reason.
type Finish = message.Finish

// ==== Session Types (internal/session/) ====

// SessionInfo contains metadata about a discovered session, used for listing
// and session picker display.
type SessionInfo = session.SessionInfo

// TreeManager manages a tree-structured JSONL session with branching,
// leaf-pointer tracking, and context building.
type TreeManager = session.TreeManager

// SessionHeader is the first line in a JSONL session file, carrying metadata.
type SessionHeader = session.SessionHeader

// MessageEntry stores a conversation message as a tree entry in JSONL sessions.
type MessageEntry = session.MessageEntry

// ==== Config Types (internal/config/) ====

// Config represents the complete application configuration including MCP
// servers, model settings, UI preferences, and API credentials.
type Config = config.Config

// MCPServerConfig represents configuration for an MCP server, supporting both
// local (stdio) and remote (StreamableHTTP/SSE) server types.
type MCPServerConfig = config.MCPServerConfig

// ==== Agent Types (internal/agent/) ====

// AgentConfig holds configuration options for creating a new Agent.
type AgentConfig = agent.AgentConfig

type (
	// ToolCallHandler is a function type for handling tool calls as they happen.
	ToolCallHandler = agent.ToolCallHandler
	// ToolExecutionHandler is a function type for handling tool execution start/end events.
	ToolExecutionHandler = agent.ToolExecutionHandler
	// ToolResultHandler is a function type for handling tool results.
	ToolResultHandler = agent.ToolResultHandler
	// ResponseHandler is a function type for handling LLM responses.
	ResponseHandler = agent.ResponseHandler
	// StreamingResponseHandler is a function type for handling streaming LLM responses.
	StreamingResponseHandler = agent.StreamingResponseHandler
	// ToolCallContentHandler is a function type for handling content that accompanies tool calls.
	ToolCallContentHandler = agent.ToolCallContentHandler
)

// ==== Provider & Model Types (internal/models/) ====

// ProviderConfig holds configuration for creating LLM providers.
type ProviderConfig = models.ProviderConfig

// ProviderResult contains the result of provider creation (model + optional
// feedback message + closer).
type ProviderResult = models.ProviderResult

// ModelInfo represents information about a specific model (capabilities,
// pricing, limits).
type ModelInfo = models.ModelInfo

// ModelCost represents the pricing information for a model.
type ModelCost = models.Cost

// ModelLimit represents the context and output limits for a model.
type ModelLimit = models.Limit

// ProviderInfo represents information about a model provider from the
// models.dev database.
type ProviderInfo = models.ProviderInfo

// ModelsRegistry provides validation and information about models, maintaining
// a registry of all supported LLM providers and their models.
type ModelsRegistry = models.ModelsRegistry

// ==== Agent Callback Types ====

// SpinnerFunc wraps a function in a loading spinner animation. Used for
// Ollama model loading. Signature: func(fn func() error) error.
type SpinnerFunc = agent.SpinnerFunc

// ==== LLM Types ====

// LLMMessageRole identifies the participant role in an LLM conversation.
type LLMMessageRole string

const (
	// LLMMessageRoleUser identifies a user message.
	LLMMessageRoleUser LLMMessageRole = "user"
	// LLMMessageRoleAssistant identifies an assistant message.
	LLMMessageRoleAssistant LLMMessageRole = "assistant"
	// LLMMessageRoleSystem identifies a system message.
	LLMMessageRoleSystem LLMMessageRole = "system"
	// LLMMessageRoleTool identifies a tool result message.
	LLMMessageRoleTool LLMMessageRole = "tool"
)

// LLMMessage represents a message in an LLM conversation. It carries the
// role and a plain-text representation of the message content.
type LLMMessage struct {
	// Role is the participant role (user, assistant, system, tool).
	Role LLMMessageRole `json:"role"`
	// Content is the text content of the message.
	Content string `json:"content"`
}

// LLMUsage contains token usage information returned by the LLM provider.
type LLMUsage struct {
	// InputTokens is the number of tokens in the prompt.
	InputTokens int64 `json:"input_tokens"`
	// OutputTokens is the number of tokens in the response.
	OutputTokens int64 `json:"output_tokens"`
	// TotalTokens is the total tokens used (input + output).
	TotalTokens int64 `json:"total_tokens"`
	// ReasoningTokens is the number of tokens used for chain-of-thought reasoning.
	ReasoningTokens int64 `json:"reasoning_tokens"`
	// CacheCreationTokens is the number of tokens written to the provider cache.
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	// CacheReadTokens is the number of tokens read from the provider cache.
	CacheReadTokens int64 `json:"cache_read_tokens"`
}

// LLMResponse represents a response from the LLM provider.
type LLMResponse struct {
	// Content is the text content of the response.
	Content string `json:"content"`
	// FinishReason explains why the LLM stopped generating
	// (e.g. "stop", "length", "tool-calls", "error").
	FinishReason string `json:"finish_reason"`
	// Usage contains the token usage for this response.
	Usage LLMUsage `json:"usage"`
}

// LLMFilePart represents a file attachment (image, document, audio, etc.)
// that can be included in a multimodal prompt via PromptResultWithFiles.
type LLMFilePart struct {
	// Filename is the optional display name of the file.
	Filename string `json:"filename"`
	// Data is the raw file bytes.
	Data []byte `json:"data"`
	// MediaType is the MIME type of the file (e.g. "image/png", "application/pdf").
	MediaType string `json:"media_type"`
}

// ==== Compaction Types (internal/compaction/) ====

// CompactionResult contains statistics from a compaction operation.
type CompactionResult = compaction.CompactionResult

// CompactionOptions configures compaction behaviour.
type CompactionOptions = compaction.CompactionOptions

// ==== Constructor & Helper Functions ====

// ParseModelString parses a model string in "provider/model" format.
// Returns provider, modelID, and an error if the format is invalid.
func ParseModelString(model string) (provider, modelID string, err error) {
	return models.ParseModelString(model)
}

// CreateProvider creates a LanguageModel based on provider config.
func CreateProvider(ctx context.Context, cfg *ProviderConfig) (*ProviderResult, error) {
	return models.CreateProvider(ctx, cfg)
}

// GetGlobalRegistry returns the global models registry instance.
func GetGlobalRegistry() *ModelsRegistry {
	return models.GetGlobalRegistry()
}

// LoadSystemPrompt loads a system prompt from a file path, or returns the
// string directly if it is not a valid file path.
func LoadSystemPrompt(pathOrContent string) (string, error) {
	return config.LoadSystemPrompt(pathOrContent)
}

// ==== Conversion Helpers ====

// ConvertToLLMMessages converts an SDK message to a slice of LLMMessages.
// Each SDK message may expand to multiple LLM messages depending on its content.
func ConvertToLLMMessages(msg *Message) []LLMMessage {
	raw := msg.ToLLMMessages()
	result := make([]LLMMessage, 0, len(raw))
	for _, fm := range raw {
		lm := LLMMessage{
			Role:    LLMMessageRole(fm.Role),
			Content: extractTextFromFantasyMessage(fm),
		}
		result = append(result, lm)
	}
	return result
}

// ConvertFromLLMMessage converts an LLMMessage to an SDK message.
func ConvertFromLLMMessage(msg LLMMessage) Message {
	fm := fantasy.Message{
		Role:    fantasy.MessageRole(msg.Role),
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: msg.Content}},
	}
	return message.FromLLMMessage(fm)
}

// extractTextFromFantasyMessage extracts plain text from a fantasy.Message.
func extractTextFromFantasyMessage(fm fantasy.Message) string {
	var b strings.Builder
	for _, part := range fm.Content {
		if tp, ok := part.(fantasy.TextPart); ok {
			b.WriteString(tp.Text)
		}
	}
	return b.String()
}
