package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/prompts"
	"github.com/mark3labs/kit/internal/ui"
	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/progress"
	"github.com/mark3labs/kit/internal/watcher"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// ---------------------------------------------------------------------------
// runModeDeps and its sub-structs
// ---------------------------------------------------------------------------

// runModeDeps bundles the shared dependencies that runNormalMode wires up
// once and threads to both runNonInteractiveModeApp and
// runInteractiveModeBubbleTea. The fields are grouped by how the TUI uses
// them: a one-time startup snapshot, read-only provider getters it polls,
// and action callbacks it invokes in response to user input.
type runModeDeps struct {
	appInstance *app.App
	cli         *ui.CLI // non-interactive only

	snapshot  startupSnapshot
	providers uiProviders
	actions   uiActions
}

// startupSnapshot holds values captured once at startup. The TUI seeds its
// initial state (banner, /tools list, usage tracker) from these and refreshes
// them later through uiProviders when a reload event fires.
type startupSnapshot struct {
	modelName                string
	providerName             string
	loadingMessage           string
	serverNames              []string
	toolNames                []string
	mcpToolCount             int
	extensionToolCount       int
	usageTracker             *ui.UsageTracker
	extCommands              []commands.ExtensionCommand
	promptTemplates          []*prompts.PromptTemplate
	contextPaths             []string
	bare                     bool
	skillItems               []ui.SkillItem
	extensionItems           []ui.ExtensionItem
	mcpPrompts               []ui.MCPPromptInfo
	isReasoningModel         bool
	thinkingLevel            string
	startupExtensionMessages []string // interactive only
}

// uiProviders holds read-only getters that the TUI polls to render extension
// chrome and to refresh lists after hot-reload. None of these mutate state.
type uiProviders struct {
	getPromptTemplates   func() []*prompts.PromptTemplate
	getSkillItems        func() []ui.SkillItem
	getExtensionItems    func() []ui.ExtensionItem
	getToolNames         func() []string
	getMCPToolCount      func() int
	getMCPPrompts        func() []ui.MCPPromptInfo
	getMCPResources      func() []ui.FileSuggestion
	getWidgets           func(string) []ui.WidgetData
	getHeader            func() *ui.WidgetData
	getFooter            func() *ui.WidgetData
	getToolRenderer      func(string) *ui.ToolRendererData
	getEditorInterceptor func() *ui.EditorInterceptor
	getUIVisibility      func() *ui.UIVisibility
	getStatusBarEntries  func() []ui.StatusBarEntryData
	getGlobalShortcuts   func() map[string]func()
	getShortcutList      func() []ui.ShortcutInfo
	getExtensionCommands func() []commands.ExtensionCommand
}

// uiActions holds callbacks that the TUI invokes to change state, emit
// extension events, or perform I/O against MCP servers.
type uiActions struct {
	expandMCPPrompt         func(string, string, map[string]string) (*ui.MCPPromptExpandResult, error)
	readMCPResource         ui.MCPResourceReader
	emitBeforeFork          func(string, bool, string) (bool, string)
	emitBeforeSessionSwitch func(string, string) (bool, string)
	setModel                func(string) error
	emitModelChange         func(string, string, string)
	emitThinkingLevelChange func(string, string, string)
	emitTerminalResize      func(int, int)
	emitTurnStateChange     func(string, string)
	setThinkingLevel        func(string) error
	switchSession           func(string) error
	reloadExtensions        func() error
}

// ---------------------------------------------------------------------------
// runNormalMode phases
// ---------------------------------------------------------------------------

// configureDebugLogging turns on file:line prefixes on the stdlib logger when
// --debug is set on the command line or in the config file, and keeps the
// package-level debugMode flag in sync with the config value.
func configureDebugLogging() {
	if viper.GetBool("debug") && !debugMode {
		debugMode = true
	}
	if debugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}
}

// startupSpinnerFunc returns the spinner wrapper kit.New uses while it loads
// MCP servers, or nil when chrome is suppressed (--quiet / --json).
func startupSpinnerFunc() kit.SpinnerFunc {
	if suppressChrome() {
		return nil
	}
	return func(fn func() error) error {
		tempCli, tempErr := ui.NewCLI(viper.GetBool("debug"))
		if tempErr == nil {
			return tempCli.ShowSpinner(fn)
		}
		return fn()
	}
}

