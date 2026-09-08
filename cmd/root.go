package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	charmlog "github.com/charmbracelet/log"
	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/daemon"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/ui"
	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/termgfx"
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
	providerWire     string
	debugMode        bool
	positionalPrompt string        // set by processPositionalArgs from CLI positional args
	positionalFiles  []ui.FilePart // binary @file parts from processPositionalArgs

	quietFlag       bool
	jsonFlag        bool
	noExitFlag      bool
	maxSteps        int
	streamFlag      bool // Enable streaming output
	autoCompactFlag bool // Enable auto-compaction near context limit

	// Session management
	sessionPath string

	// Tree session management (pi-style)
	continueFlag  bool // --continue / -c: resume most recent session for cwd
	resumeFlag    bool // --resume / -r: interactive session picker
	noSessionFlag bool // --no-session: ephemeral mode, no persistence

	// Model generation parameters
	maxTokens        int
	temperature      float32
	topP             float32
	topK             int32
	frequencyPenalty float32
	presencePenalty  float32
	stopSequences    []string
	thinkingLevel    string

	// Ollama-specific parameters
	numGPU  int32
	mainGPU int32

	// Extensions control
	noExtensionsFlag     bool
	noCoreToolsFlag      bool
	includeCoreToolsFlag []string
	excludeCoreToolsFlag []string
	extensionPaths       []string
	mcpFlags             []string

	// Shell tool
	shellFlag string

	// Skills control
	noSkillsFlag  bool
	skillsPaths   []string
	skillsDir     string
	skillsDisable []string

	// Named agents control
	noAgentsFlag bool

	// Bare mode — no automatic context discovery from any directory.
	bareFlag bool

	// TLS configuration
	tlsSkipVerify bool

	// Prompt templates
	promptTemplatePaths []string
	noPromptTemplates   bool

	// The daemon's directory picker
	// (--pick-dir, hidden — spawned by `kit daemon`).
	pickDirFlag bool

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
		// An empty --shell is a mistake rather than a request for the default:
		// the user asked for a shell and named none. Refusing here keeps the
		// failure at startup instead of on the first tool call.
		if f := cmd.PersistentFlags().Lookup("shell"); f != nil && f.Changed &&
			strings.TrimSpace(f.Value.String()) == "" {
			return fmt.Errorf(`--shell needs a shell, e.g. --shell /bin/dash or --shell "busybox ash"`)
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
	// Remote client flows never read local configuration: a broken local
	// config must not block attaching to a daemon, and the client performs
	// no local-session work that could consume it.
	if remoteSubcommandSelected(os.Args[1:]) {
		return
	}
	if err := kit.InitConfigWithOptions(kit.ConfigInitOptions{
		ConfigFile: configFile,
		Debug:      debugMode,
		Bare:       bareFlag,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	// Rebuild the model registry now that viper has the config loaded,
	// so customModels defined in the config file are picked up.
	models.ReloadGlobalRegistry()
}

// remoteSubcommandSelected reports whether the invoked command line
// selects the `kit remote` subcommand. Flag-aware: global flags (with or
// without values) before the subcommand are skipped, so
// `kit --config x remote --list` is recognized just like `kit remote`.
func remoteSubcommandSelected(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "remote" {
			return true
		}
		if strings.HasPrefix(arg, "-") {
			// Flags that take a value consume the next token unless the
			// value is attached with '='.
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				isBoolean := slices.Contains(globalBoolFlags, strings.TrimLeft(arg, "-"))
				if !isBoolean {
					i++
				}
			}
		}
	}
	return false
}

