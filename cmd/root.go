package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"github.com/mark3labs/mcphost/internal/agent"
	"github.com/mark3labs/mcphost/internal/app"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/session"
	"github.com/mark3labs/mcphost/internal/tools"
	"github.com/mark3labs/mcphost/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var (
	configFile       string
	systemPromptFile string
	modelFlag        string
	providerURL      string
	providerAPIKey   string
	debugMode        bool
	promptFlag       string
	quietFlag        bool
	noExitFlag       bool
	maxSteps         int
	streamFlag       bool           // Enable streaming output
	compactMode      bool           // Enable compact output mode
	scriptMCPConfig  *config.Config // Used to override config in script mode

	// Session management
	saveSessionPath string
	loadSessionPath string
	sessionPath     string

	// Model generation parameters
	maxTokens     int
	temperature   float32
	topP          float32
	topK          int32
	stopSequences []string

	// Ollama-specific parameters
	numGPU  int32
	mainGPU int32

	// Hooks control

	// TLS configuration
	tlsSkipVerify bool
)

// agentUIAdapter adapts agent.Agent to ui.AgentInterface
type agentUIAdapter struct {
	agent *agent.Agent
}

func (a *agentUIAdapter) GetLoadingMessage() string {
	return a.agent.GetLoadingMessage()
}

func (a *agentUIAdapter) GetTools() []any {
	tools := a.agent.GetTools()
	result := make([]any, len(tools))
	for i, tool := range tools {
		result[i] = tool
	}
	return result
}

func (a *agentUIAdapter) GetLoadedServerNames() []string {
	return a.agent.GetLoadedServerNames()
}

// rootCmd represents the base command when called without any subcommands.
// This is the main entry point for the MCPHost CLI application, providing
// an interface to interact with various AI models through a unified interface
// with support for MCP servers and tool integration.
var rootCmd = &cobra.Command{
	Use:   "mcphost",
	Short: "Chat with AI models through a unified interface",
	Long: `MCPHost is a CLI tool that allows you to interact with various AI models
through a unified interface. It supports various tools through MCP servers
and provides streaming responses.

Available models can be specified using the --model flag:
- Anthropic Claude (default): anthropic/claude-sonnet-4-5-20250929
- OpenAI: openai/gpt-4
- Ollama models: ollama/modelname
- Google: google/modelname

Examples:
  # Interactive mode
  mcphost -m ollama/qwen2.5:3b
  mcphost -m openai/gpt-4
  mcphost -m google/gemini-2.0-flash
  
  # Non-interactive mode
  mcphost -p "What is the weather like today?"
  mcphost -p "Calculate 15 * 23" --quiet
  
  # Session management
  mcphost --save-session ./my-session.json -p "Hello"
  mcphost --load-session ./my-session.json -p "Continue our conversation"
  mcphost --load-session ./session.json --save-session ./session.json -p "Next message"
  mcphost --session ./session.json -p "Next message"
  
  # Script mode
  mcphost script myscript.sh`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPHost(context.Background())
	},
}

// GetRootCommand returns the root command with the version set.
// This function is the main entry point for the MCPHost CLI and should be
// called from main.go with the appropriate version string.
func GetRootCommand(v string) *cobra.Command {
	rootCmd.Version = v
	return rootCmd
}