// newCLIMCPAuthHandler creates the OAuth handler for remote MCP servers.
//
// NewCLIMCPAuthHandler binds a TCP listener on localhost. In sandbox VMs where
// `localhost` is not in /etc/hosts this can fail. We treat that as non-fatal
// (OAuth simply is not available for remote MCP servers) and return a nil
// pointer so the caller can funnel it through a true nil interface.
func newCLIMCPAuthHandler() *kit.CLIMCPAuthHandler {
	h, err := kit.NewCLIMCPAuthHandler()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to create OAuth handler: %v\n", err)
		return nil
	}
	return h
}

// buildKitOptions translates the CLI flags and config into kit.Options.
//
// authHandler may be nil. It is assigned into Options.MCPAuthHandler (an
// interface) only when non-nil: storing a typed nil *CLIMCPAuthHandler in the
// interface would yield a non-nil interface wrapping a nil pointer, which
// panics on the first method dispatch downstream.
//
// onMCPServerLoaded is called when each MCP server finishes loading.
func buildKitOptions(mcpConfig *config.Config, authHandler *kit.CLIMCPAuthHandler, onMCPServerLoaded func(serverName string, toolCount int, err error)) (*kit.Options, error) {
	coreToolList, err := kit.FilterCoreToolNames(viper.GetStringSlice("include-core-tools"), viper.GetStringSlice("exclude-core-tools"))
	if err != nil {
		return nil, err
	}

	var mcpAuth kit.MCPAuthHandler
	if authHandler != nil {
		mcpAuth = authHandler
	}

	opts := &kit.Options{
		Quiet:             suppressChrome(),
		Debug:             debugMode,
		NoSession:         viper.GetBool("no-session"),
		Continue:          continueFlag,
		SessionPath:       sessionPath,
		AutoCompact:       autoCompactFlag,
		MCPAuthHandler:    mcpAuth,
		DisableCoreTools:  viper.GetBool("no-core-tools"),
		CoreToolList:      coreToolList,
		NoSkills:          noSkillsFlag,
		NoAgents:          noAgentsFlag,
		Bare:              bareFlag,
		Skills:            skillsPaths,
		SkillsDir:         skillsDir,
		SkillsDisable:     skillsDisable,
		SkillTrustPrompt:  skillTrustPrompt(),
		OnMCPServerLoaded: onMCPServerLoaded,
		CLI: &kit.CLIOptions{
			MCPConfig:          mcpConfig,
			ShowSpinner:        true,
			SpinnerFunc:        startupSpinnerFunc(),
			UseBufferedLogger:  true,
			ProgressReaderFunc: progress.NewProgressReadCloser,
		},
	}

	// When --resume is combined with interactive mode, the TUI session
	// picker will be shown at startup (ShowSessionPicker on AppModelOptions).
	// For non-interactive mode, fall back to auto-selecting the most recent
	// session.
	if resumeFlag && positionalPrompt != "" {
		sessions, _ := kit.ListSessions("")
		if len(sessions) > 0 {
			opts.SessionPath = sessions[0].Path
		}
	}

	return opts, nil
}

// systemPromptLoadedNotice builds the "System Prompt loaded" notice shown at
// startup, paralleling the per-server "MCP server loaded" notifications so
// users can confirm that a configured prompt file was found and applied.
// Returns "" when no custom system prompt is active.
func systemPromptLoadedNotice(k *kit.Kit) string {
	if !k.HasCustomSystemPrompt() {
		return ""
	}
	if src := k.GetSystemPromptSource(); src != "" {
		return "System Prompt loaded: " + src
	}
	return ""
}

