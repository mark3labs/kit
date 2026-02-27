package kit

import (
	"context"
	"fmt"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/hooks"
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
func BuildProviderConfig() (*models.ProviderConfig, string, error) {
	systemPrompt, err := config.LoadSystemPrompt(viper.GetString("system-prompt"))
	if err != nil {
		return nil, "", fmt.Errorf("failed to load system prompt: %w", err)
	}

	temperature := float32(viper.GetFloat64("temperature"))
	topP := float32(viper.GetFloat64("top-p"))
	topK := int32(viper.GetInt("top-k"))
	numGPU := int32(viper.GetInt("num-gpu-layers"))
	mainGPU := int32(viper.GetInt("main-gpu"))

	cfg := &models.ProviderConfig{
		ModelString:    viper.GetString("model"),
		SystemPrompt:   systemPrompt,
		ProviderAPIKey: viper.GetString("provider-api-key"),
		ProviderURL:    viper.GetString("provider-url"),
		MaxTokens:      viper.GetInt("max-tokens"),
		Temperature:    &temperature,
		TopP:           &topP,
		TopK:           &topK,
		StopSequences:  viper.GetStringSlice("stop-sequences"),
		NumGPU:         &numGPU,
		MainGPU:        &mainGPU,
		TLSSkipVerify:  viper.GetBool("tls-skip-verify"),
	}

	return cfg, systemPrompt, nil
}

// SetupAgent creates an agent from the current viper state + the provided
// options. It wraps BuildProviderConfig and agent.CreateAgent.
func SetupAgent(ctx context.Context, opts AgentSetupOptions) (*AgentSetupResult, error) {
	modelConfig, systemPrompt, err := BuildProviderConfig()
	if err != nil {
		return nil, err
	}

	// Create the appropriate debug logger.
	var debugLogger tools.DebugLogger
	var bufferedLogger *tools.BufferedDebugLogger
	if viper.GetBool("debug") {
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
	if !viper.GetBool("no-extensions") {
		var extErr error
		extRunner, extCreationOpts, extErr = loadExtensions()
		if extErr != nil {
			fmt.Printf("Warning: Failed to load extensions: %v\n", extErr)
		}
	}

	a, err := agent.CreateAgent(ctx, &agent.AgentCreationOptions{
		ModelConfig:      modelConfig,
		MCPConfig:        opts.MCPConfig,
		SystemPrompt:     systemPrompt,
		MaxSteps:         viper.GetInt("max-steps"),
		StreamingEnabled: viper.GetBool("stream"),
		ShowSpinner:      opts.ShowSpinner,
		Quiet:            opts.Quiet,
		SpinnerFunc:      opts.SpinnerFunc,
		DebugLogger:      debugLogger,
		ToolWrapper:      extCreationOpts.toolWrapper,
		ExtraTools:       extCreationOpts.extraTools,
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

// loadExtensions discovers and loads Yaegi extensions plus legacy hooks.yml,
// builds the runner, and returns the tool wrapper/extra tools.
func loadExtensions() (*extensions.Runner, extensionCreationOpts, error) {
	extraPaths := viper.GetStringSlice("extension")
	loaded, err := extensions.LoadExtensions(extraPaths)
	if err != nil {
		return nil, extensionCreationOpts{}, err
	}

	// Also load legacy hooks.yml as a compat extension.
	hooksCfg, _ := hooks.LoadHooksConfig()
	if hooksCfg != nil && len(hooksCfg.Hooks) > 0 {
		compat := extensions.HooksAsExtension(hooksCfg)
		if compat != nil {
			loaded = append([]extensions.LoadedExtension{*compat}, loaded...)
		}
	}

	if len(loaded) == 0 {
		return nil, extensionCreationOpts{}, nil
	}

	runner := extensions.NewRunner(loaded)

	wrapper := func(tools []fantasy.AgentTool) []fantasy.AgentTool {
		return extensions.WrapToolsWithExtensions(tools, runner)
	}

	extTools := extensions.ExtensionToolsAsFantasy(runner.RegisteredTools())

	return runner, extensionCreationOpts{
		toolWrapper: wrapper,
		extraTools:  extTools,
	}, nil
}
