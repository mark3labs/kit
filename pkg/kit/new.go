package kit

import (
	"fmt"
	"os"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/kitsetup"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/skills"
	"github.com/mark3labs/kit/internal/skilltool"
	"github.com/mark3labs/kit/internal/tools"

	"github.com/spf13/viper"
)

// This file holds the private construction phases of [New]. Each phase is a
// step of the original monolithic constructor, extracted without changing
// order or behaviour:
//
//  1. resolveConfig — config snapshot, no heavy I/O:
//     a. newConfigStore       — pick the per-instance or process-global store
//     b. loadConfigStore      — defaults, config file, env vars
//     c. applyOptionOverrides — push Options fields into the store
//     d. loadProjectResources — cwd, AGENTS.md, skills, named agents
//     e. buildSystemPrompt    — base prompt + context + skills + environment
//     f. resolveModelConfig   — provider config and scalar snapshots
//  2. resolveMCPConfig        — MCP servers, including in-process ones
//  3. withSkillTool, buildAgentSetupOptions, then kitsetup.SetupAgent
//  4. initSessionManager
//  5. (*Kit).initExtensions

// resolvedConfig is the snapshot of instance-derived values that [New]
// computes from the configuration store before any heavy I/O happens.
type resolvedConfig struct {
	providerConfig *models.ProviderConfig
	modelString    string

	cwd          string
	contextFiles []*ContextFile
	loadedSkills []*Skill
	namedAgents  []*AgentDefinition

	debug           bool
	noExtensions    bool
	toolList        []string
	maxSteps        int
	streaming       bool
	shellTimeout    int
	shellMaxTimeout int
	shell           []string

	hasCustomSystemPrompt bool
	systemPromptSource    string
	basePrompt            string
}

// hookSet holds the hook registries pre-created by [New] so the tool wrapper
// can reference them before the Kit exists. Hooks registered after New
// returns are still invoked because the wrapper captures them by pointer.
type hookSet struct {
	beforeToolCall  *hookRegistry[BeforeToolCallHook, BeforeToolCallResult]
	afterToolResult *hookRegistry[AfterToolResultHook, AfterToolResultResult]
	beforeTurn      *hookRegistry[BeforeTurnHook, BeforeTurnResult]
	afterTurn       *hookRegistry[AfterTurnHook, AfterTurnResult]
	contextPrepare  *hookRegistry[ContextPrepareHook, ContextPrepareResult]
	beforeCompact   *hookRegistry[BeforeCompactHook, BeforeCompactResult]
	prepareStep     *hookRegistry[PrepareStepHook, PrepareStepResult]
}

func newHookSet() hookSet {
	return hookSet{
		beforeToolCall:  newHookRegistry[BeforeToolCallHook, BeforeToolCallResult](),
		afterToolResult: newHookRegistry[AfterToolResultHook, AfterToolResultResult](),
		beforeTurn:      newHookRegistry[BeforeTurnHook, BeforeTurnResult](),
		afterTurn:       newHookRegistry[AfterTurnHook, AfterTurnResult](),
		contextPrepare:  newHookRegistry[ContextPrepareHook, ContextPrepareResult](),
		beforeCompact:   newHookRegistry[BeforeCompactHook, BeforeCompactResult](),
		prepareStep:     newHookRegistry[PrepareStepHook, PrepareStepResult](),
	}
}

// newConfigStore returns this Kit's configuration store. SDK callers get a
// fresh, isolated *viper.Viper so concurrent constructions never clobber each
// other. The CLI (Options.CLI != nil) shares the process-global store so its
// cobra flag bindings and pre-loaded config remain visible.
func newConfigStore(opts *Options) *viper.Viper {
	if opts.CLI != nil {
		return viper.GetViper()
	}
	return viper.New()
}

// loadConfigStore seeds SDK defaults and loads config files and env vars into
// the instance store.
//
// The CLI shares the process-global store, which cobra.OnInitialize has
// already populated, so re-running initConfig there is unnecessary; SDK
// callers get a fresh isolated store that must be loaded here. We key off
// opts.CLI (not a config value) because setSDKDefaults always seeds "model",
// which would otherwise mask an empty store. SkipConfig bypasses .kit.yml
// file loading (viper defaults and env vars still apply).
func loadConfigStore(v *viper.Viper, opts *Options) error {
	// When used as an SDK (without cobra), these defaults are not registered
	// via flag bindings.
	setSDKDefaults(v)

	if opts.SkipConfig && opts.CLI == nil {
		// initConfig is skipped, so register the KIT_* overrides here.
		bindEnv(v)
	}
	if !opts.SkipConfig && opts.CLI == nil {
		// createDefault=false: an embedding application must not have
		// kit drop a ~/.kit.yml into its users' home directories.
		if err := initConfig(v, opts.ConfigFile, false, opts.Bare, false); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}
	}
	return nil
}