// setupNonInteractiveCLI creates the CLI used by prompt mode and prints the
// startup diagnostics through it. It returns (nil, nil) in interactive mode,
// where the TUI handles its own rendering.
func setupNonInteractiveCLI(k *kit.Kit, mcpConfig *config.Config, provider, systemPromptLoadedMsg string) (*ui.CLI, error) {
	if positionalPrompt == "" {
		return nil, nil
	}
	cli, err := SetupCLIForNonInteractive(k)
	if err != nil {
		return nil, fmt.Errorf("failed to setup CLI: %v", err)
	}

	// Display buffered debug messages if any (non-interactive path only).
	if msgs := k.GetBufferedDebugMessages(); len(msgs) > 0 && cli != nil {
		cli.DisplayDebugMessage(strings.Join(msgs, "\n  "))
	}

	DisplayDebugConfig(cli, k, mcpConfig, provider)
	if systemPromptLoadedMsg != "" && cli != nil {
		cli.DisplayInfo(systemPromptLoadedMsg)
	}
	return cli, nil
}

// newRunApp creates the app.App instance seeded with messages from a resumed
// or continued session. It also returns the usage tracker shared between the
// app layer (recording usage after each step) and the TUI (/usage display).
func newRunApp(k *kit.Kit, cli *ui.CLI, mcpConfig *config.Config, modelName string, serverNames, toolNames []string) (*app.App, *ui.UsageTracker) {
	treeSession := k.GetTreeSession()
	var messages []kit.LLMMessage
	if treeSession != nil {
		messages = treeSession.GetLLMMessages()
	}

	appOpts := BuildAppOptions(mcpConfig, modelName, serverNames, toolNames)
	appOpts.Kit = k
	appOpts.TreeSession = treeSession

	var usageTracker *ui.UsageTracker
	if cli != nil {
		usageTracker = cli.GetUsageTracker()
	} else {
		usageTracker = ui.CreateUsageTracker(viper.GetString("model"), viper.GetString("provider-api-key"))
	}
	if usageTracker != nil {
		appOpts.UsageTracker = usageTracker
	}

	return app.New(appOpts, messages), usageTracker
}

// startExtensionSession wires the extension context and emits SessionStart.
//
// Extension output produced during SessionStart is buffered and returned so
// the TUI can print it after the startup banner; once the event has fired the
// print routes are switched to the live app instance. The returned slice
// always begins with systemPromptLoadedMsg when it is non-empty.
func startExtensionSession(ctx context.Context, k *kit.Kit, appInstance *app.App, usageTracker *ui.UsageTracker, modelName string, interactive bool, systemPromptLoadedMsg string) []string {
	var startupMessages []string
	if systemPromptLoadedMsg != "" {
		startupMessages = append(startupMessages, systemPromptLoadedMsg)
	}
	if !k.Extensions().HasExtensions() {
		return startupMessages
	}

	cwd, _ := os.Getwd()
	// Seed the size before SessionStart so handlers can lay out during
	// startup, ahead of the TUI's first WindowSizeMsg. Only in the
	// interactive TUI: headless runs have no chrome to size, and
	// terminalSize()'s 80x24 fallback would contradict the documented
	// (0, 0) that GetTerminalSize reports outside the TUI.
	if interactive {
		k.Extensions().SetTerminalSize(terminalSize())
	}
	extCtx := buildInteractiveExtensionContext(extensionContextDeps{
		ctx:          ctx,
		cwd:          cwd,
		modelName:    modelName,
		interactive:  interactive,
		kitInstance:  k,
		appInstance:  appInstance,
		usageTracker: usageTracker,
	})

	// During startup, buffer extension messages so they appear after the banner.
	buffer := func(text string) {
		startupMessages = append(startupMessages, text)
	}
	extCtx.Print = buffer
	extCtx.PrintInfo = buffer
	extCtx.PrintError = buffer
	k.Extensions().SetContext(extCtx)
	if err := k.Extensions().InitStatePersistence(); err != nil {
		log.Printf("WARN extension state init failed: %v", err)
	}
	k.Extensions().EmitSessionStart()

	// Restore normal print functions for runtime use.
	extCtx.Print = func(text string) { appInstance.PrintFromExtension("", text) }
	extCtx.PrintInfo = func(text string) { appInstance.PrintFromExtension("info", text) }
	extCtx.PrintError = func(text string) { appInstance.PrintFromExtension("error", text) }
	k.Extensions().SetContext(extCtx)

	return startupMessages
}

