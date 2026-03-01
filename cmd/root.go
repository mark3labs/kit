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
	"charm.land/lipgloss/v2"
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
	jsonFlag         bool
	noExitFlag       bool
	maxSteps         int
	streamFlag       bool // Enable streaming output
	compactMode      bool // Enable compact output mode
	autoCompactFlag  bool // Enable auto-compaction near context limit

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

// kitUIAdapter adapts *kit.Kit to ui.AgentInterface so the CLI setup layer
// can display tool/server metadata without importing internal types.
type kitUIAdapter struct {
	kit *kit.Kit
}

func (a *kitUIAdapter) GetLoadingMessage() string {
	return a.kit.GetLoadingMessage()
}

func (a *kitUIAdapter) GetTools() []any {
	names := a.kit.GetToolNames()
	result := make([]any, len(names))
	for i, name := range names {
		result[i] = name
	}
	return result
}

func (a *kitUIAdapter) GetLoadedServerNames() []string {
	return a.kit.GetLoadedServerNames()
}

func (a *kitUIAdapter) GetMCPToolCount() int {
	return a.kit.GetMCPToolCount()
}

func (a *kitUIAdapter) GetExtensionToolCount() int {
	return a.kit.GetExtensionToolCount()
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
// environment variables. It delegates to the SDK's
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
		BoolVar(&jsonFlag, "json", false, "output response as JSON (only works with --prompt)")
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
		BoolVar(&noExtensionsFlag, "no-extensions", false, "disable all extensions")
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
func extensionCommandsForUI(k *kit.Kit) []ui.ExtensionCommand {
	defs := k.ExtensionCommands()
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
				return d.Execute(args, k.GetExtensionContext())
			},
		})
	}
	return cmds
}

// widgetProviderForUI returns a function that converts extension widgets to
// ui.WidgetData for the given placement. Returns nil if extensions are
// disabled, which is safe — the UI treats a nil GetWidgets as "no widgets".
func widgetProviderForUI(k *kit.Kit) func(string) []ui.WidgetData {
	if !k.HasExtensions() {
		return nil
	}
	return func(placement string) []ui.WidgetData {
		configs := k.GetExtensionWidgets(extensions.WidgetPlacement(placement))
		if len(configs) == 0 {
			return nil
		}
		widgets := make([]ui.WidgetData, len(configs))
		for i, c := range configs {
			widgets[i] = ui.WidgetData{
				Text:        c.Content.Text,
				Markdown:    c.Content.Markdown,
				BorderColor: c.Style.BorderColor,
				NoBorder:    c.Style.NoBorder,
			}
		}
		return widgets
	}
}

// headerProviderForUI returns a function that converts the extension header
// to a *ui.WidgetData for the TUI. Returns nil if extensions are disabled,
// which is safe — the UI treats a nil GetHeader as "no header".
func headerProviderForUI(k *kit.Kit) func() *ui.WidgetData {
	if !k.HasExtensions() {
		return nil
	}
	return func() *ui.WidgetData {
		config := k.GetExtensionHeader()
		if config == nil {
			return nil
		}
		return &ui.WidgetData{
			Text:        config.Content.Text,
			Markdown:    config.Content.Markdown,
			BorderColor: config.Style.BorderColor,
			NoBorder:    config.Style.NoBorder,
		}
	}
}

// toolRendererProviderForUI returns a function that converts extension tool
// renderers to ui.ToolRendererData for the TUI. Returns nil if extensions are
// disabled, which is safe — the UI treats a nil GetToolRenderer as "no
// custom renderers".
func toolRendererProviderForUI(k *kit.Kit) func(string) *ui.ToolRendererData {
	if !k.HasExtensions() {
		return nil
	}
	return func(toolName string) *ui.ToolRendererData {
		config := k.GetExtensionToolRenderer(toolName)
		if config == nil {
			return nil
		}
		return &ui.ToolRendererData{
			DisplayName:  config.DisplayName,
			BorderColor:  config.BorderColor,
			Background:   config.Background,
			BodyMarkdown: config.BodyMarkdown,
			RenderHeader: config.RenderHeader,
			RenderBody:   config.RenderBody,
		}
	}
}