// applyOptionOverrides pushes explicitly-set Options fields into the instance
// store so downstream code (BuildProviderConfig, SetModel, modelSettings
// lookups) picks them up uniformly.
func applyOptionOverrides(v *viper.Viper, opts *Options) {
	if opts.Debug {
		v.Set("debug", true)
	}

	if opts.Model != "" {
		v.Set("model", opts.Model)
	}
	if opts.SystemPrompt != "" {
		v.Set("system-prompt", opts.SystemPrompt)
	}
	if opts.MaxSteps > 0 {
		v.Set("max-steps", opts.MaxSteps)
	}
	// Only override streaming when the caller explicitly set it. Otherwise
	// leave the precedence chain (env → config → default true) untouched so a
	// zero-valued Options does not silently force stream=false.
	if opts.Streaming != nil {
		v.Set("stream", *opts.Streaming)
	}

	// Generation parameter overrides. Pointer-typed sampling params use Set
	// only when non-nil so that nil means "leave provider/per-model default
	// in place" (BuildProviderConfig keys off IsSet).
	if opts.MaxTokens > 0 {
		v.Set("max-tokens", opts.MaxTokens)
	}
	if opts.ThinkingLevel != "" {
		v.Set("thinking-level", opts.ThinkingLevel)
	}
	if opts.Temperature != nil {
		v.Set("temperature", *opts.Temperature)
	}
	if opts.TopP != nil {
		v.Set("top-p", *opts.TopP)
	}
	if opts.TopK != nil {
		v.Set("top-k", *opts.TopK)
	}
	if opts.FrequencyPenalty != nil {
		v.Set("frequency-penalty", *opts.FrequencyPenalty)
	}
	if opts.PresencePenalty != nil {
		v.Set("presence-penalty", *opts.PresencePenalty)
	}

	// Provider overrides. TLSSkipVerify only takes effect when true —
	// callers wanting to force-disable should use the config file or
	// env var instead.
	if opts.ProviderAPIKey != "" {
		v.Set("provider-api-key", opts.ProviderAPIKey)
	}
	if opts.ProviderURL != "" {
		v.Set("provider-url", opts.ProviderURL)
	}
	if opts.ProviderWire != "" {
		v.Set("provider-wire", opts.ProviderWire)
	}
	if opts.TLSSkipVerify {
		v.Set("tls-skip-verify", true)
	}
}

// loadProjectResources resolves the working directory and loads context
// files (AGENTS.md), skills and named agent definitions into rc.
func loadProjectResources(v *viper.Viper, opts *Options, rc *resolvedConfig) error {
	// Resolve working directory for context/skill discovery.
	rc.cwd = opts.SessionDir
	if rc.cwd == "" {
		rc.cwd, _ = os.Getwd()
	}

	// Load context files (AGENTS.md) from the project root.
	if !opts.NoContextFiles && !opts.Bare {
		rc.contextFiles = loadContextFiles(rc.cwd)
	}

	// Load skills — either from explicit paths or via auto-discovery.
	// Merge viper config with opts: CLI flag / config file values are
	// already bound to viper by cmd/root.go, so v.GetBool("no-skills"),
	// v.GetStringSlice("skill"), and v.GetString("skills-dir") capture
	// both --flag and .kit.yml keys transparently.
	noSkills := opts.NoSkills || v.GetBool("no-skills")
	skillPaths := opts.Skills
	if len(skillPaths) == 0 {
		skillPaths = v.GetStringSlice("skill")
	}
	skillsDir := opts.SkillsDir
	if skillsDir == "" {
		skillsDir = v.GetString("skills-dir")
	}
	if !noSkills {
		mergedOpts := *opts
		mergedOpts.Skills = skillPaths
		mergedOpts.SkillsDir = skillsDir
		loaded, err := loadSkills(&mergedOpts)
		if err != nil {
			return fmt.Errorf("failed to load skills: %w", err)
		}
		rc.loadedSkills = loaded

		// Apply per-skill disable list (--skill-disable / skill-disable
		// config key). Disabled skills stay loaded (so /<name> still
		// works) but are hidden from the model-facing catalog.
		disable := opts.SkillsDisable
		if len(disable) == 0 {
			disable = v.GetStringSlice("skill-disable")
		}
		applySkillDisableList(rc.loadedSkills, disable)
	}

	// Discover named agent definitions (built-ins + .agents/agents/,
	// .kit/agents/, ~/.config/kit/agents/). They are advertised in the
	// subagent tool description and resolvable via SubagentConfig.Agent.
	// Per-file parse failures are non-fatal: usable agents still load and
	// a warning is printed unless quiet.
	if !opts.NoAgents && !opts.Bare && !v.GetBool("no-agents") {
		var agErr error
		rc.namedAgents, agErr = LoadAgentDefinitions(rc.cwd)
		if agErr != nil && !opts.Quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to load some agent definitions: %v\n", agErr)
		}
	}
	return nil
}

