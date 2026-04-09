// Package kitsetup contains agent creation logic used by both the CLI binary
// and the SDK's kit.New(). It is internal — external SDK consumers should use
// kit.New() which delegates here.
package kitsetup

import (
	"context"
	"fmt"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/tools"
	"github.com/spf13/viper"
)

// AgentSetupOptions configures agent creation.
type AgentSetupOptions struct {
	// MCPConfig is the MCP server configuration. Required.
	MCPConfig *config.Config
	// ShowSpinner shows a loading spinner for Ollama models.
	ShowSpinner bool
	// SpinnerFunc provides the spinner implementation (nil = no spinner).
	SpinnerFunc agent.SpinnerFunc
	// UseBufferedLogger captures debug messages for later display (root
	// non-interactive path). When false a simple logger is used instead.
	UseBufferedLogger bool
	// Quiet suppresses output. Replaces the cmd package's quietFlag variable.
	Quiet bool
	// CoreTools overrides the default core tool set. If empty, core.AllTools()
	// is used. Allows SDK users to pass custom tools (e.g. with WithWorkDir).
	CoreTools []fantasy.AgentTool
	// DisableCoreTools, when true, prevents loading any core tools.
	// If both DisableCoreTools is true and CoreTools is empty, the agent
	// will have no tools (useful for simple chat completions).
	DisableCoreTools bool
	// ExtraTools are additional tools added alongside core, MCP, and extension
	// tools. They do not replace the defaults — they extend them.
	ExtraTools []fantasy.AgentTool
	// ToolWrapper is an optional function that wraps tools after extension
	// wrapping. Used by the SDK hook system. Both wrappers compose:
	// extension wrapper runs first (inner), then this wrapper (outer).
	ToolWrapper func([]fantasy.AgentTool) []fantasy.AgentTool

	// ProviderConfig, when non-nil, is used directly instead of calling
	// BuildProviderConfig(). Callers that already hold viperInitMu can
	// pre-build this and release the lock before calling SetupAgent, so the
	// slow agent/MCP initialisation runs concurrently with other New() calls.
	ProviderConfig *models.ProviderConfig
	// Debug enables debug logging. When zero-value, viper is consulted.
	// Only meaningful when ProviderConfig is also set.
	Debug bool
	// NoExtensions skips extension loading. When false, viper is consulted.
	// Only meaningful when ProviderConfig is also set.
	NoExtensions bool
	// MaxSteps overrides the agent step limit. 0 means use viper value.
	// Only meaningful when ProviderConfig is also set.
	MaxSteps int
	// StreamingEnabled controls streaming. Only meaningful when ProviderConfig
	// is also set.
	StreamingEnabled bool
	// AuthHandler handles OAuth authorization for remote MCP servers.
	// When set, remote transports are configured with OAuth support.
	AuthHandler tools.MCPAuthHandler
	// TokenStoreFactory, if non-nil, creates a custom token store for each
	// remote MCP server's OAuth tokens. When nil, the default file-based
	// token store is used.
	TokenStoreFactory tools.TokenStoreFactory
	// OnMCPServerLoaded, if non-nil, is called when each MCP server finishes
	// loading (successfully or with error). Called from the background goroutine.
	OnMCPServerLoaded func(serverName string, toolCount int, err error)
}

// AgentSetupResult bundles the created agent and any debug logger so the caller
// can flush buffered messages when appropriate.
type AgentSetupResult struct {
	Agent          *agent.Agent
	BufferedLogger *tools.BufferedDebugLogger
	// ExtRunner is the extension runner (nil when --no-extensions or no
	// extensions were discovered).
	ExtRunner *extensions.Runner
}

// BuildProviderConfig creates a *models.ProviderConfig from the current viper
// state. All entry points (root, script, SDK) converge through this function.
//
// Generation parameter pointers (Temperature, TopP, etc.) are only set when
// the user has explicitly configured them via CLI flag, environment variable,
// or global config file. This allows per-model defaults from modelSettings
// and customModels to fill in unset parameters downstream.
func BuildProviderConfig() (*models.ProviderConfig, string, error) {
	systemPrompt, err := config.LoadSystemPrompt(viper.GetString("system-prompt"))
	if err != nil {
		return nil, "", fmt.Errorf("failed to load system prompt: %w", err)
	}

	numGPU := int32(viper.GetInt("num-gpu-layers"))
	mainGPU := int32(viper.GetInt("main-gpu"))

	cfg := &models.ProviderConfig{
		ModelString:    viper.GetString("model"),
		SystemPrompt:   systemPrompt,
		ProviderAPIKey: viper.GetString("provider-api-key"),
		ProviderURL:    viper.GetString("provider-url"),
		MaxTokens:      viper.GetInt("max-tokens"),
		StopSequences:  viper.GetStringSlice("stop-sequences"),
		NumGPU:         &numGPU,
		MainGPU:        &mainGPU,
		TLSSkipVerify:  viper.GetBool("tls-skip-verify"),
		ThinkingLevel:  models.ParseThinkingLevel(viper.GetString("thinking-level")),
	}

	// Only set generation parameter pointers when the user has explicitly
	// provided a value. This leaves nil pointers for unset params, allowing
	// per-model defaults (modelSettings / customModels params) to apply.
	if viper.IsSet("temperature") {
		v := float32(viper.GetFloat64("temperature"))
		cfg.Temperature = &v
	}
	if viper.IsSet("top-p") {
		v := float32(viper.GetFloat64("top-p"))
		cfg.TopP = &v
	}
	if viper.IsSet("top-k") {
		v := int32(viper.GetInt("top-k"))
		cfg.TopK = &v
	}
	if viper.IsSet("frequency-penalty") {
		v := float32(viper.GetFloat64("frequency-penalty"))
		cfg.FrequencyPenalty = &v
	}
	if viper.IsSet("presence-penalty") {
		v := float32(viper.GetFloat64("presence-penalty"))
		cfg.PresencePenalty = &v
	}

	return cfg, systemPrompt, nil
}

