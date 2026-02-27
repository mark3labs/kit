package app

import (
	"context"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/session"
)

// AgentRunner is the minimal interface the app layer requires from the agent
// package. *agent.Agent satisfies this interface. Defining it here allows
// unit tests to supply stub implementations without spinning up a real LLM.
type AgentRunner interface {
	GenerateWithLoopAndStreaming(
		ctx context.Context,
		messages []fantasy.Message,
		onToolCall agent.ToolCallHandler,
		onToolExecution agent.ToolExecutionHandler,
		onToolResult agent.ToolResultHandler,
		onResponse agent.ResponseHandler,
		onToolCallContent agent.ToolCallContentHandler,
		onStreamingResponse agent.StreamingResponseHandler,
	) (*agent.GenerateWithLoopResult, error)
}

// UsageUpdater is the interface the app layer uses to record token usage after
// each agent step. It is satisfied by *ui.UsageTracker (which lives in
// internal/ui) without creating an import cycle — the concrete type is wired
// in cmd/root.go, which can import both packages.
type UsageUpdater interface {
	// UpdateUsage records actual token counts returned by the provider.
	// The counts come from fantasy's TotalUsage (aggregate across all steps
	// in a multi-step tool-calling run) and are used for session cost tracking.
	UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int)
	// EstimateAndUpdateUsage falls back to text-based token estimation when
	// the provider does not return exact counts.
	EstimateAndUpdateUsage(inputText, outputText string)
	// SetContextTokens records the approximate current context window fill
	// level. This should be the final API call's input+output tokens (from
	// FinalResponse.Usage), NOT the aggregate TotalUsage.
	SetContextTokens(tokens int)
}

// Options configures an App instance. It mirrors the fields from AgenticLoopConfig
// in cmd/root.go but is owned by the app layer rather than the CLI.
type Options struct {
	// Agent is the agent used to run the agentic loop. Required.
	// *agent.Agent satisfies this interface; tests may supply stubs.
	Agent AgentRunner

	// TreeSession is the tree-structured JSONL session manager. When non-nil,
	// conversation history is persisted as an append-only JSONL tree and tree
	// navigation (/tree, /fork) is enabled.
	TreeSession *session.TreeManager

	// MCPConfig is the full MCP configuration used for session continuation and
	// slash command resolution.
	MCPConfig *config.Config

	// ModelName is the display name of the model (e.g. "claude-sonnet-4-5").
	ModelName string

	// ServerNames holds the names of loaded MCP servers, used for slash command
	// autocomplete.
	ServerNames []string

	// ToolNames holds the names of available tools, used for slash command
	// autocomplete.
	ToolNames []string

	// StreamingEnabled controls whether the agent uses streaming responses.
	StreamingEnabled bool

	// Quiet suppresses all output except the final response (non-interactive mode).
	Quiet bool

	// Debug enables verbose debug logging.
	Debug bool

	// CompactMode selects the compact renderer instead of the block renderer for
	// message formatting.
	CompactMode bool

	// UsageTracker is an optional callback for recording token usage after each
	// agent step. When non-nil, the app layer calls UpdateUsage (or
	// EstimateAndUpdateUsage as a fallback) using the usage data returned by the
	// agent. Satisfied by *ui.UsageTracker; wired in cmd/root.go.
	UsageTracker UsageUpdater

	// Extensions is the optional extension runner. When non-nil, lifecycle
	// events (Input, BeforeAgentStart, AgentEnd, etc.) are emitted through
	// it. Tool-level events (ToolCall, ToolResult) are handled by wrapper.go
	// at the tool layer, not here.
	Extensions *extensions.Runner
}
