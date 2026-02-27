package kit

import (
	"context"
	"fmt"
	"os"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/session"

	"github.com/spf13/viper"
)

// Kit provides programmatic access to kit functionality, allowing
// integration of MCP tools and LLM interactions into Go applications. It manages
// agents, sessions, and model configurations.
type Kit struct {
	agent       *agent.Agent
	treeSession *session.TreeManager
	modelString string
	events      *eventBus
}

// Subscribe registers an EventListener that will be called for every lifecycle
// event emitted during Prompt() and PromptWithCallbacks(). Returns an
// unsubscribe function that removes the listener.
func (m *Kit) Subscribe(listener EventListener) func() {
	return m.events.subscribe(listener)
}

// Options configures Kit creation with optional overrides for model,
// prompts, configuration, and behavior settings. All fields are optional
// and will use CLI defaults if not specified.
type Options struct {
	Model        string // Override model (e.g., "anthropic/claude-sonnet-4-5-20250929")
	SystemPrompt string // Override system prompt
	ConfigFile   string // Override config file path
	MaxSteps     int    // Override max steps (0 = use default)
	Streaming    bool   // Enable streaming (default from config)
	Quiet        bool   // Suppress debug output
	Tools        []Tool // Custom tool set. If empty, AllTools() is used.

	// Session configuration
	SessionDir  string // Base directory for session discovery (default: cwd)
	SessionPath string // Open a specific session file by path
	Continue    bool   // Continue the most recent session for SessionDir
	NoSession   bool   // Ephemeral mode — in-memory session, no persistence
}

// InitTreeSession creates or opens a tree session based on the given options.
// Both kit.New() and the CLI use this function so session initialisation
// logic lives in one place.
//
// Behaviour based on Options:
//   - NoSession:   in-memory tree session (no persistence)
//   - Continue:    resume most recent session for SessionDir (or cwd)
//   - SessionPath: open a specific JSONL session file
//   - default:     create a new tree session for SessionDir (or cwd)
func InitTreeSession(opts *Options) (*session.TreeManager, error) {
	if opts == nil {
		opts = &Options{}
	}

	sessionDir := opts.SessionDir
	if sessionDir == "" {
		sessionDir, _ = os.Getwd()
	}

	if opts.NoSession {
		return session.InMemoryTreeSession(sessionDir), nil
	}

	if opts.Continue {
		return session.ContinueRecent(sessionDir)
	}

	if opts.SessionPath != "" {
		return session.OpenTreeSession(opts.SessionPath)
	}

	// Default: create a new tree session for the working directory.
	return session.CreateTreeSession(sessionDir)
}

// New creates a Kit instance using the same initialization as the CLI.
// It loads configuration, initializes MCP servers, creates the LLM model, and
// sets up the agent for interaction. Returns an error if initialization fails.
func New(ctx context.Context, opts *Options) (*Kit, error) {
	if opts == nil {
		opts = &Options{}
	}

	// Set CLI-equivalent defaults for viper. When used as an SDK (without
	// cobra), these defaults are not registered via flag bindings.
	setSDKDefaults()

	// Initialize config (loads config files and env vars).
	if err := InitConfig(opts.ConfigFile, false); err != nil {
		return nil, fmt.Errorf("failed to initialize config: %w", err)
	}

	// Override viper settings with options.
	if opts.Model != "" {
		viper.Set("model", opts.Model)
	}
	if opts.SystemPrompt != "" {
		viper.Set("system-prompt", opts.SystemPrompt)
	}
	if opts.MaxSteps > 0 {
		viper.Set("max-steps", opts.MaxSteps)
	}
	viper.Set("stream", opts.Streaming)

	// Load MCP configuration.
	mcpConfig, err := config.LoadAndValidateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load MCP config: %w", err)
	}

	// Create agent using shared setup.
	agentResult, err := SetupAgent(ctx, AgentSetupOptions{
		MCPConfig: mcpConfig,
		Quiet:     opts.Quiet,
		CoreTools: opts.Tools,
	})
	if err != nil {
		return nil, err
	}

	// Initialize tree session.
	treeSession, err := InitTreeSession(opts)
	if err != nil {
		_ = agentResult.Agent.Close()
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}

	return &Kit{
		agent:       agentResult.Agent,
		treeSession: treeSession,
		modelString: viper.GetString("model"),
		events:      newEventBus(),
	}, nil
}