// SetupAgent creates an agent from the current viper state + the provided
// options. It wraps BuildProviderConfig and agent.CreateAgent.
func SetupAgent(ctx context.Context, opts AgentSetupOptions) (*AgentSetupResult, error) {
	var modelConfig *models.ProviderConfig
	var systemPrompt string

	if opts.ProviderConfig != nil {
		// Pre-built config supplied by caller (e.g. Kit.New after releasing
		// viperInitMu). Use it directly — no viper reads needed here.
		modelConfig = opts.ProviderConfig
		systemPrompt = modelConfig.SystemPrompt
	} else {
		var err error
		modelConfig, systemPrompt, err = BuildProviderConfig()
		if err != nil {
			return nil, err
		}
	}

	// Resolve debug / no-extensions / max-steps / streaming: prefer explicit
	// fields (set when ProviderConfig was pre-built) over viper fallback.
	debugEnabled := opts.Debug || viper.GetBool("debug")
	noExtensions := opts.NoExtensions || viper.GetBool("no-extensions")
	maxSteps := opts.MaxSteps
	if maxSteps == 0 {
		maxSteps = viper.GetInt("max-steps")
	}
	streamingEnabled := opts.StreamingEnabled || viper.GetBool("stream")

	// Create the appropriate debug logger.
	var debugLogger tools.DebugLogger
	var bufferedLogger *tools.BufferedDebugLogger
	if debugEnabled {
		if opts.UseBufferedLogger {
			bufferedLogger = tools.NewBufferedDebugLogger(true)
			debugLogger = bufferedLogger
		} else {
			debugLogger = tools.NewSimpleDebugLogger(true)
		}
	}

	// Load extensions unless --no-extensions is set.
	var extRunner *extensions.Runner
	var extCreationOpts extensionCreationOpts
	if !noExtensions {
		var extErr error
		extRunner, extCreationOpts, extErr = loadExtensions()
		if extErr != nil {
			fmt.Printf("Warning: Failed to load extensions: %v\n", extErr)
		}
	}

	// Compose tool wrappers: extension wrapper (inner) + caller wrapper (outer).
	toolWrapper := extCreationOpts.toolWrapper
	if opts.ToolWrapper != nil {
		if toolWrapper != nil {
			inner := toolWrapper
			outer := opts.ToolWrapper
			toolWrapper = func(t []fantasy.AgentTool) []fantasy.AgentTool {
				return outer(inner(t))
			}
		} else {
			toolWrapper = opts.ToolWrapper
		}
	}

	// Merge extra tools: extension tools + caller extra tools.
	extraTools := extCreationOpts.extraTools
	if len(opts.ExtraTools) > 0 {
		extraTools = append(extraTools, opts.ExtraTools...)
	}

	a, err := agent.CreateAgent(ctx, &agent.AgentCreationOptions{
		ModelConfig:       modelConfig,
		MCPConfig:         opts.MCPConfig,
		SystemPrompt:      systemPrompt,
		MaxSteps:          maxSteps,
		StreamingEnabled:  streamingEnabled,
		ShowSpinner:       opts.ShowSpinner,
		Quiet:             opts.Quiet,
		SpinnerFunc:       opts.SpinnerFunc,
		DebugLogger:       debugLogger,
		AuthHandler:       opts.AuthHandler,
		TokenStoreFactory: opts.TokenStoreFactory,
		CoreTools:         opts.CoreTools,
		DisableCoreTools:  opts.DisableCoreTools,
		ToolWrapper:       toolWrapper,
		ExtraTools:        extraTools,
		OnMCPServerLoaded: opts.OnMCPServerLoaded,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return &AgentSetupResult{
		Agent:          a,
		ExtRunner:      extRunner,
		BufferedLogger: bufferedLogger,
	}, nil
}

// extensionCreationOpts holds the tool wrapper and extra tools extracted from
// loaded extensions for passing into agent creation.
type extensionCreationOpts struct {
	toolWrapper func([]fantasy.AgentTool) []fantasy.AgentTool
	extraTools  []fantasy.AgentTool
}

// loadExtensions discovers and loads Yaegi extensions, builds the runner,
// and returns the tool wrapper/extra tools.
func loadExtensions() (*extensions.Runner, extensionCreationOpts, error) {
	extraPaths := viper.GetStringSlice("extension")
	loaded, err := extensions.LoadExtensions(extraPaths)
	if err != nil {
		return nil, extensionCreationOpts{}, err
	}

	if len(loaded) == 0 {
		return nil, extensionCreationOpts{}, nil
	}

	runner := extensions.NewRunner(loaded)

	wrapper := func(tools []fantasy.AgentTool) []fantasy.AgentTool {
		return extensions.WrapToolsWithExtensions(tools, runner)
	}

	extTools := extensions.ExtensionToolsAsFantasy(runner.RegisteredTools(), runner)

	return runner, extensionCreationOpts{
		toolWrapper: wrapper,
		extraTools:  extTools,
	}, nil
}