// buildSystemPrompt composes the system prompt with runtime context (base
// prompt + AGENTS.md context + skills metadata + date/cwd) and writes it back
// to the store. It must run after loadProjectResources.
//
// If the configured model has a per-model system prompt (via modelSettings or
// customModels params) and the user hasn't explicitly set system-prompt, the
// per-model prompt is used as the base instead of the global default.
func buildSystemPrompt(v *viper.Viper, opts *Options, rc *resolvedConfig) {
	rawPromptInput := v.GetString("system-prompt")

	// Resolve a file path to its content so PromptBuilder receives the
	// actual prompt text rather than a literal path string. Without this,
	// when system-prompt is set to a file path in the config file or via
	// --system-prompt, the path itself becomes the effective system prompt
	// sent to the model (LoadSystemPrompt only ran later, after viper had
	// been overwritten with the augmented base text).
	basePrompt, _ := config.LoadSystemPrompt(rawPromptInput)
	if basePrompt == "" {
		basePrompt = rawPromptInput
	}

	// Track whether the user explicitly configured a custom system
	// prompt. When they haven't (basePrompt is the built-in default
	// or empty), per-model system prompts can replace it on switch.
	userSetSystemPrompt := basePrompt != "" && basePrompt != defaultSystemPrompt
	rc.hasCustomSystemPrompt = userSetSystemPrompt
	if rc.hasCustomSystemPrompt {
		rc.systemPromptSource = rawPromptInput
	}

	// Check for per-model system prompt override when no explicit
	// global system-prompt was configured by the user.
	if !userSetSystemPrompt {
		if p, ok := perModelSystemPrompt(v); ok {
			basePrompt = p
		}
	}

	pb := skills.NewPromptBuilder(basePrompt)

	// Capture the resolved base prompt so RefreshSystemPrompt can
	// recompose later after runtime skill/context-file mutations.
	rc.basePrompt = basePrompt

	// Inject AGENTS.md content as project context.
	for _, cf := range rc.contextFiles {
		pb.WithSection("", fmt.Sprintf("Instructions from: %s\n\n%s", cf.Path, cf.Content))
	}

	// Inject skills metadata (name + description + location).
	if len(rc.loadedSkills) > 0 {
		pb.WithSkills(rc.loadedSkills)
	}

	// Append current date/time and working directory.
	pb.WithSection("", environmentSection(rc.cwd, opts.Bare))

	v.Set("system-prompt", pb.Build())
}

// resolveConfig builds the instance configuration store and snapshots every
// instance-derived value New needs. It performs no network or subprocess
// I/O; MCP and agent setup happen afterwards.
func resolveConfig(opts *Options, providers map[string]ProviderFactory) (*viper.Viper, *resolvedConfig, error) {
	v := newConfigStore(opts)
	if err := loadConfigStore(v, opts); err != nil {
		return nil, nil, err
	}
	applyOptionOverrides(v, opts)

	rc := &resolvedConfig{}
	if err := loadProjectResources(v, opts, rc); err != nil {
		return nil, nil, err
	}
	buildSystemPrompt(v, opts, rc)
	if err := resolveModelConfig(v, opts, providers, rc); err != nil {
		return nil, nil, err
	}
	return v, rc, nil
}

// perModelSystemPrompt returns the per-model system prompt configured for the
// store's current model. modelSettings takes priority over custom model
// params. ok is false when no per-model prompt is configured.
//
// When one is configured, ok is true even if the resolved value is empty
// (e.g. it points to an empty file): the caller must still replace the
// base prompt with it.
func perModelSystemPrompt(v *viper.Viper) (prompt string, ok bool) {
	modelStr := v.GetString("model")
	if modelStr == "" {
		return "", false
	}
	mi := models.LookupModelForSettings(modelStr)
	if mi == nil {
		return "", false
	}
	var perModelParams *models.GenerationParams
	if ms := models.LoadModelSettingsFrom(v); ms != nil {
		perModelParams = ms[modelStr]
	}
	if perModelParams == nil && mi.Params != nil {
		perModelParams = mi.Params
	}
	if perModelParams == nil || perModelParams.SystemPrompt == "" {
		return "", false
	}
	return models.LoadSystemPromptValue(perModelParams.SystemPrompt), true
}

