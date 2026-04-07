package kit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	charmlog "github.com/charmbracelet/log"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/core"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/kitsetup"
	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/session"
	"github.com/mark3labs/kit/internal/skills"
	"github.com/mark3labs/kit/internal/tools"

	"github.com/spf13/viper"
)

// ContextFile represents a project context file (e.g. AGENTS.md) that was
// loaded during initialization and injected into the system prompt.
type ContextFile struct {
	Path    string // Absolute filesystem path.
	Content string // Full file content.
}

// Kit provides programmatic access to kit functionality, allowing
// integration of MCP tools and LLM interactions into Go applications. It manages
// agents, sessions, and model configurations.
type Kit struct {
	agent          *agent.Agent
	treeSession    *session.TreeManager
	modelString    string
	events         *eventBus
	autoCompact    bool
	compactionOpts *CompactionOptions
	contextFiles   []*ContextFile
	skills         []*skills.Skill
	extRunner      *extensions.Runner
	bufferedLogger *tools.BufferedDebugLogger
	authHandler    MCPAuthHandler // OAuth handler for remote MCP servers (may need Close)
	opts           *Options       // stored for reload operations (skills, etc.)

	// Hook registries — interception layer (see hooks.go).
	beforeToolCall  *hookRegistry[BeforeToolCallHook, BeforeToolCallResult]
	afterToolResult *hookRegistry[AfterToolResultHook, AfterToolResultResult]
	beforeTurn      *hookRegistry[BeforeTurnHook, BeforeTurnResult]
	afterTurn       *hookRegistry[AfterTurnHook, AfterTurnResult]
	contextPrepare  *hookRegistry[ContextPrepareHook, ContextPrepareResult]
	beforeCompact   *hookRegistry[BeforeCompactHook, BeforeCompactResult]

	// lastInputTokens stores the API-reported input token count from the
	// most recent turn. Used by GetContextStats() to return accurate usage
	// instead of the text-based heuristic which misses system prompts,
	// tool definitions, etc.
	lastInputTokensMu sync.RWMutex
	lastInputTokens   int

	// subagentListeners holds per-tool-call event listeners registered via
	// SubscribeSubagent(). Keyed by toolCallID → *subagentListenerSet.
	subagentListeners sync.Map

	// skillCache holds skills discovered for this Kit instance.
	// Using a per-instance cache avoids cross-contamination when multiple
	// Kit instances exist in the same process.
	skillCache struct {
		skills []*skills.Skill
		mu     sync.RWMutex
	}

	// steerCh is a buffered channel used to inject steering messages into
	// the running agent turn via the LLM library's PrepareStep. Created fresh for
	// each generate() call and set to nil when idle. Protected by steerMu.
	steerMu       sync.Mutex
	steerCh       chan agent.SteerMessage
	leftoverSteer []agent.SteerMessage // unconsumed steer messages from the last turn
}

// Subscribe registers an EventListener that will be called for every lifecycle
// event emitted during Prompt(). Returns an unsubscribe function that removes
// the listener.
func (m *Kit) Subscribe(listener EventListener) func() {
	return m.events.subscribe(listener)
}

// --------------------------------------------------------------------------
// Narrow accessors
// --------------------------------------------------------------------------

// GetToolNames returns the names of all tools available to the agent.
func (m *Kit) GetToolNames() []string {
	agentTools := m.agent.GetTools()
	names := make([]string, len(agentTools))
	for i, t := range agentTools {
		names[i] = t.Info().Name
	}
	return names
}

// GetLoadingMessage returns the agent's startup info message (e.g. GPU
// fallback info), or empty string if none.
func (m *Kit) GetLoadingMessage() string {
	return m.agent.GetLoadingMessage()
}

// GetLoadedServerNames returns the names of successfully loaded MCP servers.
// If MCP servers are still loading in the background, this returns only the
// servers that have completed loading so far.
func (m *Kit) GetLoadedServerNames() []string {
	return m.agent.GetLoadedServerNames()
}

// GetMCPToolCount returns the number of tools loaded from external MCP servers.
// If MCP servers are still loading in the background, this returns the count
// of tools loaded so far (may be 0).
func (m *Kit) GetMCPToolCount() int {
	return m.agent.GetMCPToolCount()
}

// WaitForMCPTools blocks until background MCP tool loading completes.
// Returns nil if no MCP servers are configured or if loading succeeded.
// Returns the loading error if all servers failed. Safe to call multiple times.
func (m *Kit) WaitForMCPTools() error {
	return m.agent.WaitForMCPTools()
}

// MCPToolsReady returns true if MCP tool loading has completed (or was never
// started). This is a non-blocking check useful for UI status display.
func (m *Kit) MCPToolsReady() bool {
	return m.agent.MCPToolsReady()
}

// GetExtensionToolCount returns the number of tools registered by extensions.
func (m *Kit) GetExtensionToolCount() int {
	return m.agent.GetExtensionToolCount()
}

// GetBufferedDebugMessages returns any debug messages that were buffered
// during initialization, then clears the buffer. Returns nil if no messages
// were buffered or if buffered logging was not configured.
func (m *Kit) GetBufferedDebugMessages() []string {
	if m.bufferedLogger == nil {
		return nil
	}
	return m.bufferedLogger.GetMessages()
}

// StructuredMessage represents a conversation message with typed content parts
// (tool calls, reasoning, finish markers, etc.) instead of flattened text.
type StructuredMessage struct {
	ID        string
	ParentID  string
	Role      MessageRole
	Parts     []ContentPart
	Model     string
	Provider  string
	Timestamp string // RFC3339 format
}

// GetStructuredMessages returns the conversation messages on the current
// branch with full typed content parts. Unlike GetSessionMessages() which
// flattens all content to a single text string, this preserves tool calls,
// tool results, reasoning blocks, and finish markers as distinct typed parts.
func (m *Kit) GetStructuredMessages() []StructuredMessage {
	return iterBranchMessages(m.treeSession, func(me *session.MessageEntry, msg message.Message) StructuredMessage {
		return StructuredMessage{
			ID:        me.ID,
			ParentID:  me.ParentID,
			Role:      msg.Role,
			Parts:     msg.Parts,
			Model:     msg.Model,
			Provider:  msg.Provider,
			Timestamp: me.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		}
	})
}

// iterBranchMessages iterates over the current branch's MessageEntry items,
// converting each to a message.Message and calling fn to build the result.
// Returns nil if there is no tree session. Skips entries that are not
// MessageEntry or that fail conversion.
func iterBranchMessages[T any](tm *session.TreeManager, fn func(*session.MessageEntry, message.Message) T) []T {
	if tm == nil {
		return nil
	}
	branch := tm.GetBranch("")
	var results []T
	for _, entry := range branch {
		me, ok := entry.(*session.MessageEntry)
		if !ok {
			continue
		}
		msg, err := me.ToMessage()
		if err != nil {
			continue
		}
		results = append(results, fn(me, msg))
	}
	return results
}

