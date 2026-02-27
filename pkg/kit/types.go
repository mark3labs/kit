package kit

import (
	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
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

// Session represents a complete conversation session with metadata.
type Session = session.Session

// SessionMetadata contains contextual information about the session
// (provider, model, kit version).
type SessionMetadata = session.Metadata

// SessionManager manages session state and auto-saving functionality.
type SessionManager = session.Manager

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

// GenerateResult contains the result and conversation history from an agent
// interaction.
type GenerateResult = agent.GenerateWithLoopResult

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

// ==== Fantasy Types (re-exported) ====

// FantasyMessage is the underlying message type used by the fantasy agent
// library. Re-exported so SDK users can work with fantasy types without a
// direct import of charm.land/fantasy.
type FantasyMessage = fantasy.Message

// FantasyUsage contains token usage information from an LLM response.
type FantasyUsage = fantasy.Usage

// FantasyResponse is the response type returned by the fantasy agent library.
type FantasyResponse = fantasy.Response

// ==== Constructor & Helper Functions ====

var (
	// NewSession creates a new session with default values.
	NewSession = session.NewSession
	// NewSessionManager creates a new session manager with a fresh session.
	NewSessionManager = session.NewManager
	// ListSessions finds all sessions for a given working directory, sorted
	// by modification time (newest first).
	ListSessions = session.ListSessions
	// ListAllSessions finds all sessions across all working directories,
	// sorted by modification time (newest first).
	ListAllSessions = session.ListAllSessions
	// ParseModelString parses a model string in "provider/model" format.
	ParseModelString = models.ParseModelString
	// CreateProvider creates a fantasy LanguageModel based on provider config.
	CreateProvider = models.CreateProvider
	// GetGlobalRegistry returns the global models registry instance.
	GetGlobalRegistry = models.GetGlobalRegistry
	// LoadSystemPrompt loads system prompt from file or returns string directly.
	LoadSystemPrompt = config.LoadSystemPrompt
)

// ==== Conversion Helpers ====

// ConvertToFantasyMessages converts an SDK message to the underlying fantasy
// messages used by the agent for LLM interactions.
func ConvertToFantasyMessages(msg *Message) []fantasy.Message {
	return msg.ToFantasyMessages()
}

// ConvertFromFantasyMessage converts a fantasy message from the agent to an SDK
// message format for use in the SDK API.
func ConvertFromFantasyMessage(msg fantasy.Message) Message {
	return message.FromFantasyMessage(msg)
}