// editorInterceptorProviderForUI returns a function that converts the
// extension editor interceptor to a *ui.EditorInterceptor for the TUI.
// Returns nil if extensions are disabled, which is safe — the UI treats a
// nil GetEditorInterceptor as "no interceptor".
func editorInterceptorProviderForUI(k *kit.Kit) func() *ui.EditorInterceptor {
	if !k.HasExtensions() {
		return nil
	}
	return func() *ui.EditorInterceptor {
		config := k.GetExtensionEditor()
		if config == nil {
			return nil
		}
		var handleKey func(string, string) ui.EditorKeyAction
		if config.HandleKey != nil {
			extHandleKey := config.HandleKey
			handleKey = func(key, text string) ui.EditorKeyAction {
				r := extHandleKey(key, text)
				return ui.EditorKeyAction{
					Type:        ui.EditorKeyActionType(r.Type),
					RemappedKey: r.RemappedKey,
					SubmitText:  r.SubmitText,
				}
			}
		}
		var render func(int, string) string
		if config.Render != nil {
			extRender := config.Render
			render = func(width int, defaultContent string) string {
				return extRender(width, defaultContent)
			}
		}
		return &ui.EditorInterceptor{
			HandleKey: handleKey,
			Render:    render,
		}
	}
}

// uiVisibilityProviderForUI returns a function that converts extension UI
// visibility overrides to a *ui.UIVisibility for the TUI. Returns nil if
// extensions are disabled — the UI treats nil as "show everything".
func uiVisibilityProviderForUI(k *kit.Kit) func() *ui.UIVisibility {
	if !k.HasExtensions() {
		return nil
	}
	return func() *ui.UIVisibility {
		v := k.GetExtensionUIVisibility()
		if v == nil {
			return nil
		}
		return &ui.UIVisibility{
			HideStartupMessage: v.HideStartupMessage,
			HideStatusBar:      v.HideStatusBar,
			HideSeparator:      v.HideSeparator,
			HideInputHint:      v.HideInputHint,
		}
	}
}

// footerProviderForUI returns a function that converts the extension footer
// to a *ui.WidgetData for the TUI. Returns nil if extensions are disabled,
// which is safe — the UI treats a nil GetFooter as "no footer".
func footerProviderForUI(k *kit.Kit) func() *ui.WidgetData {
	if !k.HasExtensions() {
		return nil
	}
	return func() *ui.WidgetData {
		config := k.GetExtensionFooter()
		if config == nil {
			return nil
		}
		return &ui.WidgetData{
			Text:        config.Content.Text,
			Markdown:    config.Content.Markdown,
			BorderColor: config.Style.BorderColor,
			NoBorder:    config.Style.NoBorder,
		}
	}
}

// statusBarProviderForUI returns a function that fetches extension status bar
// entries and converts them to ui.StatusBarEntryData for the TUI. Returns nil
// if extensions are disabled, which is safe — the TUI treats a nil
// GetStatusBarEntries as "no extension entries".
func statusBarProviderForUI(k *kit.Kit) func() []ui.StatusBarEntryData {
	if !k.HasExtensions() {
		return nil
	}
	return func() []ui.StatusBarEntryData {
		entries := k.GetExtensionStatusEntries()
		if len(entries) == 0 {
			return nil
		}
		result := make([]ui.StatusBarEntryData, len(entries))
		for i, e := range entries {
			result[i] = ui.StatusBarEntryData{
				Key:      e.Key,
				Text:     e.Text,
				Priority: e.Priority,
			}
		}
		return result
	}
}