// SetModel changes the active model at runtime. The existing tools, system
// prompt, and session are preserved. The model string should be in
// "provider/model" format (e.g. "anthropic/claude-sonnet-4-5-20250929").
// Returns an error if the model string is invalid or the provider cannot
// be created.
func (m *Kit) SetModel(ctx context.Context, modelString string) error {
	// Validate the model string first.
	if _, _, err := ParseModelString(modelString); err != nil {
		return err
	}

	// Build a provider config from current settings, overriding the model.
	// Load system prompt properly (handles both file paths and inline content).
	systemPrompt, _ := config.LoadSystemPrompt(viper.GetString("system-prompt"))
	thinkingLevel := models.ParseThinkingLevel(viper.GetString("thinking-level"))

	// With message-level caching, thinking and caching can work together.
	// No need to disable caching when thinking is enabled.
	config := &models.ProviderConfig{
		ModelString:    modelString,
		SystemPrompt:   systemPrompt,
		ProviderAPIKey: viper.GetString("provider-api-key"),
		ProviderURL:    viper.GetString("provider-url"),
		MaxTokens:      viper.GetInt("max-tokens"),
		TLSSkipVerify:  viper.GetBool("tls-skip-verify"),
		ThinkingLevel:  thinkingLevel,
		DisableCaching: false, // Caching enabled by default, works with thinking
	}
	temperature := float32(viper.GetFloat64("temperature"))
	config.Temperature = &temperature
	topP := float32(viper.GetFloat64("top-p"))
	config.TopP = &topP
	topK := int32(viper.GetInt("top-k"))
	config.TopK = &topK
	frequencyPenalty := float32(viper.GetFloat64("frequency-penalty"))
	config.FrequencyPenalty = &frequencyPenalty
	presencePenalty := float32(viper.GetFloat64("presence-penalty"))
	config.PresencePenalty = &presencePenalty

	if err := m.agent.SetModel(ctx, config); err != nil {
		return err
	}

	m.modelString = modelString

	// Update extension context's Model field.
	if m.extRunner != nil {
		extCtx := m.extRunner.GetContext()
		extCtx.Model = modelString
		m.extRunner.SetContext(extCtx)
	}

	return nil
}

// GetAvailableModels returns a list of known models from the registry. Each
// entry includes provider, model ID, context limit, and whether the model
// supports reasoning. This is an advisory list — models not in the registry
// can still be used by specifying their provider/model string.
func (m *Kit) GetAvailableModels() []extensions.ModelInfoEntry {
	registry := models.GetGlobalRegistry()
	var result []extensions.ModelInfoEntry
	for _, providerID := range registry.GetLLMProviders() {
		modelsMap, err := registry.GetModelsForProvider(providerID)
		if err != nil {
			continue
		}
		for modelID, info := range modelsMap {
			result = append(result, extensions.ModelInfoEntry{
				Provider:     providerID,
				ModelID:      modelID,
				Name:         info.Name,
				ContextLimit: info.Limit.Context,
				OutputLimit:  info.Limit.Output,
				Reasoning:    info.Reasoning,
			})
		}
	}
	return result
}

// ReloadExtensions hot-reloads all extensions from disk. Event handlers,
// commands, renderers, shortcuts, and extension-defined tools all update
// immediately.
func (m *Kit) ReloadExtensions() error {
	if m.extRunner == nil {
		return fmt.Errorf("no extensions loaded")
	}

	// Emit shutdown to old extensions.
	if m.extRunner.HasHandlers(extensions.SessionShutdown) {
		_, _ = m.extRunner.Emit(extensions.SessionShutdownEvent{})
	}

	// Re-load from disk.
	extraPaths := viper.GetStringSlice("extension")
	loaded, err := extensions.LoadExtensions(extraPaths)
	if err != nil {
		return fmt.Errorf("reloading extensions: %w", err)
	}

	// Swap extensions on the runner (clears dynamic state).
	m.extRunner.Reload(loaded)

	// Update extension tools on the agent so the LLM sees changes.
	if m.agent != nil {
		extTools := extensions.ExtensionToolsAsFantasy(m.extRunner.RegisteredTools(), m.extRunner)
		m.agent.SetExtraTools(extTools)
	}

	// Re-set context and emit SessionStart.
	ctx := m.extRunner.GetContext()
	m.extRunner.SetContext(ctx)
	if m.extRunner.HasHandlers(extensions.SessionStart) {
		_, _ = m.extRunner.Emit(extensions.SessionStartEvent{SessionID: ctx.SessionID})
	}

	return nil
}

