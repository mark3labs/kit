package kit

import (
	"context"
	"fmt"

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
	sessionMgr  *session.Manager
	modelString string
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

	return &Kit{
		agent:       agentResult.Agent,
		sessionMgr:  session.NewManager(""),
		modelString: viper.GetString("model"),
	}, nil
}

// Prompt sends a message to the agent and returns the response. The agent may
// use tools as needed to generate the response. The conversation history is
// automatically maintained in the session. Returns an error if generation fails.
func (m *Kit) Prompt(ctx context.Context, message string) (string, error) {
	messages := m.sessionMgr.GetMessages()
	userMsg := fantasy.NewUserMessage(message)
	messages = append(messages, userMsg)

	result, err := m.agent.GenerateWithLoop(ctx, messages,
		nil, // onToolCall
		nil, // onToolExecution
		nil, // onToolResult
		nil, // onResponse
		nil, // onToolCallContent
	)
	if err != nil {
		return "", err
	}

	if err := m.sessionMgr.ReplaceAllMessages(result.ConversationMessages); err != nil {
		return "", fmt.Errorf("failed to update session: %w", err)
	}

	return result.FinalResponse.Content.Text(), nil
}

// PromptWithCallbacks sends a message with callbacks for monitoring tool execution
// and streaming responses. The callbacks allow real-time observation of tool calls,
// results, and response generation. Returns the final response or an error.
func (m *Kit) PromptWithCallbacks(
	ctx context.Context,
	message string,
	onToolCall func(name, args string),
	onToolResult func(name, args, result string, isError bool),
	onStreaming func(chunk string),
) (string, error) {
	messages := m.sessionMgr.GetMessages()
	userMsg := fantasy.NewUserMessage(message)
	messages = append(messages, userMsg)

	result, err := m.agent.GenerateWithLoopAndStreaming(ctx, messages,
		onToolCall,
		nil, // onToolExecution
		onToolResult,
		nil, // onResponse
		nil, // onToolCallContent
		onStreaming,
	)
	if err != nil {
		return "", err
	}

	if err := m.sessionMgr.ReplaceAllMessages(result.ConversationMessages); err != nil {
		return "", fmt.Errorf("failed to update session: %w", err)
	}

	return result.FinalResponse.Content.Text(), nil
}

// GetSessionManager returns the current session manager for direct access
// to conversation history and session manipulation.
func (m *Kit) GetSessionManager() *session.Manager {
	return m.sessionMgr
}

// LoadSession loads a previously saved session from a file, restoring the
// conversation history. Returns an error if the file cannot be loaded or parsed.
func (m *Kit) LoadSession(path string) error {
	s, err := session.LoadFromFile(path)
	if err != nil {
		return err
	}
	m.sessionMgr = session.NewManagerWithSession(s, path)
	return nil
}

// SaveSession saves the current session to a file for later restoration.
// Returns an error if the session cannot be written to the specified path.
func (m *Kit) SaveSession(path string) error {
	return m.sessionMgr.GetSession().SaveToFile(path)
}

// ClearSession clears the current session history, starting a new conversation
// with an empty message history.
func (m *Kit) ClearSession() {
	m.sessionMgr = session.NewManager("")
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

// Close cleans up resources including MCP server connections and model resources.
// Should be called when the Kit instance is no longer needed. Returns an
// error if cleanup fails.
func (m *Kit) Close() error {
	return m.agent.Close()
}