// promptLoadOptions returns the prompt template load options shared by the
// startup load and the hot-reload provider so both see the same set of paths.
func promptLoadOptions() prompts.LoadOptions {
	homeDir, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	return prompts.LoadOptions{
		Cwd:             cwd,
		HomeDir:         homeDir,
		ExtraPaths:      promptTemplatePaths,
		ConfigPaths:     viper.GetStringSlice("prompts"),
		IncludeDefaults: true,
		Bare:            bareFlag,
	}
}

// loadPromptTemplates loads prompt templates from standard locations and
// explicit paths. It returns nil when --no-prompt-templates is set. Name
// collisions are reported only on the initial load (reload == false) so a
// hot-reload does not repeat them.
func loadPromptTemplates(reload bool) []*prompts.PromptTemplate {
	if noPromptTemplates {
		return nil
	}
	tpls, diags, err := prompts.LoadAll(promptLoadOptions())
	if err != nil {
		if reload {
			log.Printf("Warning: failed to reload prompt templates: %v", err)
		} else {
			log.Printf("Warning: failed to load some prompt templates: %v", err)
		}
	}
	if !reload {
		for _, d := range diags {
			log.Printf("Prompt template collision: /%s kept from %s, dropped from %s", d.Name, d.KeptPath, d.DroppedPath)
		}
	}
	return tpls
}

// contextFilePaths returns the paths of the context files kit.New loaded,
// for the [Context] row of the startup banner.
func contextFilePaths(k *kit.Kit) []string {
	var paths []string
	for _, cf := range k.GetContextFiles() {
		paths = append(paths, cf.Path)
	}
	return paths
}

// collectSkillItems converts the loaded skills into UI items. Skills whose
// path lies under cwd are tagged "project"; all others are tagged "user".
func collectSkillItems(k *kit.Kit, cwd string) []ui.SkillItem {
	var items []ui.SkillItem
	for _, s := range k.GetSkills() {
		source := "user"
		if strings.HasPrefix(s.Path, cwd) {
			source = "project"
		}
		items = append(items, ui.SkillItem{
			Name:        s.Name,
			Path:        s.Path,
			Source:      source,
			Description: s.Description,
		})
	}
	return items
}

// reloadSkillItems re-discovers skills from disk and returns the refreshed
// UI items. Used by the TUI when ContentReloadEvent fires.
func reloadSkillItems(k *kit.Kit) []ui.SkillItem {
	if err := k.ReloadSkills(); err != nil {
		log.Printf("Warning: failed to reload skills: %v", err)
		return nil
	}
	cwd, _ := os.Getwd()
	return collectSkillItems(k, cwd)
}

// mcpPromptsForUI converts kit.MCPPrompt values to the UI-layer type.
func mcpPromptsForUI(k *kit.Kit) []ui.MCPPromptInfo {
	mcpPrompts := k.ListMCPPrompts()
	if len(mcpPrompts) == 0 {
		return nil
	}
	result := make([]ui.MCPPromptInfo, len(mcpPrompts))
	for i, p := range mcpPrompts {
		args := make([]ui.MCPPromptArgInfo, len(p.Arguments))
		for j, a := range p.Arguments {
			args[j] = ui.MCPPromptArgInfo{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			}
		}
		result[i] = ui.MCPPromptInfo{
			Name:        p.Name,
			Description: p.Description,
			Arguments:   args,
			ServerName:  p.ServerName,
		}
	}
	return result
}

// expandMCPPromptForUI returns the callback that resolves an MCP prompt into
// messages for the TUI.
func expandMCPPromptForUI(k *kit.Kit) func(serverName, promptName string, args map[string]string) (*ui.MCPPromptExpandResult, error) {
	return func(serverName, promptName string, args map[string]string) (*ui.MCPPromptExpandResult, error) {
		result, err := k.GetMCPPrompt(context.Background(), serverName, promptName, args)
		if err != nil {
			return nil, err
		}
		msgs := make([]ui.MCPPromptMessageInfo, len(result.Messages))
		for i, m := range result.Messages {
			msgs[i] = ui.MCPPromptMessageInfo{
				Role:      m.Role,
				Content:   m.Content,
				FileParts: m.FileParts,
			}
		}
		return &ui.MCPPromptExpandResult{Messages: msgs}, nil
	}
}