// globalBoolFlags lists root persistent flags that do not take a value
// (long and short forms); used by remoteSubcommandSelected to walk the
// argv correctly. Shorthands with values (-m, -s, -e) must NOT appear here.
var globalBoolFlags = []string{
	"bare", "debug", "quiet", "json", "no-exit", "no-session",
	"continue", "resume", "auto-compact", "compact", "stream",
	"no-extensions", "no-prompt-templates", "no-skills", "no-agents",
	"no-core-tools", "tls-skip-verify", "pick-dir", "version",
	"c", "r", // -c (continue), -r (resume)
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

// adoptSessionTerminalCapabilities takes the terminal background from the
// environment the daemon prepared for a session's child.
//
// A session child's stdin and stdout are a PTY. A PTY answers no OSC
// query, so probing it costs two timeouts and then reports the default,
// and every adaptive colour in the UI is chosen from that default rather
// than from the terminal the user is looking at. The client resolved the
// real value before it handed its terminal over; this adopts it.
//
// Colour depth still comes from the environment, because the daemon has
// already replaced TERM and COLORTERM there with the client's own.
func adoptSessionTerminalCapabilities() {
	bg := os.Getenv(daemon.RemoteBackgroundEnv)
	if bg == "" {
		return
	}
	ui.SetTerminalCapabilities(daemon.BackgroundIsDark(bg), colorprofile.Env(os.Environ()))
}

// kitBanner returns the KIT ASCII art title with KITT scanner lights.
// Delegates to ui.KitBanner() which owns the logo rendering.
func kitBanner() string {
	return ui.KitBanner()
}

func init() {
	// Registered before InitConfig: remote attachment and directory
	// selection must not depend on (or race) local configuration loading.
	cobra.OnInitialize(preInitDispatch)
	cobra.OnInitialize(InitConfig)

	rootCmd.Long = kitBanner() + "\n\n" + rootCmd.Long

	// Before any theme is resolved: inside a daemon session the terminal
	// belongs to the client, not to this process, so its capabilities come
	// from what the daemon was told rather than from a probe of the PTY on
	// our own fds. Resolving a theme is what triggers that probe, so this
	// has to come first.
	adoptSessionTerminalCapabilities()

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
	// --bare is deliberately NOT bound to viper. Every other flag can be set
	// from a config file, but this one exists to ignore project config, so
	// letting a project .kit.yml set it would be self-defeating.
	rootCmd.PersistentFlags().
		BoolVar(&bareFlag, "bare", false, "no project context: skip AGENTS.md, skills, extensions, agents, prompt templates and project .kit.yml")
	rootCmd.PersistentFlags().
		BoolVar(&noCoreToolsFlag, "no-core-tools", false, "disable all built-in core tools (shell, read, write, edit, grep, find, ls, subagent)")
	rootCmd.PersistentFlags().
		StringSliceVar(&includeCoreToolsFlag, "include-core-tools", nil, "comma-separated list of core tools to include")
	rootCmd.PersistentFlags().
		StringSliceVar(&excludeCoreToolsFlag, "exclude-core-tools", nil, "comma-separated list of core tools to exclude")
	// The shell tool runs one command string through this shell. The value is
	// the shell plus its own leading arguments. Empty leaves the built-in
	// default, which is bash.
	rootCmd.PersistentFlags().
		StringVar(&shellFlag, "shell", "", `shell the shell tool runs commands through, e.g. "/bin/dash" or "busybox ash" (default "bash")`)
	rootCmd.PersistentFlags().
		StringSliceVarP(&extensionPaths, "extension", "e", nil, "load additional extension file(s)")
	// StringArray (not StringSlice) so a command line with commas stays one value.
	rootCmd.PersistentFlags().
		StringArrayVar(&mcpFlags, "mcp", nil, `add an MCP server for this run (repeatable): "name=command args..." for stdio or "name=https://..." for remote`)

	// Skills flags
	rootCmd.PersistentFlags().
		BoolVar(&noSkillsFlag, "no-skills", false, "disable skill loading (auto-discovery and explicit)")
	rootCmd.PersistentFlags().
		BoolVar(&noAgentsFlag, "no-agents", false, "disable named agent discovery (built-ins and .agents/agents, .kit/agents, ~/.config/kit/agents)")
	rootCmd.PersistentFlags().
		StringSliceVar(&skillsPaths, "skill", nil, "load skill file or directory (repeatable)")
	rootCmd.PersistentFlags().
		StringVar(&skillsDir, "skills-dir", "", "scan this directory directly for skills (overrides auto-discovery)")
	rootCmd.PersistentFlags().
		StringSliceVar(&skillsDisable, "skill-disable", nil, "hide a skill from the model catalog by name (repeatable); still usable via /skill:")
	rootCmd.Flags().
		BoolVar(&pickDirFlag, "pick-dir", false, "choose a working directory with a picker before starting")

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&providerURL, "provider-url", "", "base URL for the provider API (applies to OpenAI, Anthropic, Ollama, and Google)")
	flags.StringVar(&providerAPIKey, "provider-api-key", "", "API key for the provider (applies to OpenAI, Anthropic, and Google)")
	flags.StringVar(&providerWire, "provider-wire", "", "wire protocol for auto-routed providers: openai, openai-compat, anthropic, google (overrides the model database)")
	flags.BoolVar(&tlsSkipVerify, "tls-skip-verify", false, "skip TLS certificate verification (WARNING: insecure, use only for self-signed certificates)")

	// Prompt template flags
	flags.StringArrayVar(&promptTemplatePaths, "prompt-template", nil, "load prompt template file or directory (repeatable)")
	flags.BoolVar(&noPromptTemplates, "no-prompt-templates", false, "disable prompt template discovery")

	// Model generation parameters
	flags.IntVar(&maxTokens, "max-tokens", 8192, "maximum number of output tokens per response (auto-raised up to 32768 for models with higher known output limits; see internal/models/embedded_models.json)")
	flags.Float32Var(&temperature, "temperature", 0.7, "controls randomness in responses (0.0-1.0)")
	flags.Float32Var(&topP, "top-p", 0.95, "controls diversity via nucleus sampling (0.0-1.0)")
	flags.Int32Var(&topK, "top-k", 40, "controls diversity by limiting top K tokens to sample from")
	flags.Float32Var(&frequencyPenalty, "frequency-penalty", 0.0, "penalizes tokens based on frequency of appearance (0.0-2.0)")
	flags.Float32Var(&presencePenalty, "presence-penalty", 0.0, "penalizes tokens based on whether they have appeared (0.0-2.0)")
	flags.StringSliceVar(&stopSequences, "stop-sequences", nil, "custom stop sequences (comma-separated)")
	flags.StringVar(&thinkingLevel, "thinking-level", "off", "extended thinking level: off, none, minimal, low, medium, high")

	// Ollama-specific parameters
	flags.Int32Var(&numGPU, "num-gpu-layers", -1, "number of model layers to offload to GPU for Ollama models (-1 for auto-detect)")
	_ = flags.MarkHidden("num-gpu-layers") // Advanced option, hidden from help
	flags.Int32Var(&mainGPU, "main-gpu", 0, "main GPU device to use for Ollama models")

	// Bind flags to viper for config file support
	_ = viper.BindPFlag("system-prompt", rootCmd.PersistentFlags().Lookup("system-prompt"))
	_ = viper.BindPFlag("no-session", rootCmd.PersistentFlags().Lookup("no-session"))
	_ = viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("max-steps", rootCmd.PersistentFlags().Lookup("max-steps"))
	_ = viper.BindPFlag("stream", rootCmd.PersistentFlags().Lookup("stream"))
	_ = viper.BindPFlag("auto-compact", rootCmd.PersistentFlags().Lookup("auto-compact"))

	_ = viper.BindPFlag("provider-url", rootCmd.PersistentFlags().Lookup("provider-url"))
	_ = viper.BindPFlag("provider-api-key", rootCmd.PersistentFlags().Lookup("provider-api-key"))
	_ = viper.BindPFlag("provider-wire", rootCmd.PersistentFlags().Lookup("provider-wire"))
	_ = viper.BindPFlag("max-tokens", rootCmd.PersistentFlags().Lookup("max-tokens"))
	_ = viper.BindPFlag("temperature", rootCmd.PersistentFlags().Lookup("temperature"))
	_ = viper.BindPFlag("top-p", rootCmd.PersistentFlags().Lookup("top-p"))
	_ = viper.BindPFlag("top-k", rootCmd.PersistentFlags().Lookup("top-k"))
	_ = viper.BindPFlag("frequency-penalty", rootCmd.PersistentFlags().Lookup("frequency-penalty"))
	_ = viper.BindPFlag("presence-penalty", rootCmd.PersistentFlags().Lookup("presence-penalty"))
	_ = viper.BindPFlag("stop-sequences", rootCmd.PersistentFlags().Lookup("stop-sequences"))
	_ = viper.BindPFlag("thinking-level", rootCmd.PersistentFlags().Lookup("thinking-level"))
	_ = viper.BindPFlag("num-gpu-layers", rootCmd.PersistentFlags().Lookup("num-gpu-layers"))
	_ = viper.BindPFlag("main-gpu", rootCmd.PersistentFlags().Lookup("main-gpu"))
	_ = viper.BindPFlag("tls-skip-verify", rootCmd.PersistentFlags().Lookup("tls-skip-verify"))
	_ = viper.BindPFlag("no-extensions", rootCmd.PersistentFlags().Lookup("no-extensions"))
	_ = viper.BindPFlag("no-core-tools", rootCmd.PersistentFlags().Lookup("no-core-tools"))
	_ = viper.BindPFlag("include-core-tools", rootCmd.PersistentFlags().Lookup("include-core-tools"))
	_ = viper.BindPFlag("exclude-core-tools", rootCmd.PersistentFlags().Lookup("exclude-core-tools"))
	_ = viper.BindPFlag("shell", rootCmd.PersistentFlags().Lookup("shell"))
	_ = viper.BindPFlag("extension", rootCmd.PersistentFlags().Lookup("extension"))
	_ = viper.BindPFlag("prompt-template", rootCmd.PersistentFlags().Lookup("prompt-template"))
	_ = viper.BindPFlag("no-prompt-templates", rootCmd.PersistentFlags().Lookup("no-prompt-templates"))
	_ = viper.BindPFlag("no-skills", rootCmd.PersistentFlags().Lookup("no-skills"))
	_ = viper.BindPFlag("no-agents", rootCmd.PersistentFlags().Lookup("no-agents"))
	_ = viper.BindPFlag("skill", rootCmd.PersistentFlags().Lookup("skill"))
	_ = viper.BindPFlag("skills-dir", rootCmd.PersistentFlags().Lookup("skills-dir"))
	_ = viper.BindPFlag("skill-disable", rootCmd.PersistentFlags().Lookup("skill-disable"))

	// Defaults are already set in flag definitions, no need to duplicate in viper

	// Add subcommands
	rootCmd.AddCommand(authCmd)
}

