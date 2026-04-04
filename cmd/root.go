package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/auth"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/prompts"
	"github.com/mark3labs/kit/internal/ui"
	"github.com/mark3labs/kit/internal/ui/commands"
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
	positionalPrompt string // set by processPositionalArgs from CLI positional args
	quietFlag        bool
	jsonFlag         bool
	noExitFlag       bool
	maxSteps         int
	streamFlag       bool // Enable streaming output
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
	thinkingLevel string

	// Ollama-specific parameters
	numGPU  int32
	mainGPU int32

	// Extensions control
	noExtensionsFlag bool
	extensionPaths   []string

	// TLS configuration
	tlsSkipVerify bool

	// Prompt templates
	promptTemplatePaths []string
	noPromptTemplates   bool

	// Preference restoration flags — set in RunE after cobra parses, used
	// in runNormalMode to decide whether to apply saved preferences.
	modelFlagChanged    bool
	thinkingFlagChanged bool
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
	Use:   "kit [@file...] [prompt]",
	Short: "Chat with AI models through a unified interface",
	Long:  `KIT (Knowledge Inference Tool) — A lightweight AI agent for coding`,
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse positional args: @-prefixed args are file attachments,
		// remaining args form the prompt (like Pi: kit @code.ts "Review this").
		if len(args) > 0 {
			processPositionalArgs(args)
		}
		// Record whether --model / --thinking-level were explicitly set by the
		// user so that runNormalMode can fall back to saved preferences when
		// they weren't. Must be captured here (after cobra parses) and before
		// runKit because rootCmd can't be referenced inside runNormalMode
		// without creating an initialization cycle.
		if f := cmd.PersistentFlags().Lookup("model"); f != nil {
			modelFlagChanged = f.Changed
		}
		if f := cmd.PersistentFlags().Lookup("thinking-level"); f != nil {
			thinkingFlagChanged = f.Changed
		}
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
	// Rebuild the model registry now that viper has the config loaded,
	// so customModels defined in the config file are picked up.
	models.ReloadGlobalRegistry()
}

// LoadConfigWithEnvSubstitution loads a config file with environment variable
// substitution. Delegates to the SDK implementation.
func LoadConfigWithEnvSubstitution(configPath string) error {
	return kit.LoadConfigWithEnvSubstitution(configPath)
}

// adaptiveOrDefault converts a config.AdaptiveColor to a resolved color.Color,
// falling back to fallback when both Light and Dark are empty.
func adaptiveOrDefault(ac config.AdaptiveColor, fallback color.Color) color.Color {
	if ac.Light == "" && ac.Dark == "" {
		return fallback
	}
	return ui.AdaptiveColor(ac.Light, ac.Dark)
}

func configToUiTheme(cfg config.Theme) ui.Theme {
	def := ui.DefaultTheme()
	return ui.Theme{
		Primary:     adaptiveOrDefault(cfg.Primary, def.Primary),
		Secondary:   adaptiveOrDefault(cfg.Secondary, def.Secondary),
		Success:     adaptiveOrDefault(cfg.Success, def.Success),
		Warning:     adaptiveOrDefault(cfg.Warning, def.Warning),
		Error:       adaptiveOrDefault(cfg.Error, def.Error),
		Info:        adaptiveOrDefault(cfg.Info, def.Info),
		Text:        adaptiveOrDefault(cfg.Text, def.Text),
		Muted:       adaptiveOrDefault(cfg.Muted, def.Muted),
		VeryMuted:   adaptiveOrDefault(cfg.VeryMuted, def.VeryMuted),
		Background:  adaptiveOrDefault(cfg.Background, def.Background),
		Border:      adaptiveOrDefault(cfg.Border, def.Border),
		MutedBorder: adaptiveOrDefault(cfg.MutedBorder, def.MutedBorder),
		System:      adaptiveOrDefault(cfg.System, def.System),
		Tool:        adaptiveOrDefault(cfg.Tool, def.Tool),
		Accent:      adaptiveOrDefault(cfg.Accent, def.Accent),
		Highlight:   adaptiveOrDefault(cfg.Highlight, def.Highlight),

		DiffInsertBg:  adaptiveOrDefault(cfg.DiffInsertBg, def.DiffInsertBg),
		DiffDeleteBg:  adaptiveOrDefault(cfg.DiffDeleteBg, def.DiffDeleteBg),
		DiffEqualBg:   adaptiveOrDefault(cfg.DiffEqualBg, def.DiffEqualBg),
		DiffMissingBg: adaptiveOrDefault(cfg.DiffMissingBg, def.DiffMissingBg),

		CodeBg:   adaptiveOrDefault(cfg.CodeBg, def.CodeBg),
		GutterBg: adaptiveOrDefault(cfg.GutterBg, def.GutterBg),
		WriteBg:  adaptiveOrDefault(cfg.WriteBg, def.WriteBg),

		Markdown: ui.MarkdownThemeColors{
			Text:    adaptiveOrDefault(cfg.Markdown.Text, def.Markdown.Text),
			Muted:   adaptiveOrDefault(cfg.Markdown.Muted, def.Markdown.Muted),
			Heading: adaptiveOrDefault(cfg.Markdown.Heading, def.Markdown.Heading),
			Emph:    adaptiveOrDefault(cfg.Markdown.Emph, def.Markdown.Emph),
			Strong:  adaptiveOrDefault(cfg.Markdown.Strong, def.Markdown.Strong),
			Link:    adaptiveOrDefault(cfg.Markdown.Link, def.Markdown.Link),
			Code:    adaptiveOrDefault(cfg.Markdown.Code, def.Markdown.Code),
			Error:   adaptiveOrDefault(cfg.Markdown.Error, def.Markdown.Error),
			Keyword: adaptiveOrDefault(cfg.Markdown.Keyword, def.Markdown.Keyword),
			String:  adaptiveOrDefault(cfg.Markdown.String, def.Markdown.String),
			Number:  adaptiveOrDefault(cfg.Markdown.Number, def.Markdown.Number),
			Comment: adaptiveOrDefault(cfg.Markdown.Comment, def.Markdown.Comment),
		},
	}
}

// kitBanner returns the KIT ASCII art title with KITT scanner lights.
// Delegates to ui.KitBanner() which owns the logo rendering.
func kitBanner() string {
	return ui.KitBanner()
}