// ExecuteCompletion makes a standalone LLM completion call for extensions.
// When req.Model is empty the current agent model is reused (no provider
// creation overhead). When req.Model is set a temporary provider is created,
// used, and closed.
func (m *Kit) ExecuteCompletion(ctx context.Context, req extensions.CompleteRequest) (extensions.CompleteResponse, error) {
	var (
		llmModel    fantasy.LanguageModel
		closer      func()
		usedModel   string
		providerOps fantasy.ProviderOptions
	)

	if req.Model == "" {
		// Reuse the active agent's model.
		llmModel = m.agent.GetModel()
		usedModel = m.modelString
		closer = func() {} // nothing to clean up
	} else {
		// Create a temporary provider for the requested model.
		config := &models.ProviderConfig{
			ModelString:   req.Model,
			TLSSkipVerify: viper.GetBool("tls-skip-verify"),
		}
		if req.MaxTokens > 0 {
			config.MaxTokens = req.MaxTokens
		}
		providerResult, err := models.CreateProvider(ctx, config)
		if err != nil {
			return extensions.CompleteResponse{}, fmt.Errorf("create provider for %q: %w", req.Model, err)
		}
		llmModel = providerResult.Model
		usedModel = req.Model
		providerOps = providerResult.ProviderOptions
		closer = func() {
			if providerResult.Closer != nil {
				_ = providerResult.Closer.Close()
			}
		}
	}
	defer closer()

	// Build agent options (no tools — just a simple completion).
	var agentOpts []fantasy.AgentOption
	if req.System != "" {
		agentOpts = append(agentOpts, fantasy.WithSystemPrompt(req.System))
	}
	if req.MaxTokens > 0 {
		agentOpts = append(agentOpts, fantasy.WithMaxOutputTokens(int64(req.MaxTokens)))
	}
	if providerOps != nil {
		agentOpts = append(agentOpts, fantasy.WithProviderOptions(providerOps))
	}

	completionAgent := fantasy.NewAgent(llmModel, agentOpts...)

	// Convert extension SessionMessage history to LLM message slice.
	var messages []fantasy.Message
	for _, sm := range req.Messages {
		messages = append(messages, fantasy.Message{
			Role: fantasy.MessageRole(sm.Role),
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: sm.Content},
			},
		})
	}

	// Streaming path.
	if req.OnChunk != nil {
		result, err := completionAgent.Stream(ctx, fantasy.AgentStreamCall{
			Prompt:   req.Prompt,
			Messages: messages,
			OnTextDelta: func(_, text string) error {
				req.OnChunk(text)
				return nil
			},
		})
		if err != nil {
			return extensions.CompleteResponse{}, fmt.Errorf("streaming completion: %w", err)
		}
		return extensions.CompleteResponse{
			Text:         result.Response.Content.Text(),
			InputTokens:  int(result.Response.Usage.InputTokens),
			OutputTokens: int(result.Response.Usage.OutputTokens),
			Model:        usedModel,
		}, nil
	}

	// Non-streaming path.
	result, err := completionAgent.Generate(ctx, fantasy.AgentCall{
		Prompt:   req.Prompt,
		Messages: messages,
	})
	if err != nil {
		return extensions.CompleteResponse{}, fmt.Errorf("completion: %w", err)
	}
	return extensions.CompleteResponse{
		Text:         result.Response.Content.Text(),
		InputTokens:  int(result.Response.Usage.InputTokens),
		OutputTokens: int(result.Response.Usage.OutputTokens),
		Model:        usedModel,
	}, nil
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
	ExtraTools   []Tool // Additional tools added alongside core/MCP/extension tools.

	// SkipConfig, when true, skips loading .kit.yml configuration files.
	// Viper defaults (setSDKDefaults) and environment variables (KIT_*)
	// are still applied. Use this for fully programmatic configuration.
	SkipConfig bool

	// DisableCoreTools, when true, prevents loading any core tools.
	// Use with Tools or ExtraTools to provide only custom tools.
	// If both DisableCoreTools is true and Tools is empty, the agent
	// will have no tools (useful for simple chat completions).
	DisableCoreTools bool

	// Session configuration
	SessionDir  string // Base directory for session discovery (default: cwd)
	SessionPath string // Open a specific session file by path
	Continue    bool   // Continue the most recent session for SessionDir
	NoSession   bool   // Ephemeral mode — in-memory session, no persistence

	// Skills
	Skills    []string // Explicit skill files/dirs to load (empty = auto-discover)
	SkillsDir string   // Override default project-local skills directory

	// Compaction
	AutoCompact       bool               // Auto-compact when near context limit
	CompactionOptions *CompactionOptions // Config for auto-compaction (nil = defaults)

	// Debug enables debug logging for the SDK.
	Debug bool

	// MCPAuthHandler handles OAuth authorization for remote MCP servers.
	// When set, remote transports (streamable HTTP, SSE) are configured with
	// OAuth support. If the server returns a 401, the handler is invoked to
	// let the user authorize via browser.
	//
	// If nil, a [DefaultMCPAuthHandler] is created automatically — opening the
	// system browser and listening on a local callback server.
	//
	// Set to a custom implementation to control the authorization UX (e.g.
	// display a URL in a custom UI, redirect to a web app, etc.).
	MCPAuthHandler MCPAuthHandler

	// OnMCPServerLoaded, if non-nil, is called when each MCP server finishes
	// loading during Kit initialization. The callback receives the server name,
	// tool count, and any error. Called from a background goroutine; safe to
	// call app.NotifyMCPServerLoaded() from within the callback to display
	// real-time progress in the TUI.
	OnMCPServerLoaded func(serverName string, toolCount int, err error)

	// CLI is optional CLI-specific configuration. SDK users leave this nil.
	CLI *CLIOptions
}