// InitConfig initializes the configuration for MCPHost by loading config files,
// environment variables, and hooks configuration. It follows this priority order:
// 1. Command-line specified config file (--config flag)
// 2. Current directory config file (.mcphost or .mcp)
// 3. Home directory config file (~/.mcphost or ~/.mcp)
// 4. Environment variables (MCPHOST_* prefix)
// This function is automatically called by cobra before command execution.
func InitConfig() {
	if configFile != "" {
		// Use config file from the flag
		if err := LoadConfigWithEnvSubstitution(configFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading config file '%s': %v\n", configFile, err)
			os.Exit(1)
		}
	} else {
		// Ensure a config file exists (create default if none found)
		if err := config.EnsureConfigExists(); err != nil {
			// If we can't create config, continue silently (non-fatal)
			fmt.Fprintf(os.Stderr, "Warning: Could not create default config file: %v\n", err)
		}

		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding home directory: %v\n", err)
			os.Exit(1)
		}

		// Set up viper config search paths and names
		// Current directory has higher priority than home directory
		viper.AddConfigPath(".")  // Current directory (searched first)
		viper.AddConfigPath(home) // Home directory (searched second)

		// Try to find and load config file using viper's search mechanism
		configLoaded := false
		configNames := []string{".mcphost", ".mcp"} // Try .mcphost first, then legacy .mcp

		for _, name := range configNames {
			viper.SetConfigName(name)

			// Try to read the config file
			if err := viper.ReadInConfig(); err == nil {
				// Config file found, now reload it with env substitution
				configPath := viper.ConfigFileUsed()
				if err := LoadConfigWithEnvSubstitution(configPath); err != nil {
					// Only exit on environment variable substitution errors
					if strings.Contains(err.Error(), "environment variable substitution failed") {
						fmt.Fprintf(os.Stderr, "Error reading config file '%s': %v\n", configPath, err)
						os.Exit(1)
					}
					// For other errors, continue trying other config files
					continue
				}
				configLoaded = true
				break
			}
		}

		// If no config file was loaded, continue without error (optional config)
		if !configLoaded && debugMode {
			fmt.Fprintf(os.Stderr, "No config file found in current directory or home directory\n")
		}
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("MCPHOST")
	viper.AutomaticEnv()

}

// LoadConfigWithEnvSubstitution loads a config file with environment variable substitution.
// It reads the config file, replaces any ${ENV_VAR} patterns with their corresponding
// environment variable values, and then parses the resulting configuration using viper.
// The function automatically detects JSON or YAML format based on file extension.
// Returns an error if the file cannot be read, environment variable substitution fails,
// or the configuration cannot be parsed.
func LoadConfigWithEnvSubstitution(configPath string) error {
	// Read raw config file content
	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// Apply environment variable substitution
	substituter := &config.EnvSubstituter{}
	processedContent, err := substituter.SubstituteEnvVars(string(rawContent))
	if err != nil {
		return fmt.Errorf("config env substitution failed: %v", err)
	}

	// Determine config type from file extension
	configType := "yaml"
	if strings.HasSuffix(configPath, ".json") {
		configType = "json"
	}

	config.SetConfigPath(configPath)

	// Use viper to parse the processed content
	viper.SetConfigType(configType)
	return viper.ReadConfig(strings.NewReader(processedContent))
}

func configToUiTheme(theme config.Theme) ui.Theme {
	return ui.Theme{
		Primary:     ui.AdaptiveColor(theme.Primary.Light, theme.Primary.Dark),
		Secondary:   ui.AdaptiveColor(theme.Secondary.Light, theme.Secondary.Dark),
		Success:     ui.AdaptiveColor(theme.Success.Light, theme.Success.Dark),
		Warning:     ui.AdaptiveColor(theme.Warning.Light, theme.Warning.Dark),
		Error:       ui.AdaptiveColor(theme.Error.Light, theme.Error.Dark),
		Info:        ui.AdaptiveColor(theme.Info.Light, theme.Info.Dark),
		Text:        ui.AdaptiveColor(theme.Text.Light, theme.Text.Dark),
		Muted:       ui.AdaptiveColor(theme.Muted.Light, theme.Muted.Dark),
		VeryMuted:   ui.AdaptiveColor(theme.VeryMuted.Light, theme.VeryMuted.Dark),
		Background:  ui.AdaptiveColor(theme.Background.Light, theme.Background.Dark),
		Border:      ui.AdaptiveColor(theme.Border.Light, theme.Border.Dark),
		MutedBorder: ui.AdaptiveColor(theme.MutedBorder.Light, theme.MutedBorder.Dark),
		System:      ui.AdaptiveColor(theme.System.Light, theme.System.Dark),
		Tool:        ui.AdaptiveColor(theme.Tool.Light, theme.Tool.Dark),
		Accent:      ui.AdaptiveColor(theme.Accent.Light, theme.Accent.Dark),
		Highlight:   ui.AdaptiveColor(theme.Highlight.Light, theme.Highlight.Dark),
	}
}

