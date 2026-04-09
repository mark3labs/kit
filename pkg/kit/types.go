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
	"github.com/mark3labs/mcp-go/client/transport"
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
//
// These are type aliases for the corresponding charm.land/fantasy types,
// giving them clean LLM-prefixed names without leaking the dependency name.
// SDK consumers can use these types without importing charm.land/fantasy directly.

// LLMMessage represents a message in an LLM conversation, carrying a role
// and a slice of typed content parts (text, tool calls, reasoning, etc.).
type LLMMessage = fantasy.Message

// LLMMessagePart is the interface implemented by all LLM message content parts.
type LLMMessagePart = fantasy.MessagePart

// LLMFilePart represents a file attachment (image, document, audio, etc.)
// that can be included in a multimodal prompt via PromptResultWithFiles.
type LLMFilePart = fantasy.FilePart

// LLMUsage contains token usage information returned by the LLM provider.
type LLMUsage = fantasy.Usage

// LLMResponse represents a complete response from the LLM provider.
type LLMResponse = fantasy.Response

// LLMTextPart is a plain-text content part for constructing LLM messages.
type LLMTextPart = fantasy.TextPart

// LLMReasoningPart is a reasoning/chain-of-thought content part.
type LLMReasoningPart = fantasy.ReasoningPart

// LLMToolCallPart represents an LLM-initiated tool invocation within a message.
type LLMToolCallPart = fantasy.ToolCallPart

// LLMToolResultPart represents the result of a tool execution within a message.
type LLMToolResultPart = fantasy.ToolResultPart

// LLMToolResultOutputContent is the interface for tool result output content.
type LLMToolResultOutputContent = fantasy.ToolResultOutputContent

// LLMToolResultOutputContentText is a text-valued tool result output.
type LLMToolResultOutputContentText = fantasy.ToolResultOutputContentText

// LLMToolResultOutputContentError is an error-valued tool result output.
type LLMToolResultOutputContentError = fantasy.ToolResultOutputContentError

// LLMMessageRole identifies the participant role in an LLM conversation.
type LLMMessageRole = fantasy.MessageRole

// LLMFinishReason indicates why the LLM stopped generating.
type LLMFinishReason = fantasy.FinishReason

// LLM role constants mirror fantasy.MessageRole* values under clean LLM-prefixed names.
const (
	// LLMRoleUser identifies a user message.
	LLMRoleUser = fantasy.MessageRoleUser
	// LLMRoleAssistant identifies an assistant message.
	LLMRoleAssistant = fantasy.MessageRoleAssistant
	// LLMRoleSystem identifies a system message.
	LLMRoleSystem = fantasy.MessageRoleSystem
	// LLMRoleTool identifies a tool result message.
	LLMRoleTool = fantasy.MessageRoleTool
)

// NewLLMUserMessage constructs a user-role LLMMessage with optional file
// attachments. It is equivalent to fantasy.NewUserMessage.
var NewLLMUserMessage = fantasy.NewUserMessage

// NewLLMSystemMessage constructs a system-role LLMMessage from one or more
// prompt strings. It is equivalent to fantasy.NewSystemMessage.
var NewLLMSystemMessage = fantasy.NewSystemMessage

// ==== Compaction Types (internal/compaction/) ====

// CompactionResult contains statistics from a compaction operation.
type CompactionResult = compaction.CompactionResult

// CompactionOptions configures compaction behaviour.
type CompactionOptions = compaction.CompactionOptions

// ==== MCP OAuth Types ====

// MCPTokenStore persists OAuth tokens for a single MCP server. Implementations
// must be safe for concurrent use.
//
// This is a type alias for the mcp-go transport.TokenStore interface. SDK
// consumers can implement this interface to provide custom storage backends
// (database, encrypted file, in-memory, etc.).
type MCPTokenStore = transport.TokenStore

// MCPToken represents an OAuth token for an MCP server, containing access
// and refresh tokens along with expiration metadata.
type MCPToken = transport.Token

// MCPTokenStoreFactory creates an [MCPTokenStore] for a given MCP server URL.
// It is called once per remote MCP server during connection setup.
type MCPTokenStoreFactory func(serverURL string) (MCPTokenStore, error)

// ErrMCPNoToken is the sentinel error that [MCPTokenStore] implementations
// should return from GetToken when no token is stored for the server.
// Callers can check for this with errors.Is.
var ErrMCPNoToken = transport.ErrNoToken

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
	return msg.ToLLMMessages()
}

// ConvertFromLLMMessage converts an LLMMessage to an SDK message.
func ConvertFromLLMMessage(msg LLMMessage) Message {
	return message.FromLLMMessage(msg)
}