// CLIOptions holds fields only relevant to the CLI binary. SDK users should
// not need these; they are separated to keep the main Options struct clean.
type CLIOptions struct {
	// MCPConfig is a pre-loaded MCP config. When set, LoadAndValidateConfig
	// is skipped during Kit creation.
	MCPConfig *config.Config
	// ShowSpinner shows a loading spinner for Ollama models.
	ShowSpinner bool
	// SpinnerFunc provides the spinner implementation (nil = no spinner).
	SpinnerFunc SpinnerFunc
	// UseBufferedLogger buffers debug messages for later display.
	UseBufferedLogger bool
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
// viperInitMu serializes viper writes during kit.New(). Viper's global state
// is not thread-safe, so concurrent calls (e.g. parallel subagent spawns)
// must not overlap the Set()/Get() window.
var viperInitMu sync.Mutex

func New(ctx context.Context, opts *Options) (*Kit, error) {
	if opts == nil {
		opts = &Options{}
	}

	// All viper writes (SetSDKDefaults, InitConfig, Set calls, system-prompt
	// composition) happen under viperInitMu. We also call BuildProviderConfig
	// here — it's fast (just reads) — so we can capture the full config
	// snapshot before releasing the lock. The expensive work (MCP loading,
	// provider creation, session init) then runs outside the lock, allowing
	// parallel subagent spawns to proceed concurrently.
	var (
		providerConfig *models.ProviderConfig
		modelString    string
		cwd            string
		contextFiles   []*ContextFile
		loadedSkills   []*Skill
		mcpConfig      *config.Config
		debug          bool
		noExtensions   bool
		maxSteps       int
		streaming      bool
	)

	if err := func() error {
		viperInitMu.Lock()
		defer viperInitMu.Unlock()

		// Set CLI-equivalent defaults for viper. When used as an SDK (without
		// cobra), these defaults are not registered via flag bindings.
		setSDKDefaults()

		// Initialize config (loads config files and env vars).
		// Only initialize if not already done (e.g., by CLI's cobra.OnInitialize).
		// Check if model is already set, which indicates config was loaded.
		// SkipConfig bypasses .kit.yml file loading (viper defaults and env vars still apply).
		if !opts.SkipConfig && viper.GetString("model") == "" {
			if err := InitConfig(opts.ConfigFile, false); err != nil {
				return fmt.Errorf("failed to initialize config: %w", err)
			}
		}

		// Handle CLI debug mode.
		if opts.Debug {
			viper.Set("debug", true)
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

		// Resolve working directory for context/skill discovery.
		cwd = opts.SessionDir
		if cwd == "" {
			cwd, _ = os.Getwd()
		}

		// Load context files (AGENTS.md) from the project root.
		contextFiles = loadContextFiles(cwd)

		// Load skills — either from explicit paths or via auto-discovery.
		var err error
		loadedSkills, err = loadSkills(opts)
		if err != nil {
			return fmt.Errorf("failed to load skills: %w", err)
		}

		// Always compose the system prompt with runtime context: base prompt +
		// AGENTS.md context + skills metadata + date/cwd.
		{
			basePrompt := viper.GetString("system-prompt")
			pb := skills.NewPromptBuilder(basePrompt)

			// Inject AGENTS.md content as project context.
			for _, cf := range contextFiles {
				pb.WithSection("", fmt.Sprintf("Instructions from: %s\n\n%s", cf.Path, cf.Content))
			}

			// Inject skills metadata (name + description + location).
			if len(loadedSkills) > 0 {
				pb.WithSkills(loadedSkills)
			}

			// Append current date/time and working directory.
			pb.WithSection("", fmt.Sprintf(
				"Current date and time: %s\nCurrent working directory: %s",
				time.Now().Format("Monday, January 2, 2006, 3:04:05 PM MST"), cwd,
			))

			viper.Set("system-prompt", pb.Build())
		}

		// Snapshot all viper-derived values now, while the lock is held.
		// BuildProviderConfig is fast (pure reads), so we do it here.
		var pcErr error
		providerConfig, _, pcErr = kitsetup.BuildProviderConfig()
		if pcErr != nil {
			return fmt.Errorf("failed to build provider config: %w", pcErr)
		}
		modelString = viper.GetString("model")
		debug = viper.GetBool("debug")
		noExtensions = viper.GetBool("no-extensions")
		maxSteps = viper.GetInt("max-steps")
		streaming = viper.GetBool("stream")

		return nil
	}(); err != nil {
		return nil, err
	}
	// ---- viperInitMu released — heavy I/O below runs concurrently ----

	// Load MCP configuration. Use pre-loaded config if provided via CLI options.
	if opts.CLI != nil && opts.CLI.MCPConfig != nil {
		mcpConfig = opts.CLI.MCPConfig
	}
	if mcpConfig == nil {
		var err error
		mcpConfig, err = config.LoadAndValidateConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load MCP config: %w", err)
		}
	}

	// Pre-create hook registries so the tool wrapper can reference them.
	// Hooks registered after New() returns are still invoked because the
	// wrapper captures the registries by pointer.
	beforeToolCall := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	afterToolResult := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()
	beforeTurn := newHookRegistry[BeforeTurnHook, BeforeTurnResult]()
	afterTurn := newHookRegistry[AfterTurnHook, AfterTurnResult]()
	contextPrepare := newHookRegistry[ContextPrepareHook, ContextPrepareResult]()
	beforeCompact := newHookRegistry[BeforeCompactHook, BeforeCompactResult]()

	// Build agent setup options, pulling CLI-specific fields when available.
	// Pass the pre-built ProviderConfig and scalar viper snapshots so
	// SetupAgent doesn't need to re-read viper (which would require the lock).
	setupOpts := kitsetup.AgentSetupOptions{
		MCPConfig:         mcpConfig,
		Quiet:             opts.Quiet,
		CoreTools:         opts.Tools,
		DisableCoreTools:  opts.DisableCoreTools,
		ExtraTools:        opts.ExtraTools,
		ToolWrapper:       hookToolWrapper(beforeToolCall, afterToolResult),
		ProviderConfig:    providerConfig,
		Debug:             debug,
		NoExtensions:      noExtensions,
		MaxSteps:          maxSteps,
		StreamingEnabled:  streaming,
		OnMCPServerLoaded: opts.OnMCPServerLoaded,
	}

	// Set up OAuth handler for remote MCP servers.
	// The SDK MCPAuthHandler interface is structurally identical to
	// tools.MCPAuthHandler, so any implementation satisfies both.
	if opts.MCPAuthHandler != nil {
		setupOpts.AuthHandler = opts.MCPAuthHandler
	} else {
		// Create a default handler that opens the system browser.
		defaultHandler, authErr := NewDefaultMCPAuthHandler()
		if authErr != nil {
			// Non-fatal: OAuth just won't be available for remote servers.
			charmlog.Warn("Failed to create OAuth handler; remote MCP servers requiring auth will fail", "error", authErr)
		} else {
			setupOpts.AuthHandler = defaultHandler
		}
	}

	if opts.CLI != nil {
		setupOpts.ShowSpinner = opts.CLI.ShowSpinner
		setupOpts.SpinnerFunc = opts.CLI.SpinnerFunc
		setupOpts.UseBufferedLogger = opts.CLI.UseBufferedLogger
	}

	// Create agent using shared setup with the hook tool wrapper.
	agentResult, err := kitsetup.SetupAgent(ctx, setupOpts)
	if err != nil {
		return nil, err
	}

	// Initialize tree session.
	treeSession, err := InitTreeSession(opts)
	if err != nil {
		_ = agentResult.Agent.Close()
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}

	k := &Kit{
		agent:           agentResult.Agent,
		treeSession:     treeSession,
		modelString:     modelString,
		events:          newEventBus(),
		autoCompact:     opts.AutoCompact,
		compactionOpts:  opts.CompactionOptions,
		contextFiles:    contextFiles,
		skills:          loadedSkills,
		extRunner:       agentResult.ExtRunner,
		bufferedLogger:  agentResult.BufferedLogger,
		authHandler:     setupOpts.AuthHandler,
		opts:            opts,
		beforeToolCall:  beforeToolCall,
		afterToolResult: afterToolResult,
		beforeTurn:      beforeTurn,
		afterTurn:       afterTurn,
		contextPrepare:  contextPrepare,
		beforeCompact:   beforeCompact,
	}

	// Bridge extension events to SDK hooks.
	if agentResult.ExtRunner != nil {
		k.bridgeExtensions(agentResult.ExtRunner)

		// Initialize extension context with minimal defaults. SDK users can call
		// Extensions().SetContext to override with richer implementations (TUI callbacks,
		// prompts, etc.). This ensures extensions never crash on nil function fields.
		k.Extensions().SetContext(extensions.Context{
			CWD:         cwd,
			Model:       k.modelString,
			Interactive: false, // SDK mode defaults to non-interactive
		})
	}

	return k, nil
}

// GetContextFiles returns the context files (e.g. AGENTS.md) loaded during
// initialisation. Returns nil if no context files were found.
func (m *Kit) GetContextFiles() []*ContextFile {
	return m.contextFiles
}

// GetSkills returns the skills loaded during initialisation.
func (m *Kit) GetSkills() []*Skill {
	return m.skills
}

// ---------------------------------------------------------------------------
// Context file loading
// ---------------------------------------------------------------------------

// loadContextFiles discovers and loads project context files (AGENTS.md) from
// the working directory. Returns nil if no context file is found.
func loadContextFiles(cwd string) []*ContextFile {
	path := filepath.Join(cwd, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return []*ContextFile{{
		Path:    path,
		Content: strings.TrimSpace(string(data)),
	}}
}

// ---------------------------------------------------------------------------
// Skill command expansion
// ---------------------------------------------------------------------------

// expandSkillCommand checks whether prompt starts with "/skill:<name>" and, if
// so, re-reads the skill file, strips its YAML frontmatter, wraps the body in
// a <skill> block with baseDir metadata, and appends any trailing user args.
// Returns the original text unchanged when the prefix is absent or the skill is
// not found.
func (m *Kit) expandSkillCommand(prompt string) string {
	if !strings.HasPrefix(prompt, "/skill:") {
		return prompt
	}

	// Parse: /skill:name [args]
	rest := prompt[len("/skill:"):]
	name, args, _ := strings.Cut(rest, " ")
	name = strings.TrimSpace(name)
	if name == "" {
		return prompt
	}

	// Find the skill by name.
	var skillPath string
	for _, s := range m.skills {
		if s.Name == name {
			skillPath = s.Path
			break
		}
	}
	if skillPath == "" {
		return prompt
	}

	// Re-read the file for freshness (user may have edited it since startup).
	loaded, err := skills.LoadSkill(skillPath)
	if err != nil {
		return prompt
	}

	baseDir := filepath.Dir(loaded.Path)
	var buf strings.Builder
	fmt.Fprintf(&buf, "<skill name=%q location=%q>\n", loaded.Name, loaded.Path)
	fmt.Fprintf(&buf, "References are relative to %s.\n\n", baseDir)
	buf.WriteString(loaded.Content)
	buf.WriteString("\n</skill>")

	args = strings.TrimSpace(args)
	if args != "" {
		buf.WriteString("\n\n")
		buf.WriteString(args)
	}

	return buf.String()
}

// ---------------------------------------------------------------------------
// Skills loading
// ---------------------------------------------------------------------------

// loadSkills loads skills based on Options. If explicit paths are provided
// they are loaded directly; otherwise auto-discovery runs.
func loadSkills(opts *Options) ([]*skills.Skill, error) {
	if len(opts.Skills) > 0 {
		return loadExplicitSkills(opts.Skills)
	}

	// Auto-discover from standard directories.
	cwd := opts.SkillsDir
	if cwd == "" {
		cwd = opts.SessionDir
	}
	return skills.LoadSkills(cwd)
}

// loadExplicitSkills loads skills from a list of explicit paths. Each path
// can be a file or a directory.
func loadExplicitSkills(paths []string) ([]*skills.Skill, error) {
	seen := make(map[string]bool)
	var all []*skills.Skill

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("skill path %s: %w", p, err)
		}

		if info.IsDir() {
			dirSkills, err := skills.LoadSkillsFromDir(p)
			if err != nil {
				return nil, err
			}
			for _, s := range dirSkills {
				if !seen[s.Path] {
					seen[s.Path] = true
					all = append(all, s)
				}
			}
		} else {
			abs, _ := filepath.Abs(p)
			if !seen[abs] {
				seen[abs] = true
				s, err := skills.LoadSkill(p)
				if err != nil {
					return nil, err
				}
				all = append(all, s)
			}
		}
	}

	return all, nil
}