func init() {
	cobra.OnInitialize(InitConfig)
	var theme config.Theme
	err := config.FilepathOr("theme", &theme)
	if err == nil && viper.InConfig("theme") {
		uiTheme := configToUiTheme(theme)
		ui.SetTheme(uiTheme)
	}

	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is $HOME/.mcp.json)")
	rootCmd.PersistentFlags().
		StringVar(&systemPromptFile, "system-prompt", "", "system prompt text or path to text file")

	rootCmd.PersistentFlags().
		StringVarP(&modelFlag, "model", "m", "anthropic/claude-sonnet-4-5-20250929",
			"model to use (format: provider/model)")
	rootCmd.PersistentFlags().
		BoolVar(&debugMode, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().
		StringVarP(&promptFlag, "prompt", "p", "", "run in non-interactive mode with the given prompt")
	rootCmd.PersistentFlags().
		BoolVar(&quietFlag, "quiet", false, "suppress all output (only works with --prompt)")
	rootCmd.PersistentFlags().
		BoolVar(&noExitFlag, "no-exit", false, "prevent non-interactive mode from exiting, show input prompt instead")
	rootCmd.PersistentFlags().
		IntVar(&maxSteps, "max-steps", 0, "maximum number of agent steps (0 for unlimited)")
	rootCmd.PersistentFlags().
		BoolVar(&streamFlag, "stream", true, "enable streaming output for faster response display")
	rootCmd.PersistentFlags().
		BoolVar(&compactMode, "compact", false, "enable compact output mode without fancy styling")
	rootCmd.PersistentFlags().
		StringVar(&saveSessionPath, "save-session", "", "save session to file after each message")
	rootCmd.PersistentFlags().
		StringVar(&loadSessionPath, "load-session", "", "load session from file at startup")
	rootCmd.PersistentFlags().
		StringVarP(&sessionPath, "session", "s", "", "session file to load and update")

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&providerURL, "provider-url", "", "base URL for the provider API (applies to OpenAI, Anthropic, Ollama, and Google)")
	flags.StringVar(&providerAPIKey, "provider-api-key", "", "API key for the provider (applies to OpenAI, Anthropic, and Google)")
	flags.BoolVar(&tlsSkipVerify, "tls-skip-verify", false, "skip TLS certificate verification (WARNING: insecure, use only for self-signed certificates)")

	// Model generation parameters
	flags.IntVar(&maxTokens, "max-tokens", 4096, "maximum number of tokens in the response")
	flags.Float32Var(&temperature, "temperature", 0.7, "controls randomness in responses (0.0-1.0)")
	flags.Float32Var(&topP, "top-p", 0.95, "controls diversity via nucleus sampling (0.0-1.0)")
	flags.Int32Var(&topK, "top-k", 40, "controls diversity by limiting top K tokens to sample from")
	flags.StringSliceVar(&stopSequences, "stop-sequences", nil, "custom stop sequences (comma-separated)")

	// Ollama-specific parameters
	flags.Int32Var(&numGPU, "num-gpu-layers", -1, "number of model layers to offload to GPU for Ollama models (-1 for auto-detect)")
	_ = flags.MarkHidden("num-gpu-layers") // Advanced option, hidden from help
	flags.Int32Var(&mainGPU, "main-gpu", 0, "main GPU device to use for Ollama models")

	// Bind flags to viper for config file support
	_ = viper.BindPFlag("system-prompt", rootCmd.PersistentFlags().Lookup("system-prompt"))
	_ = viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("prompt", rootCmd.PersistentFlags().Lookup("prompt"))
	_ = viper.BindPFlag("max-steps", rootCmd.PersistentFlags().Lookup("max-steps"))
	_ = viper.BindPFlag("stream", rootCmd.PersistentFlags().Lookup("stream"))
	_ = viper.BindPFlag("compact", rootCmd.PersistentFlags().Lookup("compact"))

	_ = viper.BindPFlag("provider-url", rootCmd.PersistentFlags().Lookup("provider-url"))
	_ = viper.BindPFlag("provider-api-key", rootCmd.PersistentFlags().Lookup("provider-api-key"))
	_ = viper.BindPFlag("max-tokens", rootCmd.PersistentFlags().Lookup("max-tokens"))
	_ = viper.BindPFlag("temperature", rootCmd.PersistentFlags().Lookup("temperature"))
	_ = viper.BindPFlag("top-p", rootCmd.PersistentFlags().Lookup("top-p"))
	_ = viper.BindPFlag("top-k", rootCmd.PersistentFlags().Lookup("top-k"))
	_ = viper.BindPFlag("stop-sequences", rootCmd.PersistentFlags().Lookup("stop-sequences"))
	_ = viper.BindPFlag("num-gpu-layers", rootCmd.PersistentFlags().Lookup("num-gpu-layers"))
	_ = viper.BindPFlag("main-gpu", rootCmd.PersistentFlags().Lookup("main-gpu"))
	_ = viper.BindPFlag("tls-skip-verify", rootCmd.PersistentFlags().Lookup("tls-skip-verify"))

	// Defaults are already set in flag definitions, no need to duplicate in viper

	// Add subcommands
	rootCmd.AddCommand(authCmd)
}

