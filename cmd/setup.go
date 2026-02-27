package cmd

import (
	"strings"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/ui"
	kit "github.com/mark3labs/kit/pkg/kit"
	"github.com/spf13/viper"
)

// CollectAgentMetadata extracts model display info and tool/server name lists
// from the agent, used to populate app.Options and UI setup.
// It also returns the number of MCP tools and extension tools separately.
func CollectAgentMetadata(mcpAgent *agent.Agent, mcpConfig *config.Config) (provider, modelName string, serverNames, toolNames []string, mcpToolCount, extensionToolCount int) {
	modelString := viper.GetString("model")
	provider, modelName, _ = kit.ParseModelString(modelString)
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

	mcpToolCount = mcpAgent.GetMCPToolCount()
	extensionToolCount = mcpAgent.GetExtensionToolCount()

	return provider, modelName, serverNames, toolNames, mcpToolCount, extensionToolCount
}

// BuildAppOptions constructs the app.Options struct from the current state.
func BuildAppOptions(mcpConfig *config.Config, modelName string, serverNames, toolNames []string) app.Options {
	return app.Options{
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
// the CLI for non-interactive mode.
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
// mode (--prompt). Returns nil when quiet mode is active.
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
