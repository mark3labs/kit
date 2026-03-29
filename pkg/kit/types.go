package kit

import (
	"context"

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

// LLMMessage is the underlying message type used by the LLM agent
// library. Re-exported so SDK users can work with LLM types without a
// direct import of the underlying LLM library.
type LLMMessage = fantasy.Message

// LLMUsage contains token usage information from an LLM response.
type LLMUsage = fantasy.Usage

// LLMResponse is the response type returned by the LLM agent library.
type LLMResponse = fantasy.Response

// LLMFilePart represents a file attachment (image, document, etc.) that can
// be included in a prompt via PromptResultWithFiles.
type LLMFilePart = fantasy.FilePart

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

// ConvertToLLMMessages converts an SDK message to the underlying LLM
// messages used by the agent for LLM interactions.
func ConvertToLLMMessages(msg *Message) []fantasy.Message {
	return msg.ToFantasyMessages()
}

// ConvertFromLLMMessage converts an LLM message from the agent to an SDK
// message format for use in the SDK API.
func ConvertFromLLMMessage(msg fantasy.Message) Message {
	return message.FromFantasyMessage(msg)
}

// Deprecated: Use ConvertToLLMMessages instead.
func ConvertToFantasyMessages(msg *Message) []fantasy.Message {
	return ConvertToLLMMessages(msg)
}

// Deprecated: Use ConvertFromLLMMessage instead.
func ConvertFromFantasyMessage(msg fantasy.Message) Message {
	return ConvertFromLLMMessage(msg)
}