// Prompt sends a message to the agent and returns the response. The agent may
// use tools as needed to generate the response. The conversation history is
// automatically maintained in the session (tree session if configured,
// otherwise the legacy session manager). Lifecycle events are emitted to all
// registered subscribers. Returns an error if generation fails.
func (m *Kit) Prompt(ctx context.Context, message string) (string, error) {
	userMsg := fantasy.NewUserMessage(message)

	// Persist user message to tree session before building context.
	_, _ = m.treeSession.AppendFantasyMessage(userMsg)

	// Build context from the tree (walks leaf-to-root) so that only the
	// current branch is sent to the LLM.
	messages := m.treeSession.GetFantasyMessages()
	sentCount := len(messages)

	m.events.emit(TurnStartEvent{Prompt: message})
	m.events.emit(MessageStartEvent{})

	result, err := m.agent.GenerateWithLoopAndStreaming(ctx, messages,
		func(toolName, toolArgs string) {
			m.events.emit(ToolCallEvent{ToolName: toolName, ToolArgs: toolArgs})
		},
		func(toolName string, isStarting bool) {
			if isStarting {
				m.events.emit(ToolExecutionStartEvent{ToolName: toolName})
			} else {
				m.events.emit(ToolExecutionEndEvent{ToolName: toolName})
			}
		},
		func(toolName, toolArgs, resultText string, isError bool) {
			m.events.emit(ToolResultEvent{
				ToolName: toolName, ToolArgs: toolArgs,
				Result: resultText, IsError: isError,
			})
		},
		func(content string) {
			m.events.emit(ResponseEvent{Content: content})
		},
		func(content string) {
			m.events.emit(ToolCallContentEvent{Content: content})
		},
		func(chunk string) {
			m.events.emit(MessageUpdateEvent{Chunk: chunk})
		},
	)
	if err != nil {
		m.events.emit(TurnEndEvent{Error: err})
		return "", err
	}

	responseText := result.FinalResponse.Content.Text()

	m.events.emit(MessageEndEvent{Content: responseText})
	m.events.emit(TurnEndEvent{Response: responseText})

	// Persist new messages (tool calls, tool results, assistant response).
	if len(result.ConversationMessages) > sentCount {
		for _, msg := range result.ConversationMessages[sentCount:] {
			_, _ = m.treeSession.AppendFantasyMessage(msg)
		}
	}

	return responseText, nil
}

// PromptWithCallbacks sends a message with callbacks for monitoring tool execution
// and streaming responses. The callbacks allow real-time observation of tool calls,
// results, and response generation. Lifecycle events are also emitted to all
// registered subscribers (via Subscribe). Returns the final response or an error.
//
// Deprecated: Use Subscribe/OnToolCall/OnToolResult/OnStreaming instead of
// inline callbacks. PromptWithCallbacks is retained for backward compatibility.
func (m *Kit) PromptWithCallbacks(
	ctx context.Context,
	message string,
	onToolCall func(name, args string),
	onToolResult func(name, args, result string, isError bool),
	onStreaming func(chunk string),
) (string, error) {
	userMsg := fantasy.NewUserMessage(message)

	// Persist user message to tree session.
	_, _ = m.treeSession.AppendFantasyMessage(userMsg)

	// Build context from tree.
	messages := m.treeSession.GetFantasyMessages()
	sentCount := len(messages)

	m.events.emit(TurnStartEvent{Prompt: message})
	m.events.emit(MessageStartEvent{})

	result, err := m.agent.GenerateWithLoopAndStreaming(ctx, messages,
		func(toolName, toolArgs string) {
			m.events.emit(ToolCallEvent{ToolName: toolName, ToolArgs: toolArgs})
			if onToolCall != nil {
				onToolCall(toolName, toolArgs)
			}
		},
		func(toolName string, isStarting bool) {
			if isStarting {
				m.events.emit(ToolExecutionStartEvent{ToolName: toolName})
			} else {
				m.events.emit(ToolExecutionEndEvent{ToolName: toolName})
			}
		},
		func(toolName, toolArgs, resultText string, isError bool) {
			m.events.emit(ToolResultEvent{
				ToolName: toolName, ToolArgs: toolArgs,
				Result: resultText, IsError: isError,
			})
			if onToolResult != nil {
				onToolResult(toolName, toolArgs, resultText, isError)
			}
		},
		func(content string) {
			m.events.emit(ResponseEvent{Content: content})
		},
		func(content string) {
			m.events.emit(ToolCallContentEvent{Content: content})
		},
		func(chunk string) {
			m.events.emit(MessageUpdateEvent{Chunk: chunk})
			if onStreaming != nil {
				onStreaming(chunk)
			}
		},
	)
	if err != nil {
		m.events.emit(TurnEndEvent{Error: err})
		return "", err
	}

	responseText := result.FinalResponse.Content.Text()

	m.events.emit(MessageEndEvent{Content: responseText})
	m.events.emit(TurnEndEvent{Response: responseText})

	// Persist new messages.
	if len(result.ConversationMessages) > sentCount {
		for _, msg := range result.ConversationMessages[sentCount:] {
			_, _ = m.treeSession.AppendFantasyMessage(msg)
		}
	}

	return responseText, nil
}

// ClearSession resets the tree session's leaf pointer to the root, starting
// a fresh conversation branch.
func (m *Kit) ClearSession() {
	m.treeSession.ResetLeaf()
}

// GetModelString returns the current model string identifier (e.g.,
// "anthropic/claude-sonnet-4-5-20250929" or "openai/gpt-4") being used by the agent.
func (m *Kit) GetModelString() string {
	return m.modelString
}

// GetTools returns all tools available to the agent (core + MCP + extensions).
func (m *Kit) GetTools() []Tool {
	return m.agent.GetTools()
}

// Close cleans up resources including MCP server connections, model resources,
// and the tree session file handle. Should be called when the Kit instance is
// no longer needed. Returns an error if cleanup fails.
func (m *Kit) Close() error {
	if m.treeSession != nil {
		_ = m.treeSession.Close()
	}
	return m.agent.Close()
}