// ---------------------------------------------------------------------------
// TurnResult
// ---------------------------------------------------------------------------

// TurnResult contains the full result of a prompt turn, including usage
// statistics and the updated conversation. Use PromptResult() instead of
// Prompt() when you need access to this data.
type TurnResult struct {
	// Response is the assistant's final text response.
	Response string

	// StopReason indicates why the turn ended. Derived from the LLM
	// provider's finish reason: "stop", "length" (max tokens), "tool-calls",
	// "content-filter", "error", "other", "unknown".
	StopReason string

	// SessionID is the UUID of the session this turn belongs to.
	SessionID string

	// TotalUsage is the aggregate token usage across all steps in the turn
	// (includes tool-calling loop iterations). Nil if the provider didn't
	// report usage.
	TotalUsage *LLMUsage

	// FinalUsage is the token usage from the last API call only. Use this
	// for context window fill estimation (InputTokens + OutputTokens ≈
	// current context size). Nil if unavailable.
	FinalUsage *LLMUsage

	// Messages is the full updated conversation after the turn, including
	// any tool call/result messages added during the agent loop.
	// Each message carries role and plain-text content.
	Messages []LLMMessage
}

// ---------------------------------------------------------------------------
// In-process subagent
// ---------------------------------------------------------------------------

// SubagentConfig configures an in-process subagent spawned via Kit.Subagent().
type SubagentConfig struct {
	// Prompt is the task/instruction for the subagent (required).
	Prompt string

	// Model overrides the parent's model (e.g. "anthropic/claude-haiku-3-5-20241022").
	// Empty string uses the parent's current model.
	Model string

	// SystemPrompt provides domain-specific instructions for the subagent.
	// Empty string uses a minimal default prompt.
	SystemPrompt string

	// Tools overrides the tool set. If nil, SubagentTools() is used (all
	// core tools except subagent, preventing infinite recursion).
	Tools []Tool

	// NoSession, when true, uses an in-memory ephemeral session. When false
	// (default), the subagent's session is persisted and can be loaded for
	// replay/inspection.
	NoSession bool

	// Timeout limits execution time. Zero means 5 minute default.
	Timeout time.Duration

	// OnEvent, when set, receives all events from the subagent's event bus.
	// This enables the parent to stream subagent tool calls, text chunks,
	// etc. in real time.
	OnEvent func(Event)
}

// SubagentResult contains the outcome of an in-process subagent execution.
// Errors are returned as the error return value of Subagent(), not in this struct.
type SubagentResult struct {
	// Response is the subagent's final text response.
	Response string
	// SessionID is the subagent's session identifier (for replay).
	SessionID string
	// StopReason is the LLM's finish reason for the subagent's final turn.
	StopReason string
	// Usage contains token usage from the subagent's run.
	Usage *LLMUsage
	// Elapsed is the total execution time.
	Elapsed time.Duration
}

// Subagent spawns an in-process child Kit instance to perform a task. The
// child gets its own session, event bus, and agent loop but shares the
// parent's config (API keys, provider settings) and defaults to the parent's
// model when SubagentConfig.Model is empty.
//
// This is the recommended way to run subagents in the SDK — no subprocess,
// no kit binary dependency, native Go types for results.
func (m *Kit) Subagent(ctx context.Context, cfg SubagentConfig) (*SubagentResult, error) {
	if cfg.Prompt == "" {
		return nil, fmt.Errorf("subagent prompt is required")
	}

	start := time.Now()

	// Default timeout.
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	// Pre-flight check: if the incoming context is already dead, don't
	// waste time attempting init. This catches the case where the parent
	// generation loop's context was cancelled (e.g. user ESC, step cancel)
	// between when the LLM requested the subagent tool and when this code
	// runs. We replace it with a fresh context carrying only the timeout,
	// since the subagent should be independently bounded.
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Resolve model: fall back to parent's model, and inherit the parent's
	// provider when only a bare model name is given (e.g. "claude-haiku"
	// instead of "anthropic/claude-haiku"). This avoids provider guessing.
	model := cfg.Model
	if model == "" {
		model = m.modelString
	} else if !strings.Contains(model, "/") {
		// Bare model name — prepend parent's provider.
		if parts := strings.SplitN(m.modelString, "/", 2); len(parts) == 2 {
			model = parts[0] + "/" + model
		}
	}

	// Early validation: check model format and provider before doing any
	// expensive work (MCP init, system prompt composition, etc.). This
	// gives the calling agent immediate feedback it can act on — e.g.
	// correcting a typo — instead of waiting for a full Kit.New() cycle
	// that silently falls back to the parent model.
	if model != m.modelString {
		if err := models.GetGlobalRegistry().ValidateModelString(model); err != nil {
			return nil, fmt.Errorf("invalid subagent model %q: %w", model, err)
		}
	}

	// Default system prompt.
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful coding assistant. Complete the task efficiently and thoroughly."
	}

	// Default tools: everything except subagent.
	tools := cfg.Tools
	if tools == nil {
		tools = SubagentTools()
	}

	// Create child Kit instance.
	childOpts := &Options{
		Model:        model,
		SystemPrompt: systemPrompt,
		Tools:        tools,
		NoSession:    cfg.NoSession,
		Quiet:        true,
	}
	child, err := New(ctx, childOpts)
	if err != nil {
		return &SubagentResult{Elapsed: time.Since(start)}, fmt.Errorf("failed to create subagent: %w", err)
	}
	defer func() { _ = child.Close() }()

	// Forward events to parent if requested.
	if cfg.OnEvent != nil {
		child.Subscribe(cfg.OnEvent)
	}

	// Run the prompt.
	result, err := child.PromptResult(ctx, cfg.Prompt)
	elapsed := time.Since(start)

	if err != nil {
		return &SubagentResult{Elapsed: elapsed}, err
	}

	subResult := &SubagentResult{
		Response:   result.Response,
		SessionID:  child.GetSessionID(),
		StopReason: result.StopReason,
		Elapsed:    elapsed,
	}
	if result.TotalUsage != nil {
		subResult.Usage = result.TotalUsage
	}

	return subResult, nil
}

