package kit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/kitsetup"
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

	// Hook registries — interception layer (see hooks.go).
	beforeToolCall  *hookRegistry[BeforeToolCallHook, BeforeToolCallResult]
	afterToolResult *hookRegistry[AfterToolResultHook, AfterToolResultResult]
	beforeTurn      *hookRegistry[BeforeTurnHook, BeforeTurnResult]
	afterTurn       *hookRegistry[AfterTurnHook, AfterTurnResult]
}

// Subscribe registers an EventListener that will be called for every lifecycle
// event emitted during Prompt() and PromptWithCallbacks(). Returns an
// unsubscribe function that removes the listener.
func (m *Kit) Subscribe(listener EventListener) func() {
	return m.events.subscribe(listener)
}

// GetExtRunner returns the extension runner (nil if extensions are disabled).
//
// Deprecated: Use SetExtensionContext and EmitSessionStart instead. GetExtRunner
// leaks the internal extensions.Runner type across the SDK boundary.
func (m *Kit) GetExtRunner() *extensions.Runner { return m.extRunner }

// GetBufferedLogger returns the buffered debug logger (nil if not configured).
//
// Deprecated: Use GetBufferedDebugMessages instead.
func (m *Kit) GetBufferedLogger() *tools.BufferedDebugLogger { return m.bufferedLogger }

// GetAgent returns the underlying agent.
//
// Deprecated: Use GetToolNames, GetLoadingMessage, GetLoadedServerNames,
// GetMCPToolCount, GetExtensionToolCount instead.
func (m *Kit) GetAgent() *agent.Agent { return m.agent }

// --------------------------------------------------------------------------
// Narrow accessors — prefer these over GetAgent/GetExtRunner/GetBufferedLogger
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
func (m *Kit) GetLoadedServerNames() []string {
	return m.agent.GetLoadedServerNames()
}

// GetMCPToolCount returns the number of tools loaded from external MCP servers.
func (m *Kit) GetMCPToolCount() int {
	return m.agent.GetMCPToolCount()
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

// SetExtensionContext configures the extension runner with the given context
// functions. No-op if extensions are disabled.
func (m *Kit) SetExtensionContext(ctx extensions.Context) {
	if m.extRunner != nil {
		m.extRunner.SetContext(ctx)
	}
}

// EmitSessionStart fires the SessionStart event for extensions.
// No-op if extensions are disabled or no handlers are registered.
func (m *Kit) EmitSessionStart() {
	if m.extRunner != nil && m.extRunner.HasHandlers(extensions.SessionStart) {
		_, _ = m.extRunner.Emit(extensions.SessionStartEvent{})
	}
}

// ExtensionCommands returns the slash commands registered by extensions.
// Returns nil if extensions are disabled or no commands are registered.
func (m *Kit) ExtensionCommands() []extensions.CommandDef {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.RegisteredCommands()
}

// SetExtensionWidget places or updates a persistent extension widget.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) SetExtensionWidget(config extensions.WidgetConfig) {
	if m.extRunner != nil {
		m.extRunner.SetWidget(config)
	}
}

// RemoveExtensionWidget removes a previously placed extension widget by ID.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) RemoveExtensionWidget(id string) {
	if m.extRunner != nil {
		m.extRunner.RemoveWidget(id)
	}
}

// GetExtensionWidgets returns extension widgets matching the given placement.
// Returns nil if extensions are disabled or no widgets match.
func (m *Kit) GetExtensionWidgets(placement extensions.WidgetPlacement) []extensions.WidgetConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetWidgets(placement)
}

// SetExtensionHeader places or replaces the custom header from extensions.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) SetExtensionHeader(config extensions.HeaderFooterConfig) {
	if m.extRunner != nil {
		m.extRunner.SetHeader(config)
	}
}

// RemoveExtensionHeader removes the custom extension header.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) RemoveExtensionHeader() {
	if m.extRunner != nil {
		m.extRunner.RemoveHeader()
	}
}

// GetExtensionHeader returns the current custom header, or nil if none is set.
// Returns nil if extensions are disabled.
func (m *Kit) GetExtensionHeader() *extensions.HeaderFooterConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetHeader()
}

