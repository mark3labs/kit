package app

import (
	"context"

	"charm.land/fantasy"

	"github.com/mark3labs/mcphost/internal/agent"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/hooks"
	"github.com/mark3labs/mcphost/internal/session"
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
		onToolApproval agent.ToolApprovalHandler,
	) (*agent.GenerateWithLoopResult, error)
}

// ToolApprovalFunc is the callback invoked by the app layer when the agent needs
// user approval before executing a tool call. It must return true to approve or
// false to deny. The ctx is used to unblock the call if the app is shutting down.
type ToolApprovalFunc func(ctx context.Context, toolName, toolArgs string) (bool, error)

// UsageUpdater is the interface the app layer uses to record token usage after
// each agent step. It is satisfied by *ui.UsageTracker (which lives in
// internal/ui) without creating an import cycle — the concrete type is wired
// in cmd/root.go, which can import both packages.
type UsageUpdater interface {
	// UpdateUsage records actual token counts returned by the provider.
	UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int)
	// EstimateAndUpdateUsage falls back to text-based token estimation when
	// the provider does not return exact counts.
	EstimateAndUpdateUsage(inputText, outputText string)
}

// AutoApproveFunc is a ToolApprovalFunc that always approves tool calls.
// Used in non-interactive mode.
var AutoApproveFunc ToolApprovalFunc = func(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

// Options configures an App instance. It mirrors the fields from AgenticLoopConfig
// in cmd/root.go but is owned by the app layer rather than the CLI.
type Options struct {
	// Agent is the agent used to run the agentic loop. Required.
	// *agent.Agent satisfies this interface; tests may supply stubs.
	Agent AgentRunner

	// ToolApprovalFunc is called when the agent needs user confirmation before
	// running a tool. Required; use AutoApproveFunc for non-interactive mode.
	ToolApprovalFunc ToolApprovalFunc

	// HookExecutor is the optional hooks executor for firing UserPromptSubmit,
	// PreToolUse, PostToolUse, and Stop events around the agentic loop.
	HookExecutor *hooks.Executor

	// SessionManager is the optional session manager for persisting conversation
	// history to disk. When non-nil, the MessageStore calls it on every mutation.
	SessionManager *session.Manager

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
}