// ---------------------------------------------------------------------------
// Shared generation helpers
// ---------------------------------------------------------------------------

// generate calls the agent's generation loop with event-emitting handlers.
// All prompt modes (Prompt, Steer, FollowUp, PromptWithOptions) share this
// single code path so callback wiring is never duplicated.
func (m *Kit) generate(ctx context.Context, messages []fantasy.Message) (*agent.GenerateWithLoopResult, error) {
	// Create a per-turn steer channel and attach it to the context so the
	// agent's PrepareStep can inject steering messages between steps.
	steerCh := make(chan agent.SteerMessage, 16)
	m.steerMu.Lock()
	m.steerCh = steerCh
	m.steerMu.Unlock()
	defer func() {
		// Drain any unconsumed steer messages before nilling the channel.
		// These are stored in leftoverSteer so DrainSteer() can return them.
		var leftover []agent.SteerMessage
		for {
			select {
			case msg := <-steerCh:
				leftover = append(leftover, msg)
			default:
				m.steerMu.Lock()
				m.steerCh = nil
				m.leftoverSteer = leftover
				m.steerMu.Unlock()
				return
			}
		}
	}()
	ctx = agent.ContextWithSteerCh(ctx, steerCh)
	ctx = agent.ContextWithSteerConsumed(ctx, func(count int) {
		m.events.emit(SteerConsumedEvent{Count: count})
	})

	// Inject the in-process subagent spawner into the context so the
	// subagent core tool can create child Kit instances without
	// importing pkg/kit (which would create an import cycle).
	ctx = core.WithSubagentSpawner(ctx, func(
		spawnCtx context.Context, toolCallID, prompt, model, systemPrompt string, timeout time.Duration,
	) (*core.SubagentSpawnResult, error) {
		// Build OnEvent: dispatch to per-tool-call listeners if any are
		// registered via SubscribeSubagent(). Listeners are cleaned up
		// after the subagent completes.
		var onEvent func(Event)
		if listeners := m.getSubagentListenerSet(toolCallID); listeners != nil {
			onEvent = listeners.emit
		}
		result, err := m.Subagent(spawnCtx, SubagentConfig{
			Prompt:       prompt,
			Model:        model,
			SystemPrompt: systemPrompt,
			Timeout:      timeout,
			OnEvent:      onEvent,
		})
		m.cleanupSubagentListeners(toolCallID)
		if result == nil {
			return &core.SubagentSpawnResult{Error: err}, err
		}
		sr := &core.SubagentSpawnResult{
			Response:  result.Response,
			Error:     err,
			SessionID: result.SessionID,
			Elapsed:   result.Elapsed,
		}
		if result.Usage != nil {
			sr.InputTokens = result.Usage.InputTokens
			sr.OutputTokens = result.Usage.OutputTokens
		}
		return sr, err
	})

	return m.agent.GenerateWithLoopAndStreaming(ctx, messages,
		func(toolCallID, toolName, toolArgs string) {
			m.events.emit(ToolCallEvent{
				ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName),
				ToolArgs: toolArgs, ParsedArgs: parseToolArgs(toolArgs),
			})
		},
		func(toolCallID, toolName, toolArgs string, isStarting bool) {
			if isStarting {
				m.events.emit(ToolExecutionStartEvent{ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName), ToolArgs: toolArgs})
			} else {
				m.events.emit(ToolExecutionEndEvent{ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName)})
			}
		},
		func(toolCallID, toolName, toolArgs, resultText, metadata string, isError bool) {
			evt := ToolResultEvent{
				ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName),
				ToolArgs: toolArgs, ParsedArgs: parseToolArgs(toolArgs),
				Result: resultText, IsError: isError,
			}
			if metadata != "" {
				var meta ToolResultMetadata
				if err := json.Unmarshal([]byte(metadata), &meta); err == nil {
					evt.Metadata = &meta
				}
			}
			m.events.emit(evt)
		},
		func(content string) {
			m.events.emit(ResponseEvent{Content: content})
		},
		func(content string) {
			m.events.emit(ToolCallContentEvent{Content: content})
		},
		// <think> tag filtering: models like Qwen/DeepSeek wrap reasoning inside
		// <think>...</think> tags in the regular text stream. We intercept those
		// spans here and re-route them as ReasoningDeltaEvent/ReasoningCompleteEvent
		// so callers always receive clean, tag-free text and structured reasoning.
		func() func(chunk string) {
			const (
				thinkOpen  = "<think>"
				thinkClose = "</think>"
			)
			var inThinkTag bool
			return func(chunk string) {
				remaining := chunk
				for remaining != "" {
					if inThinkTag {
						i := strings.Index(remaining, thinkClose)
						if i == -1 {
							m.events.emit(ReasoningDeltaEvent{Delta: remaining})
							return
						}
						if i > 0 {
							m.events.emit(ReasoningDeltaEvent{Delta: remaining[:i]})
						}
						inThinkTag = false
						m.events.emit(ReasoningCompleteEvent{})
						remaining = remaining[i+len(thinkClose):]
					} else {
						i := strings.Index(remaining, thinkOpen)
						if i == -1 {
							m.events.emit(MessageUpdateEvent{Chunk: remaining})
							return
						}
						if i > 0 {
							m.events.emit(MessageUpdateEvent{Chunk: remaining[:i]})
						}
						inThinkTag = true
						remaining = remaining[i+len(thinkOpen):]
					}
				}
			}
		}(),
		func(delta string) {
			m.events.emit(ReasoningDeltaEvent{Delta: delta})
		},
		func() {
			m.events.emit(ReasoningCompleteEvent{})
		},
		func(toolCallID, toolName, chunk string, isStderr bool) {
			// Emit tool output chunk event for streaming bash output
			m.events.emit(ToolOutputEvent{
				ToolCallID: toolCallID,
				ToolName:   toolName,
				Chunk:      chunk,
				IsStderr:   isStderr,
			})
		},
		func(inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int64) {
			// Emit step usage event for real-time cost tracking
			if viper.GetBool("debug") {
				charmlog.Debug("Kit.generate emitting StepUsageEvent",
					"input", inputTokens,
					"output", outputTokens,
					"cacheRead", cacheReadTokens,
					"cacheCreate", cacheCreationTokens,
				)
			}
			m.events.emit(StepUsageEvent{
				InputTokens:      uint64(inputTokens),
				OutputTokens:     uint64(outputTokens),
				CacheReadTokens:  uint64(cacheReadTokens),
				CacheWriteTokens: uint64(cacheCreationTokens),
			})
		},
	)
}