// SetExtensionFooter places or replaces the custom footer from extensions.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) SetExtensionFooter(config extensions.HeaderFooterConfig) {
	if m.extRunner != nil {
		m.extRunner.SetFooter(config)
	}
}

// RemoveExtensionFooter removes the custom extension footer.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) RemoveExtensionFooter() {
	if m.extRunner != nil {
		m.extRunner.RemoveFooter()
	}
}

// GetExtensionFooter returns the current custom footer, or nil if none is set.
// Returns nil if extensions are disabled.
func (m *Kit) GetExtensionFooter() *extensions.HeaderFooterConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetFooter()
}

// GetExtensionToolRenderer returns the custom renderer for the named tool, or
// nil if no extension registered a renderer for it. Returns nil if extensions
// are disabled.
func (m *Kit) GetExtensionToolRenderer(toolName string) *extensions.ToolRenderConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetToolRenderer(toolName)
}

// SetExtensionEditor installs an editor interceptor from extensions.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) SetExtensionEditor(config extensions.EditorConfig) {
	if m.extRunner != nil {
		m.extRunner.SetEditor(config)
	}
}

// ResetExtensionEditor removes the active editor interceptor from extensions.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) ResetExtensionEditor() {
	if m.extRunner != nil {
		m.extRunner.ResetEditor()
	}
}

// GetExtensionEditor returns the current editor interceptor, or nil if none
// is set. Returns nil if extensions are disabled.
func (m *Kit) GetExtensionEditor() *extensions.EditorConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetEditor()
}