// resolveModelConfig snapshots the provider config and the scalar settings
// that agent setup needs, so SetupAgent does not re-read the store.
func resolveModelConfig(v *viper.Viper, opts *Options, providers map[string]ProviderFactory, rc *resolvedConfig) error {
	// BuildProviderConfig is fast (pure reads).
	providerConfig, _, err := kitsetup.BuildProviderConfig(v)
	if err != nil {
		return fmt.Errorf("failed to build provider config: %w", err)
	}

	// SDK last-resort max-tokens floor. When nothing — Options, env,
	// config, nor a per-model default — supplied a value, we land on
	// zero here (GetInt returns 0 for unset keys). Apply the
	// SDK default directly on the struct rather than via the store so
	// IsSet("max-tokens") stays false: downstream right-sizing
	// can still raise this toward the model's known output ceiling,
	// and per-model modelSettings[...].maxTokens can still win.
	if providerConfig.MaxTokens == 0 && opts.MaxTokens == 0 {
		providerConfig.MaxTokens = sdkDefaultMaxTokens
	}
	providerConfig.ProviderFactories = providers
	rc.providerConfig = providerConfig

	rc.modelString = v.GetString("model")
	rc.debug = v.GetBool("debug")
	rc.noExtensions = opts.NoExtensions || v.GetBool("no-extensions")

	toolList := opts.CoreToolList
	if toolList == nil {
		toolList, err = FilterCoreToolNames(v.GetStringSlice("include-core-tools"), v.GetStringSlice("exclude-core-tools"))
		if err != nil {
			return err
		}
	}
	rc.toolList = handleCoreToolList(toolList, opts.DisableCoreTools || v.GetBool("no-core-tools"))

	rc.maxSteps = v.GetInt("max-steps")
	rc.streaming = v.GetBool("stream")
	// Each of the two timeouts has a shell-named form and the bash-named
	// form it had before the tool's shell became configurable; see
	// resolveShellTimeouts for the precedence.
	rc.shellTimeout, rc.shellMaxTimeout = resolveShellTimeouts(opts, v)
	rc.shell = opts.Shell
	if len(rc.shell) == 0 {
		rc.shell = v.GetStringSlice("shell")
	}
	return nil
}

// resolveMCPConfig returns the MCP configuration: a pre-loaded config from
// Options or CLIOptions when set, otherwise the one loaded from the store.
// In-process MCP servers from Options are merged in; these bypass
// subprocess spawning and network I/O.
func resolveMCPConfig(v *viper.Viper, opts *Options) (*config.Config, error) {
	var mcpConfig *config.Config
	if opts.MCPConfig != nil {
		mcpConfig = opts.MCPConfig
	} else if opts.CLI != nil && opts.CLI.MCPConfig != nil {
		mcpConfig = opts.CLI.MCPConfig
	}
	if mcpConfig == nil {
		var err error
		mcpConfig, err = config.LoadAndValidateConfigFrom(v)
		if err != nil {
			return nil, fmt.Errorf("failed to load MCP config: %w", err)
		}
	}

	if len(opts.InProcessMCPServers) > 0 {
		if mcpConfig.MCPServers == nil {
			mcpConfig.MCPServers = make(map[string]config.MCPServerConfig, len(opts.InProcessMCPServers))
		}
		for name, srv := range opts.InProcessMCPServers {
			mcpConfig.MCPServers[name] = config.MCPServerConfig{
				Type:            "inprocess",
				InProcessServer: srv,
			}
		}
	}
	return mcpConfig, nil
}

// withSkillTool appends the dedicated activate_skill tool to extraTools when
// at least one skill is loaded (issue #65, gaps #13/#14). liveKit returns the
// Kit once it exists (nil before); the tool's provider then reads the live
// skill set so runtime additions resolve.
func withSkillTool(extraTools []Tool, loadedSkills []*Skill, liveKit func() *Kit) []Tool {
	if len(loadedSkills) == 0 {
		return extraTools
	}
	names := make([]string, 0, len(loadedSkills))
	for _, s := range loadedSkills {
		if !s.DisableModelInvocation {
			names = append(names, s.Name)
		}
	}
	provider := func() []*skills.Skill {
		k := liveKit()
		if k == nil {
			return loadedSkills
		}
		return k.GetSkills()
	}
	if t := skilltool.New(names, provider); t != nil {
		extraTools = append(extraTools, t)
	}
	return extraTools
}