// runTurn is the shared lifecycle for every prompt mode:
//  1. Run BeforeTurn hooks (can modify prompt, inject messages).
//  2. Persist pre-generation messages to the tree session.
//  3. Build context from the tree (walks leaf-to-root for current branch).
//  4. Emit turn/message start events.
//  5. Run generation.
//  6. Emit turn/message end events.
//  7. Persist post-generation messages (tool calls, results, assistant).
//  8. Run AfterTurn hooks.
//
// promptLabel is the human-readable label emitted in TurnStartEvent.Prompt.
// prompt is the raw user text passed to BeforeTurn hooks.
func (m *Kit) runTurn(ctx context.Context, promptLabel string, prompt string, preMessages []fantasy.Message) (*TurnResult, error) {
	// Expand /skill:name commands — reads the skill file, wraps it in a
	// <skill> block, and appends any trailing user args.
	if expanded := m.expandSkillCommand(prompt); expanded != prompt {
		prompt = expanded
		// Replace the last user message in preMessages with the expanded text,
		// preserving any file parts (e.g. clipboard images).
		for i := len(preMessages) - 1; i >= 0; i-- {
			if preMessages[i].Role == fantasy.MessageRoleUser {
				files := extractFileParts(preMessages[i])
				preMessages[i] = fantasy.NewUserMessage(expanded, files...)
				break
			}
		}
	}

	// Run BeforeTurn hooks — can modify the prompt, inject system/context messages.
	if hookResult := m.beforeTurn.run(BeforeTurnHook{Prompt: prompt}); hookResult != nil {
		// Override prompt text in the last user message, preserving
		// any file parts (e.g. clipboard images).
		if hookResult.Prompt != nil {
			for i := len(preMessages) - 1; i >= 0; i-- {
				if preMessages[i].Role == fantasy.MessageRoleUser {
					files := extractFileParts(preMessages[i])
					preMessages[i] = fantasy.NewUserMessage(*hookResult.Prompt, files...)
					break
				}
			}
		}
		// Inject messages before the original preMessages.
		var injected []fantasy.Message
		if hookResult.SystemPrompt != nil {
			injected = append(injected, fantasy.NewSystemMessage(*hookResult.SystemPrompt))
		}
		if hookResult.InjectText != nil {
			injected = append(injected, fantasy.NewUserMessage(*hookResult.InjectText))
		}
		if len(injected) > 0 {
			preMessages = append(injected, preMessages...)
		}
	}

	// Persist pre-generation messages to tree session.
	for _, msg := range preMessages {
		_, _ = m.treeSession.AppendLLMMessage(msg)
	}

	// Auto-compact if enabled and conversation is near the context limit.
	if m.autoCompact && m.ShouldCompact() {
		_, _ = m.compactInternal(ctx, m.compactionOpts, "", true) // best-effort, automatic
	}

	// Build context from the tree so only the current branch is sent.
	messages := m.treeSession.GetLLMMessages()

	// Run ContextPrepare hooks — extensions can filter, reorder, or inject messages.
	if hookResult := m.contextPrepare.run(ContextPrepareHook{Messages: messages}); hookResult != nil && hookResult.Messages != nil {
		messages = hookResult.Messages
	}

	sentCount := len(messages)

	m.events.emit(TurnStartEvent{Prompt: promptLabel})
	m.events.emit(MessageStartEvent{})

	result, err := m.generate(ctx, messages)
	if err != nil {
		// Persist any messages from completed steps (tool call/result
		// pairs) so partial progress is not lost. The agent layer only
		// includes fully-paired tool_use + tool_result messages in
		// completedStepMessages, so there are no orphaned entries that
		// would break subsequent API requests. The user message and any
		// completed work remain in the session; only the in-progress
		// (pending) message or tool call is discarded.
		if result != nil && len(result.ConversationMessages) > sentCount {
			for _, msg := range result.ConversationMessages[sentCount:] {
				_, _ = m.treeSession.AppendLLMMessage(msg)
			}
		}
		m.events.emit(TurnEndEvent{Error: err})
		// Run AfterTurn hooks even on error.
		m.afterTurn.run(AfterTurnHook{Error: err})
		return nil, err
	}

	responseText := result.FinalResponse.Content.Text()

	// Persist new messages (tool calls, tool results, assistant response)
	// BEFORE emitting events so that extension handlers calling
	// GetContextStats() see up-to-date token counts.
	if len(result.ConversationMessages) > sentCount {
		for _, msg := range result.ConversationMessages[sentCount:] {
			_, _ = m.treeSession.AppendLLMMessage(msg)
		}
	}

	// Store the API-reported token count so GetContextStats() matches the
	// built-in status bar (which uses input + output tokens). The
	// text-based heuristic misses system prompts, tool definitions, etc.
	if result.FinalResponse != nil {
		u := result.FinalResponse.Usage
		m.lastInputTokensMu.Lock()
		m.lastInputTokens = int(u.InputTokens) + int(u.OutputTokens)
		m.lastInputTokensMu.Unlock()
	}

	stopReason := result.StopReason

	m.events.emit(MessageEndEvent{Content: responseText})
	m.events.emit(TurnEndEvent{Response: responseText, StopReason: stopReason})

	// Run AfterTurn hooks.
	m.afterTurn.run(AfterTurnHook{Response: responseText})

	// Build TurnResult with usage stats.
	turnResult := &TurnResult{
		Response:   responseText,
		StopReason: stopReason,
		SessionID:  m.GetSessionID(),
		Messages:   result.ConversationMessages,
	}
	totalUsage := result.TotalUsage
	turnResult.TotalUsage = &totalUsage
	if result.FinalResponse != nil {
		finalUsage := result.FinalResponse.Usage
		turnResult.FinalUsage = &finalUsage
	}

	return turnResult, nil
}

// ---------------------------------------------------------------------------
// Prompt modes
// ---------------------------------------------------------------------------