// mcpResourcesForUI lists the MCP resources as file suggestions for the @
// autocomplete popup.
func mcpResourcesForUI(k *kit.Kit) []ui.FileSuggestion {
	resources := k.ListMCPResources()
	suggestions := make([]ui.FileSuggestion, len(resources))
	for i, r := range resources {
		suggestions[i] = ui.FileSuggestion{
			RelPath:        r.Name,
			IsMCPResource:  true,
			MCPServerName:  r.ServerName,
			MCPResourceURI: r.URI,
			MCPMIMEType:    r.MIMEType,
			Score:          100, // default score, filtered later
		}
	}
	return suggestions
}

// mcpResourceReaderForUI returns the reader used to resolve @resource
// references at submit time.
func mcpResourceReaderForUI(k *kit.Kit) ui.MCPResourceReader {
	return func(serverName, uri string) (string, []byte, string, bool, error) {
		content, err := k.ReadMCPResource(context.Background(), serverName, uri)
		if err != nil {
			return "", nil, "", false, err
		}
		return content.Text, content.BlobData, content.MIMEType, content.IsBlob, nil
	}
}

// buildUIProviders wires the read-only getters the TUI polls.
func buildUIProviders(k *kit.Kit) uiProviders {
	return uiProviders{
		getPromptTemplates: func() []*prompts.PromptTemplate { return loadPromptTemplates(true) },
		getSkillItems:      func() []ui.SkillItem { return reloadSkillItems(k) },
		// getExtensionItems re-collects the loaded extension list, used by the
		// TUI after an extension hot-reload to refresh the [Extensions] row.
		getExtensionItems: func() []ui.ExtensionItem {
			cwd, _ := os.Getwd()
			return buildExtensionItems(k, cwd)
		},
		// Tool name and MCP tool count providers are called by the TUI when
		// MCPToolsReadyEvent fires to refresh the /tools list and startup
		// info bar after background MCP tool loading completes.
		getToolNames:         k.GetToolNames,
		getMCPToolCount:      k.GetMCPToolCount,
		getMCPPrompts:        func() []ui.MCPPromptInfo { return mcpPromptsForUI(k) },
		getMCPResources:      func() []ui.FileSuggestion { return mcpResourcesForUI(k) },
		getWidgets:           widgetProviderForUI(k),
		getHeader:            headerProviderForUI(k),
		getFooter:            footerProviderForUI(k),
		getToolRenderer:      toolRendererProviderForUI(k),
		getEditorInterceptor: editorInterceptorProviderForUI(k),
		getUIVisibility:      uiVisibilityProviderForUI(k),
		getStatusBarEntries:  statusBarProviderForUI(k),
		getGlobalShortcuts:   globalShortcutsProviderForUI(k),
		getShortcutList:      shortcutListProviderForUI(k),
		getExtensionCommands: func() []commands.ExtensionCommand { return extensionCommandsForUI(k) },
	}
}

// buildUIActions wires the callbacks the TUI invokes to change state or emit
// extension events.
func buildUIActions(k *kit.Kit, appInstance *app.App, usageTracker *ui.UsageTracker) uiActions {
	return uiActions{
		expandMCPPrompt:         expandMCPPromptForUI(k),
		readMCPResource:         mcpResourceReaderForUI(k),
		emitBeforeFork:          beforeForkProviderForUI(k),
		emitBeforeSessionSwitch: beforeSessionSwitchProviderForUI(k),
		// setModel backs the /model command.
		setModel: func(modelString string) error {
			if err := k.SetModel(context.Background(), modelString); err != nil {
				return err
			}
			// Update the extension context's Model field so handlers see it.
			k.Extensions().UpdateContextModel(modelString)
			// NOTE: We do NOT call appInstance.NotifyModelChanged() here because
			// this callback runs synchronously inside BubbleTea's Update(), and
			// NotifyModelChanged calls prog.Send() which deadlocks. The UI layer
			// updates m.providerName and m.modelName directly after setModel returns.
			// Update usage tracker with new model info for correct token counting.
			ui.UpdateUsageTrackerForModel(usageTracker, modelString, viper.GetString("provider-api-key"))
			return nil
		},
		emitModelChange:         k.Extensions().EmitModelChange,
		emitThinkingLevelChange: k.Extensions().EmitThinkingLevelChange,
		emitTerminalResize: func(width, height int) {
			// Record the size first so a handler calling ctx.GetTerminalSize
			// during this event sees the new value rather than the old one.
			k.Extensions().SetTerminalSize(width, height)
			k.Extensions().EmitTerminalResize(width, height)
		},
		emitTurnStateChange: k.Extensions().EmitTurnStateChange,
		setThinkingLevel: func(level string) error {
			return k.SetThinkingLevel(context.Background(), level)
		},
		// switchSession opens a JSONL session file and replaces the active
		// tree session on both the Kit SDK and App layer.
		switchSession: func(path string) error {
			ts, err := kit.OpenTreeSession(path)
			if err != nil {
				return fmt.Errorf("failed to open session: %w", err)
			}
			k.SetTreeSession(ts)
			appInstance.SwitchTreeSession(ts)
			return nil
		},
		// reloadExtensions backs the /reload-ext command and the file watcher.
		reloadExtensions: func() error {
			if err := k.Extensions().Reload(); err != nil {
				return err
			}
			go appInstance.NotifyWidgetUpdate()
			return nil
		},
	}
}