func runMCPHost(ctx context.Context) error {
	return runNormalMode(ctx)
}

func runNormalMode(ctx context.Context) error {
	// Validate flag combinations
	if quietFlag && promptFlag == "" {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}
	if noExitFlag && promptFlag == "" {
		return fmt.Errorf("--no-exit flag can only be used with --prompt/-p")
	}

	// Set up logging
	if debugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Load MCP configuration
	var mcpConfig *config.Config
	var err error

	if scriptMCPConfig != nil {
		// Use script-provided config
		mcpConfig = scriptMCPConfig
	} else {
		// Use the new config loader
		mcpConfig, err = config.LoadAndValidateConfig()
		if err != nil {
			return fmt.Errorf("failed to load MCP config: %v", err)
		}
	}

	// Update debug mode from viper
	if viper.GetBool("debug") && !debugMode {
		debugMode = viper.GetBool("debug")
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	systemPrompt, err := config.LoadSystemPrompt(viper.GetString("system-prompt"))
	if err != nil {
		return fmt.Errorf("failed to load system prompt: %v", err)
	}

	// Create model configuration
	temperature := float32(viper.GetFloat64("temperature"))
	topP := float32(viper.GetFloat64("top-p"))
	topK := int32(viper.GetInt("top-k"))
	numGPU := int32(viper.GetInt("num-gpu-layers"))
	mainGPU := int32(viper.GetInt("main-gpu"))

	modelConfig := &models.ProviderConfig{
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

	// Create spinner function for agent creation
	var spinnerFunc agent.SpinnerFunc
	if !quietFlag {
		spinnerFunc = func(fn func() error) error {
			tempCli, tempErr := ui.NewCLI(viper.GetBool("debug"), viper.GetBool("compact"))
			if tempErr == nil {
				return tempCli.ShowSpinner(fn)
			}
			// Fallback without spinner
			return fn()
		}
	}

	// Create the agent using the factory
	// Use a buffered debug logger to capture messages during initialization
	var bufferedLogger *tools.BufferedDebugLogger
	var debugLogger tools.DebugLogger
	if viper.GetBool("debug") {
		bufferedLogger = tools.NewBufferedDebugLogger(true)
		debugLogger = bufferedLogger
	}

	mcpAgent, err := agent.CreateAgent(ctx, &agent.AgentCreationOptions{
		ModelConfig:      modelConfig,
		MCPConfig:        mcpConfig,
		SystemPrompt:     systemPrompt,
		MaxSteps:         viper.GetInt("max-steps"),
		StreamingEnabled: viper.GetBool("stream"),
		ShowSpinner:      true,
		Quiet:            quietFlag,
		SpinnerFunc:      spinnerFunc,
		DebugLogger:      debugLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create agent: %v", err)
	}
	defer func() { _ = mcpAgent.Close() }()

	// Initialize hook executor if hooks are configured
	// Get model name for display
	modelString := viper.GetString("model")
	parsedProvider, modelName, _ := models.ParseModelString(modelString)
	if modelName == "" {
		modelName = "Unknown"
	}

	// Create CLI for non-interactive mode only. SetupCLI is the factory for the
	// non-interactive (quiet and non-quiet) path; interactive mode uses the full
	// Bubble Tea TUI (AppModel) which handles its own rendering.
	// cli is nil in interactive mode and in quiet non-interactive mode.
	var cli *ui.CLI
	if promptFlag != "" {
		agentAdapter := &agentUIAdapter{agent: mcpAgent}
		cli, err = ui.SetupCLI(&ui.CLISetupOptions{
			Agent:          agentAdapter,
			ModelString:    modelString,
			Debug:          viper.GetBool("debug"),
			Compact:        viper.GetBool("compact"),
			Quiet:          quietFlag,
			ShowDebug:      false, // Handled separately below
			ProviderAPIKey: viper.GetString("provider-api-key"),
		})
		if err != nil {
			return fmt.Errorf("failed to setup CLI: %v", err)
		}

		// Display buffered debug messages if any (non-interactive path only).
		if bufferedLogger != nil && cli != nil {
			msgs := bufferedLogger.GetMessages()
			if len(msgs) > 0 {
				combinedMessage := strings.Join(msgs, "\n  ")
				cli.DisplayDebugMessage(combinedMessage)
			}
		}

		// Display debug configuration if debug mode is enabled (non-interactive path only).
		if !quietFlag && cli != nil && viper.GetBool("debug") {
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

			// Add TLS skip verify if enabled
			if viper.GetBool("tls-skip-verify") {
				debugConfig["tls-skip-verify"] = true
			}

			// Add Ollama-specific parameters if using Ollama
			if parsedProvider == "ollama" {
				debugConfig["num-gpu-layers"] = viper.GetInt("num-gpu-layers")
				debugConfig["main-gpu"] = viper.GetInt("main-gpu")
			}

			// Only include non-empty stop sequences
			stopSequences := viper.GetStringSlice("stop-sequences")
			if len(stopSequences) > 0 {
				debugConfig["stop-sequences"] = stopSequences
			}

			// Only include API keys if they're set (but don't show the actual values for security)
			if viper.GetString("provider-api-key") != "" {
				debugConfig["provider-api-key"] = "[SET]"
			}

			// Add MCP server configuration for debugging
			if len(mcpConfig.MCPServers) > 0 {
				mcpServers := make(map[string]any)
				loadedServers := mcpAgent.GetLoadedServerNames()
				loadedServerSet := make(map[string]bool)
				for _, name := range loadedServers {
					loadedServerSet[name] = true
				}

				for name, server := range mcpConfig.MCPServers {
					serverInfo := map[string]any{
						"type":   server.Type,
						"status": "failed", // Default to failed
					}

					// Mark as loaded if it's in the loaded servers list
					if loadedServerSet[name] {
						serverInfo["status"] = "loaded"
					}

					if len(server.Command) > 0 {
						serverInfo["command"] = server.Command
					}
					if len(server.Environment) > 0 {
						// Mask sensitive environment variables
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
					if server.Name != "" {
						serverInfo["name"] = server.Name
					}
					mcpServers[name] = serverInfo
				}
				debugConfig["mcpServers"] = mcpServers
			}
			cli.DisplayDebugConfig(debugConfig)
		}
	}

	// Prepare data for slash commands
	var serverNames []string
	for name := range mcpConfig.MCPServers {
		serverNames = append(serverNames, name)
	}

	tools := mcpAgent.GetTools()
	var toolNames []string
	for _, tool := range tools {
		info := tool.Info()
		toolNames = append(toolNames, info.Name)
	}

	// Main interaction logic
	var messages []fantasy.Message
	var sessionManager *session.Manager
	if sessionPath != "" {
		_, err := os.Stat(sessionPath)
		if os.IsNotExist(err) {
			content := []byte("{}")
			if err := os.WriteFile(sessionPath, content, 0664); err != nil {
				panic(err)
			}
		}
		loadSessionPath = sessionPath
		saveSessionPath = sessionPath
	}

	// Load existing session if specified
	if loadSessionPath != "" {
		loadedSession, err := session.LoadFromFile(loadSessionPath)
		if err != nil {
			return fmt.Errorf("failed to load session: %v", err)
		}

		// Convert session messages to schema messages
		for _, msg := range loadedSession.Messages {
			fantasyMsg := msg.ConvertToFantasyMessage()
			messages = append(messages, fantasyMsg)
		}

		// If we're also saving, use the loaded session with the session manager
		if saveSessionPath != "" {
			sessionManager = session.NewManagerWithSession(loadedSession, saveSessionPath)
		}

		if !quietFlag && cli != nil {
			// Create a map of tool call IDs to tool calls for quick lookup
			toolCallMap := make(map[string]session.ToolCall)
			for _, sessionMsg := range loadedSession.Messages {
				if sessionMsg.Role == "assistant" && len(sessionMsg.ToolCalls) > 0 {
					for _, tc := range sessionMsg.ToolCalls {
						toolCallMap[tc.ID] = tc
					}
				}
			}

			// Display all previous messages as they would have appeared
			for _, sessionMsg := range loadedSession.Messages {
				switch sessionMsg.Role {
				case "user":
					cli.DisplayUserMessage(sessionMsg.Content)
				case "assistant":
					// Display tool calls if present
					if len(sessionMsg.ToolCalls) > 0 {
						for _, tc := range sessionMsg.ToolCalls {
							// Convert arguments to string
							var argsStr string
							if argBytes, err := json.Marshal(tc.Arguments); err == nil {
								argsStr = string(argBytes)
							}

							// Display tool call
							cli.DisplayToolCallMessage(tc.Name, argsStr)
						}
					}

					// Display assistant response (only if there's content)
					if sessionMsg.Content != "" {
						_ = cli.DisplayAssistantMessage(sessionMsg.Content)
					}
				case "tool":
					// Display tool result
					if sessionMsg.ToolCallID != "" {
						if toolCall, exists := toolCallMap[sessionMsg.ToolCallID]; exists {
							// Convert arguments to string
							var argsStr string
							if argBytes, err := json.Marshal(toolCall.Arguments); err == nil {
								argsStr = string(argBytes)
							}

							// Parse tool result content - it might be JSON-encoded MCP content
							resultContent := sessionMsg.Content

							// Try to parse as MCP content structure
							var mcpContent struct {
								Content []struct {
									Type string `json:"type"`
									Text string `json:"text"`
								} `json:"content"`
							}

							// First try to unmarshal as-is
							if err := json.Unmarshal([]byte(sessionMsg.Content), &mcpContent); err == nil {
								// Extract text from MCP content structure
								if len(mcpContent.Content) > 0 && mcpContent.Content[0].Type == "text" {
									resultContent = mcpContent.Content[0].Text
								}
							} else {
								// If that fails, try unquoting first (in case it's double-encoded)
								var unquoted string
								if err := json.Unmarshal([]byte(sessionMsg.Content), &unquoted); err == nil {
									if err := json.Unmarshal([]byte(unquoted), &mcpContent); err == nil {
										if len(mcpContent.Content) > 0 && mcpContent.Content[0].Type == "text" {
											resultContent = mcpContent.Content[0].Text
										}
									}
								}
							}

							// Display tool result (assuming no error for saved results)
							cli.DisplayToolMessage(toolCall.Name, argsStr, resultContent, false)
						}
					}
				}
			}
		}
	} else if saveSessionPath != "" {
		// Only saving, create new session manager
		sessionManager = session.NewManager(saveSessionPath)

		// Set metadata
		_ = sessionManager.SetMetadata(session.Metadata{
			MCPHostVersion: "dev", // TODO: Get actual version
			Provider:       parsedProvider,
			Model:          modelName,
		})
	}

	// Create the app.App instance now that session messages are loaded.
	appOpts := app.Options{
		Agent:            mcpAgent,
		SessionManager:   sessionManager,
		MCPConfig:        mcpConfig,
		ModelName:        modelName,
		ServerNames:      serverNames,
		ToolNames:        toolNames,
		StreamingEnabled: viper.GetBool("stream"),
		Quiet:            quietFlag,
		Debug:            viper.GetBool("debug"),
		CompactMode:      viper.GetBool("compact"),
	}

	// Create a usage tracker that is shared between the app layer (for recording
	// usage after each step) and the TUI (for /usage display). For non-interactive
	// mode the tracker comes from the CLI factory; for interactive mode we create
	// one directly.
	var usageTracker *ui.UsageTracker
	if cli != nil {
		usageTracker = cli.GetUsageTracker()
	} else {
		// Interactive mode: create a tracker using the same logic as SetupCLI.
		usageTracker = ui.CreateUsageTracker(modelString, viper.GetString("provider-api-key"))
	}
	if usageTracker != nil {
		appOpts.UsageTracker = usageTracker
	}

	appInstance := app.New(appOpts, messages)
	defer appInstance.Close()

	// Check if running in non-interactive mode
	if promptFlag != "" {
		return runNonInteractiveModeApp(ctx, appInstance, promptFlag, quietFlag, noExitFlag, modelName, parsedProvider, mcpAgent.GetLoadingMessage(), serverNames, toolNames, usageTracker)
	}

	// Quiet mode is not allowed in interactive mode
	if quietFlag {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}

	return runInteractiveModeBubbleTea(ctx, appInstance, modelName, parsedProvider, mcpAgent.GetLoadingMessage(), serverNames, toolNames, usageTracker)
}

// runNonInteractiveModeApp executes a single prompt via the app layer and exits,
// or transitions to the interactive BubbleTea TUI when --no-exit is set.
//
// RunOnce does not create a tea.Program, so there is no spinner or tool-call
// display; only the final response text is written to os.Stdout.  This
// satisfies both the normal and --quiet non-interactive cases (quiet simply
// means "no intermediate output", which RunOnce already guarantees).
//
// When --no-exit is set, after RunOnce completes the interactive BubbleTea TUI
// is started so the user can continue the conversation.
func runNonInteractiveModeApp(ctx context.Context, appInstance *app.App, prompt string, _, noExit bool, modelName, providerName, loadingMessage string, serverNames, toolNames []string, usageTracker *ui.UsageTracker) error {
	if err := appInstance.RunOnce(ctx, prompt, os.Stdout); err != nil {
		return err
	}

	// If --no-exit was requested, hand off to the interactive TUI.
	if noExit {
		return runInteractiveModeBubbleTea(ctx, appInstance, modelName, providerName, loadingMessage, serverNames, toolNames, usageTracker)
	}

	return nil
}

// runInteractiveModeBubbleTea starts the new unified Bubble Tea interactive TUI.
//
// It:
//  1. Gets the terminal dimensions (falls back to 80x24 if unavailable).
//  2. Creates a ui.AppModel (parent model) with the appInstance as the controller,
//     wiring up all child components (InputComponent, StreamComponent).
//  3. Creates a single tea.NewProgram and registers it with appInstance via SetProgram
//     so that agent events are routed to the TUI.
//  4. Calls program.Run() which blocks until the user quits (Ctrl+C or /quit).
//
// SetupCLI is not used for interactive mode; the TUI (AppModel) handles its own rendering.
func runInteractiveModeBubbleTea(_ context.Context, appInstance *app.App, modelName, providerName, loadingMessage string, serverNames, toolNames []string, usageTracker *ui.UsageTracker) error {
	// Determine terminal size; fall back gracefully.
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termWidth == 0 {
		termWidth = 80
		termHeight = 24
	}

	appModel := ui.NewAppModel(appInstance, ui.AppModelOptions{
		CompactMode:    viper.GetBool("compact"),
		ModelName:      modelName,
		ProviderName:   providerName,
		LoadingMessage: loadingMessage,
		Width:          termWidth,
		Height:         termHeight,
		ServerNames:    serverNames,
		ToolNames:      toolNames,
		UsageTracker:   usageTracker,
	})

	// Print startup info to stdout before Bubble Tea takes over the screen.
	appModel.PrintStartupInfo()

	program := tea.NewProgram(appModel)

	// Register the program with the app layer so agent events are sent to the TUI.
	appInstance.SetProgram(program)

	_, runErr := program.Run()
	return runErr
}