// Prompt sends a message to the agent and returns the response. The agent may
// use tools as needed to generate the response. The conversation history is
// automatically maintained in the tree session. Lifecycle events are emitted
// to all registered subscribers. Returns an error if generation fails.
func (m *Kit) Prompt(ctx context.Context, message string) (string, error) {
	result, err := m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message),
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// Steer injects a system-level instruction and triggers a new agent turn.
// Use Steer to dynamically adjust agent behavior mid-conversation without a
// visible user message — for example, changing tone, focus, or constraints.
//
// Under the hood, Steer appends a system message (the instruction) followed by
// a synthetic user message so the agent acknowledges and follows the directive.
// Both messages are persisted to the session.
func (m *Kit) Steer(ctx context.Context, instruction string) (string, error) {
	result, err := m.runTurn(ctx, "[steer] "+instruction, instruction, []fantasy.Message{
		fantasy.NewSystemMessage(instruction),
		fantasy.NewUserMessage("Please acknowledge and follow the above instruction."),
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// FollowUp continues the conversation without explicit new user input.
// If text is empty, "Continue." is used as the prompt. Use FollowUp when the
// agent's previous response was truncated or you want the agent to elaborate.
//
// Returns an error if there are no previous messages in the session.
func (m *Kit) FollowUp(ctx context.Context, text string) (string, error) {
	// Verify there is conversation history to follow up on.
	if len(m.treeSession.GetLLMMessages()) == 0 {
		return "", fmt.Errorf("cannot follow up: no previous messages")
	}

	if text == "" {
		text = "Continue."
	}

	result, err := m.runTurn(ctx, "[follow-up]", text, []fantasy.Message{
		fantasy.NewUserMessage(text),
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// InjectSteer sends a steering message into the currently active agent turn.
// The message will be injected as a user message between steps (after the
// current tool execution finishes, before the next LLM call). If no turn is
// active the message is silently dropped — callers should check IsGenerating()
// or use Prompt()/Steer() for idle-state messaging.
//
// InjectSteer is safe to call from any goroutine. Multiple calls queue
// messages in order; all pending steer messages are drained and injected
// together at the next step boundary.
//
// This is the preferred way to redirect an agent mid-turn without cancelling
// in-progress tool execution.
func (m *Kit) InjectSteer(message string) {
	m.InjectSteerWithFiles(message, nil)
}

// InjectSteerWithFiles sends a steering message with optional file attachments
// (e.g. pasted images) into the currently active agent turn. Behaves like
// InjectSteer but includes file parts in the injected user message.
func (m *Kit) InjectSteerWithFiles(message string, files []LLMFilePart) {
	m.steerMu.Lock()
	ch := m.steerCh
	m.steerMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- agent.SteerMessage{Text: message, Files: files}:
	default:
		// Channel full — extremely unlikely with buffer of 16, but don't block.
	}
}

// IsGenerating returns true if an agent turn is currently in progress.
// Use this to decide between InjectSteer (mid-turn) and Prompt (new turn).
func (m *Kit) IsGenerating() bool {
	m.steerMu.Lock()
	defer m.steerMu.Unlock()
	return m.steerCh != nil
}

// DrainSteer removes and returns all unconsumed steer messages. Called after
// a turn completes so the app layer can process any steer messages that
// arrived after the last PrepareStep fired (e.g. during a text-only response
// with no tool calls, or after the agent finished its last step).
func (m *Kit) DrainSteer() []agent.SteerMessage {
	m.steerMu.Lock()
	defer m.steerMu.Unlock()

	// First check leftover messages saved when generate() returned.
	if len(m.leftoverSteer) > 0 {
		msgs := m.leftoverSteer
		m.leftoverSteer = nil
		return msgs
	}

	// If a turn is still active, drain from the live channel.
	if m.steerCh != nil {
		var msgs []agent.SteerMessage
		for {
			select {
			case msg := <-m.steerCh:
				msgs = append(msgs, msg)
			default:
				return msgs
			}
		}
	}

	return nil
}

// PromptOptions configures a single PromptWithOptions call.
type PromptOptions struct {
	// SystemMessage is prepended as a system message before the user prompt.
	// Use it to inject per-call instructions or context without permanently
	// modifying the agent's system prompt.
	SystemMessage string
}

// PromptWithOptions sends a message with per-call configuration. It behaves
// like Prompt but allows injecting an additional system message before the
// user prompt. Both messages are persisted to the session.
func (m *Kit) PromptWithOptions(ctx context.Context, msg string, opts PromptOptions) (string, error) {
	var preMessages []fantasy.Message
	if opts.SystemMessage != "" {
		preMessages = append(preMessages, fantasy.NewSystemMessage(opts.SystemMessage))
	}
	preMessages = append(preMessages, fantasy.NewUserMessage(msg))

	result, err := m.runTurn(ctx, msg, msg, preMessages)
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// PromptResult sends a message and returns the full turn result including
// usage statistics and conversation messages. Use this instead of Prompt()
// when you need more than just the response text.
func (m *Kit) PromptResult(ctx context.Context, message string) (*TurnResult, error) {
	return m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message),
	})
}

// PromptResultWithFiles sends a multimodal message (text + images) and returns
// the full turn result. The files parameter carries binary file data (e.g.
// clipboard images) that are included alongside the text in the user message.
func (m *Kit) PromptResultWithFiles(ctx context.Context, message string, files []LLMFilePart) (*TurnResult, error) {
	return m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message, files...),
	})
}

// PromptResultWithMessages submits multiple user messages in a single turn.
// All messages are persisted to the session and sent to the agent together.
// The agent will respond once to the combined context of all messages.
// Returns the full turn result including usage statistics and conversation messages.
func (m *Kit) PromptResultWithMessages(ctx context.Context, messages []string) (*TurnResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	// Build prompt label from all messages
	promptLabel := strings.Join(messages, " | ")
	if len(promptLabel) > 100 {
		promptLabel = promptLabel[:100] + "..."
	}

	// Build LLM messages from all strings
	var preMessages []fantasy.Message
	for _, msg := range messages {
		preMessages = append(preMessages, fantasy.NewUserMessage(msg))
	}

	return m.runTurn(ctx, promptLabel, messages[len(messages)-1], preMessages)
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

// GetModelInfo returns detailed information about the current model
// (capabilities, pricing, limits). Returns nil if the model is not in the
// registry — this is expected for new models or custom fine-tunes.
func (m *Kit) GetModelInfo() *ModelInfo {
	provider, modelID, err := ParseModelString(m.modelString)
	if err != nil {
		return nil
	}
	return LookupModel(provider, modelID)
}

// IsReasoningModel returns true if the current model supports extended thinking / reasoning.
func (m *Kit) IsReasoningModel() bool {
	info := m.GetModelInfo()
	return info != nil && info.Reasoning
}

// GetThinkingLevel returns the current thinking level.
func (m *Kit) GetThinkingLevel() string {
	return viper.GetString("thinking-level")
}

// SetThinkingLevel changes the thinking level and recreates the agent with
// the new thinking budget. Returns an error if provider recreation fails.
//
// With message-level caching, both thinking and caching work together.
// Caching reduces costs by 60-90% for repeated context.
func (m *Kit) SetThinkingLevel(ctx context.Context, level string) error {
	viper.Set("thinking-level", level)
	// Recreate agent with new thinking config by re-running SetModel
	// with the same model string. SetModel rebuilds the provider and
	// passes the updated viper config (including thinking-level).
	return m.SetModel(ctx, m.modelString)
}

// GetTools returns all tools available to the agent (core + MCP + extensions).
func (m *Kit) GetTools() []Tool {
	return m.agent.GetTools()
}

// extractFileParts returns all FilePart entries from a message's Content.
// Used to preserve image attachments when replacing user message text.
func extractFileParts(msg fantasy.Message) []fantasy.FilePart {
	var files []fantasy.FilePart
	for _, part := range msg.Content {
		if fp, ok := part.(fantasy.FilePart); ok {
			files = append(files, fp)
		}
	}
	return files
}

// Close cleans up resources including MCP server connections, model resources,
// and the tree session file handle. Should be called when the Kit instance is
// no longer needed. Returns an error if cleanup fails.
func (m *Kit) Close() error {
	// Emit SessionShutdown for extensions.
	if m.extRunner != nil && m.extRunner.HasHandlers(extensions.SessionShutdown) {
		_, _ = m.extRunner.Emit(extensions.SessionShutdownEvent{})
	}
	if m.treeSession != nil {
		_ = m.treeSession.Close()
	}
	// Release the OAuth callback port if we own the handler.
	if closer, ok := m.authHandler.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	return m.agent.Close()
}