// startExtensionWatcher starts the file watcher for automatic extension
// hot-reload and returns a stop function. The stop function is a no-op when
// no watcher was started.
//
// In bare mode only explicitly named extensions are loaded, so only those are
// watched. Watching the discovery directories would let a reload pull in
// extensions that startup deliberately skipped.
func startExtensionWatcher(ctx context.Context, appInstance *app.App, reload func() error) func() {
	extraPaths := viper.GetStringSlice("extension")
	var watchDirs []string
	if bareFlag {
		watchDirs = watcher.CollectDirs(nil, extraPaths)
	} else {
		watchDirs = extensions.WatchedDirs(extraPaths)
	}
	if len(watchDirs) == 0 {
		return func() {}
	}

	extWatcher, err := extensions.NewWatcher(watchDirs, func() {
		if err := reload(); err != nil {
			log.Printf("auto-reload extensions failed: %v", err)
			appInstance.PrintFromExtension("error", fmt.Sprintf("Extension auto-reload failed: %v", err))
			return
		}
		appInstance.PrintFromExtension("info", "Extensions reloaded.")
	})
	if err != nil {
		log.Printf("extension file watcher not started: %v", err)
		return func() {}
	}
	go extWatcher.Start(ctx)
	return func() { _ = extWatcher.Close() }
}

// startContentWatcher starts a single file watcher for automatic prompt
// template and skill hot-reload and returns a stop function. The stop
// function is a no-op when no watcher was started.
//
// Bare mode watches only explicitly supplied paths: the standard directories
// were never loaded, so reacting to changes in them would reintroduce the
// context the mode exists to avoid.
func startContentWatcher(ctx context.Context, appInstance *app.App) func() {
	homeDir, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	var promptStdDirs, skillStdDirs []string
	if !bareFlag {
		promptStdDirs = []string{
			filepath.Join(homeDir, ".kit", "prompts"),
			prompts.GlobalDir(),
			filepath.Join(cwd, ".kit", "prompts"),
		}
		skillStdDirs = []string{
			filepath.Join(homeDir, ".config", "kit", "skills"),
			filepath.Join(cwd, ".agents", "skills"),
			filepath.Join(cwd, ".kit", "skills"),
		}
	}

	promptDirs := watcher.CollectDirs(
		promptStdDirs,
		append(promptTemplatePaths, viper.GetStringSlice("prompts")...),
	)
	skillDirs := watcher.CollectDirs(skillStdDirs, skillsPaths)

	allContentDirs := append(promptDirs, skillDirs...)
	if len(allContentDirs) == 0 {
		return func() {}
	}

	contentWatcher, err := watcher.New(watcher.Options{
		Dirs:       allContentDirs,
		Extensions: []string{".md", ".txt"},
		Label:      "prompts/skills",
		OnReload: func() {
			log.Printf("auto-reloading prompts and skills")
			appInstance.NotifyContentReload()
		},
	})
	if err != nil {
		log.Printf("content file watcher not started: %v", err)
		return func() {}
	}
	go contentWatcher.Start(ctx)
	return func() { _ = contentWatcher.Close() }
}