func runNormalMode(ctx context.Context) error {
	// Validate flag combinations
	if quietFlag && promptFlag == "" {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}
	if jsonFlag && promptFlag == "" {
		return fmt.Errorf("--json flag can only be used with --prompt/-p")
	}
	if jsonFlag && noExitFlag {
		return fmt.Errorf("--json and --no-exit flags cannot be used together")
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
	mcpConfig, err := config.LoadAndValidateConfig()
	if err != nil {
		return fmt.Errorf("failed to load MCP config: %v", err)
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
	// kit.New() handles: config → skills → agent → session → extension bridge.
	kitOpts := &kit.Options{
		Quiet:       quietFlag,
		Debug:       debugMode,
		NoSession:   noSessionFlag,
		Continue:    continueFlag,
		SessionPath: sessionPath,
		AutoCompact: autoCompactFlag,
		CLI: &kit.CLIOptions{
			MCPConfig:         mcpConfig,
			ShowSpinner:       true,
			SpinnerFunc:       spinnerFunc,
			UseBufferedLogger: true,
		},
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

	// Extract metadata for display and app options.
	parsedProvider, modelName, serverNames, toolNames, mcpToolCount, extensionToolCount := CollectAgentMetadata(kitInstance, mcpConfig)

	// Create CLI for non-interactive mode only.
	var cli *ui.CLI
	if promptFlag != "" {
		cli, err = SetupCLIForNonInteractive(kitInstance)
		if err != nil {
			return fmt.Errorf("failed to setup CLI: %v", err)
		}

		// Display buffered debug messages if any (non-interactive path only).
		if msgs := kitInstance.GetBufferedDebugMessages(); len(msgs) > 0 && cli != nil {
			cli.DisplayDebugMessage(strings.Join(msgs, "\n  "))
		}

		DisplayDebugConfig(cli, kitInstance, mcpConfig, parsedProvider)
	}

	// Load existing messages from resumed/continued sessions.
	treeSession := kitInstance.GetTreeSession()
	var messages []fantasy.Message
	if treeSession != nil {
		messages = treeSession.GetFantasyMessages()
	}

	// Create the app.App instance.
	appOpts := BuildAppOptions(mcpConfig, modelName, serverNames, toolNames)
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
	if kitInstance.HasExtensions() {
		cwd, _ := os.Getwd()
		kitInstance.SetExtensionContext(extensions.Context{
			CWD:         cwd,
			Model:       modelName,
			Interactive: promptFlag == "",
			Print:       func(text string) { appInstance.PrintFromExtension("", text) },
			PrintInfo:   func(text string) { appInstance.PrintFromExtension("info", text) },
			PrintError:  func(text string) { appInstance.PrintFromExtension("error", text) },
			PrintBlock:  appInstance.PrintBlockFromExtension,
			SendMessage: func(text string) { appInstance.Run(text) },
			SetWidget: func(config extensions.WidgetConfig) {
				kitInstance.SetExtensionWidget(config)
				appInstance.NotifyWidgetUpdate()
			},
			RemoveWidget: func(id string) {
				kitInstance.RemoveExtensionWidget(id)
				appInstance.NotifyWidgetUpdate()
			},
			SetHeader: func(config extensions.HeaderFooterConfig) {
				kitInstance.SetExtensionHeader(config)
				appInstance.NotifyWidgetUpdate()
			},
			RemoveHeader: func() {
				kitInstance.RemoveExtensionHeader()
				appInstance.NotifyWidgetUpdate()
			},
			SetFooter: func(config extensions.HeaderFooterConfig) {
				kitInstance.SetExtensionFooter(config)
				appInstance.NotifyWidgetUpdate()
			},
			RemoveFooter: func() {
				kitInstance.RemoveExtensionFooter()
				appInstance.NotifyWidgetUpdate()
			},
			PromptSelect: func(config extensions.PromptSelectConfig) extensions.PromptSelectResult {
				ch := make(chan app.PromptResponse, 1)
				appInstance.SendPromptRequest(app.PromptRequestEvent{
					PromptType: "select",
					Message:    config.Message,
					Options:    config.Options,
					ResponseCh: ch,
				})
				resp := <-ch
				if resp.Cancelled {
					return extensions.PromptSelectResult{Cancelled: true}
				}
				return extensions.PromptSelectResult{Value: resp.Value, Index: resp.Index}
			},
			PromptConfirm: func(config extensions.PromptConfirmConfig) extensions.PromptConfirmResult {
				ch := make(chan app.PromptResponse, 1)
				def := "false"
				if config.DefaultValue {
					def = "true"
				}
				appInstance.SendPromptRequest(app.PromptRequestEvent{
					PromptType: "confirm",
					Message:    config.Message,
					Default:    def,
					ResponseCh: ch,
				})
				resp := <-ch
				if resp.Cancelled {
					return extensions.PromptConfirmResult{Cancelled: true}
				}
				return extensions.PromptConfirmResult{Value: resp.Confirmed}
			},
			PromptInput: func(config extensions.PromptInputConfig) extensions.PromptInputResult {
				ch := make(chan app.PromptResponse, 1)
				appInstance.SendPromptRequest(app.PromptRequestEvent{
					PromptType:  "input",
					Message:     config.Message,
					Placeholder: config.Placeholder,
					Default:     config.Default,
					ResponseCh:  ch,
				})
				resp := <-ch
				if resp.Cancelled {
					return extensions.PromptInputResult{Cancelled: true}
				}
				return extensions.PromptInputResult{Value: resp.Value}
			},
			SetUIVisibility: func(v extensions.UIVisibility) {
				kitInstance.SetExtensionUIVisibility(v)
				appInstance.NotifyWidgetUpdate()
			},
			GetContextStats: func() extensions.ContextStats {
				s := kitInstance.GetContextStats()
				return extensions.ContextStats{
					EstimatedTokens: s.EstimatedTokens,
					ContextLimit:    s.ContextLimit,
					UsagePercent:    s.UsagePercent,
					MessageCount:    s.MessageCount,
				}
			},
			SetEditor: func(config extensions.EditorConfig) {
				kitInstance.SetExtensionEditor(config)
				// Use a goroutine for NotifyWidgetUpdate because this may be
				// called from within an editor HandleKey callback, which runs
				// synchronously inside BubbleTea's Update(). Calling prog.Send()
				// directly from Update() deadlocks the event loop.
				go appInstance.NotifyWidgetUpdate()
			},
			ResetEditor: func() {
				kitInstance.ResetExtensionEditor()
				go appInstance.NotifyWidgetUpdate()
			},
			GetMessages: func() []extensions.SessionMessage {
				return kitInstance.GetSessionMessages()
			},
			GetSessionPath: func() string {
				return kitInstance.GetSessionFilePath()
			},
			AppendEntry: func(entryType string, data string) (string, error) {
				return kitInstance.AppendExtensionEntry(entryType, data)
			},
			GetEntries: func(entryType string) []extensions.ExtensionEntry {
				return kitInstance.GetExtensionEntries(entryType)
			},
			SetEditorText: func(text string) {
				appInstance.SetEditorTextFromExtension(text)
			},
			SetStatus: func(key string, text string, priority int) {
				kitInstance.SetExtensionStatus(extensions.StatusBarEntry{
					Key:      key,
					Text:     text,
					Priority: priority,
				})
				appInstance.NotifyWidgetUpdate()
			},
			RemoveStatus: func(key string) {
				kitInstance.RemoveExtensionStatus(key)
				appInstance.NotifyWidgetUpdate()
			},
			ShowOverlay: func(config extensions.OverlayConfig) extensions.OverlayResult {
				ch := make(chan app.OverlayResponse, 1)
				appInstance.SendOverlayRequest(app.OverlayRequestEvent{
					Title:       config.Title,
					Content:     config.Content.Text,
					Markdown:    config.Content.Markdown,
					BorderColor: config.Style.BorderColor,
					Background:  config.Style.Background,
					Width:       config.Width,
					MaxHeight:   config.MaxHeight,
					Anchor:      string(config.Anchor),
					Actions:     config.Actions,
					ResponseCh:  ch,
				})
				resp := <-ch
				if resp.Cancelled {
					return extensions.OverlayResult{Cancelled: true, Index: -1}
				}
				return extensions.OverlayResult{
					Action: resp.Action,
					Index:  resp.Index,
				}
			},
		})
		kitInstance.EmitSessionStart()
	}

	// Convert extension commands to UI-layer type for the interactive TUI.
	extCommands := extensionCommandsForUI(kitInstance)

	// Build context/skills display metadata for the startup banner.
	var contextPaths []string
	for _, cf := range kitInstance.GetContextFiles() {
		contextPaths = append(contextPaths, cf.Path)
	}
	cwd, _ := os.Getwd()
	var skillItems []ui.SkillItem
	for _, s := range kitInstance.GetSkills() {
		source := "user"
		if strings.HasPrefix(s.Path, cwd) {
			source = "project"
		}
		skillItems = append(skillItems, ui.SkillItem{
			Name:   s.Name,
			Path:   s.Path,
			Source: source,
		})
	}

	// Build extension UI providers once (shared between both modes).
	getWidgets := widgetProviderForUI(kitInstance)
	getHeader := headerProviderForUI(kitInstance)
	getFooter := footerProviderForUI(kitInstance)
	getToolRenderer := toolRendererProviderForUI(kitInstance)
	getEditorInterceptor := editorInterceptorProviderForUI(kitInstance)
	getUIVisibility := uiVisibilityProviderForUI(kitInstance)
	getStatusBarEntries := statusBarProviderForUI(kitInstance)

	// Check if running in non-interactive mode
	if promptFlag != "" {
		return runNonInteractiveModeApp(ctx, appInstance, cli, promptFlag, quietFlag, jsonFlag, noExitFlag, modelName, parsedProvider, kitInstance.GetLoadingMessage(), serverNames, toolNames, mcpToolCount, extensionToolCount, usageTracker, extCommands, contextPaths, skillItems, getWidgets, getHeader, getFooter, getToolRenderer, getEditorInterceptor, getUIVisibility, getStatusBarEntries)
	}

	// Quiet mode is not allowed in interactive mode
	if quietFlag {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}

	return runInteractiveModeBubbleTea(ctx, appInstance, modelName, parsedProvider, kitInstance.GetLoadingMessage(), serverNames, toolNames, mcpToolCount, extensionToolCount, usageTracker, extCommands, contextPaths, skillItems, getWidgets, getHeader, getFooter, getToolRenderer, getEditorInterceptor, getUIVisibility, getStatusBarEntries)
}

// runNonInteractiveModeApp executes a single prompt via the app layer and exits,
// or transitions to the interactive BubbleTea TUI when --no-exit is set.
//
// In quiet mode, RunOnce is used (no intermediate output, final response only).
// Otherwise, RunOnceWithDisplay streams tool calls and responses through the
// shared CLIEventHandler — giving --prompt mode the same rich output as
// interactive mode.
//
// When --no-exit is set, after the prompt completes the interactive BubbleTea
// TUI is started so the user can continue the conversation.
func runNonInteractiveModeApp(ctx context.Context, appInstance *app.App, cli *ui.CLI, prompt string, quiet, jsonOutput, noExit bool, modelName, providerName, loadingMessage string, serverNames, toolNames []string, mcpToolCount, extensionToolCount int, usageTracker *ui.UsageTracker, extCommands []ui.ExtensionCommand, contextPaths []string, skillItems []ui.SkillItem, getWidgets func(string) []ui.WidgetData, getHeader, getFooter func() *ui.WidgetData, getToolRenderer func(string) *ui.ToolRendererData, getEditorInterceptor func() *ui.EditorInterceptor, getUIVisibility func() *ui.UIVisibility, getStatusBarEntries func() []ui.StatusBarEntryData) error {
	if jsonOutput {
		// JSON mode: no intermediate display, structured JSON output.
		result, err := appInstance.RunOnceResult(ctx, prompt)
		if err != nil {
			writeJSONError(err)
			return err
		}
		data, err := buildJSONOutput(result, modelName)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON output: %w", err)
		}
		fmt.Println(string(data))
	} else if quiet {
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
		return runInteractiveModeBubbleTea(ctx, appInstance, modelName, providerName, loadingMessage, serverNames, toolNames, mcpToolCount, extensionToolCount, usageTracker, extCommands, contextPaths, skillItems, getWidgets, getHeader, getFooter, getToolRenderer, getEditorInterceptor, getUIVisibility, getStatusBarEntries)
	}

	return nil
}

// ---------------------------------------------------------------------------
// JSON output helpers (--json mode)
// ---------------------------------------------------------------------------

// buildJSONOutput converts a TurnResult into a structured JSON byte slice
// suitable for machine consumption.
func buildJSONOutput(result *kit.TurnResult, model string) ([]byte, error) {
	type jsonPart struct {
		Type string `json:"type"`
		Data any    `json:"data"`
	}
	type jsonMessage struct {
		Role  string     `json:"role"`
		Parts []jsonPart `json:"parts"`
	}
	type jsonUsage struct {
		InputTokens         int64 `json:"input_tokens"`
		OutputTokens        int64 `json:"output_tokens"`
		TotalTokens         int64 `json:"total_tokens"`
		CacheReadTokens     int64 `json:"cache_read_tokens"`
		CacheCreationTokens int64 `json:"cache_creation_tokens"`
	}
	type jsonEnvelope struct {
		Response string        `json:"response"`
		Model    string        `json:"model"`
		Usage    *jsonUsage    `json:"usage,omitempty"`
		Messages []jsonMessage `json:"messages"`
	}

	out := jsonEnvelope{
		Response: result.Response,
		Model:    model,
	}

	if result.TotalUsage != nil {
		out.Usage = &jsonUsage{
			InputTokens:         result.TotalUsage.InputTokens,
			OutputTokens:        result.TotalUsage.OutputTokens,
			TotalTokens:         result.TotalUsage.TotalTokens,
			CacheReadTokens:     result.TotalUsage.CacheReadTokens,
			CacheCreationTokens: result.TotalUsage.CacheCreationTokens,
		}
	}

	for _, fmsg := range result.Messages {
		converted := kit.ConvertFromFantasyMessage(fmsg)
		m := jsonMessage{Role: string(converted.Role)}
		for _, p := range converted.Parts {
			switch c := p.(type) {
			case kit.TextContent:
				m.Parts = append(m.Parts, jsonPart{Type: "text", Data: c})
			case kit.ToolCall:
				m.Parts = append(m.Parts, jsonPart{Type: "tool_call", Data: c})
			case kit.ToolResult:
				m.Parts = append(m.Parts, jsonPart{Type: "tool_result", Data: c})
			case kit.ReasoningContent:
				m.Parts = append(m.Parts, jsonPart{Type: "reasoning", Data: c})
			case kit.Finish:
				m.Parts = append(m.Parts, jsonPart{Type: "finish", Data: c})
			}
		}
		out.Messages = append(out.Messages, m)
	}

	return json.MarshalIndent(out, "", "  ")
}

// writeJSONError writes a JSON-formatted error object to stdout so that
// callers using --json always receive parseable output.
func writeJSONError(err error) {
	type jsonError struct {
		Error string `json:"error"`
	}
	data, _ := json.MarshalIndent(jsonError{Error: err.Error()}, "", "  ")
	fmt.Fprintln(os.Stderr, string(data))
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
func runInteractiveModeBubbleTea(_ context.Context, appInstance *app.App, modelName, providerName, loadingMessage string, serverNames, toolNames []string, mcpToolCount, extensionToolCount int, usageTracker *ui.UsageTracker, extCommands []ui.ExtensionCommand, contextPaths []string, skillItems []ui.SkillItem, getWidgets func(string) []ui.WidgetData, getHeader, getFooter func() *ui.WidgetData, getToolRenderer func(string) *ui.ToolRendererData, getEditorInterceptor func() *ui.EditorInterceptor, getUIVisibility func() *ui.UIVisibility, getStatusBarEntries func() []ui.StatusBarEntryData) error {
	// Determine terminal size; fall back gracefully.
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termWidth == 0 {
		termWidth = 80
		termHeight = 24
	}

	appModel := ui.NewAppModel(appInstance, ui.AppModelOptions{
		CompactMode:          viper.GetBool("compact"),
		ModelName:            modelName,
		ProviderName:         providerName,
		LoadingMessage:       loadingMessage,
		Width:                termWidth,
		Height:               termHeight,
		ServerNames:          serverNames,
		ToolNames:            toolNames,
		MCPToolCount:         mcpToolCount,
		ExtensionToolCount:   extensionToolCount,
		UsageTracker:         usageTracker,
		ExtensionCommands:    extCommands,
		ContextPaths:         contextPaths,
		SkillItems:           skillItems,
		GetWidgets:           getWidgets,
		GetHeader:            getHeader,
		GetFooter:            getFooter,
		GetToolRenderer:      getToolRenderer,
		GetEditorInterceptor: getEditorInterceptor,
		GetUIVisibility:      getUIVisibility,
		GetStatusBarEntries:  getStatusBarEntries,
	})

	// Print startup info to stdout before Bubble Tea takes over the screen.
	appModel.PrintStartupInfo()

	program := tea.NewProgram(appModel)

	// Register the program with the app layer so agent events are sent to the TUI.
	appInstance.SetProgram(program)

	_, runErr := program.Run()
	return runErr
}
