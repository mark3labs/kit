package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"charm.land/lipgloss/v2"
	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/ui"
	kit "github.com/mark3labs/kit/pkg/kit"
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
	autoCompactFlag  bool           // Enable auto-compaction near context limit
	scriptMCPConfig  *config.Config // Used to override config in script mode

	// Session management
	sessionPath string

	// Tree session management (pi-style)
	continueFlag  bool // --continue / -c: resume most recent session for cwd
	resumeFlag    bool // --resume / -r: interactive session picker
	noSessionFlag bool // --no-session: ephemeral mode, no persistence

	// Model generation parameters
	maxTokens     int
	temperature   float32
	topP          float32
	topK          int32
	stopSequences []string

	// Ollama-specific parameters
	numGPU  int32
	mainGPU int32

	// Extensions control
	noExtensionsFlag bool
	extensionPaths   []string

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
// This is the main entry point for the KIT CLI application, providing
// an interface to interact with various AI models through a unified interface
// with support for MCP servers and tool integration.
var rootCmd = &cobra.Command{
	Use:   "kit",
	Short: "Chat with AI models through a unified interface",
	Long:  `KIT (Knowledge Inference Tool) — A lightweight AI agent for coding`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runKit(context.Background())
	},
}

// GetRootCommand returns the root command with the version set.
// This function is the main entry point for the KIT CLI and should be
// called from main.go with the appropriate version string.
func GetRootCommand(v string) *cobra.Command {
	rootCmd.Version = v
	return rootCmd
}

// InitConfig initializes the configuration for KIT by loading config files,
// environment variables, and hooks configuration. It delegates to the SDK's
// InitConfig, injecting the CLI-specific configFile flag and debug mode.
// This function is automatically called by cobra before command execution.
func InitConfig() {
	if err := kit.InitConfig(configFile, debugMode); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// LoadConfigWithEnvSubstitution loads a config file with environment variable
// substitution. Delegates to the SDK implementation.
func LoadConfigWithEnvSubstitution(configPath string) error {
	return kit.LoadConfigWithEnvSubstitution(configPath)
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

// kitBanner returns the KIT ASCII art title with KITT scanner lights,
// rendered with a KITT red gradient.
func kitBanner() string {
	kittDark := lipgloss.Color("#8B0000")
	kittBright := lipgloss.Color("#FF2200")
	lines := []string{
		"            ██╗  ██╗ ██╗ ████████╗",
		"            ██║ ██╔╝ ██║ ╚══██╔══╝",
		"            █████╔╝  ██║    ██║",
		"            ██╔═██╗  ██║    ██║",
		"            ██║  ██╗ ██║    ██║",
		"            ╚═╝  ╚═╝ ╚═╝    ╚═╝",
		" ░░░░░░▒▒▒▒▒▓▓▓▓███████████████▓▓▓▓▒▒▒▒▒░░░░░░",
	}

	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(ui.ApplyGradient(line, kittDark, kittBright))
	}
	return result.String()
}

func init() {
	cobra.OnInitialize(InitConfig)

	rootCmd.Long = kitBanner() + "\n\n" + rootCmd.Long

	var theme config.Theme
	err := config.FilepathOr("theme", &theme)
	if err == nil && viper.InConfig("theme") {
		uiTheme := configToUiTheme(theme)
		ui.SetTheme(uiTheme)
	}

	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is $HOME/.kit.yml)")
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
		BoolVar(&autoCompactFlag, "auto-compact", false, "auto-compact conversation when near context limit")
	rootCmd.PersistentFlags().
		StringVarP(&sessionPath, "session", "s", "", "open a specific JSONL session file")
	rootCmd.PersistentFlags().
		BoolVarP(&continueFlag, "continue", "c", false, "continue the most recent session for the current directory")
	rootCmd.PersistentFlags().
		BoolVarP(&resumeFlag, "resume", "r", false, "interactive session picker")
	rootCmd.PersistentFlags().
		BoolVar(&noSessionFlag, "no-session", false, "ephemeral mode — no session persistence")
	rootCmd.PersistentFlags().
		BoolVar(&noExtensionsFlag, "no-extensions", false, "disable all extensions and hooks")
	rootCmd.PersistentFlags().
		StringSliceVarP(&extensionPaths, "extension", "e", nil, "load additional extension file(s)")

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
	_ = viper.BindPFlag("auto-compact", rootCmd.PersistentFlags().Lookup("auto-compact"))

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
	_ = viper.BindPFlag("no-extensions", rootCmd.PersistentFlags().Lookup("no-extensions"))
	_ = viper.BindPFlag("extension", rootCmd.PersistentFlags().Lookup("extension"))

	// Defaults are already set in flag definitions, no need to duplicate in viper

	// Add subcommands
	rootCmd.AddCommand(authCmd)
}

