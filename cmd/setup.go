package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/tools"
	"github.com/mark3labs/kit/internal/ui"
	"github.com/spf13/viper"
)

// BuildProviderConfig creates a *models.ProviderConfig from the current viper
// state. All three entry points (root, script, SDK) converge through this
// function, eliminating the previously triplicated ModelConfig assembly.
//
// The caller is responsible for ensuring viper holds the correct values before
// calling this function (e.g. script mode merges frontmatter into viper in its
// PreRun hook, the SDK sets overrides explicitly).
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

// AgentSetupOptions controls agent creation behaviour that varies between
// entry points (e.g. spinners are only used by the interactive CLI).
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
}

// AgentSetupResult bundles the created agent and any debug logger so the caller
// can flush buffered messages when appropriate.
type AgentSetupResult struct {
	Agent          *agent.Agent
	BufferedLogger *tools.BufferedDebugLogger
}

// SetupAgent creates an agent from the current viper state + the provided
// options. It wraps BuildProviderConfig and agent.CreateAgent, eliminating the
// triplicated agent-creation boilerplate from root.go, script.go and the SDK.
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

	a, err := agent.CreateAgent(ctx, &agent.AgentCreationOptions{
		ModelConfig:      modelConfig,
		MCPConfig:        opts.MCPConfig,
		SystemPrompt:     systemPrompt,
		MaxSteps:         viper.GetInt("max-steps"),
		StreamingEnabled: viper.GetBool("stream"),
		ShowSpinner:      opts.ShowSpinner,
		Quiet:            quietFlag,
		SpinnerFunc:      opts.SpinnerFunc,
		DebugLogger:      debugLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return &AgentSetupResult{
		Agent:          a,
		BufferedLogger: bufferedLogger,
	}, nil
}

// CollectAgentMetadata extracts model display info and tool/server name lists
// from the agent. This is used by both root.go and script.go to populate
// app.Options and UI setup.
func CollectAgentMetadata(mcpAgent *agent.Agent, mcpConfig *config.Config) (provider, modelName string, serverNames, toolNames []string) {
	modelString := viper.GetString("model")
	provider, modelName, _ = models.ParseModelString(modelString)
	if modelName == "" {
		modelName = "Unknown"
	}

	for name := range mcpConfig.MCPServers {
		serverNames = append(serverNames, name)
	}

	for _, tool := range mcpAgent.GetTools() {
		info := tool.Info()
		toolNames = append(toolNames, info.Name)
	}

	return provider, modelName, serverNames, toolNames
}

// BuildAppOptions constructs the app.Options struct from the current state.
// Both root.go and script.go converge here after agent creation.
func BuildAppOptions(mcpAgent *agent.Agent, mcpConfig *config.Config, modelName string, serverNames, toolNames []string) app.Options {
	return app.Options{
		Agent:            mcpAgent,
		MCPConfig:        mcpConfig,
		ModelName:        modelName,
		ServerNames:      serverNames,
		ToolNames:        toolNames,
		StreamingEnabled: viper.GetBool("stream"),
		Quiet:            quietFlag,
		Debug:            viper.GetBool("debug"),
		CompactMode:      viper.GetBool("compact"),
	}
}

// DisplayDebugConfig builds and displays the debug configuration map through
// the CLI. Shared by root.go (non-interactive) and script.go.
func DisplayDebugConfig(cli *ui.CLI, mcpAgent *agent.Agent, mcpConfig *config.Config, provider string) {
	if quietFlag || cli == nil || !viper.GetBool("debug") {
		return
	}

	debugConfig := map[string]any{
		"model":         viper.GetString("model"),
		"max-steps":     viper.GetInt("max-steps"),
		"max-tokens":    viper.GetInt("max-tokens"),
		"temperature":   viper.GetFloat64("temperature"),
		"top-p":         viper.GetFloat64("top-p"),
		"top-k":         viper.GetInt("top-k"),
		"provider-url":  viper.GetString("provider-url"),
		"system-prompt": viper.GetString("system-prompt"),
	}

	if viper.GetBool("tls-skip-verify") {
		debugConfig["tls-skip-verify"] = true
	}

	if provider == "ollama" {
		debugConfig["num-gpu-layers"] = viper.GetInt("num-gpu-layers")
		debugConfig["main-gpu"] = viper.GetInt("main-gpu")
	}

	if stopSeqs := viper.GetStringSlice("stop-sequences"); len(stopSeqs) > 0 {
		debugConfig["stop-sequences"] = stopSeqs
	}

	if viper.GetString("provider-api-key") != "" {
		debugConfig["provider-api-key"] = "[SET]"
	}

	// MCP server info
	if len(mcpConfig.MCPServers) > 0 {
		mcpServers := make(map[string]any)
		loadedServerSet := make(map[string]bool)
		for _, name := range mcpAgent.GetLoadedServerNames() {
			loadedServerSet[name] = true
		}

		for name, server := range mcpConfig.MCPServers {
			serverInfo := map[string]any{
				"type":   server.Type,
				"status": "failed",
			}
			if loadedServerSet[name] {
				serverInfo["status"] = "loaded"
			}
			if len(server.Command) > 0 {
				serverInfo["command"] = server.Command
			}
			if len(server.Environment) > 0 {
				maskedEnv := make(map[string]string)
				for k, v := range server.Environment {
					if strings.Contains(strings.ToLower(k), "token") ||
						strings.Contains(strings.ToLower(k), "key") ||
						strings.Contains(strings.ToLower(k), "secret") {
						maskedEnv[k] = "[MASKED]"
					} else {
						maskedEnv[k] = v
					}
				}
				serverInfo["environment"] = maskedEnv
			}
			if server.URL != "" {
				serverInfo["url"] = server.URL
			}
			mcpServers[name] = serverInfo
		}
		debugConfig["mcpServers"] = mcpServers
	}

	cli.DisplayDebugConfig(debugConfig)
}

// SetupCLIForNonInteractive creates the CLI display layer for non-interactive
// modes (--prompt and script). Returns nil when quiet mode is active.
func SetupCLIForNonInteractive(mcpAgent *agent.Agent) (*ui.CLI, error) {
	agentAdapter := &agentUIAdapter{agent: mcpAgent}
	return ui.SetupCLI(&ui.CLISetupOptions{
		Agent:          agentAdapter,
		ModelString:    viper.GetString("model"),
		Debug:          viper.GetBool("debug"),
		Compact:        viper.GetBool("compact"),
		Quiet:          quietFlag,
		ShowDebug:      false,
		ProviderAPIKey: viper.GetString("provider-api-key"),
	})
}