// processPositionalArgs separates positional CLI arguments into @file
// attachments and prompt text. Text file content is read and prepended to
// positionalPrompt; binary files (images, audio) are stored in positionalFiles
// for multimodal submission. Positional args are the primary way to run
// non-interactive mode:
//
//	kit "Explain this codebase"
//	kit @code.ts @test.ts "Review these files"
//	kit @screenshot.png "What's in this image?"
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
	// Text files are XML-wrapped inline; binary files become multimodal parts.
	var fileContent strings.Builder
	for _, token := range fileTokens {
		result := ui.ProcessFileAttachments(token, cwd)
		if result.ProcessedText != token {
			// Text file was resolved — add it.
			fileContent.WriteString(result.ProcessedText)
			fileContent.WriteString("\n\n")
		}
		// Collect binary file parts for multimodal submission.
		positionalFiles = append(positionalFiles, result.FileParts...)
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

// preInitDispatch handles the directory-picker entry point before any
// configuration is loaded, so project-level configuration discovery
// (.kit.* in the chosen directory) resolves against the chosen directory
// instead of whatever directory kit happened to start in. The dispatcher
// exits the process directly on cancellation or failure.
func preInitDispatch() {
	if !pickDirFlag {
		return
	}
	home, _ := os.UserHomeDir()
	chosen, err := ui.RunDirPicker(home)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if chosen == "" {
		os.Exit(0) // cancelled
	}
	if err := os.Chdir(chosen); err != nil {
		fmt.Fprintf(os.Stderr, "change to %s: %v\n", chosen, err)
		os.Exit(1)
	}
	// Tell the hosting daemon (if any) which directory this session
	// settled on, so it appears in the session list. A no-op when kit was
	// not spawned by a daemon.
	daemon.ReportSessionCwd(chosen)
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

// buildExtensionItems converts the loaded extensions into ui.ExtensionItem
// values for the startup [Extensions] section. The display name is the file
// basename without the .go suffix; for subdirectory extensions the parent
// directory name is used (matching `kit extensions list`).
func buildExtensionItems(k *kit.Kit, cwd string) []ui.ExtensionItem {
	infos := k.Extensions().Loaded()
	if len(infos) == 0 {
		return nil
	}
	items := make([]ui.ExtensionItem, 0, len(infos))
	for _, info := range infos {
		name := filepath.Base(info.Path)
		if name == "main.go" {
			// Subdirectory extension: use the parent directory's name.
			name = filepath.Base(filepath.Dir(info.Path))
		}
		name = strings.TrimSuffix(name, ".go")
		source := "user"
		if cwd != "" && strings.HasPrefix(info.Path, cwd) {
			source = "project"
		}
		items = append(items, ui.ExtensionItem{
			Name:   name,
			Path:   info.Path,
			Source: source,
		})
	}
	return items
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
				ID:          c.ID,
				Text:        c.Content.Text,
				Markdown:    c.Content.Markdown,
				Render:      c.Content.Render,
				RefreshHz:   c.Content.RefreshHz,
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
			Render:      cfg.Content.Render,
			RefreshHz:   cfg.Content.RefreshHz,
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
// if extensions are disabled — the UI treats nil as "no hook". The
// initialPrompt argument is forwarded to the event so extensions can
// inspect the prompt that will be submitted as the first turn of the
// new session.
func beforeSessionSwitchProviderForUI(k *kit.Kit) func(switchReason, initialPrompt string) (bool, string) {
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func(switchReason, initialPrompt string) (bool, string) {
		return k.Extensions().EmitBeforeSessionSwitchWithPrompt(switchReason, initialPrompt)
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

// shortcutListProviderForUI returns a callback that lists extension-registered
// shortcuts for the /shortcuts command, converting them to the UI's type.
func shortcutListProviderForUI(k *kit.Kit) func() []ui.ShortcutInfo {
	if !k.Extensions().HasExtensions() {
		return nil
	}
	return func() []ui.ShortcutInfo {
		infos := k.Extensions().GetShortcutInfos()
		if len(infos) == 0 {
			return nil
		}
		out := make([]ui.ShortcutInfo, 0, len(infos))
		for _, info := range infos {
			out = append(out, ui.ShortcutInfo{
				Key:         info.Key,
				Description: info.Description,
				Source:      info.Source,
			})
		}
		return out
	}
}

// suppressChrome reports whether all decorative output (startup banners,
// spinners, warnings, extension prints) must stay off stdout.
//
// --quiet prints only the final response; --json prints only the JSON
// envelope. Both modes are consumed by pipes and scripts, so any extra
// stdout byte breaks the caller (e.g. `kit "..." --json | jq`).
func suppressChrome() bool {
	return quietFlag || jsonFlag
}

// validateModeFlags rejects invalid flag combinations for the root command.
func validateModeFlags() error {
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
	return nil
}

// restorePersistedPreferences applies saved model / thinking-level
// preferences into viper when neither a CLI flag nor a config-file value
// takes precedence. Precedence: CLI flag > config file > saved preference >
// built-in default. This mirrors how themes are persisted.
func restorePersistedPreferences() {
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
}

// applyProviderURLRouting rewrites the model in viper when --provider-url
// is set, routing requests through the "custom" (OpenAI-compatible)
// provider. Must run after restorePersistedPreferences.
func applyProviderURLRouting() {
	if viper.GetString("provider-url") == "" {
		return
	}

	// When --provider-wire is set alongside --provider-url the user is being
	// explicit about the wire protocol; the custom/ rewrite below would force
	// the OpenAI-compatible wire, so skip it and let the model's provider
	// prefix route through the auto-router (which honors --provider-wire and
	// synthesizes unknown providers when both flags are present).
	if viper.GetString("provider-wire") != "" {
		return
	}

	// When --provider-url is set but no explicit --model was provided,
	// default to "custom/custom" so the user doesn't need to remember a
	// provider/model pair for custom OpenAI-compatible endpoints.
	// This intentionally overrides saved preferences but respects config-file
	// models — if you specify a model in ~/.kit.yml, it will be used with
	// custom/custom's provider routing.
	if !modelFlagChanged && !viper.InConfig("model") {
		viper.Set("model", "custom/custom")
	}

	// When --provider-url is set with an explicit --model, route through the
	// "custom" provider (OpenAI-compatible wire). This honors the user's
	// intent: passing a custom URL means "use THIS endpoint", not "speak
	// the Google/Anthropic/etc. wire protocol against this endpoint".
	//
	// Any provider prefix on the model is stripped so a model name that
	// happens to collide with a known provider (e.g. `google/gemma-4-12b`
	// served by LM Studio) still resolves correctly. If you genuinely need
	// to point a non-OpenAI wire (Anthropic, Google, ...) at a proxy URL,
	// use the explicit `custom/<name>` form to opt out of the rewrite by
	// configuring the proxy as that provider in your config file instead.
	if modelFlagChanged {
		model := viper.GetString("model")
		if model != "" {
			name := model
			if _, after, ok := strings.Cut(model, "/"); ok {
				name = after
			}
			if !strings.HasPrefix(model, "custom/") {
				viper.Set("model", "custom/"+name)
			}
		}
	}
}

func runNormalMode(ctx context.Context) error {
	if err := validateModeFlags(); err != nil {
		return err
	}

	configureDebugLogging()
	restorePersistedPreferences()
	applyProviderURLRouting()

	mcpConfig, err := config.LoadAndValidateConfig()
	if err != nil {
		return fmt.Errorf("failed to load MCP config: %v", err)
	}
	if err := applyMCPFlags(mcpConfig, mcpFlags); err != nil {
		return err
	}

	// appInstancePtr is used to break the circular dependency between
	// kit.New (which needs the OnMCPServerLoaded callback) and app.New
	// (which is needed by the callback to send events to the TUI). The
	// closure captures the variable; it is assigned after app.New below.
	var appInstancePtr *app.App
	cliAuthHandler := newCLIMCPAuthHandler()
	kitOpts, err := buildKitOptions(mcpConfig, cliAuthHandler, func(serverName string, toolCount int, err error) {
		if appInstancePtr != nil {
			appInstancePtr.NotifyMCPServerLoaded(serverName, toolCount, err)
		}
	})
	if err != nil {
		return err
	}

	// kit.New() handles: config → skills → agent → session → extension bridge.
	kitInstance, err := kit.New(ctx, kitOpts)
	if err != nil {
		return err
	}
	defer func() { _ = kitInstance.Close() }()

	interactive := positionalPrompt == ""
	systemPromptLoadedMsg := systemPromptLoadedNotice(kitInstance)
	parsedProvider, modelName, serverNames, toolNames, mcpToolCount, extensionToolCount := CollectAgentMetadata(kitInstance, mcpConfig)

	cli, err := setupNonInteractiveCLI(kitInstance, mcpConfig, parsedProvider, systemPromptLoadedMsg)
	if err != nil {
		return err
	}

	appInstance, usageTracker := newRunApp(kitInstance, cli, mcpConfig, modelName, serverNames, toolNames)
	appInstancePtr = appInstance // Wire up the MCP server loaded callback.
	defer appInstance.Close()

	// Wire OAuth handler to route messages through the TUI once it's running.
	if cliAuthHandler != nil {
		cliAuthHandler.NotifyFunc = func(serverName, message string) {
			appInstance.PrintFromExtension("info", message)
		}
	}

	startupExtensionMessages := startExtensionSession(ctx, kitInstance, appInstance, usageTracker, modelName, interactive, systemPromptLoadedMsg)

	// Wait for background MCP tool loading to complete and notify the TUI so
	// it can refresh tool names and counts.
	if len(mcpConfig.MCPServers) > 0 {
		go func() {
			_ = kitInstance.WaitForMCPTools()
			appInstance.NotifyMCPToolsReady()
		}()
	}

	cwd, _ := os.Getwd()
	deps := runModeDeps{
		appInstance: appInstance,
		cli:         cli,
		snapshot: startupSnapshot{
			modelName:                modelName,
			providerName:             parsedProvider,
			loadingMessage:           kitInstance.GetLoadingMessage(),
			serverNames:              serverNames,
			toolNames:                toolNames,
			mcpToolCount:             mcpToolCount,
			extensionToolCount:       extensionToolCount,
			usageTracker:             usageTracker,
			extCommands:              extensionCommandsForUI(kitInstance),
			promptTemplates:          loadPromptTemplates(false),
			contextPaths:             contextFilePaths(kitInstance),
			bare:                     bareFlag,
			skillItems:               collectSkillItems(kitInstance, cwd),
			extensionItems:           buildExtensionItems(kitInstance, cwd),
			mcpPrompts:               mcpPromptsForUI(kitInstance),
			isReasoningModel:         kitInstance.IsReasoningModel(),
			thinkingLevel:            kitInstance.GetThinkingLevel(),
			startupExtensionMessages: startupExtensionMessages,
		},
		providers: buildUIProviders(kitInstance),
		actions:   buildUIActions(kitInstance, appInstance, usageTracker),
	}

	stopExtensionWatcher := startExtensionWatcher(ctx, appInstance, deps.actions.reloadExtensions)
	defer stopExtensionWatcher()
	stopContentWatcher := startContentWatcher(ctx, appInstance)
	defer stopContentWatcher()

	if !interactive {
		return runNonInteractiveModeApp(ctx, deps, positionalPrompt, quietFlag, jsonFlag, noExitFlag)
	}

	// Quiet mode is not allowed in interactive mode
	if quietFlag {
		return fmt.Errorf("--quiet requires a prompt")
	}

	return runInteractiveModeBubbleTea(ctx, deps)
}

// runNonInteractiveModeApp executes a single prompt via the app layer and exits,
// or transitions to the interactive BubbleTea TUI when --no-exit is set.
//
// In quiet mode, RunOnceWithFiles is used (no intermediate output, final response only).
// Otherwise, RunOnceWithDisplay streams tool calls and responses through the
// shared CLIEventHandler — giving --prompt mode the same rich output as
// interactive mode.
//
// When --no-exit is set, after the prompt completes the interactive BubbleTea
// TUI is started so the user can continue the conversation.
func runNonInteractiveModeApp(ctx context.Context, deps runModeDeps, prompt string, quiet, jsonOutput, noExit bool) error {
	appInstance := deps.appInstance
	cli := deps.cli
	modelName := deps.snapshot.modelName
	// Expand @file references in the prompt before sending to the agent.
	// Text files are XML-inlined; binary files are extracted as multimodal parts.
	var fileParts []kit.LLMFilePart
	if cwd, err := os.Getwd(); err == nil {
		result := ui.ProcessFileAttachments(prompt, cwd, deps.actions.readMCPResource)
		prompt = result.ProcessedText
		for _, fp := range result.FileParts {
			fileParts = append(fileParts, kit.LLMFilePart{
				Filename:  fp.Filename,
				Data:      fp.Data,
				MediaType: fp.MediaType,
			})
		}
	}
	// Also include binary files from processPositionalArgs (CLI @file args).
	for _, fp := range positionalFiles {
		fileParts = append(fileParts, kit.LLMFilePart{
			Filename:  fp.Filename,
			Data:      fp.Data,
			MediaType: fp.MediaType,
		})
	}

	if jsonOutput {
		// JSON mode: no intermediate display, structured JSON output.
		result, err := appInstance.RunOnceResultWithFiles(ctx, prompt, fileParts)
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
		if err := appInstance.RunOnceWithFiles(ctx, prompt, fileParts); err != nil {
			return err
		}
	} else if cli != nil {
		// Display user message before running the agent.
		cli.DisplayUserMessage(prompt)

		// Route events through the shared CLI event handler.
		eventHandler := ui.NewCLIEventHandler(cli, modelName)
		err := appInstance.RunOnceWithDisplayAndFiles(ctx, prompt, eventHandler.Handle, fileParts)
		eventHandler.Cleanup()
		if err != nil {
			return err
		}
	} else {
		// No CLI available (shouldn't happen in non-quiet mode, but be safe).
		if err := appInstance.RunOnceWithFiles(ctx, prompt, fileParts); err != nil {
			return err
		}
	}

	// If --no-exit was requested, hand off to the interactive TUI.
	if noExit {
		// Drop the cli (interactive mode doesn't use it) and clear the
		// interactive-only fields explicitly; deps carries everything else.
		interactive := deps
		interactive.cli = nil
		interactive.snapshot.startupExtensionMessages = nil
		return runInteractiveModeBubbleTea(ctx, interactive)
	}

	return nil
}

// terminalSize reports the current terminal dimensions, falling back to a
// conventional 80x24 when stdout is not a TTY. Shared by the TUI and the
// extension context so both start from the same numbers.
func terminalSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		return 80, 24
	}
	return w, h
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
func runInteractiveModeBubbleTea(_ context.Context, deps runModeDeps) error {
	appInstance := deps.appInstance
	// Redirect all log output (stdlib and charm) to a file so that log
	// messages don't write to stderr and corrupt the TUI. Bubble Tea
	// captures stdout for rendering; any stray stderr output from
	// background goroutines (watchers, extension handlers, SDK internals)
	// will visually corrupt the terminal.
	logDir := filepath.Join(os.TempDir(), "kit")
	_ = os.MkdirAll(logDir, 0o700)
	logFile, logErr := tea.LogToFile(filepath.Join(logDir, "kit.log"), "kit")
	if logErr == nil {
		defer func() { _ = logFile.Close() }()
		// tea.LogToFile only redirects the stdlib log package. The
		// charmbracelet/log default logger (used by internal packages such
		// as skills for collision diagnostics) still writes to stderr,
		// which corrupts the alt-screen when a hot-reload fires while the
		// TUI is running. Point it at the same file so no structured log
		// output reaches the terminal.
		charmlog.SetOutput(logFile)
		// --debug turns on the structured debug output that diagnoses
		// terminal-capability and layout decisions. Without this the default
		// Info level silently drops it.
		if debugMode {
			charmlog.SetLevel(charmlog.DebugLevel)
		}
	}

	// Determine terminal size; fall back gracefully.
	termWidth, termHeight := terminalSize()

	cwd, _ := os.Getwd()

	snap, prov, act := deps.snapshot, deps.providers, deps.actions
	appModel := ui.NewAppModel(appInstance, ui.AppModelOptions{
		ModelName:                snap.modelName,
		ProviderName:             snap.providerName,
		LoadingMessage:           snap.loadingMessage,
		Cwd:                      cwd,
		Shell:                    viper.GetStringSlice("shell"),
		Width:                    termWidth,
		Height:                   termHeight,
		ServerNames:              snap.serverNames,
		ToolNames:                snap.toolNames,
		GetToolNames:             prov.getToolNames,
		GetMCPToolCount:          prov.getMCPToolCount,
		MCPToolCount:             snap.mcpToolCount,
		ExtensionToolCount:       snap.extensionToolCount,
		UsageTracker:             snap.usageTracker,
		ExtensionCommands:        snap.extCommands,
		PromptTemplates:          snap.promptTemplates,
		GetPromptTemplates:       prov.getPromptTemplates,
		MCPPrompts:               snap.mcpPrompts,
		GetMCPPrompts:            prov.getMCPPrompts,
		ExpandMCPPrompt:          act.expandMCPPrompt,
		ContextPaths:             snap.contextPaths,
		Bare:                     snap.bare,
		SkillItems:               snap.skillItems,
		GetSkillItems:            prov.getSkillItems,
		ExtensionItems:           snap.extensionItems,
		GetExtensionItems:        prov.getExtensionItems,
		StartupExtensionMessages: snap.startupExtensionMessages,
		GetWidgets:               prov.getWidgets,
		GetHeader:                prov.getHeader,
		GetFooter:                prov.getFooter,
		GetToolRenderer:          prov.getToolRenderer,
		GetEditorInterceptor:     prov.getEditorInterceptor,
		GetUIVisibility:          prov.getUIVisibility,
		GetStatusBarEntries:      prov.getStatusBarEntries,
		EmitBeforeFork:           act.emitBeforeFork,
		EmitBeforeSessionSwitch:  act.emitBeforeSessionSwitch,
		GetGlobalShortcuts:       prov.getGlobalShortcuts,
		GetShortcutList:          prov.getShortcutList,
		GetExtensionCommands:     prov.getExtensionCommands,
		SetModel:                 act.setModel,
		EmitModelChange:          act.emitModelChange,
		EmitThinkingLevelChange:  act.emitThinkingLevelChange,
		EmitTerminalResize:       act.emitTerminalResize,
		EmitTurnStateChange:      act.emitTurnStateChange,
		ThinkingLevel:            snap.thinkingLevel,
		IsReasoningModel:         snap.isReasoningModel,
		SetThinkingLevel:         act.setThinkingLevel,
		SwitchSession:            act.switchSession,
		ReloadExtensions:         act.reloadExtensions,
		ShowSessionPicker:        resumeFlag,
		GetMCPResources:          prov.getMCPResources,
		MCPResourceReader:        act.readMCPResource,
	})

	// Resolve terminal capabilities (background, colour profile) now, before
	// the TUI takes over stdin. The detection runs a synchronous OSC query, so
	// it must happen here rather than lazily mid-render where it would race the
	// event loop. No-op if a theme in config already resolved them.
	ui.ResolveTerminalCapabilities()

	// Detect the inline-graphics protocols the terminal honours, for the same
	// reason and at the same point: the probe reads raw replies from stdin, so
	// it must finish before the event loop owns that fd. Image previews fall
	// back to half-block thumbnails when the probe finds no support.
	termgfx.Resolve()

	program := tea.NewProgram(appModel)

	// Register the program with the app layer so agent events are sent to the TUI.
	appInstance.SetProgram(program)

	_, runErr := program.Run()
	return runErr
}