func runKit(ctx context.Context) error {
	return runNormalMode(ctx)
}

// extensionCommandsForUI converts extension-registered CommandDefs into the
// ui.ExtensionCommand type used by the interactive TUI. Command names are
// normalised to start with "/" so they integrate with the slash-command
// autocomplete and dispatch pipeline.
func extensionCommandsForUI(runner *extensions.Runner) []ui.ExtensionCommand {
	if runner == nil {
		return nil
	}
	defs := runner.RegisteredCommands()
	if len(defs) == 0 {
		return nil
	}
	cmds := make([]ui.ExtensionCommand, 0, len(defs))
	for _, d := range defs {
		name := d.Name
		if len(name) > 0 && name[0] != '/' {
			name = "/" + name
		}
		cmds = append(cmds, ui.ExtensionCommand{
			Name:        name,
			Description: d.Description,
			Execute: func(args string) (string, error) {
				return d.Execute(args, runner.GetContext())
			},
		})
	}
	return cmds
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

	// Update debug mode from viper
	if viper.GetBool("debug") && !debugMode {
		debugMode = viper.GetBool("debug")
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Load MCP configuration.
	var mcpConfig *config.Config
	var err error
	if scriptMCPConfig != nil {
		mcpConfig = scriptMCPConfig
	} else {
		mcpConfig, err = config.LoadAndValidateConfig()
		if err != nil {
			return fmt.Errorf("failed to load MCP config: %v", err)
		}
	}

	// Create spinner function for agent creation.
	var spinnerFunc kit.SpinnerFunc
	if !quietFlag {
		spinnerFunc = func(fn func() error) error {
			tempCli, tempErr := ui.NewCLI(viper.GetBool("debug"), viper.GetBool("compact"))
			if tempErr == nil {
				return tempCli.ShowSpinner(fn)
			}
			return fn()
		}
	}

	// Build Kit options from CLI flags and create the SDK instance.
	// kit.New() handles: config → skills → agent → session → hooks → extension bridge.
	kitOpts := &kit.Options{
		MCPConfig:         mcpConfig,
		ShowSpinner:       true,
		SpinnerFunc:       spinnerFunc,
		UseBufferedLogger: true,
		Quiet:             quietFlag,
		Debug:             debugMode,
		NoSession:         noSessionFlag,
		Continue:          continueFlag,
		SessionPath:       sessionPath,
		AutoCompact:       autoCompactFlag,
	}
	if resumeFlag {
		// TODO: TUI session picker.
		sessions, _ := kit.ListSessions("")
		if len(sessions) > 0 {
			kitOpts.SessionPath = sessions[0].Path
		}
	}

	kitInstance, err := kit.New(ctx, kitOpts)
	if err != nil {
		return err
	}
	defer func() { _ = kitInstance.Close() }()

	// Extract agent + metadata for display and app options.
	mcpAgent := kitInstance.GetAgent()
	parsedProvider, modelName, serverNames, toolNames := CollectAgentMetadata(mcpAgent, mcpConfig)

	// Create CLI for non-interactive mode only.
	var cli *ui.CLI
	if promptFlag != "" {
		cli, err = SetupCLIForNonInteractive(mcpAgent)
		if err != nil {
			return fmt.Errorf("failed to setup CLI: %v", err)
		}

		// Display buffered debug messages if any (non-interactive path only).
		if bl := kitInstance.GetBufferedLogger(); bl != nil && cli != nil {
			msgs := bl.GetMessages()
			if len(msgs) > 0 {
				cli.DisplayDebugMessage(strings.Join(msgs, "\n  "))
			}
		}

		DisplayDebugConfig(cli, mcpAgent, mcpConfig, parsedProvider)
	}

	// Load existing messages from resumed/continued sessions.
	treeSession := kitInstance.GetTreeSession()
	var messages []fantasy.Message
	if treeSession != nil {
		messages = treeSession.GetFantasyMessages()
	}

	// Create the app.App instance.
	extRunner := kitInstance.GetExtRunner()
	appOpts := BuildAppOptions(mcpAgent, mcpConfig, modelName, serverNames, toolNames, extRunner)
	appOpts.Kit = kitInstance
	appOpts.TreeSession = treeSession

	// Create a usage tracker that is shared between the app layer (for recording
	// usage after each step) and the TUI (for /usage display).
	var usageTracker *ui.UsageTracker
	if cli != nil {
		usageTracker = cli.GetUsageTracker()
	} else {
		usageTracker = ui.CreateUsageTracker(viper.GetString("model"), viper.GetString("provider-api-key"))
	}
	if usageTracker != nil {
		appOpts.UsageTracker = usageTracker
	}

	appInstance := app.New(appOpts, messages)
	defer appInstance.Close()

	// Set up extension context and emit SessionStart.
	if extRunner != nil {
		cwd, _ := os.Getwd()
		extRunner.SetContext(extensions.Context{
			CWD:         cwd,
			Model:       modelName,
			Interactive: promptFlag == "",
			Print:       func(text string) { appInstance.PrintFromExtension("", text) },
			PrintInfo:   func(text string) { appInstance.PrintFromExtension("info", text) },
			PrintError:  func(text string) { appInstance.PrintFromExtension("error", text) },
			PrintBlock:  appInstance.PrintBlockFromExtension,
			SendMessage: func(text string) { appInstance.Run(text) },
		})
		if extRunner.HasHandlers(extensions.SessionStart) {
			_, _ = extRunner.Emit(extensions.SessionStartEvent{})
		}
	}

	// Convert extension commands to UI-layer type for the interactive TUI.
	extCommands := extensionCommandsForUI(extRunner)

	// Check if running in non-interactive mode
	if promptFlag != "" {
		return runNonInteractiveModeApp(ctx, appInstance, cli, promptFlag, quietFlag, noExitFlag, modelName, parsedProvider, mcpAgent.GetLoadingMessage(), serverNames, toolNames, usageTracker, extCommands)
	}

	// Quiet mode is not allowed in interactive mode
	if quietFlag {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}

	return runInteractiveModeBubbleTea(ctx, appInstance, modelName, parsedProvider, mcpAgent.GetLoadingMessage(), serverNames, toolNames, usageTracker, extCommands)
}

// runNonInteractiveModeApp executes a single prompt via the app layer and exits,
// or transitions to the interactive BubbleTea TUI when --no-exit is set.
//
// In quiet mode, RunOnce is used (no intermediate output, final response only).
// Otherwise, RunOnceWithDisplay streams tool calls and responses through the
// shared CLIEventHandler — giving --prompt mode the same rich output as script
// mode. This eliminates the previous split where --prompt silently swallowed
// all intermediate events.
//
// When --no-exit is set, after the prompt completes the interactive BubbleTea
// TUI is started so the user can continue the conversation.
func runNonInteractiveModeApp(ctx context.Context, appInstance *app.App, cli *ui.CLI, prompt string, quiet, noExit bool, modelName, providerName, loadingMessage string, serverNames, toolNames []string, usageTracker *ui.UsageTracker, extCommands []ui.ExtensionCommand) error {
	if quiet {
		// Quiet mode: no intermediate display, just print final response.
		if err := appInstance.RunOnce(ctx, prompt); err != nil {
			return err
		}
	} else if cli != nil {
		// Display user message before running the agent.
		cli.DisplayUserMessage(prompt)

		// Route events through the shared CLI event handler.
		eventHandler := ui.NewCLIEventHandler(cli, modelName)
		err := appInstance.RunOnceWithDisplay(ctx, prompt, eventHandler.Handle)
		eventHandler.Cleanup()
		if err != nil {
			return err
		}
	} else {
		// No CLI available (shouldn't happen in non-quiet mode, but be safe).
		if err := appInstance.RunOnce(ctx, prompt); err != nil {
			return err
		}
	}

	// If --no-exit was requested, hand off to the interactive TUI.
	if noExit {
		return runInteractiveModeBubbleTea(ctx, appInstance, modelName, providerName, loadingMessage, serverNames, toolNames, usageTracker, extCommands)
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
func runInteractiveModeBubbleTea(_ context.Context, appInstance *app.App, modelName, providerName, loadingMessage string, serverNames, toolNames []string, usageTracker *ui.UsageTracker, extCommands []ui.ExtensionCommand) error {
	// Determine terminal size; fall back gracefully.
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termWidth == 0 {
		termWidth = 80
		termHeight = 24
	}

	appModel := ui.NewAppModel(appInstance, ui.AppModelOptions{
		CompactMode:       viper.GetBool("compact"),
		ModelName:         modelName,
		ProviderName:      providerName,
		LoadingMessage:    loadingMessage,
		Width:             termWidth,
		Height:            termHeight,
		ServerNames:       serverNames,
		ToolNames:         toolNames,
		UsageTracker:      usageTracker,
		ExtensionCommands: extCommands,
	})

	// Print startup info to stdout before Bubble Tea takes over the screen.
	appModel.PrintStartupInfo()

	program := tea.NewProgram(appModel)

	// Register the program with the app layer so agent events are sent to the TUI.
	appInstance.SetProgram(program)

	_, runErr := program.Run()
	return runErr
}