// buildAgentSetupOptions assembles the options for kitsetup.SetupAgent. It
// passes the pre-built ProviderConfig and scalar snapshots so SetupAgent
// doesn't need to re-read the store, and pulls CLI-specific fields when
// available.
func buildAgentSetupOptions(v *viper.Viper, opts *Options, rc *resolvedConfig, mcpConfig *config.Config, extraTools []Tool, hooks hookSet) kitsetup.AgentSetupOptions {
	setupOpts := kitsetup.AgentSetupOptions{
		MCPConfig:               mcpConfig,
		Quiet:                   opts.Quiet,
		CoreTools:               opts.Tools,
		CoreToolList:            rc.toolList,
		ExtraTools:              extraTools,
		NamedAgents:             namedAgentSpecs(rc.namedAgents),
		ShellTimeout:            rc.shellTimeout,
		ShellMaxTimeout:         rc.shellMaxTimeout,
		Shell:                   rc.shell,
		ToolWrapper:             hookToolWrapper(hooks.beforeToolCall, hooks.afterToolResult),
		ProviderConfig:          rc.providerConfig,
		Debug:                   rc.debug,
		DebugLogger:             opts.DebugLogger,
		NoExtensions:            rc.noExtensions,
		AllowMissingCredentials: opts.AllowMissingCredentials,
		Bare:                    opts.Bare,
		MaxSteps:                rc.maxSteps,
		StreamingEnabled:        rc.streaming,
		OnMCPServerLoaded:       opts.OnMCPServerLoaded,
		MCPTaskConfig: mcpTaskOptions{
			perServer:       opts.MCPTaskMode,
			defaultTTL:      opts.MCPTaskTTL,
			pollInterval:    opts.MCPTaskPollInterval,
			maxPollInterval: opts.MCPTaskMaxPollInterval,
			timeout:         opts.MCPTaskTimeout,
			progress:        opts.MCPTaskProgress,
		}.toToolsConfig(),
		Viper: v,
	}

	// Set up OAuth handler for remote MCP servers. The SDK does not create
	// a default handler: auto-construction would bind a local TCP port and
	// (historically) shell out to a browser without the consumer asking,
	// which is a surprise for library/daemon/web-app embedders. Consumers
	// that want CLI behavior pass a [CLIMCPAuthHandler] explicitly; other
	// consumers implement [MCPAuthHandler] themselves. If nil, remote MCP
	// servers requiring OAuth will fail to connect with the underlying
	// authorization-required error surfaced to the caller.
	//
	// The SDK MCPAuthHandler interface is structurally identical to
	// tools.MCPAuthHandler, so any implementation satisfies both.
	if opts.MCPAuthHandler != nil {
		setupOpts.AuthHandler = opts.MCPAuthHandler
	}

	// Set up custom token store factory for MCP OAuth tokens.
	// The SDK MCPTokenStoreFactory is structurally identical to
	// tools.TokenStoreFactory, so it can be assigned directly.
	if opts.MCPTokenStoreFactory != nil {
		setupOpts.TokenStoreFactory = tools.TokenStoreFactory(opts.MCPTokenStoreFactory)
	}

	if opts.CLI != nil {
		setupOpts.ShowSpinner = opts.CLI.ShowSpinner
		setupOpts.SpinnerFunc = agent.SpinnerFunc(opts.CLI.SpinnerFunc)
		setupOpts.UseBufferedLogger = opts.CLI.UseBufferedLogger
		if opts.CLI.ProgressReaderFunc != nil {
			rc.providerConfig.ProgressReaderFunc = opts.CLI.ProgressReaderFunc
		}
	}
	return setupOpts
}

// initSessionManager returns the caller's custom SessionManager when set,
// otherwise the built-in file-based TreeManager wrapped in an adapter.
func initSessionManager(opts *Options) (SessionManager, error) {
	if opts.SessionManager != nil {
		return opts.SessionManager, nil
	}
	treeSession, err := InitTreeSession(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}
	return NewTreeManagerAdapter(treeSession), nil
}

// initExtensions bridges extension events to SDK hooks and seeds the
// extension context with minimal defaults. SDK users can call
// Extensions().SetContext to override with richer implementations (TUI
// callbacks, prompts, etc.). This ensures extensions never crash on nil
// function fields. It is a no-op when extensions are disabled.
func (m *Kit) initExtensions(runner *extensions.Runner, cwd string) {
	if runner == nil {
		return
	}
	m.bridgeExtensions(runner)
	m.Extensions().SetContext(extensions.Context{
		CWD:         cwd,
		Model:       m.modelString,
		Interactive: false, // SDK mode defaults to non-interactive
	})
}