// HasExtensions returns true if the extension runner is configured and active.
func (m *Kit) HasExtensions() bool {
	return m.extRunner != nil
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
	cwd := opts.SessionDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Load context files (AGENTS.md) from the project root.
	contextFiles := loadContextFiles(cwd)

	// Load skills — either from explicit paths or via auto-discovery.
	loadedSkills, err := loadSkills(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load skills: %w", err)
	}

	// Always compose the system prompt with runtime context: base prompt +
	// AGENTS.md context + skills metadata + date/cwd. This matches Pi's
	// buildSystemPrompt() convention.
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

	// Load MCP configuration. Use pre-loaded config if provided via CLI options.
	var mcpConfig *config.Config
	if opts.CLI != nil {
		mcpConfig = opts.CLI.MCPConfig
	}
	if mcpConfig == nil {
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

	// Build agent setup options, pulling CLI-specific fields when available.
	setupOpts := kitsetup.AgentSetupOptions{
		MCPConfig:   mcpConfig,
		Quiet:       opts.Quiet,
		CoreTools:   opts.Tools,
		ExtraTools:  opts.ExtraTools,
		ToolWrapper: hookToolWrapper(beforeToolCall, afterToolResult),
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
		modelString:     viper.GetString("model"),
		events:          newEventBus(),
		autoCompact:     opts.AutoCompact,
		compactionOpts:  opts.CompactionOptions,
		contextFiles:    contextFiles,
		skills:          loadedSkills,
		extRunner:       agentResult.ExtRunner,
		bufferedLogger:  agentResult.BufferedLogger,
		beforeToolCall:  beforeToolCall,
		afterToolResult: afterToolResult,
		beforeTurn:      beforeTurn,
		afterTurn:       afterTurn,
	}

	// Bridge extension events to SDK hooks.
	if agentResult.ExtRunner != nil {
		k.bridgeExtensions(agentResult.ExtRunner)
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
// not found. This matches Pi's _expandSkillCommand() convention.
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

	// TotalUsage is the aggregate token usage across all steps in the turn
	// (includes tool-calling loop iterations). Nil if the provider didn't
	// report usage.
	TotalUsage *FantasyUsage

	// FinalUsage is the token usage from the last API call only. Use this
	// for context window fill estimation (InputTokens + OutputTokens ≈
	// current context size). Nil if unavailable.
	FinalUsage *FantasyUsage

	// Messages is the full updated conversation after the turn, including
	// any tool call/result messages added during the agent loop.
	Messages []FantasyMessage
}

// ---------------------------------------------------------------------------
// Shared generation helpers
// ---------------------------------------------------------------------------

// generate calls the agent's generation loop with event-emitting handlers.
// All prompt modes (Prompt, Steer, FollowUp, PromptWithOptions) share this
// single code path so callback wiring is never duplicated.
func (m *Kit) generate(ctx context.Context, messages []fantasy.Message) (*agent.GenerateWithLoopResult, error) {
	return m.agent.GenerateWithLoopAndStreaming(ctx, messages,
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
		// Replace the last user message in preMessages with the expanded text.
		for i := len(preMessages) - 1; i >= 0; i-- {
			if preMessages[i].Role == fantasy.MessageRoleUser {
				preMessages[i] = fantasy.NewUserMessage(expanded)
				break
			}
		}
	}

	// Run BeforeTurn hooks — can modify the prompt, inject system/context messages.
	if m.beforeTurn.hasHooks() {
		if hookResult := m.beforeTurn.run(BeforeTurnHook{Prompt: prompt}); hookResult != nil {
			// Override prompt text in the last user message.
			if hookResult.Prompt != nil {
				for i := len(preMessages) - 1; i >= 0; i-- {
					if preMessages[i].Role == fantasy.MessageRoleUser {
						preMessages[i] = fantasy.NewUserMessage(*hookResult.Prompt)
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
	}

	// Persist pre-generation messages to tree session.
	for _, msg := range preMessages {
		_, _ = m.treeSession.AppendFantasyMessage(msg)
	}

	// Auto-compact if enabled and conversation is near the context limit.
	if m.autoCompact && m.ShouldCompact() {
		_, _ = m.Compact(ctx, m.compactionOpts, "") // best-effort
	}

	// Build context from the tree so only the current branch is sent.
	messages := m.treeSession.GetFantasyMessages()
	sentCount := len(messages)

	m.events.emit(TurnStartEvent{Prompt: promptLabel})
	m.events.emit(MessageStartEvent{})

	result, err := m.generate(ctx, messages)
	if err != nil {
		m.events.emit(TurnEndEvent{Error: err})
		// Run AfterTurn hooks even on error.
		if m.afterTurn.hasHooks() {
			m.afterTurn.run(AfterTurnHook{Error: err})
		}
		return nil, err
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

	// Run AfterTurn hooks.
	if m.afterTurn.hasHooks() {
		m.afterTurn.run(AfterTurnHook{Response: responseText})
	}

	// Build TurnResult with usage stats.
	turnResult := &TurnResult{
		Response: responseText,
		Messages: result.ConversationMessages,
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
	if len(m.treeSession.GetFantasyMessages()) == 0 {
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

// PromptWithCallbacks sends a message with callbacks for monitoring tool
// execution and streaming responses. Lifecycle events are also emitted to all
// registered subscribers (via Subscribe).
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
	// Register temporary subscribers for the inline callbacks.
	var unsubs []func()
	if onToolCall != nil {
		unsubs = append(unsubs, m.OnToolCall(func(e ToolCallEvent) {
			onToolCall(e.ToolName, e.ToolArgs)
		}))
	}
	if onToolResult != nil {
		unsubs = append(unsubs, m.OnToolResult(func(e ToolResultEvent) {
			onToolResult(e.ToolName, e.ToolArgs, e.Result, e.IsError)
		}))
	}
	if onStreaming != nil {
		unsubs = append(unsubs, m.OnStreaming(func(e MessageUpdateEvent) {
			onStreaming(e.Chunk)
		}))
	}
	defer func() {
		for _, unsub := range unsubs {
			unsub()
		}
	}()

	return m.Prompt(ctx, message)
}

// PromptResult sends a message and returns the full turn result including
// usage statistics and conversation messages. Use this instead of Prompt()
// when you need more than just the response text.
func (m *Kit) PromptResult(ctx context.Context, message string) (*TurnResult, error) {
	return m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message),
	})
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

// GetTools returns all tools available to the agent (core + MCP + extensions).
func (m *Kit) GetTools() []Tool {
	return m.agent.GetTools()
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
	return m.agent.Close()
}