func init() {
	cobra.OnInitialize(InitConfig)

	rootCmd.Long = kitBanner() + "\n\n" + rootCmd.Long

	var theme config.Theme
	err := config.FilepathOr("theme", &theme)
	if err == nil && viper.InConfig("theme") {
		uiTheme := configToUiTheme(theme)
		ui.SetTheme(uiTheme)
	} else if pref := ui.LoadThemePreference(); pref != "" {
		// No explicit theme in config — fall back to persisted preference.
		_ = ui.ApplyThemeWithoutSave(pref)
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
		BoolVar(&quietFlag, "quiet", false, "suppress all output (non-interactive mode only)")
	rootCmd.PersistentFlags().
		BoolVar(&jsonFlag, "json", false, "output response as JSON (non-interactive mode only)")
	rootCmd.PersistentFlags().
		BoolVar(&noExitFlag, "no-exit", false, "enter interactive mode after non-interactive prompt completes")
	rootCmd.PersistentFlags().
		IntVar(&maxSteps, "max-steps", 0, "maximum number of agent steps (0 for unlimited)")
	rootCmd.PersistentFlags().
		BoolVar(&streamFlag, "stream", true, "enable streaming output for faster response display")
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

	// Prompt template flags
	flags.StringArrayVar(&promptTemplatePaths, "prompt-template", nil, "load prompt template file or directory (repeatable)")
	flags.BoolVar(&noPromptTemplates, "no-prompt-templates", false, "disable prompt template discovery")

	// Model generation parameters
	flags.IntVar(&maxTokens, "max-tokens", 4096, "maximum number of tokens in the response")
	flags.Float32Var(&temperature, "temperature", 0.7, "controls randomness in responses (0.0-1.0)")
	flags.Float32Var(&topP, "top-p", 0.95, "controls diversity via nucleus sampling (0.0-1.0)")
	flags.Int32Var(&topK, "top-k", 40, "controls diversity by limiting top K tokens to sample from")
	flags.StringSliceVar(&stopSequences, "stop-sequences", nil, "custom stop sequences (comma-separated)")
	flags.StringVar(&thinkingLevel, "thinking-level", "off", "extended thinking level: off, minimal, low, medium, high")

	// Ollama-specific parameters
	flags.Int32Var(&numGPU, "num-gpu-layers", -1, "number of model layers to offload to GPU for Ollama models (-1 for auto-detect)")
	_ = flags.MarkHidden("num-gpu-layers") // Advanced option, hidden from help
	flags.Int32Var(&mainGPU, "main-gpu", 0, "main GPU device to use for Ollama models")

	// Bind flags to viper for config file support
	_ = viper.BindPFlag("system-prompt", rootCmd.PersistentFlags().Lookup("system-prompt"))
	_ = viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("max-steps", rootCmd.PersistentFlags().Lookup("max-steps"))
	_ = viper.BindPFlag("stream", rootCmd.PersistentFlags().Lookup("stream"))
	_ = viper.BindPFlag("auto-compact", rootCmd.PersistentFlags().Lookup("auto-compact"))

	_ = viper.BindPFlag("provider-url", rootCmd.PersistentFlags().Lookup("provider-url"))
	_ = viper.BindPFlag("provider-api-key", rootCmd.PersistentFlags().Lookup("provider-api-key"))
	_ = viper.BindPFlag("max-tokens", rootCmd.PersistentFlags().Lookup("max-tokens"))
	_ = viper.BindPFlag("temperature", rootCmd.PersistentFlags().Lookup("temperature"))
	_ = viper.BindPFlag("top-p", rootCmd.PersistentFlags().Lookup("top-p"))
	_ = viper.BindPFlag("top-k", rootCmd.PersistentFlags().Lookup("top-k"))
	_ = viper.BindPFlag("stop-sequences", rootCmd.PersistentFlags().Lookup("stop-sequences"))
	_ = viper.BindPFlag("thinking-level", rootCmd.PersistentFlags().Lookup("thinking-level"))
	_ = viper.BindPFlag("num-gpu-layers", rootCmd.PersistentFlags().Lookup("num-gpu-layers"))
	_ = viper.BindPFlag("main-gpu", rootCmd.PersistentFlags().Lookup("main-gpu"))
	_ = viper.BindPFlag("tls-skip-verify", rootCmd.PersistentFlags().Lookup("tls-skip-verify"))
	_ = viper.BindPFlag("no-extensions", rootCmd.PersistentFlags().Lookup("no-extensions"))
	_ = viper.BindPFlag("extension", rootCmd.PersistentFlags().Lookup("extension"))
	_ = viper.BindPFlag("prompt-template", rootCmd.PersistentFlags().Lookup("prompt-template"))
	_ = viper.BindPFlag("no-prompt-templates", rootCmd.PersistentFlags().Lookup("no-prompt-templates"))

	// Defaults are already set in flag definitions, no need to duplicate in viper

	// Add subcommands
	rootCmd.AddCommand(authCmd)
}

// processPositionalArgs separates positional CLI arguments into @file
// attachments and prompt text. File content is read and prepended to
// positionalPrompt so the agent receives it. Positional args are the primary
// way to run non-interactive mode:
//
//	kit "Explain this codebase"
//	kit @code.ts @test.ts "Review these files"
func processPositionalArgs(args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	var fileTokens []string
	var promptParts []string

	for _, arg := range args {
		if strings.HasPrefix(arg, "@") && len(arg) > 1 {
			fileTokens = append(fileTokens, arg)
		} else {
			promptParts = append(promptParts, arg)
		}
	}

	// Build file content prefix from @file arguments.
	var fileContent strings.Builder
	for _, token := range fileTokens {
		expanded := ui.ProcessFileAttachments(token, cwd)
		if expanded != token {
			// File was resolved — add it.
			fileContent.WriteString(expanded)
			fileContent.WriteString("\n\n")
		}
	}

	// Combine: positional prompt text is appended to any existing --prompt
	// value (for backward compat with subprocess invocations).
	if len(promptParts) > 0 {
		extra := strings.Join(promptParts, " ")
		if positionalPrompt != "" {
			positionalPrompt = positionalPrompt + " " + extra
		} else {
			positionalPrompt = extra
		}
	}

	// Prepend file content to the prompt.
	if fileContent.Len() > 0 {
		if positionalPrompt == "" {
			positionalPrompt = strings.TrimSpace(fileContent.String())
		} else {
			positionalPrompt = strings.TrimSpace(fileContent.String()) + "\n\n" + positionalPrompt
		}
	}
}

func runKit(ctx context.Context) error {
	return runNormalMode(ctx)
}

// extensionCommandsForUI converts extension-registered CommandDefs into the
// commands.ExtensionCommand type used by the interactive TUI. Command names are
// normalised to start with "/" so they integrate with the slash-command
// autocomplete and dispatch pipeline.
func extensionCommandsForUI(k *kit.Kit) []commands.ExtensionCommand {
	defs := k.Extensions().Commands()
	if len(defs) == 0 {
		return nil
	}
	cmds := make([]commands.ExtensionCommand, 0, len(defs))
	for _, d := range defs {
		name := d.Name
		if len(name) > 0 && name[0] != '/' {
			name = "/" + name
		}
		ec := commands.ExtensionCommand{
			Name:        name,
			Description: d.Description,
			Execute: func(args string) (string, error) {
				return d.Execute(args, k.Extensions().GetContext())
			},
		}
		if d.Complete != nil {
			ec.Complete = func(prefix string) []string {
				return d.Complete(prefix, k.Extensions().GetContext())
			}
		}
		cmds = append(cmds, ec)
	}
	return cmds
}

// widgetProviderForUI returns a function that converts extension widgets to
// ui.WidgetData for the given placement. Returns nil if extensions are
// disabled, which is safe — the UI treats a nil GetWidgets as "no widgets".
func widgetProviderForUI(k *kit.Kit) func(string) []ui.WidgetData {
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func(placement string) []ui.WidgetData {
		configs := k.Extensions().GetWidgets(extensions.WidgetPlacement(placement))
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

// headerFooterProviderForUI returns a provider func that maps an
// extensions.HeaderFooterConfig getter into the ui.WidgetData shape
// expected by AppModel. The getter argument selects header vs footer.
func headerFooterProviderForUI(k *kit.Kit, getter func() *extensions.HeaderFooterConfig) func() *ui.WidgetData {
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func() *ui.WidgetData {
		cfg := getter()
		if cfg == nil {
			return nil
		}
		return &ui.WidgetData{
			Text:        cfg.Content.Text,
			Markdown:    cfg.Content.Markdown,
			BorderColor: cfg.Style.BorderColor,
			NoBorder:    cfg.Style.NoBorder,
		}
	}
}

// headerProviderForUI returns a function that converts the extension header
// to a *ui.WidgetData for the TUI. Returns nil if extensions are disabled,
// which is safe — the UI treats a nil GetHeader as "no header".
func headerProviderForUI(k *kit.Kit) func() *ui.WidgetData {
	return headerFooterProviderForUI(k, func() *extensions.HeaderFooterConfig {
		return k.Extensions().GetHeader()
	})
}

// toolRendererProviderForUI returns a function that converts extension tool
// renderers to ui.ToolRendererData for the TUI. Returns nil if extensions are
// disabled, which is safe — the UI treats a nil GetToolRenderer as "no
// custom renderers".
func toolRendererProviderForUI(k *kit.Kit) func(string) *ui.ToolRendererData {
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func(toolName string) *ui.ToolRendererData {
		config := k.Extensions().GetToolRenderer(toolName)
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
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func() *ui.EditorInterceptor {
		config := k.Extensions().GetEditor()
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
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func() *ui.UIVisibility {
		v := k.Extensions().GetUIVisibility()
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
	return headerFooterProviderForUI(k, func() *extensions.HeaderFooterConfig {
		return k.Extensions().GetFooter()
	})
}

// statusBarProviderForUI returns a function that fetches extension status bar
// entries and converts them to ui.StatusBarEntryData for the TUI. Returns nil
// if extensions are disabled, which is safe — the TUI treats a nil
// GetStatusBarEntries as "no extension entries".
func statusBarProviderForUI(k *kit.Kit) func() []ui.StatusBarEntryData {
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func() []ui.StatusBarEntryData {
		entries := k.Extensions().GetStatusEntries()
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

// beforeForkProviderForUI returns a callback that emits a BeforeFork event
// and returns (cancelled, reason). Returns nil if extensions are disabled —
// the UI treats nil as "no hook".
func beforeForkProviderForUI(k *kit.Kit) func(string, bool, string) (bool, string) {
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func(targetID string, isUserMsg bool, userText string) (bool, string) {
		return k.Extensions().EmitBeforeFork(targetID, isUserMsg, userText)
	}
}

// beforeSessionSwitchProviderForUI returns a callback that emits a
// BeforeSessionSwitch event and returns (cancelled, reason). Returns nil
// if extensions are disabled — the UI treats nil as "no hook".
func beforeSessionSwitchProviderForUI(k *kit.Kit) func(string) (bool, string) {
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func(switchReason string) (bool, string) {
		return k.Extensions().EmitBeforeSessionSwitch(switchReason)
	}
}

// globalShortcutsProviderForUI returns a callback that queries the extension
// runner for registered keyboard shortcuts. Returns nil if extensions are
// disabled — the UI treats nil as "no shortcuts".
func globalShortcutsProviderForUI(k *kit.Kit) func() map[string]func() {
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func() map[string]func() {
		return k.Extensions().GetShortcuts()
	}
}

func runNormalMode(ctx context.Context) error {
	// Validate flag combinations
	if quietFlag && positionalPrompt == "" {
		return fmt.Errorf("--quiet requires a prompt (e.g. kit \"your question\" --quiet)")
	}
	if jsonFlag && positionalPrompt == "" {
		return fmt.Errorf("--json requires a prompt (e.g. kit \"your question\" --json)")
	}
	if jsonFlag && noExitFlag {
		return fmt.Errorf("--json and --no-exit flags cannot be used together")
	}
	if noExitFlag && positionalPrompt == "" {
		return fmt.Errorf("--no-exit requires a prompt (e.g. kit \"your question\" --no-exit)")
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

	// Restore persisted model preference when no explicit --model flag or
	// config file model is set. Precedence: CLI flag > config file > saved
	// preference > built-in default. This mirrors how themes are persisted.
	// Skip custom/* models unless --provider-url is also provided, since the
	// custom provider requires a URL that was only valid for the previous session.
	if !modelFlagChanged && !viper.InConfig("model") {
		if pref := ui.LoadModelPreference(); pref != "" {
			if strings.HasPrefix(pref, "custom/") && viper.GetString("provider-url") == "" {
				// Don't restore custom models without a provider URL
			} else {
				viper.Set("model", pref)
			}
		}
	}

	// Restore persisted thinking level preference (same precedence chain).
	if !thinkingFlagChanged && !viper.InConfig("thinking-level") {
		if pref := ui.LoadThinkingLevelPreference(); pref != "" {
			viper.Set("thinking-level", pref)
		}
	}

	// When --provider-url is set but no explicit --model was provided,
	// default to "custom/custom" so the user doesn't need to remember a
	// provider/model pair for custom OpenAI-compatible endpoints.
	// This intentionally overrides saved preferences but respects config-file
	// models — if you specify a model in ~/.kit.yml, it will be used with
	// custom/custom's provider routing.
	if viper.GetString("provider-url") != "" && !modelFlagChanged && !viper.InConfig("model") {
		viper.Set("model", "custom/custom")
	}

	// When --provider-url is set with an explicit --model that lacks a provider
	// prefix (no "/"), auto-prefix with "custom/" for OpenAI-compatible endpoints.
	if viper.GetString("provider-url") != "" && modelFlagChanged {
		model := viper.GetString("model")
		if model != "" && !strings.Contains(model, "/") {
			viper.Set("model", "custom/"+model)
		}
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
			tempCli, tempErr := ui.NewCLI(viper.GetBool("debug"))
			if tempErr == nil {
				return tempCli.ShowSpinner(fn)
			}
			return fn()
		}
	}

	// Build Kit options from CLI flags and create the SDK instance.
	// kit.New() handles: config → skills → agent → session → extension bridge.
	authHandler, authErr := kit.NewCLIMCPAuthHandler()
	if authErr != nil {
		// Non-fatal: OAuth just won't be available for remote MCP servers.
		fmt.Fprintf(os.Stderr, "Warning: Failed to create OAuth handler: %v\n", authErr)
	}

	kitOpts := &kit.Options{
		Quiet:          quietFlag,
		Debug:          debugMode,
		NoSession:      noSessionFlag,
		Continue:       continueFlag,
		SessionPath:    sessionPath,
		AutoCompact:    autoCompactFlag,
		MCPAuthHandler: authHandler,
		CLI: &kit.CLIOptions{
			MCPConfig:         mcpConfig,
			ShowSpinner:       true,
			SpinnerFunc:       spinnerFunc,
			UseBufferedLogger: true,
		},
	}
	if resumeFlag {
		// When --resume is combined with interactive mode, the TUI session
		// picker will be shown at startup. For non-interactive mode, fall
		// back to auto-selecting the most recent session.
		if positionalPrompt != "" {
			sessions, _ := kit.ListSessions("")
			if len(sessions) > 0 {
				kitOpts.SessionPath = sessions[0].Path
			}
		}
		// Interactive mode: ShowSessionPicker is set below on AppModelOptions.
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
	if positionalPrompt != "" {
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
	var messages []kit.LLMMessage
	if treeSession != nil {
		messages = treeSession.GetLLMMessages()
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

	// Wire OAuth handler to route messages through the TUI once it's running.
	if authHandler != nil {
		authHandler.NotifyFunc = func(serverName, message string) {
			appInstance.PrintFromExtension("info", message)
		}
	}

	// Buffer for extension messages during startup (printed after startup banner).
	var startupExtensionMessages []string

	// Set up extension context and emit SessionStart.
	if kitInstance.Extensions().HasExtensions() {
		cwd, _ := os.Getwd()
		kitInstance.Extensions().SetContext(extensions.Context{
			CWD:         cwd,
			Model:       modelName,
			Interactive: positionalPrompt == "",
			Print: func(text string) {
				// Capture messages during startup, print after startup banner.
				startupExtensionMessages = append(startupExtensionMessages, text)
			},
			PrintInfo: func(text string) {
				startupExtensionMessages = append(startupExtensionMessages, text)
			},
			PrintError: func(text string) {
				startupExtensionMessages = append(startupExtensionMessages, text)
			},
			PrintBlock:    appInstance.PrintBlockFromExtension,
			SendMessage:   func(text string) { appInstance.Run(text) },
			CancelAndSend: func(text string) { appInstance.InterruptAndSend(text) },
			Abort:         func() { appInstance.Abort() },
			IsIdle:        func() bool { return !appInstance.IsBusy() },
			Compact: func(cfg extensions.CompactConfig) error {
				return appInstance.CompactAsync(cfg.CustomInstructions, cfg.OnComplete, cfg.OnError)
			},
			SendMultimodalMessage: func(text string, files []extensions.FilePart) {
				parts := make([]kit.LLMFilePart, len(files))
				for i, f := range files {
					parts[i] = kit.LLMFilePart{
						Filename:  f.Filename,
						Data:      f.Data,
						MediaType: f.MediaType,
					}
				}
				appInstance.RunWithFiles(text, parts)
			},
			GetSessionUsage: func() extensions.SessionUsage {
				if usageTracker == nil {
					return extensions.SessionUsage{}
				}
				stats := usageTracker.GetSessionStats()
				return extensions.SessionUsage{
					TotalInputTokens:      stats.TotalInputTokens,
					TotalOutputTokens:     stats.TotalOutputTokens,
					TotalCacheReadTokens:  stats.TotalCacheReadTokens,
					TotalCacheWriteTokens: stats.TotalCacheWriteTokens,
					TotalCost:             stats.TotalCost,
					RequestCount:          stats.RequestCount,
				}
			},
			Exit: func() { appInstance.QuitFromExtension() },
			SetWidget: func(config extensions.WidgetConfig) {
				kitInstance.Extensions().SetWidget(config)
				go appInstance.NotifyWidgetUpdate()
			},
			RemoveWidget: func(id string) {
				kitInstance.Extensions().RemoveWidget(id)
				go appInstance.NotifyWidgetUpdate()
			},
			SetHeader: func(config extensions.HeaderFooterConfig) {
				kitInstance.Extensions().SetHeader(config)
				go appInstance.NotifyWidgetUpdate()
			},
			RemoveHeader: func() {
				kitInstance.Extensions().RemoveHeader()
				go appInstance.NotifyWidgetUpdate()
			},
			SetFooter: func(config extensions.HeaderFooterConfig) {
				kitInstance.Extensions().SetFooter(config)
				go appInstance.NotifyWidgetUpdate()
			},
			RemoveFooter: func() {
				kitInstance.Extensions().RemoveFooter()
				go appInstance.NotifyWidgetUpdate()
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
				kitInstance.Extensions().SetUIVisibility(v)
				go appInstance.NotifyWidgetUpdate()
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
				kitInstance.Extensions().SetEditor(config)
				// Always use a goroutine for NotifyWidgetUpdate: prog.Send()
				// deadlocks if called synchronously from inside BubbleTea's
				// Update() handler. All call sites use go-routines uniformly.
				go appInstance.NotifyWidgetUpdate()
			},
			ResetEditor: func() {
				kitInstance.Extensions().ResetEditor()
				go appInstance.NotifyWidgetUpdate()
			},
			GetMessages: func() []extensions.SessionMessage {
				return kitInstance.Extensions().GetSessionMessages()
			},
			GetSessionPath: func() string {
				return kitInstance.GetSessionPath()
			},
			AppendEntry: func(entryType string, data string) (string, error) {
				return kitInstance.Extensions().AppendEntry(entryType, data)
			},
			GetEntries: func(entryType string) []extensions.ExtensionEntry {
				return kitInstance.Extensions().GetEntries(entryType)
			},
			SetEditorText: func(text string) {
				appInstance.SetEditorTextFromExtension(text)
			},
			SetStatus: func(key string, text string, priority int) {
				kitInstance.Extensions().SetStatus(extensions.StatusBarEntry{
					Key:      key,
					Text:     text,
					Priority: priority,
				})
				go appInstance.NotifyWidgetUpdate()
			},
			RemoveStatus: func(key string) {
				kitInstance.Extensions().RemoveStatus(key)
				go appInstance.NotifyWidgetUpdate()
			},
			GetOption: func(name string) string {
				return kitInstance.Extensions().GetOption(name)
			},
			SetOption: func(name string, value string) {
				kitInstance.Extensions().SetOption(name, value)
			},
			SetModel: func(modelString string) error {
				// Capture previous model for the ModelChange event.
				previousModel := kitInstance.Extensions().GetContext().Model
				err := kitInstance.SetModel(context.Background(), modelString)
				if err != nil {
					return err
				}
				// Notify TUI so it updates model in status bar.
				p, m, _ := models.ParseModelString(modelString)
				appInstance.NotifyModelChanged(p, m)
				// Update the context's Model field so handlers see it.
				kitInstance.Extensions().UpdateContextModel(modelString)
				// Fire OnModelChange event to extensions.
				kitInstance.Extensions().EmitModelChange(modelString, previousModel, "extension")
				// Update usage tracker with new model info for correct token counting.
				if usageTracker != nil {
					newProvider, newModel, _ := models.ParseModelString(modelString)
					if newProvider != "unknown" && newModel != "unknown" && newProvider != "ollama" {
						registry := models.GetGlobalRegistry()
						if modelInfo := registry.LookupModel(newProvider, newModel); modelInfo != nil {
							// Check OAuth status for Anthropic models
							isOAuth := false
							if newProvider == "anthropic" {
								_, source, err := auth.GetAnthropicAPIKey(viper.GetString("provider-api-key"))
								if err == nil && strings.HasPrefix(source, "stored OAuth") {
									isOAuth = true
								}
							}
							usageTracker.UpdateModelInfo(modelInfo, newProvider, isOAuth)
						}
					}
				}
				return nil
			},
			GetAvailableModels: func() []extensions.ModelInfoEntry {
				return kitInstance.GetAvailableModels()
			},
			EmitCustomEvent: func(name string, data string) {
				kitInstance.Extensions().EmitCustomEvent(name, data)
			},
			Complete: func(req extensions.CompleteRequest) (extensions.CompleteResponse, error) {
				return kitInstance.ExecuteCompletion(context.Background(), req)
			},
			SuspendTUI: func(callback func()) error {
				return appInstance.SuspendTUI(callback)
			},
			RenderMessage: func(rendererName, content string) {
				renderer := kitInstance.Extensions().GetMessageRenderer(rendererName)
				if renderer == nil || renderer.Render == nil {
					appInstance.PrintFromExtension("", content)
					return
				}
				w, _, _ := term.GetSize(int(os.Stdout.Fd()))
				if w == 0 {
					w = 80
				}
				rendered := renderer.Render(content, w)
				appInstance.PrintFromExtension("", rendered)
			},
			ReloadExtensions: func() error {
				err := kitInstance.Extensions().Reload()
				if err != nil {
					return err
				}
				// Notify TUI that widgets/status/commands may have changed.
				go appInstance.NotifyWidgetUpdate()
				return nil
			},
			GetAllTools: func() []extensions.ToolInfo {
				return kitInstance.Extensions().GetToolInfos()
			},
			SetActiveTools: func(names []string) {
				kitInstance.Extensions().SetActiveTools(names)
			},
			RegisterTheme: func(name string, config extensions.ThemeColorConfig) {
				tc := func(c extensions.ThemeColor) [2]string { return [2]string{c.Light, c.Dark} }
				ui.RegisterThemeFromConfig(name,
					tc(config.Primary), tc(config.Secondary),
					tc(config.Success), tc(config.Warning),
					tc(config.Error), tc(config.Info),
					tc(config.Text), tc(config.Muted),
					tc(config.VeryMuted), tc(config.Background),
					tc(config.Border), tc(config.MutedBorder),
					tc(config.System), tc(config.Tool),
					tc(config.Accent), tc(config.Highlight),
					tc(config.MdHeading), tc(config.MdLink),
					tc(config.MdKeyword), tc(config.MdString),
					tc(config.MdNumber), tc(config.MdComment),
				)
			},
			SetTheme: func(name string) error {
				return ui.ApplyTheme(name)
			},
			ListThemes: func() []string {
				return ui.ListThemes()
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
			SpawnSubagent: func(config extensions.SubagentConfig) (*extensions.SubagentHandle, *extensions.SubagentResult, error) {
				// In-process subagent via SDK.
				sdkCfg := kit.SubagentConfig{
					Prompt:       config.Prompt,
					Model:        config.Model,
					SystemPrompt: config.SystemPrompt,
					Timeout:      config.Timeout,
					NoSession:    config.NoSession,
				}
				// Bridge SDK events to extension SubagentEvents.
				if config.OnEvent != nil {
					sdkCfg.OnEvent = func(e kit.Event) {
						se := sdkEventToSubagentEvent(e)
						if se.Type != "" {
							config.OnEvent(se)
						}
					}
				}
				result, err := kitInstance.Subagent(ctx, sdkCfg)
				if result == nil {
					return nil, &extensions.SubagentResult{Error: err}, err
				}
				extResult := &extensions.SubagentResult{
					Response:  result.Response,
					Error:     err,
					SessionID: result.SessionID,
					Elapsed:   result.Elapsed,
				}
				if result.Usage != nil {
					extResult.Usage = &extensions.SubagentUsage{
						InputTokens:  result.Usage.InputTokens,
						OutputTokens: result.Usage.OutputTokens,
					}
				}
				return nil, extResult, err
			},

			// -------------------------------------------------------------------------
			// Tree Navigation API (Phase 1 Bridge)
			// -------------------------------------------------------------------------
			GetTreeNode: func(entryID string) *extensions.TreeNode {
				node := kitInstance.GetTreeNode(entryID)
				if node == nil {
					return nil
				}
				return &extensions.TreeNode{
					ID:        node.ID,
					ParentID:  node.ParentID,
					Type:      node.Type,
					Role:      node.Role,
					Content:   node.Content,
					Model:     node.Model,
					Provider:  node.Provider,
					Timestamp: node.Timestamp,
					Children:  node.Children,
				}
			},
			GetCurrentBranch: func() []extensions.TreeNode {
				nodes := kitInstance.GetCurrentBranch()
				result := make([]extensions.TreeNode, len(nodes))
				for i, n := range nodes {
					result[i] = extensions.TreeNode{
						ID:        n.ID,
						ParentID:  n.ParentID,
						Type:      n.Type,
						Role:      n.Role,
						Content:   n.Content,
						Model:     n.Model,
						Provider:  n.Provider,
						Timestamp: n.Timestamp,
						Children:  n.Children,
					}
				}
				return result
			},
			GetChildren: kitInstance.GetChildren,
			NavigateTo: func(entryID string) extensions.TreeNavigationResult {
				err := kitInstance.NavigateTo(entryID)
				if err != nil {
					return extensions.TreeNavigationResult{Success: false, Error: err.Error()}
				}
				return extensions.TreeNavigationResult{Success: true}
			},
			SummarizeBranch: func(fromID, toID string) string {
				summary, _ := kitInstance.SummarizeBranch(fromID, toID)
				return summary
			},
			CollapseBranch: func(fromID, toID, summary string) extensions.TreeNavigationResult {
				err := kitInstance.CollapseBranch(fromID, toID, summary)
				if err != nil {
					return extensions.TreeNavigationResult{Success: false, Error: err.Error()}
				}
				return extensions.TreeNavigationResult{Success: true}
			},

			// -------------------------------------------------------------------------
			// Skill Loading API (Phase 2 Bridge)
			// -------------------------------------------------------------------------
			LoadSkill: func(path string) (*extensions.Skill, string) {
				s, err := kitInstance.LoadSkillForExtension(path)
				return s, err
			},
			LoadSkillsFromDir: func(dir string) extensions.SkillLoadResult {
				return kitInstance.LoadSkillsFromDirForExtension(dir)
			},
			DiscoverSkills: func() extensions.SkillLoadResult {
				skills := kitInstance.DiscoverSkillsForExtension()
				return extensions.SkillLoadResult{Skills: skills}
			},
			InjectSkillAsContext: func(skillName string) string {
				// Find skill by name
				skills := kitInstance.DiscoverSkillsForExtension()
				for _, s := range skills {
					if s.Name == skillName {
						// Inject via SendMessage as a system context message
						appInstance.Run(fmt.Sprintf("<skill name=%q>\n%s\n</skill>", s.Name, s.Content))
						return ""
					}
				}
				return fmt.Sprintf("skill not found: %s", skillName)
			},
			InjectRawSkillAsContext: func(path string) string {
				s, err := kitInstance.LoadSkillForExtension(path)
				if err != "" {
					return err
				}
				appInstance.Run(fmt.Sprintf("<skill name=%q>\n%s\n</skill>", s.Name, s.Content))
				return ""
			},
			GetAvailableSkills: kitInstance.DiscoverSkillsForExtension,

			// -------------------------------------------------------------------------
			// Template Parsing API (Phase 3 Bridge)
			// -------------------------------------------------------------------------
			ParseTemplate:        kit.ParseTemplate,
			RenderTemplate:       kit.RenderTemplate,
			ParseArguments:       kit.ParseArguments,
			SimpleParseArguments: kit.SimpleParseArguments,
			EvaluateModelConditional: func(condition string) bool {
				return kit.EvaluateModelConditional(kitInstance.Extensions().GetContext().Model, condition)
			},
			RenderWithModelConditionals: func(content string) string {
				return kit.RenderWithModelConditionals(content, kitInstance.Extensions().GetContext().Model)
			},

			// -------------------------------------------------------------------------
			// Model Resolution API (Phase 4 Bridge)
			// -------------------------------------------------------------------------
			ResolveModelChain: kit.ResolveModelChain,
			GetModelCapabilities: func(model string) (extensions.ModelCapabilities, string) {
				return kit.GetModelCapabilities(model)
			},
			CheckModelAvailable: kit.CheckModelAvailable,
			GetCurrentProvider: func() string {
				return kit.GetCurrentProvider(kitInstance.Extensions().GetContext().Model)
			},
			GetCurrentModelID: func() string {
				return kit.GetCurrentModelID(kitInstance.Extensions().GetContext().Model)
			},
		})
		kitInstance.Extensions().EmitSessionStart()

		// Restore normal print functions for runtime use.
		kitInstance.Extensions().SetContext(extensions.Context{
			CWD:           cwd,
			Model:         modelName,
			Interactive:   positionalPrompt == "",
			Print:         func(text string) { appInstance.PrintFromExtension("", text) },
			PrintInfo:     func(text string) { appInstance.PrintFromExtension("info", text) },
			PrintError:    func(text string) { appInstance.PrintFromExtension("error", text) },
			PrintBlock:    appInstance.PrintBlockFromExtension,
			SendMessage:   func(text string) { appInstance.Run(text) },
			CancelAndSend: func(text string) { appInstance.InterruptAndSend(text) },
			Abort:         func() { appInstance.Abort() },
			IsIdle:        func() bool { return !appInstance.IsBusy() },
			Compact: func(cfg extensions.CompactConfig) error {
				return appInstance.CompactAsync(cfg.CustomInstructions, cfg.OnComplete, cfg.OnError)
			},
			SendMultimodalMessage: func(text string, files []extensions.FilePart) {
				parts := make([]kit.LLMFilePart, len(files))
				for i, f := range files {
					parts[i] = kit.LLMFilePart{
						Filename:  f.Filename,
						Data:      f.Data,
						MediaType: f.MediaType,
					}
				}
				appInstance.RunWithFiles(text, parts)
			},
			GetSessionUsage: func() extensions.SessionUsage {
				if usageTracker == nil {
					return extensions.SessionUsage{}
				}
				stats := usageTracker.GetSessionStats()
				return extensions.SessionUsage{
					TotalInputTokens:      stats.TotalInputTokens,
					TotalOutputTokens:     stats.TotalOutputTokens,
					TotalCacheReadTokens:  stats.TotalCacheReadTokens,
					TotalCacheWriteTokens: stats.TotalCacheWriteTokens,
					TotalCost:             stats.TotalCost,
					RequestCount:          stats.RequestCount,
				}
			},
			Exit: func() { appInstance.QuitFromExtension() },
			SetWidget: func(config extensions.WidgetConfig) {
				kitInstance.Extensions().SetWidget(config)
				go appInstance.NotifyWidgetUpdate()
			},
			RemoveWidget: func(id string) {
				kitInstance.Extensions().RemoveWidget(id)
				go appInstance.NotifyWidgetUpdate()
			},
			SetHeader: func(config extensions.HeaderFooterConfig) {
				kitInstance.Extensions().SetHeader(config)
				go appInstance.NotifyWidgetUpdate()
			},
			RemoveHeader: func() {
				kitInstance.Extensions().RemoveHeader()
				go appInstance.NotifyWidgetUpdate()
			},
			SetFooter: func(config extensions.HeaderFooterConfig) {
				kitInstance.Extensions().SetFooter(config)
				go appInstance.NotifyWidgetUpdate()
			},
			RemoveFooter: func() {
				kitInstance.Extensions().RemoveFooter()
				go appInstance.NotifyWidgetUpdate()
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
			SpawnSubagent: func(config extensions.SubagentConfig) (*extensions.SubagentHandle, *extensions.SubagentResult, error) {
				// In-process subagent via SDK.
				sdkCfg := kit.SubagentConfig{
					Prompt:       config.Prompt,
					Model:        config.Model,
					SystemPrompt: config.SystemPrompt,
					Timeout:      config.Timeout,
					NoSession:    config.NoSession,
				}
				// Bridge SDK events to extension SubagentEvents.
				if config.OnEvent != nil {
					sdkCfg.OnEvent = func(e kit.Event) {
						se := sdkEventToSubagentEvent(e)
						if se.Type != "" {
							config.OnEvent(se)
						}
					}
				}
				result, err := kitInstance.Subagent(ctx, sdkCfg)
				if result == nil {
					return nil, &extensions.SubagentResult{Error: err}, err
				}
				extResult := &extensions.SubagentResult{
					Response:  result.Response,
					Error:     err,
					SessionID: result.SessionID,
					Elapsed:   result.Elapsed,
				}
				if result.Usage != nil {
					extResult.Usage = &extensions.SubagentUsage{
						InputTokens:  result.Usage.InputTokens,
						OutputTokens: result.Usage.OutputTokens,
					}
				}
				return nil, extResult, err
			},

			// -------------------------------------------------------------------------
			// Tree Navigation API (Phase 1 Bridge) - Second Context
			// -------------------------------------------------------------------------
			GetTreeNode: func(entryID string) *extensions.TreeNode {
				node := kitInstance.GetTreeNode(entryID)
				if node == nil {
					return nil
				}
				return &extensions.TreeNode{
					ID:        node.ID,
					ParentID:  node.ParentID,
					Type:      node.Type,
					Role:      node.Role,
					Content:   node.Content,
					Model:     node.Model,
					Provider:  node.Provider,
					Timestamp: node.Timestamp,
					Children:  node.Children,
				}
			},
			GetCurrentBranch: func() []extensions.TreeNode {
				nodes := kitInstance.GetCurrentBranch()
				result := make([]extensions.TreeNode, len(nodes))
				for i, n := range nodes {
					result[i] = extensions.TreeNode{
						ID:        n.ID,
						ParentID:  n.ParentID,
						Type:      n.Type,
						Role:      n.Role,
						Content:   n.Content,
						Model:     n.Model,
						Provider:  n.Provider,
						Timestamp: n.Timestamp,
						Children:  n.Children,
					}
				}
				return result
			},
			GetChildren: kitInstance.GetChildren,
			NavigateTo: func(entryID string) extensions.TreeNavigationResult {
				err := kitInstance.NavigateTo(entryID)
				if err != nil {
					return extensions.TreeNavigationResult{Success: false, Error: err.Error()}
				}
				return extensions.TreeNavigationResult{Success: true}
			},
			SummarizeBranch: func(fromID, toID string) string {
				summary, _ := kitInstance.SummarizeBranch(fromID, toID)
				return summary
			},
			CollapseBranch: func(fromID, toID, summary string) extensions.TreeNavigationResult {
				err := kitInstance.CollapseBranch(fromID, toID, summary)
				if err != nil {
					return extensions.TreeNavigationResult{Success: false, Error: err.Error()}
				}
				return extensions.TreeNavigationResult{Success: true}
			},

			// -------------------------------------------------------------------------
			// Skill Loading API (Phase 2 Bridge) - Second Context
			// -------------------------------------------------------------------------
			LoadSkill: func(path string) (*extensions.Skill, string) {
				s, err := kitInstance.LoadSkillForExtension(path)
				return s, err
			},
			LoadSkillsFromDir: func(dir string) extensions.SkillLoadResult {
				return kitInstance.LoadSkillsFromDirForExtension(dir)
			},
			DiscoverSkills: func() extensions.SkillLoadResult {
				skills := kitInstance.DiscoverSkillsForExtension()
				return extensions.SkillLoadResult{Skills: skills}
			},
			InjectSkillAsContext: func(skillName string) string {
				skills := kitInstance.DiscoverSkillsForExtension()
				for _, s := range skills {
					if s.Name == skillName {
						appInstance.Run(fmt.Sprintf("<skill name=%q>\n%s\n</skill>", s.Name, s.Content))
						return ""
					}
				}
				return fmt.Sprintf("skill not found: %s", skillName)
			},
			InjectRawSkillAsContext: func(path string) string {
				s, err := kitInstance.LoadSkillForExtension(path)
				if err != "" {
					return err
				}
				appInstance.Run(fmt.Sprintf("<skill name=%q>\n%s\n</skill>", s.Name, s.Content))
				return ""
			},
			GetAvailableSkills: func() []extensions.Skill {
				return kitInstance.DiscoverSkillsForExtension()
			},

			// -------------------------------------------------------------------------
			// Template Parsing API (Phase 3 Bridge) - Second Context
			// -------------------------------------------------------------------------
			ParseTemplate:        kit.ParseTemplate,
			RenderTemplate:       kit.RenderTemplate,
			ParseArguments:       kit.ParseArguments,
			SimpleParseArguments: kit.SimpleParseArguments,
			EvaluateModelConditional: func(condition string) bool {
				return kit.EvaluateModelConditional(kitInstance.Extensions().GetContext().Model, condition)
			},
			RenderWithModelConditionals: func(content string) string {
				return kit.RenderWithModelConditionals(content, kitInstance.Extensions().GetContext().Model)
			},

			// -------------------------------------------------------------------------
			// Model Resolution API (Phase 4 Bridge) - Second Context
			// -------------------------------------------------------------------------
			ResolveModelChain: kit.ResolveModelChain,
			GetModelCapabilities: func(model string) (extensions.ModelCapabilities, string) {
				return kit.GetModelCapabilities(model)
			},
			CheckModelAvailable: kit.CheckModelAvailable,
			GetCurrentProvider: func() string {
				return kit.GetCurrentProvider(kitInstance.Extensions().GetContext().Model)
			},
			GetCurrentModelID: func() string {
				return kit.GetCurrentModelID(kitInstance.Extensions().GetContext().Model)
			},
		})
	}

	// Convert extension commands to UI-layer type for the interactive TUI.
	extCommands := extensionCommandsForUI(kitInstance)

	// Load prompt templates from standard locations and explicit paths.
	var promptTemplates []*prompts.PromptTemplate
	if !noPromptTemplates {
		homeDir, _ := os.UserHomeDir()
		cwd, _ := os.Getwd()
		tpls, diags, err := prompts.LoadAll(prompts.LoadOptions{
			Cwd:             cwd,
			HomeDir:         homeDir,
			ExtraPaths:      promptTemplatePaths,
			ConfigPaths:     viper.GetStringSlice("prompts"),
			IncludeDefaults: true,
		})
		if err != nil {
			log.Printf("Warning: failed to load some prompt templates: %v", err)
		}
		promptTemplates = tpls
		for _, d := range diags {
			log.Printf("Prompt template collision: /%s kept from %s, dropped from %s", d.Name, d.KeptPath, d.DroppedPath)
		}
	}

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
	emitBeforeFork := beforeForkProviderForUI(kitInstance)
	emitBeforeSessionSwitch := beforeSessionSwitchProviderForUI(kitInstance)
	getGlobalShortcuts := globalShortcutsProviderForUI(kitInstance)
	getExtensionCommands := func() []commands.ExtensionCommand {
		return extensionCommandsForUI(kitInstance)
	}

	// Build model switching callbacks for the /model command.
	setModelForUI := func(modelString string) error {
		err := kitInstance.SetModel(context.Background(), modelString)
		if err != nil {
			return err
		}
		// Update the extension context's Model field so handlers see it.
		kitInstance.Extensions().UpdateContextModel(modelString)
		// NOTE: We do NOT call appInstance.NotifyModelChanged() here because
		// this callback runs synchronously inside BubbleTea's Update(), and
		// NotifyModelChanged calls prog.Send() which deadlocks. The UI layer
		// updates m.providerName and m.modelName directly after setModel returns.
		// Update usage tracker with new model info for correct token counting.
		if usageTracker != nil {
			newProvider, newModel, _ := models.ParseModelString(modelString)
			if newProvider != "unknown" && newModel != "unknown" && newProvider != "ollama" {
				registry := models.GetGlobalRegistry()
				if modelInfo := registry.LookupModel(newProvider, newModel); modelInfo != nil {
					// Check OAuth status for Anthropic models
					isOAuth := false
					if newProvider == "anthropic" {
						_, source, err := auth.GetAnthropicAPIKey(viper.GetString("provider-api-key"))
						if err == nil && strings.HasPrefix(source, "stored OAuth") {
							isOAuth = true
						}
					}
					usageTracker.UpdateModelInfo(modelInfo, newProvider, isOAuth)
				}
			}
		}
		return nil
	}
	emitModelChangeForUI := func(newModel, previousModel, source string) {
		kitInstance.Extensions().EmitModelChange(newModel, previousModel, source)
	}

	// Build thinking level callback.
	setThinkingLevelForUI := func(level string) error {
		return kitInstance.SetThinkingLevel(context.Background(), level)
	}

	// Build session-switching callback. Opens a JSONL session file and
	// replaces the active tree session on both the Kit SDK and App layer.
	switchSessionForUI := func(path string) error {
		ts, err := kit.OpenTreeSession(path)
		if err != nil {
			return fmt.Errorf("failed to open session: %w", err)
		}
		kitInstance.SetTreeSession(ts)
		appInstance.SwitchTreeSession(ts)
		return nil
	}

	// Build extension reload callback for the /reload-ext command.
	reloadExtensionsForUI := func() error {
		err := kitInstance.Extensions().Reload()
		if err != nil {
			return err
		}
		go appInstance.NotifyWidgetUpdate()
		return nil
	}

	// Start file watcher for automatic extension hot-reload.
	extraPaths := viper.GetStringSlice("extension")
	watchDirs := extensions.WatchedDirs(extraPaths)
	if len(watchDirs) > 0 {
		extWatcher, watchErr := extensions.NewWatcher(watchDirs, func() {
			if err := reloadExtensionsForUI(); err != nil {
				log.Printf("auto-reload extensions failed: %v", err)
			}
		})
		if watchErr != nil {
			log.Printf("extension file watcher not started: %v", watchErr)
		} else {
			go extWatcher.Start(ctx)
			defer func() { _ = extWatcher.Close() }()
		}
	}

	// Check if running in non-interactive mode
	if positionalPrompt != "" {
		return runNonInteractiveModeApp(ctx, appInstance, cli, positionalPrompt, quietFlag, jsonFlag, noExitFlag, modelName, parsedProvider, kitInstance.GetLoadingMessage(), serverNames, toolNames, mcpToolCount, extensionToolCount, usageTracker, extCommands, promptTemplates, contextPaths, skillItems, getWidgets, getHeader, getFooter, getToolRenderer, getEditorInterceptor, getUIVisibility, getStatusBarEntries, emitBeforeFork, emitBeforeSessionSwitch, getGlobalShortcuts, getExtensionCommands, setModelForUI, emitModelChangeForUI, kitInstance.IsReasoningModel(), kitInstance.GetThinkingLevel(), setThinkingLevelForUI, switchSessionForUI, reloadExtensionsForUI)
	}

	// Quiet mode is not allowed in interactive mode
	if quietFlag {
		return fmt.Errorf("--quiet requires a prompt")
	}

	return runInteractiveModeBubbleTea(ctx, appInstance, modelName, parsedProvider, kitInstance.GetLoadingMessage(), serverNames, toolNames, mcpToolCount, extensionToolCount, usageTracker, extCommands, promptTemplates, contextPaths, skillItems, getWidgets, getHeader, getFooter, getToolRenderer, getEditorInterceptor, getUIVisibility, getStatusBarEntries, emitBeforeFork, emitBeforeSessionSwitch, getGlobalShortcuts, getExtensionCommands, setModelForUI, emitModelChangeForUI, kitInstance.IsReasoningModel(), kitInstance.GetThinkingLevel(), setThinkingLevelForUI, switchSessionForUI, reloadExtensionsForUI, startupExtensionMessages)
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
func runNonInteractiveModeApp(ctx context.Context, appInstance *app.App, cli *ui.CLI, prompt string, quiet, jsonOutput, noExit bool, modelName, providerName, loadingMessage string, serverNames, toolNames []string, mcpToolCount, extensionToolCount int, usageTracker *ui.UsageTracker, extCommands []commands.ExtensionCommand, promptTemplates []*prompts.PromptTemplate, contextPaths []string, skillItems []ui.SkillItem, getWidgets func(string) []ui.WidgetData, getHeader, getFooter func() *ui.WidgetData, getToolRenderer func(string) *ui.ToolRendererData, getEditorInterceptor func() *ui.EditorInterceptor, getUIVisibility func() *ui.UIVisibility, getStatusBarEntries func() []ui.StatusBarEntryData, emitBeforeFork func(string, bool, string) (bool, string), emitBeforeSessionSwitch func(string) (bool, string), getGlobalShortcuts func() map[string]func(), getExtensionCommands func() []commands.ExtensionCommand, setModel func(string) error, emitModelChange func(string, string, string), isReasoningModel bool, thinkingLevel string, setThinkingLevel func(string) error, switchSession func(string) error, reloadExtensions func() error) error {
	// Expand @file references in the prompt before sending to the agent.
	if cwd, err := os.Getwd(); err == nil {
		prompt = ui.ProcessFileAttachments(prompt, cwd)
	}

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
		return runInteractiveModeBubbleTea(ctx, appInstance, modelName, providerName, loadingMessage, serverNames, toolNames, mcpToolCount, extensionToolCount, usageTracker, extCommands, promptTemplates, contextPaths, skillItems, getWidgets, getHeader, getFooter, getToolRenderer, getEditorInterceptor, getUIVisibility, getStatusBarEntries, emitBeforeFork, emitBeforeSessionSwitch, getGlobalShortcuts, getExtensionCommands, setModel, emitModelChange, isReasoningModel, thinkingLevel, setThinkingLevel, switchSession, reloadExtensions, nil)
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
		Response   string        `json:"response"`
		Model      string        `json:"model"`
		StopReason string        `json:"stop_reason,omitempty"`
		SessionID  string        `json:"session_id,omitempty"`
		Usage      *jsonUsage    `json:"usage,omitempty"`
		Messages   []jsonMessage `json:"messages"`
	}

	out := jsonEnvelope{
		Response:   result.Response,
		Model:      model,
		StopReason: result.StopReason,
		SessionID:  result.SessionID,
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
		converted := kit.ConvertFromLLMMessage(fmsg)
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
func runInteractiveModeBubbleTea(_ context.Context, appInstance *app.App, modelName, providerName, loadingMessage string, serverNames, toolNames []string, mcpToolCount, extensionToolCount int, usageTracker *ui.UsageTracker, extCommands []commands.ExtensionCommand, promptTemplates []*prompts.PromptTemplate, contextPaths []string, skillItems []ui.SkillItem, getWidgets func(string) []ui.WidgetData, getHeader, getFooter func() *ui.WidgetData, getToolRenderer func(string) *ui.ToolRendererData, getEditorInterceptor func() *ui.EditorInterceptor, getUIVisibility func() *ui.UIVisibility, getStatusBarEntries func() []ui.StatusBarEntryData, emitBeforeFork func(string, bool, string) (bool, string), emitBeforeSessionSwitch func(string) (bool, string), getGlobalShortcuts func() map[string]func(), getExtensionCommands func() []commands.ExtensionCommand, setModel func(string) error, emitModelChange func(string, string, string), isReasoningModel bool, thinkingLevel string, setThinkingLevel func(string) error, switchSession func(string) error, reloadExtensions func() error, startupExtensionMessages []string) error {
	// Determine terminal size; fall back gracefully.
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termWidth == 0 {
		termWidth = 80
		termHeight = 24
	}

	cwd, _ := os.Getwd()

	appModel := ui.NewAppModel(appInstance, ui.AppModelOptions{
		ModelName:                modelName,
		ProviderName:             providerName,
		LoadingMessage:           loadingMessage,
		Cwd:                      cwd,
		Width:                    termWidth,
		Height:                   termHeight,
		ServerNames:              serverNames,
		ToolNames:                toolNames,
		MCPToolCount:             mcpToolCount,
		ExtensionToolCount:       extensionToolCount,
		UsageTracker:             usageTracker,
		ExtensionCommands:        extCommands,
		PromptTemplates:          promptTemplates,
		ContextPaths:             contextPaths,
		SkillItems:               skillItems,
		StartupExtensionMessages: startupExtensionMessages,
		GetWidgets:               getWidgets,
		GetHeader:                getHeader,
		GetFooter:                getFooter,
		GetToolRenderer:          getToolRenderer,
		GetEditorInterceptor:     getEditorInterceptor,
		GetUIVisibility:          getUIVisibility,
		GetStatusBarEntries:      getStatusBarEntries,
		EmitBeforeFork:           emitBeforeFork,
		EmitBeforeSessionSwitch:  emitBeforeSessionSwitch,
		GetGlobalShortcuts:       getGlobalShortcuts,
		GetExtensionCommands:     getExtensionCommands,
		SetModel:                 setModel,
		EmitModelChange:          emitModelChange,
		ThinkingLevel:            thinkingLevel,
		IsReasoningModel:         isReasoningModel,
		SetThinkingLevel:         setThinkingLevel,
		SwitchSession:            switchSession,
		ReloadExtensions:         reloadExtensions,
		ShowSessionPicker:        resumeFlag,
	})

	program := tea.NewProgram(appModel)

	// Register the program with the app layer so agent events are sent to the TUI.
	appInstance.SetProgram(program)

	_, runErr := program.Run()
	return runErr
}

// sdkEventToSubagentEvent converts an SDK event to an extension-facing
// SubagentEvent. Returns a zero-value event (Type=="") for events that
// don't map to anything useful.
func sdkEventToSubagentEvent(e kit.Event) extensions.SubagentEvent {
	switch ev := e.(type) {
	case kit.MessageUpdateEvent:
		return extensions.SubagentEvent{Type: "text", Content: ev.Chunk}
	case kit.ReasoningDeltaEvent:
		return extensions.SubagentEvent{Type: "reasoning", Content: ev.Delta}
	case kit.ToolCallEvent:
		return extensions.SubagentEvent{
			Type: "tool_call", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind, ToolArgs: ev.ToolArgs,
		}
	case kit.ToolExecutionStartEvent:
		return extensions.SubagentEvent{
			Type: "tool_execution_start", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind,
		}
	case kit.ToolExecutionEndEvent:
		return extensions.SubagentEvent{
			Type: "tool_execution_end", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind,
		}
	case kit.ToolResultEvent:
		return extensions.SubagentEvent{
			Type: "tool_result", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind,
			ToolResult: ev.Result, IsError: ev.IsError,
		}
	case kit.TurnStartEvent:
		return extensions.SubagentEvent{Type: "turn_start"}
	case kit.TurnEndEvent:
		return extensions.SubagentEvent{Type: "turn_end"}
	default:
		return extensions.SubagentEvent{}
	}
}
