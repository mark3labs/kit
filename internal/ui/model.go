package ui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/core"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/session"
)

// appState represents the current state of the parent TUI model.
type appState int

const (
	// stateInput is the default state: input is focused and the user is waiting
	// to type. The agent is not running.
	stateInput appState = iota

	// stateWorking means the agent is running. The stream component is active.
	// The input component remains visible and editable for queueing messages.
	stateWorking

	// stateTreeSelector means the /tree viewer is active.
	stateTreeSelector

	// statePrompt means an extension-triggered interactive prompt is active.
	// The prompt overlay takes full focus until the user completes or cancels.
	statePrompt

	// stateOverlay means an extension-triggered modal overlay dialog is active.
	// The overlay takes over the full view until the user completes or cancels.
	stateOverlay

	// stateModelSelector means the /model selector overlay is active.
	stateModelSelector
)

// AppController is the interface the parent TUI model uses to interact with the
// app layer. It is satisfied by *app.App once that is created (TAS-4).
// Using an interface here keeps model.go compilable before app.App exists, and
// makes the parent model easily testable with a mock.
type AppController interface {
	// Run queues or immediately starts a new agent step with the given prompt.
	// Returns the current queue depth: 0 means the prompt started immediately
	// (or the app is closed), >0 means it was queued. The caller must update
	// UI state (e.g. queueCount) based on the return value — Run does NOT
	// send events to the program to avoid deadlocking when called from
	// within Update().
	Run(prompt string) int
	// CancelCurrentStep cancels any in-progress agent step.
	CancelCurrentStep()
	// QueueLength returns the number of prompts currently waiting in the queue.
	QueueLength() int
	// ClearQueue discards all queued prompts. The caller must update UI state
	// (e.g. queueCount) — ClearQueue does NOT send events to the program to
	// avoid deadlocking when called from within Update().
	ClearQueue()
	// ClearMessages clears the conversation history.
	ClearMessages()
	// CompactConversation summarises older messages to free context space.
	// Runs asynchronously; results are delivered via CompactCompleteEvent or
	// CompactErrorEvent sent through the registered tea.Program. Returns an
	// error synchronously if compaction cannot be started (e.g. agent is busy).
	// customInstructions is optional text appended to the summary prompt.
	CompactConversation(customInstructions string) error
	// GetTreeSession returns the tree session manager, or nil if tree sessions
	// are not enabled. Used by slash commands like /tree, /fork, /session.
	GetTreeSession() *session.TreeManager
	// SendEvent sends a tea.Msg to the program asynchronously. Safe to call
	// from any goroutine. Used by extension command goroutines to deliver
	// results back to the TUI without going through tea.Cmd (which can stall
	// when the goroutine blocks on interactive prompts).
	SendEvent(tea.Msg)
	// AddContextMessage adds a user-role message to the conversation history
	// without triggering an LLM response. Used by the ! shell command prefix
	// to inject command output into context so the LLM can reference it in
	// subsequent turns.
	AddContextMessage(text string)
	// RunWithFiles queues a multimodal prompt (text + images) for execution.
	// Behaves like Run but includes file parts (e.g. clipboard images)
	// alongside the text. Returns the current queue depth (0 = started
	// immediately, >0 = queued).
	RunWithFiles(prompt string, files []fantasy.FilePart) int
}

// SkillItem holds display metadata about a loaded skill for the startup
// [Skills] section. Built by the CLI layer from the SDK's []*kit.Skill.
type SkillItem struct {
	Name   string // Skill name (e.g. "btca-cli").
	Path   string // Absolute path to the skill file.
	Source string // "project" or "user" (global).
}

// ToolRendererData holds extension-provided rendering functions for a specific
// tool. The UI layer uses this to override the default tool header/body
// rendering without depending on the extensions package directly.
type ToolRendererData struct {
	// DisplayName, if non-empty, replaces the auto-capitalized tool name
	// in the header line.
	DisplayName string

	// BorderColor, if non-empty, overrides the default success/error border
	// color. Hex string (e.g. "#89b4fa").
	BorderColor string

	// Background, if non-empty, sets a background color for the tool block.
	// Hex string (e.g. "#1e1e2e").
	Background string

	// BodyMarkdown, when true, renders the RenderBody output as markdown
	// via glamour. Ignored when RenderBody is nil or returns empty.
	BodyMarkdown bool

	// RenderHeader, if non-nil, replaces the default parameter formatting
	// in the tool header line. Receives the JSON-encoded arguments and max
	// width. Return a short summary string, or empty to fall back to default.
	RenderHeader func(toolArgs string, width int) string

	// RenderBody, if non-nil, replaces the default tool result body. Receives
	// the result text, error flag, and available width. Return the full styled
	// body content, or empty to fall back to builtin/default renderer.
	RenderBody func(toolResult string, isError bool, width int) string
}

// ---------------------------------------------------------------------------
// Editor interceptor types (UI-layer, decoupled from extensions package)
// ---------------------------------------------------------------------------

// EditorKeyActionType defines the outcome of an editor key interception.
// Mirrors extensions.EditorKeyActionType for package decoupling.
type EditorKeyActionType string

const (
	// EditorKeyPassthrough lets the built-in editor handle the key normally.
	EditorKeyPassthrough EditorKeyActionType = "passthrough"
	// EditorKeyConsumed means the extension handled the key.
	EditorKeyConsumed EditorKeyActionType = "consumed"
	// EditorKeyRemap transforms the key into a different key.
	EditorKeyRemap EditorKeyActionType = "remap"
	// EditorKeySubmit forces immediate text submission.
	EditorKeySubmit EditorKeyActionType = "submit"
)

// EditorKeyAction is the UI-layer equivalent of extensions.EditorKeyAction.
type EditorKeyAction struct {
	// Type determines the action taken.
	Type EditorKeyActionType
	// RemappedKey is the target key name for EditorKeyRemap.
	RemappedKey string
	// SubmitText is the text to submit for EditorKeySubmit.
	SubmitText string
}

// EditorInterceptor is the UI-layer representation of an extension editor
// interceptor. It decouples the UI package from the extensions package.
// The CLI layer converts the extension EditorConfig to this type.
type EditorInterceptor struct {
	// HandleKey intercepts key presses before the built-in editor.
	HandleKey func(key string, currentText string) EditorKeyAction
	// Render wraps the built-in editor's rendered output.
	Render func(width int, defaultContent string) string
}

// WidgetData is the UI-layer representation of an extension widget. It
// decouples the UI package from the extensions package. The CLI layer
// converts extension WidgetConfig values to WidgetData for rendering.
type WidgetData struct {
	// Text is the content to display.
	Text string
	// Markdown, when true, renders Text as styled markdown.
	Markdown bool
	// BorderColor is a hex color (e.g. "#a6e3a1") for the left border.
	// Empty uses the theme's default accent color.
	BorderColor string
	// NoBorder disables the left border entirely.
	NoBorder bool
}

// StatusBarEntryData represents a keyed extension entry in the TUI status bar.
// Multiple entries from different extensions coexist, ordered by Priority
// (lower values render further left).
type StatusBarEntryData struct {
	Key      string // unique identifier (e.g. "myext:git-branch")
	Text     string // rendered content shown in the status bar
	Priority int    // lower = further left; built-in entries use 100-110
}

// UIVisibility controls which built-in TUI chrome elements are visible.
// The zero value shows everything (backward compatible).
type UIVisibility struct {
	HideStartupMessage bool // Hide the "Model loaded..." startup block
	HideStatusBar      bool // Hide the "provider · model  Tokens: ..." line
	HideSeparator      bool // Hide the "────────" divider between stream and input
	HideInputHint      bool // Hide the "enter submit · ctrl+j..." hint below input
}

// AppModelOptions holds configuration passed to NewAppModel.
type AppModelOptions struct {
	// CompactMode selects the compact renderer for message formatting.
	CompactMode bool

	// ModelName is the display name of the model (e.g. "claude-sonnet-4-5").
	ModelName string

	// ProviderName is the LLM provider (e.g. "anthropic", "openai").
	// Used for the startup "Model loaded" message.
	ProviderName string

	// LoadingMessage is an optional informational message from the agent
	// (e.g. GPU fallback info). Displayed at startup when non-empty.
	LoadingMessage string

	// Cwd is the working directory for @file autocomplete and path resolution.
	// If empty, @file features are disabled.
	Cwd string

	// Width is the initial terminal width in columns.
	Width int

	// Height is the initial terminal height in rows.
	Height int

	// ServerNames holds loaded MCP server names for the /servers command.
	ServerNames []string

	// ToolNames holds available tool names for the /tools command.
	ToolNames []string

	// UsageTracker provides token usage statistics for /usage and /reset-usage.
	// May be nil if usage tracking is unavailable for the current model.
	UsageTracker *UsageTracker

	// ExtensionCommands are slash commands registered by extensions. They
	// appear in autocomplete, /help, and are dispatched when submitted.
	ExtensionCommands []ExtensionCommand

	// ContextPaths lists absolute paths of loaded context files (e.g.
	// AGENTS.md). Displayed in the [Context] startup section.
	ContextPaths []string

	// SkillItems lists loaded skills for the [Skills] startup section.
	SkillItems []SkillItem

	// MCPToolCount is the number of tools loaded from external MCP servers.
	MCPToolCount int

	// ExtensionToolCount is the number of tools registered by extensions.
	ExtensionToolCount int

	// GetWidgets returns current extension widgets for a given placement
	// ("above" or "below"). Called during View() to render persistent
	// extension widgets. May be nil if no extensions are loaded.
	GetWidgets func(placement string) []WidgetData

	// GetHeader returns the current custom header set by an extension, or
	// nil if no header is active. Called during View() to render a
	// persistent header above the stream region. May be nil.
	GetHeader func() *WidgetData

	// GetFooter returns the current custom footer set by an extension, or
	// nil if no footer is active. Called during View() to render a
	// persistent footer below the status bar. May be nil.
	GetFooter func() *WidgetData

	// GetToolRenderer returns the extension-provided tool renderer for a
	// specific tool name, or nil if no custom renderer is registered.
	// Called during tool result rendering to check for custom formatting.
	// May be nil if no extensions are loaded.
	GetToolRenderer func(toolName string) *ToolRendererData

	// GetEditorInterceptor returns the current editor interceptor set by
	// an extension, or nil if none is active. Called during Update() to
	// intercept key events and during View() to wrap input rendering.
	// May be nil if no extensions are loaded.
	GetEditorInterceptor func() *EditorInterceptor

	// GetUIVisibility returns the current UI visibility overrides set by
	// an extension, or nil if none have been set (show everything).
	// Called during View() and PrintStartupInfo() to conditionally hide
	// built-in chrome elements. May be nil if no extensions are loaded.
	GetUIVisibility func() *UIVisibility

	// GetStatusBarEntries returns extension-provided status bar entries,
	// sorted by priority. Called during renderStatusBar() to inject
	// extension entries alongside the built-in model/usage display.
	// May be nil if no extensions are loaded.
	GetStatusBarEntries func() []StatusBarEntryData

	// EmitBeforeFork, if non-nil, is called before branching to a
	// different session tree entry. Returns (cancelled, reason) where
	// cancelled=true means the fork should be aborted. May be nil if
	// no extensions are loaded.
	EmitBeforeFork func(targetID string, isUserMsg bool, userText string) (bool, string)

	// EmitBeforeSessionSwitch, if non-nil, is called before switching
	// to a new session branch (e.g. /new, /clear). Returns (cancelled,
	// reason). May be nil if no extensions are loaded.
	EmitBeforeSessionSwitch func(reason string) (bool, string)

	// GetGlobalShortcuts, if non-nil, returns extension-registered global
	// keyboard shortcuts. Keys are binding strings (e.g., "ctrl+p").
	// Handlers are called in a goroutine to avoid blocking the TUI event
	// loop. May be nil if no extensions are loaded.
	GetGlobalShortcuts func() map[string]func()

	// GetExtensionCommands, if non-nil, returns the current extension
	// commands. Called on WidgetUpdateEvent to refresh the command list
	// after an extension hot-reload. May be nil if no extensions loaded.
	GetExtensionCommands func() []ExtensionCommand

	// SetModel changes the active model at runtime. The model string uses
	// "provider/model" format (e.g. "anthropic/claude-sonnet-4-5-20250929").
	// Returns an error if the model string is invalid or the provider cannot
	// be created. May be nil if model switching is not supported.
	SetModel func(modelString string) error

	// EmitModelChange fires the OnModelChange extension event after a
	// successful model switch. Parameters are (newModel, previousModel, source).
	// May be nil if extensions are not loaded.
	EmitModelChange func(newModel, previousModel, source string)

	// ThinkingLevel is the initial thinking level (e.g. "off", "medium").
	ThinkingLevel string
	// IsReasoningModel is true when the current model supports reasoning.
	IsReasoningModel bool
	// SetThinkingLevel changes the thinking level on the agent/provider.
	SetThinkingLevel func(level string) error
}

// AppModel is the root Bubble Tea model for the interactive TUI. It owns the
// state machine, routes events to child components, and manages the overall
// layout. It holds a reference to the app layer (AppController) for triggering
// agent work and queue operations.
//
// Layout (stacked, no alt screen):
//
//	┌─ [custom header] (optional, from extension) ──────┐
//	├─ stream region (variable height) ─────────────────┤
//	│                                                    │
//	├─ separator line (with optional queue count) ───────┤
//	│  [above widgets]                                   │
//	│  queued  How do I fix the build?                   │
//	│  queued  Also check the tests                      │
//	├─ input region (fixed height from textarea) ────────┤
//	│  [below widgets]                                   │
//	│ Tokens: 23.4K (12%) | Cost: $0.00  provider·model │
//	├─ [custom footer] (optional, from extension) ──────┤
//	└────────────────────────────────────────────────────┘
//
// The status bar is always present (1 line) to avoid layout shifts that
// occurred when usage info appeared/disappeared conditionally.
//
// Completed responses are emitted above the BT-managed region via tea.Println()
// before the model resets for the next interaction.
type AppModel struct {
	// state is the current state machine state.
	state appState

	// appCtrl is the app layer reference. Used to call Run(), CancelCurrentStep(), etc.
	// Accepts *app.App via the AppController interface.
	appCtrl AppController

	// input is the child input component (slash commands + autocomplete).
	input inputComponentIface

	// stream is the child streaming display component (spinner + streaming text).
	stream streamComponentIface

	// renderer renders completed messages for tea.Println output. It is either
	// a *MessageRenderer (standard mode) or a *CompactRenderer (compact mode),
	// chosen at construction time via the Renderer interface.
	renderer Renderer

	// compactMode is retained for StreamComponent selection and any remaining
	// mode-specific logic (e.g. startup info formatting).
	compactMode bool

	// modelName is the LLM model name shown in rendered messages.
	modelName string

	// queuedMessages stores the text of prompts that were queued (not yet
	// submitted to the agent). They are rendered with a "queued" badge above
	// the input and move to scrollback when the agent picks them up.
	queuedMessages []string

	// pendingUserPrints holds user messages that have been consumed from the
	// queue but not yet printed to scrollback. They are deferred until
	// SpinnerEvent{Show: true} so the previous assistant response can be
	// flushed first, preserving chronological order.
	pendingUserPrints []string

	// scrollbackBuf collects rendered content during a single Update() call.
	// All print helpers append here instead of returning tea.Println directly.
	// The buffer is drained into a single atomic tea.Println at the end of
	// each Update call via drainScrollback(). If the stream component has
	// unflushed content, it is automatically prepended so that new messages
	// always appear below the previous assistant response.
	scrollbackBuf []string

	// canceling tracks whether the user has pressed ESC once during stateWorking.
	// A second ESC within 2 seconds will cancel the current step.
	canceling bool

	// providerName is the LLM provider for the startup message.
	providerName string

	// loadingMessage is an optional agent startup message (e.g. GPU fallback).
	loadingMessage string

	// serverNames, toolNames are used by /servers and /tools commands.
	serverNames []string
	toolNames   []string

	// usageTracker provides token usage stats for /usage and /reset-usage.
	// May be nil when usage tracking is unavailable.
	usageTracker *UsageTracker

	// extensionCommands are slash commands from extensions, dispatched via
	// handleExtensionCommand when submitted.
	extensionCommands []ExtensionCommand

	// treeSelector is the tree navigation overlay, active in stateTreeSelector.
	treeSelector *TreeSelectorComponent

	// contextPaths and skillItems are used by PrintStartupInfo for the
	// [Context] and [Skills] sections.
	contextPaths []string
	skillItems   []SkillItem

	// mcpToolCount and extensionToolCount track tool counts by source for
	// the startup info display.
	mcpToolCount       int
	extensionToolCount int

	// getWidgets returns extension widgets for a given placement. May be nil.
	getWidgets func(placement string) []WidgetData

	// getHeader returns the current custom header. May be nil.
	getHeader func() *WidgetData

	// getFooter returns the current custom footer. May be nil.
	getFooter func() *WidgetData

	// getEditorInterceptor returns the current editor interceptor. May be nil.
	getEditorInterceptor func() *EditorInterceptor

	// getUIVisibility returns extension-provided UI visibility overrides. May be nil.
	getUIVisibility func() *UIVisibility

	// getStatusBarEntries returns extension-provided status bar entries. May be nil.
	getStatusBarEntries func() []StatusBarEntryData

	// emitBeforeFork emits a before-fork event to extensions. Returns
	// (cancelled, reason). May be nil if no extensions are loaded.
	emitBeforeFork func(targetID string, isUserMsg bool, userText string) (bool, string)

	// emitBeforeSessionSwitch emits a before-session-switch event to extensions.
	// Returns (cancelled, reason). May be nil if no extensions are loaded.
	emitBeforeSessionSwitch func(reason string) (bool, string)

	// thinkingLevel is the current extended thinking level.
	thinkingLevel string
	// thinkingVisible controls whether reasoning blocks are shown or collapsed.
	thinkingVisible bool
	// isReasoningModel is true when the current model supports reasoning.
	isReasoningModel bool
	// setThinkingLevel is a callback to change the thinking level on the agent.
	// It takes the new level string and returns an error if the change fails.
	setThinkingLevel func(level string) error

	// getGlobalShortcuts returns extension-registered keyboard shortcuts.
	// May be nil if no extensions are loaded.
	getGlobalShortcuts func() map[string]func()

	// getExtensionCommands returns the current extension commands. Used
	// to refresh the command list after an extension hot-reload. May be nil.
	getExtensionCommands func() []ExtensionCommand

	// setModel changes the active model at runtime. Wired from cmd/root.go.
	// May be nil if model switching is not supported.
	setModel func(modelString string) error

	// emitModelChange fires the OnModelChange extension event. May be nil.
	emitModelChange func(newModel, previousModel, source string)

	// modelSelector is the model selection overlay, active in stateModelSelector.
	modelSelector *ModelSelectorComponent

	// prompt holds the state of an active interactive prompt overlay. Nil
	// when no prompt is active. Managed by updatePromptState().
	prompt *promptOverlay

	// promptResponseCh is the write-side of the channel used to deliver the
	// user's prompt answer back to the blocking extension goroutine. Set
	// alongside prompt; nil when no prompt is active.
	promptResponseCh chan<- app.PromptResponse

	// prePromptState remembers the state before the prompt overlay took
	// over, so the model can return to it when the prompt completes.
	prePromptState appState

	// overlay holds the state of an active modal overlay dialog. Nil when
	// no overlay is active. Managed by updateOverlayState().
	overlay *overlayDialog

	// overlayResponseCh is the write-side of the channel used to deliver
	// the user's overlay response back to the blocking extension goroutine.
	// Set alongside overlay; nil when no overlay is active.
	overlayResponseCh chan<- app.OverlayResponse

	// preOverlayState remembers the state before the overlay took over,
	// so the model can return to it when the overlay completes.
	preOverlayState appState

	// cwd is the working directory for @file path resolution.
	cwd string

	// width and height track the terminal dimensions.
	width  int
	height int
}

// --------------------------------------------------------------------------
// Child component interfaces (stubs until TAS-15/16/17 implement them)
// --------------------------------------------------------------------------

// inputComponentIface is the interface the parent requires from InputComponent.
// It will be satisfied by the real InputComponent created in TAS-15.
type inputComponentIface interface {
	tea.Model
}

// streamComponentIface is the interface the parent requires from StreamComponent.
// It will be satisfied by the real StreamComponent created in TAS-16.
type streamComponentIface interface {
	tea.Model
	// Reset clears accumulated state between agent steps.
	Reset()
	// SetHeight constrains the render output to at most h lines (0 = unconstrained).
	SetHeight(h int)
	// GetRenderedContent returns the rendered assistant message from accumulated
	// streaming text, or empty string if nothing has been accumulated.
	GetRenderedContent() string
	// SpinnerView returns the rendered spinner line (animation + optional label).
	// Returns "" when the spinner is not active. The parent renders this in the
	// status bar so the spinner never changes the view height.
	SpinnerView() string
	// SetThinkingVisible sets whether reasoning blocks are shown or collapsed.
	SetThinkingVisible(visible bool)
	// HasReasoning returns true if any reasoning content has been accumulated.
	HasReasoning() bool
}

// --------------------------------------------------------------------------
// Constructor
// --------------------------------------------------------------------------

// NewAppModel creates a new AppModel. The appCtrl parameter must not be nil.
// opts provides display configuration; zero values are valid (uses defaults).
//
// To use with the concrete *app.App type, pass it directly — *app.App
// satisfies AppController once the app layer is implemented (TAS-4).
//
// NewAppModel constructs all child components (InputComponent, StreamComponent)
// using the provided options.
func NewAppModel(appCtrl AppController, opts AppModelOptions) *AppModel {
	width := opts.Width
	if width == 0 {
		width = 80 // sensible fallback
	}
	height := opts.Height
	if height == 0 {
		height = 24 // sensible fallback
	}

	// Choose the renderer implementation based on compact mode.
	var rdr Renderer
	if opts.CompactMode {
		cr := NewCompactRenderer(width, false)
		cr.getToolRenderer = opts.GetToolRenderer
		rdr = cr
	} else {
		mr := newMessageRenderer(width, false)
		mr.getToolRenderer = opts.GetToolRenderer
		rdr = mr
	}

	m := &AppModel{
		state:          stateInput,
		appCtrl:        appCtrl,
		renderer:       rdr,
		compactMode:    opts.CompactMode,
		modelName:      opts.ModelName,
		providerName:   opts.ProviderName,
		loadingMessage: opts.LoadingMessage,
		serverNames:    opts.ServerNames,
		toolNames:      opts.ToolNames,
		usageTracker:   opts.UsageTracker,
		cwd:            opts.Cwd,
		width:          width,
		height:         height,
	}

	// Store extension commands for dispatch.
	m.extensionCommands = opts.ExtensionCommands
	m.getWidgets = opts.GetWidgets
	m.getHeader = opts.GetHeader
	m.getFooter = opts.GetFooter
	m.getEditorInterceptor = opts.GetEditorInterceptor
	m.getUIVisibility = opts.GetUIVisibility
	m.getStatusBarEntries = opts.GetStatusBarEntries
	m.emitBeforeFork = opts.EmitBeforeFork
	m.emitBeforeSessionSwitch = opts.EmitBeforeSessionSwitch
	m.getGlobalShortcuts = opts.GetGlobalShortcuts
	m.getExtensionCommands = opts.GetExtensionCommands
	m.setModel = opts.SetModel
	m.emitModelChange = opts.EmitModelChange
	m.thinkingLevel = opts.ThinkingLevel
	m.thinkingVisible = true // default to showing thinking blocks
	m.isReasoningModel = opts.IsReasoningModel
	m.setThinkingLevel = opts.SetThinkingLevel

	// Store context/skills metadata and tool counts for startup display.
	m.contextPaths = opts.ContextPaths
	m.skillItems = opts.SkillItems
	m.mcpToolCount = opts.MCPToolCount
	m.extensionToolCount = opts.ExtensionToolCount

	// Wire up child components now that we have the concrete implementations.
	m.input = NewInputComponent(width, "Enter your prompt (Type /help for commands, Ctrl+C to quit)", appCtrl)

	// Wire up cwd for @file autocomplete.
	if ic, ok := m.input.(*InputComponent); ok && opts.Cwd != "" {
		ic.SetCwd(opts.Cwd)
	}

	// Merge extension commands into the InputComponent's autocomplete source.
	if ic, ok := m.input.(*InputComponent); ok && len(opts.ExtensionCommands) > 0 {
		for _, ec := range opts.ExtensionCommands {
			ic.commands = append(ic.commands, SlashCommand{
				Name:        ec.Name,
				Description: ec.Description,
				Category:    "Extensions",
				Complete:    ec.Complete,
			})
		}
	}

	m.stream = NewStreamComponent(opts.CompactMode, width, opts.ModelName)
	m.stream.SetThinkingVisible(m.thinkingVisible)

	// Propagate initial height distribution to children.
	m.distributeHeight()

	return m
}

// --------------------------------------------------------------------------
// tea.Model interface
// --------------------------------------------------------------------------

// Init implements tea.Model. Initialises child components. Startup info is
// printed to stdout before the program starts via PrintStartupInfo().
func (m *AppModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	if m.input != nil {
		cmds = append(cmds, m.input.Init())
	}
	if m.stream != nil {
		cmds = append(cmds, m.stream.Init())
	}

	return tea.Batch(cmds...)
}

// uiVis returns the current UIVisibility, defaulting to zero value (show all)
// if no extension has set visibility overrides.
func (m *AppModel) uiVis() UIVisibility {
	if m.getUIVisibility != nil {
		if v := m.getUIVisibility(); v != nil {
			return *v
		}
	}
	return UIVisibility{}
}

// PrintStartupInfo prints the startup banner (model name, context, skills,
// tool counts) to stdout. Call this before program.Run() so the messages are
// visible above the Bubble Tea managed region.
//
// All startup information is rendered inside a single system message block.
func (m *AppModel) PrintStartupInfo() {
	if m.uiVis().HideStartupMessage {
		return
	}

	render := func(text string) string {
		return m.renderer.RenderSystemMessage(text, time.Now()).Content
	}

	fmt.Println()

	// Build the combined startup content.
	var lines []string

	if m.providerName != "" && m.modelName != "" {
		lines = append(lines, fmt.Sprintf("Model loaded: %s (%s)", m.providerName, m.modelName))
	}

	if m.loadingMessage != "" {
		lines = append(lines, m.loadingMessage)
	}

	// Context — loaded AGENTS.md files.
	if len(m.contextPaths) > 0 {
		for _, p := range m.contextPaths {
			lines = append(lines, fmt.Sprintf("Context: %s", tildeHome(p)))
		}
	}

	// Skills — listed by name.
	if len(m.skillItems) > 0 {
		names := make([]string, len(m.skillItems))
		for i, si := range m.skillItems {
			names[i] = si.Name
		}
		lines = append(lines, fmt.Sprintf("Skills: %s", strings.Join(names, ", ")))
	}

	// Extension tool count (only shown when > 0).
	if m.extensionToolCount > 0 {
		lines = append(lines, fmt.Sprintf("Loaded %d extension tools", m.extensionToolCount))
	}

	// MCP tool count (only shown when > 0).
	if m.mcpToolCount > 0 {
		lines = append(lines, fmt.Sprintf("Loaded %d tools from MCP servers", m.mcpToolCount))
	}

	if len(lines) > 0 {
		fmt.Println(render(strings.Join(lines, "\n\n")))
	}
}

// tildeHome replaces the user's home directory prefix with ~ for display.
func tildeHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// Update implements tea.Model. It is the heart of the state machine: it routes
// incoming messages to children and handles state transitions.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Prompt overlay takes precedence when active — it is fully modal.
	if m.state == statePrompt && m.prompt != nil {
		return m.updatePromptState(msg)
	}

	// Overlay dialog takes precedence when active — it is fully modal.
	if m.state == stateOverlay && m.overlay != nil {
		return m.updateOverlayState(msg)
	}

	switch msg := msg.(type) {

	// ── Tree selector events ─────────────────────────────────────────────────
	case TreeNodeSelectedMsg:
		// User selected a node in the tree. Branch to it and return to input.
		if ts := m.appCtrl.GetTreeSession(); ts != nil {
			// For user messages: branch to parent (so user can resubmit).
			// For other entries: branch directly to the selected entry.
			targetID := msg.ID
			if msg.IsUser {
				// Branch to parent of user message, place text in editor.
				if node := ts.GetEntry(msg.ID); node != nil {
					if me, ok := node.(*session.MessageEntry); ok {
						targetID = me.ParentID
					}
				}
			}

			// Emit before-fork event in a goroutine so that extension handlers
			// can call blocking operations (e.g. ctx.PromptConfirm) without
			// deadlocking the BubbleTea event loop.
			if m.emitBeforeFork != nil {
				emit := m.emitBeforeFork
				ctrl := m.appCtrl
				forkTargetID := targetID
				forkIsUser := msg.IsUser
				forkUserText := msg.UserText
				go func() {
					cancelled, reason := emit(forkTargetID, forkIsUser, forkUserText)
					ctrl.SendEvent(beforeForkResultMsg{
						cancelled: cancelled,
						reason:    reason,
						targetID:  forkTargetID,
						isUser:    forkIsUser,
						userText:  forkUserText,
					})
				}()
				m.treeSelector = nil
				m.state = stateInput
				return m, func() tea.Msg { return nil }
			}

			cmds = append(cmds, m.performFork(targetID, msg.IsUser, msg.UserText))
		}
		m.treeSelector = nil
		m.state = stateInput
		return m, tea.Batch(cmds...)

	case TreeCancelledMsg:
		m.treeSelector = nil
		m.state = stateInput
		return m, nil

	// ── Model selector events ────────────────────────────────────────────────
	case ModelSelectedMsg:
		m.modelSelector = nil
		m.state = stateInput
		if m.setModel != nil {
			previousModel := m.providerName + "/" + m.modelName
			if err := m.setModel(msg.ModelString); err != nil {
				m.printSystemMessage(fmt.Sprintf("Failed to switch model: %v", err))
			} else {
				// Update display state directly — we cannot use
				// NotifyModelChanged (prog.Send) from inside Update()
				// without deadlocking BubbleTea.
				parts := strings.SplitN(msg.ModelString, "/", 2)
				if len(parts) == 2 {
					m.providerName = parts[0]
					m.modelName = parts[1]
				}
				m.printSystemMessage(fmt.Sprintf("Switched to %s", msg.ModelString))
				if m.emitModelChange != nil {
					emit := m.emitModelChange
					newModel := msg.ModelString
					prev := previousModel
					go emit(newModel, prev, "user")
				}
			}
		}
		cmds = append(cmds, m.drainScrollback())
		return m, tea.Batch(cmds...)

	case ModelSelectorCancelledMsg:
		m.modelSelector = nil
		m.state = stateInput
		return m, nil

	// ── Window resize ────────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.distributeHeight()
		// Propagate to children.
		if m.input != nil {
			_, cmd := m.input.Update(msg)
			cmds = append(cmds, cmd)
		}
		if m.stream != nil {
			_, cmd := m.stream.Update(msg)
			cmds = append(cmds, cmd)
		}

	// ── Keyboard input ───────────────────────────────────────────────────────
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			// Cancel any active prompt before quitting.
			if m.promptResponseCh != nil {
				m.promptResponseCh <- app.PromptResponse{Cancelled: true}
				m.promptResponseCh = nil
				m.prompt = nil
			}
			// Cancel any active overlay before quitting.
			if m.overlayResponseCh != nil {
				m.overlayResponseCh <- app.OverlayResponse{Cancelled: true}
				m.overlayResponseCh = nil
				m.overlay = nil
			}
			// Graceful quit: app.Close() is deferred in cmd/root.go.
			return m, tea.Quit
		}

		// Check extension-registered global keyboard shortcuts. These fire
		// in all app states except modal prompts/overlays (which return early
		// above). Matched shortcuts are consumed — the key does not propagate
		// to child components.
		if m.getGlobalShortcuts != nil {
			if shortcuts := m.getGlobalShortcuts(); shortcuts != nil {
				if handler, ok := shortcuts[msg.String()]; ok {
					// Run in goroutine so blocking extension calls
					// (PromptSelect, etc.) don't stall the event loop.
					go handler()
					return m, tea.Batch(cmds...)
				}
			}
		}

		// Thinking keybindings — only when the model supports reasoning.
		if m.isReasoningModel {
			switch msg.String() {
			case "ctrl+t":
				// Toggle thinking block visibility.
				m.thinkingVisible = !m.thinkingVisible
				if m.stream != nil {
					m.stream.SetThinkingVisible(m.thinkingVisible)
				}
				return m, tea.Batch(cmds...)
			case "shift+tab":
				// Cycle thinking level.
				m.cycleThinkingLevel()
				return m, tea.Batch(cmds...)
			}
		}

		// Route to tree selector when active.
		if m.state == stateTreeSelector && m.treeSelector != nil {
			updated, cmd := m.treeSelector.Update(msg)
			m.treeSelector = updated.(*TreeSelectorComponent)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		// Route to model selector when active.
		if m.state == stateModelSelector && m.modelSelector != nil {
			updated, cmd := m.modelSelector.Update(msg)
			m.modelSelector = updated.(*ModelSelectorComponent)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "esc":
			if m.state == stateWorking {
				if m.canceling {
					// Second ESC within the timer window — cancel the step.
					m.canceling = false
					if m.appCtrl != nil {
						m.appCtrl.CancelCurrentStep()
					}
				} else {
					// First ESC — set canceling, start 2s timer.
					m.canceling = true
					cmds = append(cmds, cancelTimerCmd())
				}
				return m, tea.Batch(cmds...)
			}
			// In other states pass ESC through to children below.
		}

		// Route key events to the focused child. Check for editor
		// interceptor first — it can consume, remap, or force-submit keys.
		if m.input != nil {
			var intercepted bool
			if m.getEditorInterceptor != nil {
				if interceptor := m.getEditorInterceptor(); interceptor != nil && interceptor.HandleKey != nil {
					var currentText string
					if ic, ok := m.input.(*InputComponent); ok {
						currentText = ic.textarea.Value()
					}
					action := interceptor.HandleKey(msg.String(), currentText)
					switch action.Type {
					case EditorKeyConsumed:
						intercepted = true
					case EditorKeyRemap:
						if remapped, ok := remapKey(action.RemappedKey); ok {
							updated, cmd := m.input.Update(remapped)
							m.input, _ = updated.(inputComponentIface)
							cmds = append(cmds, cmd)
							intercepted = true
						}
						// If remap target is unrecognized, fall through to normal handling.
					case EditorKeySubmit:
						text := action.SubmitText
						var images []ImageAttachment
						if text == "" {
							if ic, ok := m.input.(*InputComponent); ok {
								text = strings.TrimSpace(ic.textarea.Value())
								images = ic.ClearPendingImages()
								ic.textarea.SetValue("")
								ic.textarea.CursorEnd()
							}
						}
						if text != "" {
							cmds = append(cmds, func() tea.Msg {
								return submitMsg{Text: text, Images: images}
							})
						}
						intercepted = true
					}
					// EditorKeyPassthrough falls through to normal input handling.
				}
			}
			if !intercepted {
				updated, cmd := m.input.Update(msg)
				m.input, _ = updated.(inputComponentIface)
				cmds = append(cmds, cmd)
			}
		}

	// ── Cancel timer expired ─────────────────────────────────────────────────
	case cancelTimerExpiredMsg:
		m.canceling = false

	// ── Input submitted ──────────────────────────────────────────────────────
	case submitMsg:
		// Handle slash commands locally — they should never reach app.Run().
		if sc := GetCommandByName(msg.Text); sc != nil {
			if cmd := m.handleSlashCommand(sc); cmd != nil {
				cmds = append(cmds, cmd)
			}
			cmds = append(cmds, m.drainScrollback())
			return m, tea.Batch(cmds...)
		}

		// /compact and /model support optional args (e.g. "/compact Focus on API",
		// "/model anthropic/claude-haiku-3-5-20241022").
		// GetCommandByName won't match the full text, so check the prefix.
		if name, args, ok := strings.Cut(msg.Text, " "); ok {
			if sc := GetCommandByName(name); sc != nil {
				switch sc.Name {
				case "/compact":
					if cmd := m.handleCompactCommand(strings.TrimSpace(args)); cmd != nil {
						cmds = append(cmds, cmd)
					}
					cmds = append(cmds, m.drainScrollback())
					return m, tea.Batch(cmds...)
				case "/model":
					if cmd := m.handleModelCommand(strings.TrimSpace(args)); cmd != nil {
						cmds = append(cmds, cmd)
					}
					cmds = append(cmds, m.drainScrollback())
					return m, tea.Batch(cmds...)
				case "/thinking":
					if cmd := m.handleThinkingCommand(strings.TrimSpace(args)); cmd != nil {
						cmds = append(cmds, cmd)
					}
					cmds = append(cmds, m.drainScrollback())
					return m, tea.Batch(cmds...)
				case "/theme":
					if cmd := m.handleThemeCommand(strings.TrimSpace(args)); cmd != nil {
						cmds = append(cmds, cmd)
					}
					cmds = append(cmds, m.drainScrollback())
					return m, tea.Batch(cmds...)
				}
			}
		}

		// Check extension-registered slash commands. These support arguments
		// (e.g. "/sub list files"), so we split on the first space.
		if cmd := m.handleExtensionCommand(msg.Text); cmd != nil {
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		// Regular prompt — forward to the app layer.
		// Preprocess @file references: expand them into XML-wrapped file
		// content before sending to the agent. The display text (shown in
		// scrollback) uses the original user text so the UI stays clean.
		processedText := msg.Text
		if m.cwd != "" {
			processedText = ProcessFileAttachments(msg.Text, m.cwd)
		}

		// Convert image attachments to fantasy.FilePart for the app layer.
		var fileParts []fantasy.FilePart
		for _, img := range msg.Images {
			fileParts = append(fileParts, fantasy.FilePart{
				Data:      img.Data,
				MediaType: img.MediaType,
			})
		}

		// Build display text for scrollback (include image count if any).
		displayText := msg.Text
		if len(msg.Images) > 0 {
			displayText = fmt.Sprintf("%s\n[%d image(s) attached]", msg.Text, len(msg.Images))
		}

		if m.appCtrl != nil {
			// Run returns the queue depth: >0 means the prompt was queued
			// (agent is busy). We update queuedMessages directly here
			// instead of relying on an event from prog.Send(), which would
			// deadlock when called synchronously from within Update().
			var qLen int
			if len(fileParts) > 0 {
				qLen = m.appCtrl.RunWithFiles(processedText, fileParts)
			} else {
				qLen = m.appCtrl.Run(processedText)
			}
			if qLen > 0 {
				// Queued: anchor the message text above the input with a
				// "queued" badge. It will be printed to scrollback when
				// the agent picks it up (via SpinnerEvent).
				m.queuedMessages = append(m.queuedMessages, displayText)
				m.distributeHeight()
			} else {
				// Started immediately. Flush any leftover stream content
				// from the previous step first, then print the user
				// message — combined via the scrollback buffer so
				// scrollback stays in chronological order.
				m.pendingUserPrints = append(m.pendingUserPrints, displayText)
				m.flushStreamAndPendingUserMessages()
			}
		} else {
			m.printUserMessage(displayText)
		}
		if m.state != stateWorking {
			m.state = stateWorking
		}

	// ── Shell command (! / !!) ───────────────────────────────────────────────
	case shellCommandMsg:
		// Show spinner while the shell command runs.
		m.state = stateWorking
		if m.stream != nil {
			_, cmd := m.stream.Update(app.SpinnerEvent{Show: true})
			cmds = append(cmds, cmd)
		}
		// Execute the shell command asynchronously so the TUI stays responsive.
		cmds = append(cmds, m.executeShellCommand(msg))

	case shellCommandResultMsg:
		// Stop spinner now that the command has finished.
		if m.stream != nil {
			_, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			cmds = append(cmds, cmd)
		}
		m.state = stateInput
		cmds = append(cmds, m.handleShellCommandResult(msg))

	// ── App layer events ─────────────────────────────────────────────────────

	case app.SpinnerEvent:
		// SpinnerEvent{Show: true} means a new agent step has started (either
		// freshly or from the queue after a previous step completed). Flush
		// any leftover stream content from the previous step to scrollback
		// before starting the new one, followed by any pending user messages
		// from the queue. Everything goes through the scrollback buffer to
		// guarantee chronological ordering.
		if msg.Show {
			m.flushStreamAndPendingUserMessages()
			m.state = stateWorking
			m.distributeHeight()
		}
		if m.stream != nil {
			_, cmd := m.stream.Update(msg)
			cmds = append(cmds, cmd)
		}

	case app.ReasoningChunkEvent:
		if m.stream != nil {
			_, cmd := m.stream.Update(msg)
			cmds = append(cmds, cmd)
		}

	case app.StreamChunkEvent:
		if m.stream != nil {
			_, cmd := m.stream.Update(msg)
			cmds = append(cmds, cmd)
		}

	case app.ToolCallStartedEvent:
		// Flush any accumulated streaming text to scrollback first (streaming
		// always completes before tool calls fire). The tool call itself is
		// NOT printed here — a unified block (header + result) will be
		// rendered when the ToolResultEvent arrives.
		m.flushStreamContent()

	case app.ToolExecutionEvent:
		// Pass to stream component for execution spinner display.
		if m.stream != nil {
			_, cmd := m.stream.Update(msg)
			cmds = append(cmds, cmd)
		}

	case app.ToolResultEvent:
		// Buffer tool result for scrollback.
		m.printToolResult(msg)
		// Start spinner again while waiting for the next LLM response.
		if m.stream != nil {
			_, cmd := m.stream.Update(app.SpinnerEvent{Show: true})
			cmds = append(cmds, cmd)
		}

	case app.ToolCallContentEvent:
		// In streaming mode this text was already delivered via StreamChunkEvents
		// and will be flushed before the next tool call. Ignore to avoid
		// double-printing.

	case app.ResponseCompleteEvent:
		// This event fires for both streaming and non-streaming paths.
		// In streaming mode, the content was already delivered via StreamChunkEvents
		// and is sitting in the stream component (possibly with reasoning). Don't
		// print or reset — flushStreamContent() handles it on the next step.
		// In non-streaming mode (no stream content accumulated), print the text.
		hasStreamContent := m.stream != nil && m.stream.GetRenderedContent() != ""
		if !hasStreamContent && msg.Content != "" {
			m.printAssistantMessage(msg.Content)
			if m.stream != nil {
				m.stream.Reset()
			}
		}

	case app.MessageCreatedEvent:
		// Informational — no action needed by parent.

	case app.QueueUpdatedEvent:
		// drainQueue popped item(s) from the queue. Move consumed
		// messages to pendingUserPrints — they will be printed to
		// scrollback in the next SpinnerEvent{Show: true} after the
		// previous assistant response is flushed.
		for len(m.queuedMessages) > msg.Length {
			text := m.queuedMessages[0]
			m.queuedMessages = m.queuedMessages[1:]
			m.pendingUserPrints = append(m.pendingUserPrints, text)
		}
		m.distributeHeight()

	case app.StepCompleteEvent:
		// Keep stream content visible in the view — don't flush to scrollback
		// yet. Flushing + resetting in the same frame would shrink the view
		// height, and bubbletea's inline renderer leaves blank lines at the
		// bottom for the orphaned rows. The content will be flushed to
		// scrollback when the next step starts (SpinnerEvent{Show: true}).
		// Just stop the spinner and return to input state.
		if m.stream != nil {
			_, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			cmds = append(cmds, cmd)
		}
		m.state = stateInput
		m.canceling = false

	case app.StepCancelledEvent:
		// User cancelled the step (double-ESC). Keep partial stream content
		// visible (same reasoning as StepCompleteEvent). Just stop the spinner.
		if m.stream != nil {
			_, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			cmds = append(cmds, cmd)
		}
		m.state = stateInput
		m.canceling = false

	case app.StepErrorEvent:
		// Keep partial stream content visible (same reasoning as
		// StepCompleteEvent). Print the error to scrollback — it appears
		// above the view, and the partial response stays visible below.
		if m.stream != nil {
			_, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			cmds = append(cmds, cmd)
		}
		if msg.Err != nil {
			m.printErrorResponse(msg)
		}
		m.state = stateInput
		m.canceling = false

	case app.CompactCompleteEvent:
		if m.stream != nil {
			m.stream.Reset()
		}
		m.state = stateInput
		m.printCompactResult(msg)

	case app.CompactErrorEvent:
		if m.stream != nil {
			m.stream.Reset()
		}
		m.state = stateInput
		m.printSystemMessage(fmt.Sprintf("Compaction failed: %v", msg.Err))

	case app.ModelChangedEvent:
		// Extension changed the model — update display name in status bar
		// and message attribution.
		m.providerName = msg.ProviderName
		m.modelName = msg.ModelName

	case app.WidgetUpdateEvent:
		// Extension widget changed — recalculate height distribution so the
		// stream region accounts for widget space. View() will read the
		// latest widget state on the next render.
		m.distributeHeight()

		// Refresh extension commands (e.g. after hot-reload). The callback
		// returns the current set from the runner which may have changed.
		if m.getExtensionCommands != nil {
			newCmds := m.getExtensionCommands()
			m.extensionCommands = newCmds
			if ic, ok := m.input.(*InputComponent); ok {
				// Remove old extension commands and add fresh ones.
				var builtins []SlashCommand
				for _, sc := range ic.commands {
					if sc.Category != "Extensions" {
						builtins = append(builtins, sc)
					}
				}
				for _, ec := range newCmds {
					builtins = append(builtins, SlashCommand{
						Name:        ec.Name,
						Description: ec.Description,
						Category:    "Extensions",
						Complete:    ec.Complete,
					})
				}
				ic.commands = builtins
			}
		}

	case app.EditorTextSetEvent:
		// Extension wants to pre-fill the input editor with text.
		if ic, ok := m.input.(*InputComponent); ok {
			ic.textarea.SetValue(msg.Text)
			ic.textarea.CursorEnd()
		}

	case app.PromptRequestEvent:
		// Extension wants to show an interactive prompt. Enter prompt state.
		// If already in prompt state (concurrent prompt from another
		// extension), immediately cancel the new request.
		if m.state == statePrompt {
			if msg.ResponseCh != nil {
				msg.ResponseCh <- app.PromptResponse{Cancelled: true}
			}
			return m, tea.Batch(cmds...)
		}
		m.prePromptState = m.state
		m.state = statePrompt
		m.promptResponseCh = msg.ResponseCh

		switch msg.PromptType {
		case "select":
			m.prompt = newSelectPrompt(msg.Message, msg.Options, m.width, m.height)
		case "confirm":
			defaultVal := msg.Default == "true"
			m.prompt = newConfirmPrompt(msg.Message, defaultVal, m.width, m.height)
		case "input":
			m.prompt = newInputPrompt(msg.Message, msg.Placeholder, msg.Default, m.width, m.height)
		default:
			// Unknown prompt type — cancel immediately.
			if msg.ResponseCh != nil {
				msg.ResponseCh <- app.PromptResponse{Cancelled: true}
			}
			m.state = m.prePromptState
			m.promptResponseCh = nil
			return m, tea.Batch(cmds...)
		}
		if m.prompt != nil {
			cmds = append(cmds, m.prompt.Init())
		}

	case app.OverlayRequestEvent:
		// Extension wants to show a modal overlay dialog. Enter overlay state.
		// If already in overlay or prompt state, immediately cancel the request.
		if m.state == stateOverlay || m.state == statePrompt {
			if msg.ResponseCh != nil {
				msg.ResponseCh <- app.OverlayResponse{Cancelled: true}
			}
			return m, tea.Batch(cmds...)
		}
		m.preOverlayState = m.state
		m.state = stateOverlay
		m.overlayResponseCh = msg.ResponseCh

		m.overlay = newOverlayDialog(
			msg.Title, msg.Content, msg.Markdown,
			msg.BorderColor, msg.Background,
			msg.Width, msg.MaxHeight, msg.Anchor,
			msg.Actions,
			m.width, m.height,
		)
		if m.overlay != nil {
			cmds = append(cmds, m.overlay.Init())
		}

	case extensionCmdResultMsg:
		// Async extension slash command completed. Render output/error.
		if msg.err != nil {
			m.printSystemMessage(fmt.Sprintf("Command %s error: %v", msg.name, msg.err))
		} else if msg.output != "" {
			m.printSystemMessage(msg.output)
		}

	case beforeSessionSwitchResultMsg:
		// Async before-session-switch hook completed. Proceed with the
		// session reset if the hook did not cancel.
		if msg.cancelled {
			m.printSystemMessage(msg.reason)
		} else {
			cmds = append(cmds, m.performNewSession())
		}

	case beforeForkResultMsg:
		// Async before-fork hook completed. Proceed with the fork if the
		// hook did not cancel.
		if msg.cancelled {
			m.printSystemMessage(msg.reason)
		} else {
			cmds = append(cmds, m.performFork(msg.targetID, msg.isUser, msg.userText))
		}

	case app.ExtensionPrintEvent:
		// Extension output — route through styled renderers when a level is set.
		switch msg.Level {
		case "info":
			m.printSystemMessage(msg.Text)
		case "error":
			m.printErrorResponse(app.StepErrorEvent{
				Err: fmt.Errorf("%s", msg.Text),
			})
		case "block":
			m.printExtensionBlock(msg)
		default:
			m.appendScrollback(msg.Text)
		}

	default:
		// Pass unrecognised messages to all children.
		if m.input != nil {
			_, cmd := m.input.Update(msg)
			cmds = append(cmds, cmd)
		}
		if m.stream != nil {
			_, cmd := m.stream.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	cmds = append(cmds, m.drainScrollback())
	return m, tea.Batch(cmds...)
}

// View implements tea.Model. It renders the stacked layout:
// stream region + separator + [queued messages] + input region + status bar.
// The status bar is always present (1 line) to avoid layout shifts.
// When the tree selector is active, it replaces the stream region.
func (m *AppModel) View() tea.View {
	// Tree selector overlay replaces the normal layout.
	if m.state == stateTreeSelector && m.treeSelector != nil {
		return m.treeSelector.View()
	}

	// Model selector overlay replaces the normal layout.
	if m.state == stateModelSelector && m.modelSelector != nil {
		return m.modelSelector.View()
	}

	// Overlay dialog replaces the normal layout.
	if m.state == stateOverlay && m.overlay != nil {
		return tea.NewView(m.overlay.Render())
	}

	vis := m.uiVis()

	streamView := m.renderStream()

	// Propagate hint visibility to the input component before rendering.
	if ic, ok := m.input.(*InputComponent); ok {
		ic.hideHint = vis.HideInputHint
	}

	// When a prompt is active, it replaces the input area for consistency
	// (appears below the separator, in the same position as the input).
	var inputView string
	if m.state == statePrompt && m.prompt != nil {
		inputView = m.prompt.Render()
	} else {
		inputView = m.renderInput()
	}

	// Build the stacked layout. Optional header/footer wrap the core layout.
	var parts []string

	// Custom header (if set by extension) — above everything.
	if headerView := m.renderHeaderFooter(m.getHeader); headerView != "" {
		parts = append(parts, headerView)
	}

	// Only include the stream region when it has content. When idle the
	// stream renders "" which JoinVertical would pad to a full-width blank
	// line, inflating the view unnecessarily.
	if streamView != "" {
		parts = append(parts, streamView)
	}

	if !vis.HideSeparator {
		parts = append(parts, m.renderSeparator())
	}

	// Render "above" widgets between separator and queued messages.
	if aboveView := m.renderWidgetSlot("above"); aboveView != "" {
		parts = append(parts, aboveView)
	}

	if queuedView := m.renderQueuedMessages(); queuedView != "" {
		parts = append(parts, queuedView)
	}

	parts = append(parts, inputView)

	// Render "below" widgets between input and status bar.
	if belowView := m.renderWidgetSlot("below"); belowView != "" {
		parts = append(parts, belowView)
	}

	if !vis.HideStatusBar {
		parts = append(parts, m.renderStatusBar())
	}

	// Custom footer (if set by extension) — below everything.
	if footerView := m.renderHeaderFooter(m.getFooter); footerView != "" {
		parts = append(parts, footerView)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return tea.NewView(content)
}

// --------------------------------------------------------------------------
// Rendering helpers
// --------------------------------------------------------------------------

// renderStream returns the stream region content.
func (m *AppModel) renderStream() string {
	if m.stream == nil {
		return ""
	}

	// Show canceling warning if set.
	if m.canceling {
		theme := GetTheme()
		warning := lipgloss.NewStyle().
			Foreground(theme.Warning).
			Bold(true).
			Render("  ⚠ Press ESC again to cancel")
		return lipgloss.JoinVertical(lipgloss.Left,
			m.stream.View().Content,
			warning,
		)
	}

	return m.stream.View().Content
}

// renderStatusBar renders a persistent single-line status bar below the input.
// Left side: spinner (when active). Middle: extension status entries (sorted by
// priority). Right side: provider · model + usage stats.
// This bar is always present so its height is constant, eliminating layout
// shifts from spinner or usage info appearing/disappearing.
func (m *AppModel) renderStatusBar() string {
	theme := GetTheme()

	// Left side: spinner animation (when active).
	var leftSide string
	if m.stream != nil {
		leftSide = m.stream.SpinnerView()
	}

	// Middle: thinking level (when reasoning model) + extension status bar entries.
	var middleParts []string
	if m.isReasoningModel && m.thinkingLevel != "" && m.thinkingLevel != "off" {
		thinkingLabel := "Thinking: " + m.thinkingLevel
		middleParts = append(middleParts, lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Render(thinkingLabel))
	}
	if m.getStatusBarEntries != nil {
		entries := m.getStatusBarEntries()
		for _, e := range entries {
			middleParts = append(middleParts, lipgloss.NewStyle().
				Foreground(theme.Muted).
				Render(e.Text))
		}
	}
	middleSide := strings.Join(middleParts, "  ")
	if middleSide != "" && leftSide != "" {
		middleSide = "  " + middleSide
	}

	// Right side: provider · model + usage stats.
	var rightParts []string

	var modelLabel string
	if m.providerName != "" && m.modelName != "" {
		modelLabel = m.providerName + " · " + m.modelName
	} else if m.modelName != "" {
		modelLabel = m.modelName
	}
	if modelLabel != "" {
		rightParts = append(rightParts, lipgloss.NewStyle().
			Foreground(theme.Muted).
			Render(modelLabel))
	}

	if m.usageTracker != nil {
		if usage := m.usageTracker.RenderUsageInfo(); usage != "" {
			rightParts = append(rightParts, usage)
		}
	}

	rightSide := strings.Join(rightParts, "  ")

	// Progressive truncation to keep the status bar on one line.
	// When content exceeds terminal width, drop sections in order:
	// middle (extensions/thinking) → usage stats → model label → right side.
	leftW := lipgloss.Width(leftSide)
	middleW := lipgloss.Width(middleSide)
	rightW := lipgloss.Width(rightSide)

	// Need at least 1 space gap between left+middle and right.
	if leftW+middleW+rightW+1 > m.width {
		// Drop middle section first (extensions/thinking status).
		middleSide = ""
		middleW = 0
	}
	if leftW+rightW+1 > m.width && len(rightParts) > 1 {
		// Drop usage stats, keep model label.
		rightSide = rightParts[0]
		rightW = lipgloss.Width(rightSide)
	}
	if leftW+rightW+1 > m.width {
		// Drop right side entirely.
		rightSide = ""
		rightW = 0
	}

	gap := max(m.width-leftW-middleW-rightW, 1)
	return leftSide + middleSide + strings.Repeat(" ", gap) + rightSide
}

// cycleThinkingLevel advances to the next thinking level and applies it.
func (m *AppModel) cycleThinkingLevel() {
	levels := []string{"off", "minimal", "low", "medium", "high"}
	current := m.thinkingLevel
	if current == "" {
		current = "off"
	}

	// Find current index and advance to next.
	idx := 0
	for i, l := range levels {
		if l == current {
			idx = i
			break
		}
	}
	next := levels[(idx+1)%len(levels)]
	m.thinkingLevel = next

	// Apply the change to the agent/provider.
	if m.setThinkingLevel != nil {
		// Run in goroutine to avoid blocking the event loop (provider
		// recreation may take time).
		go func() {
			_ = m.setThinkingLevel(next)
		}()
	}
}

// renderSeparator renders the separator line with an optional queue count badge.
func (m *AppModel) renderSeparator() string {
	theme := GetTheme()
	lineStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	queueLen := len(m.queuedMessages)

	if queueLen > 0 {
		badge := lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Render(fmt.Sprintf("%d queued", queueLen))

		// Fill the separator with dashes up to the badge.
		dashWidth := max(m.width-lipgloss.Width(badge)-1, 0)
		dashes := lineStyle.Render(repeatRune('─', dashWidth))
		return dashes + " " + badge
	}

	return lineStyle.Render(repeatRune('─', m.width))
}

// renderInput returns the input region content. If an editor interceptor
// is active and provides a Render function, the default content is passed
// through it for wrapping/modification.
func (m *AppModel) renderInput() string {
	if m.input == nil {
		return ""
	}
	content := m.input.View().Content
	if m.getEditorInterceptor != nil {
		if interceptor := m.getEditorInterceptor(); interceptor != nil && interceptor.Render != nil {
			content = interceptor.Render(m.width, content)
		}
	}
	return content
}

// renderWidgetSlot renders all extension widgets for the given placement
// ("above" or "below"). Returns "" if no widgets exist for that slot.
func (m *AppModel) renderWidgetSlot(placement string) string {
	if m.getWidgets == nil {
		return ""
	}
	widgets := m.getWidgets(placement)
	if len(widgets) == 0 {
		return ""
	}

	theme := GetTheme()
	var blocks []string
	for _, w := range widgets {
		content := w.Text

		var opts []renderingOption
		opts = append(opts, WithAlign(lipgloss.Left))

		if w.NoBorder {
			opts = append(opts, WithNoBorder())
		} else {
			borderClr := theme.Accent
			if w.BorderColor != "" {
				borderClr = lipgloss.Color(w.BorderColor)
			}
			opts = append(opts, WithBorderColor(borderClr))
		}

		// Use tighter padding for widgets (less vertical padding than
		// full message blocks) so they feel compact and unobtrusive.
		opts = append(opts, WithPaddingTop(0), WithPaddingBottom(0))

		blocks = append(blocks, renderContentBlock(content, m.width, opts...))
	}
	return strings.Join(blocks, "\n")
}

// renderHeaderFooter renders a custom header or footer from an extension. The
// getter function returns the current data (*WidgetData) or nil when inactive.
// Returns "" when the getter is nil or returns nil. Uses the same rendering
// pipeline as widgets for visual consistency.
func (m *AppModel) renderHeaderFooter(getter func() *WidgetData) string {
	if getter == nil {
		return ""
	}
	data := getter()
	if data == nil {
		return ""
	}

	theme := GetTheme()

	var opts []renderingOption
	opts = append(opts, WithAlign(lipgloss.Left))

	if data.NoBorder {
		opts = append(opts, WithNoBorder())
	} else {
		borderClr := theme.Accent
		if data.BorderColor != "" {
			borderClr = lipgloss.Color(data.BorderColor)
		}
		opts = append(opts, WithBorderColor(borderClr))
	}

	// Compact padding like widgets.
	opts = append(opts, WithPaddingTop(0), WithPaddingBottom(0))

	return renderContentBlock(data.Text, m.width, opts...)
}

// renderQueuedMessages renders queued prompts as styled content blocks with a
// "QUEUED" badge, anchored between the separator and input. Each message is
// displayed in a bordered block matching the overall message styling.
func (m *AppModel) renderQueuedMessages() string {
	if len(m.queuedMessages) == 0 {
		return ""
	}
	theme := GetTheme()
	badge := CreateBadge("QUEUED", theme.Accent)

	var blocks []string
	for _, msg := range m.queuedMessages {
		content := msg + "\n" + badge
		rendered := renderContentBlock(
			content,
			m.width,
			WithAlign(lipgloss.Left),
			WithBorderColor(theme.Muted),
		)
		blocks = append(blocks, rendered)
	}
	return strings.Join(blocks, "\n")
}

// --------------------------------------------------------------------------
// Print helpers — emit content to scrollback via tea.Println
// --------------------------------------------------------------------------

// printUserMessage renders a user message into the scrollback buffer.
func (m *AppModel) printUserMessage(text string) {
	m.appendScrollback(m.renderer.RenderUserMessage(text, time.Now()).Content)
}

// printAssistantMessage renders an assistant message into the scrollback buffer.
func (m *AppModel) printAssistantMessage(text string) {
	if text != "" {
		m.appendScrollback(m.renderer.RenderAssistantMessage(text, time.Now(), m.modelName).Content)
	}
}

// printToolResult renders a tool result message into the scrollback buffer.
func (m *AppModel) printToolResult(evt app.ToolResultEvent) {
	m.appendScrollback(m.renderer.RenderToolMessage(evt.ToolName, evt.ToolArgs, evt.Result, evt.IsError).Content)
}

// printErrorResponse renders an error message into the scrollback buffer.
func (m *AppModel) printErrorResponse(evt app.StepErrorEvent) {
	if evt.Err != nil {
		m.appendScrollback(m.renderer.RenderErrorMessage(evt.Err.Error(), time.Now()).Content)
	}
}

// --------------------------------------------------------------------------
// Slash command handlers
// --------------------------------------------------------------------------

// handleSlashCommand executes a recognized slash command and returns a tea.Cmd
// that emits the appropriate output to scrollback. Returns tea.Quit for /quit,
// nil for commands with no visible output, or a tea.Println cmd for display.
func (m *AppModel) handleSlashCommand(sc *SlashCommand) tea.Cmd {
	switch sc.Name {
	case "/quit":
		return tea.Quit
	case "/help":
		m.printHelpMessage()
	case "/tools":
		m.printToolsMessage()
	case "/servers":
		m.printServersMessage()
	case "/usage":
		m.printUsageMessage()
	case "/reset-usage":
		m.printResetUsage()
	case "/model":
		return m.handleModelCommand("")
	case "/theme":
		return m.handleThemeCommand("")
	case "/thinking":
		return m.handleThinkingCommand("")
	case "/compact":
		return m.handleCompactCommand("")
	case "/clear":
		if m.appCtrl != nil {
			m.appCtrl.ClearMessages()
		}
		m.printSystemMessage("Conversation cleared. Starting fresh.")
	case "/clear-queue":
		if m.appCtrl != nil {
			m.appCtrl.ClearQueue()
		}
		m.queuedMessages = m.queuedMessages[:0]
		m.distributeHeight()

	case "/tree":
		return m.handleTreeCommand()
	case "/fork":
		return m.handleForkCommand()
	case "/new":
		return m.handleNewCommand()
	case "/name":
		return m.handleNameCommand()
	case "/session":
		return m.handleSessionInfoCommand()

	default:
		m.printSystemMessage(fmt.Sprintf("Unknown command: %s", sc.Name))
	}
	return nil
}

// printSystemMessage renders a system-level message into the scrollback buffer.
func (m *AppModel) printSystemMessage(text string) {
	m.appendScrollback(m.renderer.RenderSystemMessage(text, time.Now()).Content)
}

// printExtensionBlock renders a custom styled block from an extension with
// caller-chosen border color and optional subtitle into the scrollback buffer.
func (m *AppModel) printExtensionBlock(evt app.ExtensionPrintEvent) {
	theme := GetTheme()

	// Resolve border color: use the extension's hex value, fall back to theme info.
	borderClr := theme.Info
	if evt.BorderColor != "" {
		borderClr = lipgloss.Color(evt.BorderColor)
	}

	// Build content: main text + optional subtitle line.
	content := evt.Text
	if evt.Subtitle != "" {
		sub := lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(" " + evt.Subtitle)
		content = strings.TrimSuffix(content, "\n") + "\n" + sub
	}

	rendered := renderContentBlock(
		content,
		m.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(borderClr),
		WithMarginBottom(1),
	)
	m.appendScrollback(rendered)
}

// handleExtensionCommand checks if the submitted text matches an extension-
// registered slash command and returns a tea.Cmd that runs it. Returns nil
// if no extension command matches.
//
// Extension commands execute asynchronously (via tea.Cmd goroutine) so they
// can safely call blocking operations like ctx.PromptSelect() without
// deadlocking the TUI's Update loop. The result is delivered back as an
// extensionCmdResultMsg.
//
// Extension commands support arguments: "/sub list files" is split into
// command name "/sub" and args "list files".
func (m *AppModel) handleExtensionCommand(text string) tea.Cmd {
	if len(m.extensionCommands) == 0 {
		return nil
	}

	// Only consider inputs that look like slash commands.
	if !strings.HasPrefix(text, "/") {
		return nil
	}

	// Split: "/sub list files" → name="/sub", args="list files"
	name, args, _ := strings.Cut(text, " ")
	ecmd := FindExtensionCommand(name, m.extensionCommands)
	if ecmd == nil {
		return nil
	}

	// Run the command in a dedicated goroutine — NOT as a tea.Cmd. Extension
	// commands may block on interactive prompts (ctx.PromptSelect etc.) which
	// wait for the TUI to respond via a channel. A blocking tea.Cmd can stall
	// BubbleTea's internal Cmd scheduler, causing intermittent freezes.
	// The goroutine delivers its result via SendEvent (prog.Send) instead.
	cmdName := ecmd.Name
	cmdExec := ecmd.Execute
	cmdArgs := args
	ctrl := m.appCtrl
	go func() {
		output, err := cmdExec(cmdArgs)
		ctrl.SendEvent(extensionCmdResultMsg{name: cmdName, output: output, err: err})
	}()
	// Return a non-nil Cmd so the caller knows the command was handled
	// and doesn't fall through to the regular prompt path. The Cmd itself
	// is a no-op.
	return func() tea.Msg { return nil }
}

// printHelpMessage renders the help text listing all available slash commands.
func (m *AppModel) printHelpMessage() {
	help := "## Available Commands\n\n" +
		"**Info:**\n" +
		"- `/help`: Show this help message\n" +
		"- `/tools`: List all available tools\n" +
		"- `/servers`: List configured MCP servers\n" +
		"- `/usage`: Show token usage and cost statistics\n" +
		"- `/session`: Show session info and statistics\n\n" +
		"**Navigation:**\n" +
		"- `/tree`: Navigate session tree (switch branches)\n" +
		"- `/fork`: Branch from an earlier message\n" +
		"- `/new`: Start a new branch (preserves history)\n\n" +
		"**System:**\n" +
		"- `/compact [instructions]`: Summarise older messages to free context space\n" +
		"- `/clear`: Clear message history\n" +
		"- `/reset-usage`: Reset usage statistics\n" +
		"- `/quit`: Exit the application\n\n"

	if len(m.extensionCommands) > 0 {
		var extHelp strings.Builder
		extHelp.WriteString("**Extensions:**\n")
		for _, ec := range m.extensionCommands {
			fmt.Fprintf(&extHelp, "- `%s`: %s\n", ec.Name, ec.Description)
		}
		extHelp.WriteString("\n")
		help += extHelp.String()
	}

	if len(m.skillItems) > 0 {
		var skillHelp strings.Builder
		skillHelp.WriteString("**Skills:**\n")
		skillHelp.WriteString("- `/skill:<name> [args]`: Load a skill into context and run with optional args\n")
		skillHelp.WriteString("  Available skills: ")
		for i, si := range m.skillItems {
			if i > 0 {
				skillHelp.WriteString(", ")
			}
			skillHelp.WriteString("`" + si.Name + "`")
		}
		skillHelp.WriteString("\n\n")
		help += skillHelp.String()
	}

	help += "**Shell Commands:**\n" +
		"- `!command`: Run shell command, output included in LLM context\n" +
		"- `!!command`: Run shell command, output excluded from LLM context\n\n" +
		"**Keys:**\n" +
		"- `Ctrl+C`: Exit at any time\n" +
		"- `ESC` (x2): Cancel ongoing LLM generation\n\n" +
		"You can also just type your message to chat with the AI assistant."
	m.printSystemMessage(help)
}

// printToolsMessage renders the list of available tools.
func (m *AppModel) printToolsMessage() {
	var content string
	content = "## Available Tools\n\n"
	if len(m.toolNames) == 0 {
		content += "No tools are currently available."
	} else {
		for i, tool := range m.toolNames {
			content += fmt.Sprintf("%d. `%s`\n", i+1, tool)
		}
	}
	m.printSystemMessage(content)
}

// printServersMessage renders the list of configured MCP servers.
func (m *AppModel) printServersMessage() {
	var content string
	content = "## Configured MCP Servers\n\n"
	if len(m.serverNames) == 0 {
		content += "No MCP servers are currently configured."
	} else {
		for i, server := range m.serverNames {
			content += fmt.Sprintf("%d. `%s`\n", i+1, server)
		}
	}
	m.printSystemMessage(content)
}

// printUsageMessage renders token usage statistics.
func (m *AppModel) printUsageMessage() {
	if m.usageTracker == nil {
		m.printSystemMessage("Usage tracking is not available for this model.")
		return
	}

	sessionStats := m.usageTracker.GetSessionStats()
	lastStats := m.usageTracker.GetLastRequestStats()

	content := "## Usage Statistics\n\n"
	if lastStats != nil {
		content += fmt.Sprintf("**Last Request:** %d input + %d output tokens = $%.6f\n",
			lastStats.InputTokens, lastStats.OutputTokens, lastStats.TotalCost)
	}
	content += fmt.Sprintf("**Session Total:** %d input + %d output tokens = $%.6f (%d requests)\n",
		sessionStats.TotalInputTokens, sessionStats.TotalOutputTokens, sessionStats.TotalCost, sessionStats.RequestCount)

	m.printSystemMessage(content)
}

// printResetUsage resets usage statistics and prints a confirmation.
func (m *AppModel) printResetUsage() {
	if m.usageTracker == nil {
		m.printSystemMessage("Usage tracking is not available for this model.")
		return
	}
	m.usageTracker.Reset()
	m.printSystemMessage("Usage statistics have been reset.")
}

// handleCompactCommand starts an async compaction. It returns a tea.Cmd that
// prints a "compacting..." message and transitions to the working state. If
// the app controller rejects the request (busy, closed) it prints an error
// instead. customInstructions is optional text appended to the summary
// prompt (e.g. "Focus on the API design decisions").
func (m *AppModel) handleCompactCommand(customInstructions string) tea.Cmd {
	if m.appCtrl == nil {
		m.printSystemMessage("Compaction is not available.")
		return nil
	}
	if err := m.appCtrl.CompactConversation(customInstructions); err != nil {
		m.printSystemMessage(fmt.Sprintf("Cannot compact: %v", err))
		return nil
	}
	// Transition to working state so the spinner shows while compaction runs.
	m.state = stateWorking
	m.printSystemMessage("Compacting conversation...")
	var spinnerCmd tea.Cmd
	if m.stream != nil {
		_, spinnerCmd = m.stream.Update(app.SpinnerEvent{Show: true})
	}
	return spinnerCmd
}

// printCompactResult renders the compaction summary in a styled block with
// a distinct border color and a stats subtitle into the scrollback buffer.
func (m *AppModel) printCompactResult(evt app.CompactCompleteEvent) {
	theme := GetTheme()

	saved := evt.OriginalTokens - evt.CompactedTokens
	subtitle := fmt.Sprintf(
		"%d messages summarised, ~%dk tokens freed (%dk -> %dk)",
		evt.MessagesRemoved, saved/1000, evt.OriginalTokens/1000, evt.CompactedTokens/1000,
	)

	content := evt.Summary
	if subtitle != "" {
		sub := lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(" " + subtitle)
		content = strings.TrimSuffix(content, "\n") + "\n\n" + sub
	}

	rendered := renderContentBlock(
		content,
		m.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.Secondary),
		WithMarginBottom(1),
	)
	m.appendScrollback(rendered)
}

// flushStreamContent moves rendered content from the stream component into the
// scrollback buffer and resets the stream. Called before tool calls (streaming
// completes before tools fire). The actual tea.Println is deferred to
// drainScrollback() at the end of the Update cycle.
func (m *AppModel) flushStreamContent() {
	if m.stream == nil {
		return
	}
	content := m.stream.GetRenderedContent()
	if content == "" {
		return
	}
	m.stream.Reset()
	m.appendScrollback(content)
}

// flushStreamAndPendingUserMessages moves the previous assistant response and
// any pending queued user messages into the scrollback buffer. Called from
// SpinnerEvent{Show: true} where all previous stream chunks are guaranteed to
// have been processed. The actual tea.Println is deferred to drainScrollback().
func (m *AppModel) flushStreamAndPendingUserMessages() {
	// 1. Flush previous stream content (assistant response).
	if m.stream != nil {
		if content := m.stream.GetRenderedContent(); content != "" {
			m.stream.Reset()
			m.appendScrollback(content)
		}
	}

	// 2. Render pending user messages from the queue.
	for _, text := range m.pendingUserPrints {
		rendered := m.renderer.RenderUserMessage(text, time.Now()).Content
		m.appendScrollback(rendered)
	}
	m.pendingUserPrints = nil
}

// appendScrollback adds rendered content to the scrollback buffer. The content
// will be emitted via tea.Println when drainScrollback is called at the end of
// the current Update cycle.
func (m *AppModel) appendScrollback(content string) {
	if content != "" {
		m.scrollbackBuf = append(m.scrollbackBuf, content)
	}
}

// drainScrollback flushes the scrollback buffer into a single tea.Println. If
// the stream component has unflushed content, it is automatically prepended so
// that new messages always appear below the previous assistant response. When
// stream content is flushed a ClearScreen follows to clean up orphaned terminal
// rows left after the view height shrinks. Returns nil if there is nothing to
// print.
func (m *AppModel) drainScrollback() tea.Cmd {
	if len(m.scrollbackBuf) == 0 {
		return nil
	}

	var parts []string
	needsClear := false

	// Auto-flush any stream content so it appears before new messages.
	if m.stream != nil {
		if content := m.stream.GetRenderedContent(); content != "" {
			m.stream.Reset()
			parts = append(parts, content)
			needsClear = true
		}
	}

	parts = append(parts, m.scrollbackBuf...)
	m.scrollbackBuf = m.scrollbackBuf[:0]

	printCmd := tea.Println(strings.Join(parts, "\n"))
	if needsClear {
		return tea.Sequence(
			printCmd,
			func() tea.Msg { return tea.ClearScreen() },
		)
	}
	return printCmd
}

// distributeHeight recalculates child component heights after a window resize,
// queue change, widget update, or state transition, and propagates the computed
// stream height to the StreamComponent.
//
// Layout (line counts):
//
//	header         = measured dynamically (0 if not set)
//	stream region  = total - header - separator(1) - widgets - queued(N*5) - input(measured) - widgets - statusBar(1) - footer
//	separator      = 1 line
//	above widgets  = measured dynamically
//	queued msgs    = measured dynamically via lipgloss.Height()
//	input region   = measured dynamically via lipgloss.Height()
//	below widgets  = measured dynamically
//	status bar     = 1 line (always present)
//	footer         = measured dynamically (0 if not set)
func (m *AppModel) distributeHeight() {
	vis := m.uiVis()

	separatorLines := 1
	if vis.HideSeparator {
		separatorLines = 0
	}
	statusBarLines := 1
	if vis.HideStatusBar {
		statusBarLines = 0
	}
	// Measure actual queued message height instead of using a fixed estimate,
	// since text wrapping at different widths changes the rendered line count.
	var queuedLines int
	if queuedView := m.renderQueuedMessages(); queuedView != "" {
		queuedLines = lipgloss.Height(queuedView)
	}

	// Propagate hint visibility before measuring input height.
	if ic, ok := m.input.(*InputComponent); ok {
		ic.hideHint = vis.HideInputHint
	}

	// Measure the actual rendered input (or prompt overlay) height so we
	// don't rely on a fragile constant that drifts when styling changes.
	// Use renderInput() which includes the editor interceptor's Render
	// wrapper so the measured height matches what View() actually renders.
	inputLines := 9 // fallback: title(1)+margin(1)+nl(1)+textarea(3)+nl(1)+margin(1)+help(1)
	if m.state == statePrompt && m.prompt != nil {
		if rendered := m.prompt.Render(); rendered != "" {
			inputLines = lipgloss.Height(rendered)
		}
	} else {
		if rendered := m.renderInput(); rendered != "" {
			inputLines = lipgloss.Height(rendered)
		}
	}

	// Measure widget heights.
	var widgetLines int
	if above := m.renderWidgetSlot("above"); above != "" {
		widgetLines += lipgloss.Height(above)
	}
	if below := m.renderWidgetSlot("below"); below != "" {
		widgetLines += lipgloss.Height(below)
	}

	// Measure header/footer heights.
	var headerFooterLines int
	if headerView := m.renderHeaderFooter(m.getHeader); headerView != "" {
		headerFooterLines += lipgloss.Height(headerView)
	}
	if footerView := m.renderHeaderFooter(m.getFooter); footerView != "" {
		headerFooterLines += lipgloss.Height(footerView)
	}

	streamHeight := max(m.height-separatorLines-widgetLines-headerFooterLines-queuedLines-inputLines-statusBarLines, 0)

	if m.stream != nil {
		m.stream.SetHeight(streamHeight)
	}
}

// clamp constrains v to the range [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// repeatRune returns a string consisting of n repetitions of r.
func repeatRune(r rune, n int) string {
	if n <= 0 {
		return ""
	}
	runes := make([]rune, n)
	for i := range runes {
		runes[i] = r
	}
	return string(runes)
}

// --------------------------------------------------------------------------
// Editor key remapping
// --------------------------------------------------------------------------

// remapKey converts a key name string to a tea.KeyPressMsg for editor key
// remapping. Returns the KeyPressMsg and true if the key name is recognized,
// or a zero value and false if unknown.
func remapKey(name string) (tea.KeyPressMsg, bool) {
	switch name {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}, true
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}, true
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}, true
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}, true
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}, true
	case "delete":
		return tea.KeyPressMsg{Code: tea.KeyDelete}, true
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}, true
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}, true
	case "esc", "escape":
		return tea.KeyPressMsg{Code: tea.KeyEscape}, true
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}, true
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}, true
	case "pgup", "pageup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}, true
	case "pgdown", "pagedown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}, true
	case "space":
		return tea.KeyPressMsg{Code: ' ', Text: " "}, true
	default:
		// Single printable character.
		runes := []rune(name)
		if len(runes) == 1 {
			return tea.KeyPressMsg{Code: runes[0], Text: name}, true
		}
		return tea.KeyPressMsg{}, false
	}
}

// --------------------------------------------------------------------------
// Model command handler
// --------------------------------------------------------------------------

// handleModelCommand handles the /model slash command. With no arguments, it
// opens an interactive model selector overlay with fuzzy finding. With an
// argument (e.g. "/model anthropic/claude-haiku-3-5-20241022"), it switches
// to that model directly.
func (m *AppModel) handleModelCommand(args string) tea.Cmd {
	if m.setModel == nil {
		m.printSystemMessage("Model switching is not available.")
		return nil
	}

	if args == "" {
		// Open the interactive model selector.
		currentModel := m.providerName + "/" + m.modelName
		m.modelSelector = NewModelSelector(currentModel, m.width, m.height)
		m.state = stateModelSelector
		return nil
	}

	// Direct model switch with the provided model string.
	previousModel := m.providerName + "/" + m.modelName
	if err := m.setModel(args); err != nil {
		m.printSystemMessage(fmt.Sprintf("Failed to switch model: %v", err))
		return nil
	}

	// Update display state directly (cannot use prog.Send from Update).
	parts := strings.SplitN(args, "/", 2)
	if len(parts) == 2 {
		m.providerName = parts[0]
		m.modelName = parts[1]
	}

	if m.emitModelChange != nil {
		emit := m.emitModelChange
		prev := previousModel
		newModel := args
		go emit(newModel, prev, "user")
	}

	m.printSystemMessage(fmt.Sprintf("Switched to %s", args))
	return nil
}

// --------------------------------------------------------------------------
// Theme command handler
// --------------------------------------------------------------------------

// handleThemeCommand switches the active color theme. With no arguments it
// lists available themes and highlights the active one. With a name argument
// (e.g. "/theme catppuccin") it switches immediately.
func (m *AppModel) handleThemeCommand(args string) tea.Cmd {
	if args == "" {
		// List available themes.
		names := ListThemes()
		active := ActiveThemeName()

		var lines []string
		lines = append(lines, "Available themes:")
		for _, name := range names {
			if name == active {
				lines = append(lines, fmt.Sprintf("  * %s (active)", name))
			} else {
				lines = append(lines, fmt.Sprintf("    %s", name))
			}
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("User themes:    %s", userThemesDir()))
		if pdir := projectThemesDir(); pdir != "" {
			lines = append(lines, fmt.Sprintf("Project themes: %s", pdir))
		} else {
			lines = append(lines, "Project themes: .kit/themes/ (not found)")
		}
		m.printSystemMessage(strings.Join(lines, "\n"))
		return nil
	}

	if err := ApplyTheme(args); err != nil {
		m.printSystemMessage(fmt.Sprintf("Theme error: %v", err))
		return nil
	}

	m.printSystemMessage(fmt.Sprintf("Switched to theme: %s", args))
	return nil
}

// --------------------------------------------------------------------------
// Thinking command handler
// --------------------------------------------------------------------------

// handleThinkingCommand changes or displays the current thinking/reasoning level.
// With no arguments, it shows the current level. With a level argument (off,
// minimal, low, medium, high) it switches to that level.
func (m *AppModel) handleThinkingCommand(args string) tea.Cmd {
	if !m.isReasoningModel {
		m.printSystemMessage("Current model does not support thinking/reasoning.")
		return nil
	}

	if args == "" {
		// Show current level with descriptions.
		var lines []string
		levels := models.ThinkingLevels()
		for _, l := range levels {
			marker := "  "
			if string(l) == m.thinkingLevel {
				marker = "▸ "
			}
			lines = append(lines, fmt.Sprintf("%s%s — %s", marker, l, models.ThinkingLevelDescription(l)))
		}
		header := fmt.Sprintf("Current thinking level: %s\n\nAvailable levels:", m.thinkingLevel)
		m.printSystemMessage(header + "\n" + strings.Join(lines, "\n"))
		return nil
	}

	// Parse and validate the level.
	level := models.ParseThinkingLevel(args)
	if string(level) != strings.ToLower(args) {
		m.printSystemMessage(fmt.Sprintf("Unknown thinking level: %q. Use: off, minimal, low, medium, high", args))
		return nil
	}

	// Apply the change.
	m.thinkingLevel = string(level)
	if m.setThinkingLevel != nil {
		go func() {
			_ = m.setThinkingLevel(string(level))
		}()
	}
	m.printSystemMessage(fmt.Sprintf("Thinking level set to: %s — %s", level, models.ThinkingLevelDescription(level)))
	return nil
}

// --------------------------------------------------------------------------
// Tree session command handlers
// --------------------------------------------------------------------------

// handleTreeCommand opens the tree selector overlay.
func (m *AppModel) handleTreeCommand() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		m.printSystemMessage("No tree session active. Start with `--continue` or `--resume` to enable tree sessions.")
		return nil
	}
	if ts.EntryCount() == 0 {
		m.printSystemMessage("No entries in session yet.")
		return nil
	}

	m.treeSelector = NewTreeSelector(ts, m.width, m.height)
	m.state = stateTreeSelector
	return nil
}

// handleForkCommand creates a branch from the current position. Like /tree
// but opens the selector directly for fork semantics.
func (m *AppModel) handleForkCommand() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		m.printSystemMessage("No tree session active. Start with `--continue` or `--resume` to enable tree sessions.")
		return nil
	}
	if ts.EntryCount() == 0 {
		m.printSystemMessage("No entries to fork from.")
		return nil
	}

	m.treeSelector = NewTreeSelector(ts, m.width, m.height)
	m.state = stateTreeSelector
	return nil
}

// handleNewCommand starts a fresh session by resetting the tree leaf.
func (m *AppModel) handleNewCommand() tea.Cmd {
	// Emit before-session-switch event in a goroutine so that extension
	// handlers can call blocking operations (e.g. ctx.PromptConfirm) without
	// deadlocking the BubbleTea event loop.
	if m.emitBeforeSessionSwitch != nil {
		emit := m.emitBeforeSessionSwitch
		ctrl := m.appCtrl
		go func() {
			cancelled, reason := emit("new")
			ctrl.SendEvent(beforeSessionSwitchResultMsg{
				cancelled: cancelled,
				reason:    reason,
			})
		}()
		return func() tea.Msg { return nil }
	}

	return m.performNewSession()
}

// performNewSession performs the actual session reset. Called either directly
// (when no before-hook exists) or after the async hook completes.
func (m *AppModel) performNewSession() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		// No tree session — just clear messages.
		if m.appCtrl != nil {
			m.appCtrl.ClearMessages()
		}
		m.printSystemMessage("Conversation cleared. Starting fresh.")
		return nil
	}

	ts.ResetLeaf()
	if m.appCtrl != nil {
		m.appCtrl.ClearMessages()
	}
	m.printSystemMessage("New branch started. Previous conversation is preserved in the tree.")
	return nil
}

// performFork performs the actual tree branch. Called either directly (when no
// before-hook exists) or after the async before-fork hook completes.
func (m *AppModel) performFork(targetID string, isUser bool, userText string) tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		m.printSystemMessage("No tree session active.")
		return nil
	}

	_ = ts.Branch(targetID)
	m.appCtrl.ClearMessages()

	// If it was a user message, populate the input with the text.
	if isUser && userText != "" {
		if ic, ok := m.input.(*InputComponent); ok {
			ic.textarea.SetValue(userText)
			ic.textarea.CursorEnd()
		}
	}

	m.printSystemMessage(
		fmt.Sprintf("Navigated to branch point. %s",
			func() string {
				if isUser {
					return "Edit and resubmit to create a new branch."
				}
				return "Continue from this point."
			}()))
	return nil
}

// handleNameCommand sets a display name for the current session.
func (m *AppModel) handleNameCommand() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		m.printSystemMessage("No tree session active.")
		return nil
	}
	// For now, prompt user to provide name via input. We print instructions
	// and the next non-command input starting with "name:" will be captured.
	// TODO: inline input dialog.
	currentName := ts.GetSessionName()
	if currentName != "" {
		m.printSystemMessage(fmt.Sprintf("Current session name: %q\nTo rename, type: `/name <new name>` (not yet implemented — use the session file directly).", currentName))
		return nil
	}
	m.printSystemMessage("To name this session, use: `/name <new name>` (not yet implemented — use the session file directly).")
	return nil
}

// handleSessionInfoCommand shows session statistics.
func (m *AppModel) handleSessionInfoCommand() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		m.printSystemMessage("No tree session active.")
		return nil
	}

	header := ts.GetHeader()
	info := fmt.Sprintf("## Session Info\n\n"+
		"- **ID:** `%s`\n"+
		"- **File:** `%s`\n"+
		"- **Working Dir:** `%s`\n"+
		"- **Created:** %s\n"+
		"- **Entries:** %d\n"+
		"- **Messages:** %d\n"+
		"- **Current Leaf:** `%s`\n",
		header.ID,
		ts.GetFilePath(),
		header.Cwd,
		header.Timestamp.Format(time.RFC3339),
		ts.EntryCount(),
		ts.MessageCount(),
		ts.GetLeafID(),
	)

	if name := ts.GetSessionName(); name != "" {
		info += fmt.Sprintf("- **Name:** %s\n", name)
	}

	m.printSystemMessage(info)
	return nil
}

// --------------------------------------------------------------------------
// Cancel timer command
// --------------------------------------------------------------------------

// cancelTimerCmd returns a tea.Cmd that fires cancelTimerExpiredMsg after 2s.
// This is used for the double-tap ESC cancel flow.
func cancelTimerCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return cancelTimerExpiredMsg{}
	})
}

// --------------------------------------------------------------------------
// Interactive prompt support
// --------------------------------------------------------------------------

// extensionCmdResultMsg carries the result of an asynchronously executed
// extension slash command. Extension commands run async (via tea.Cmd) so they
// can safely call blocking operations like ctx.PromptSelect().
type extensionCmdResultMsg struct {
	name   string
	output string
	err    error
}

// beforeSessionSwitchResultMsg carries the result of an asynchronously
// executed before-session-switch hook. The hook runs in a goroutine so that
// blocking operations like ctx.PromptConfirm() do not deadlock the TUI.
type beforeSessionSwitchResultMsg struct {
	cancelled bool
	reason    string
}

// beforeForkResultMsg carries the result of an asynchronously executed
// before-fork hook along with the fork context needed to complete the
// operation if the hook allows it.
type beforeForkResultMsg struct {
	cancelled bool
	reason    string
	// Fork context — preserved so the operation can proceed after the hook.
	targetID string
	isUser   bool
	userText string
}

// updatePromptState handles all messages while the prompt overlay is active.
// It routes keys to the prompt overlay, detects completion/cancellation, and
// restores the previous state when done.
func (m *AppModel) updatePromptState(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			// Cancel prompt and quit the application.
			m.resolvePrompt(app.PromptResponse{Cancelled: true})
			return m, tea.Quit
		}
		result, cmd := m.prompt.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if result != nil {
			if result.cancelled {
				m.resolvePrompt(app.PromptResponse{Cancelled: true})
			} else {
				m.resolvePrompt(app.PromptResponse{
					Value:     result.value,
					Index:     result.index,
					Confirmed: result.confirmed,
				})
			}
		}

	case app.PromptRequestEvent:
		// Already handling a prompt — reject concurrent requests.
		if msg.ResponseCh != nil {
			msg.ResponseCh <- app.PromptResponse{Cancelled: true}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		_, cmd := m.prompt.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	default:
		// Pass blink ticks and other messages to the prompt overlay.
		_, cmd := m.prompt.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// resolvePrompt sends the response through the channel, clears prompt state,
// and restores the previous app state.
func (m *AppModel) resolvePrompt(resp app.PromptResponse) {
	if m.promptResponseCh != nil {
		m.promptResponseCh <- resp
		m.promptResponseCh = nil
	}
	m.prompt = nil
	m.state = m.prePromptState
}

// --------------------------------------------------------------------------
// Overlay dialog support
// --------------------------------------------------------------------------

// updateOverlayState handles all messages while the overlay dialog is active.
// It routes keys to the overlay, detects completion/cancellation, and restores
// the previous state when done.
func (m *AppModel) updateOverlayState(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			// Cancel overlay and quit the application.
			m.resolveOverlay(app.OverlayResponse{Cancelled: true})
			return m, tea.Quit
		}
		result, cmd := m.overlay.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if result != nil {
			if result.cancelled {
				m.resolveOverlay(app.OverlayResponse{Cancelled: true})
			} else {
				m.resolveOverlay(app.OverlayResponse{
					Action: result.action,
					Index:  result.index,
				})
			}
		}

	case app.OverlayRequestEvent:
		// Already handling an overlay — reject concurrent requests.
		if msg.ResponseCh != nil {
			msg.ResponseCh <- app.OverlayResponse{Cancelled: true}
		}

	case app.PromptRequestEvent:
		// Can't show a prompt while an overlay is active — reject.
		if msg.ResponseCh != nil {
			msg.ResponseCh <- app.PromptResponse{Cancelled: true}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		_, cmd := m.overlay.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// resolveOverlay sends the response through the channel, clears overlay state,
// and restores the previous app state.
func (m *AppModel) resolveOverlay(resp app.OverlayResponse) {
	if m.overlayResponseCh != nil {
		m.overlayResponseCh <- resp
		m.overlayResponseCh = nil
	}
	m.overlay = nil
	m.state = m.preOverlayState
}

// --------------------------------------------------------------------------
// Shell command execution (! and !!)
// --------------------------------------------------------------------------

// shellCommandTimeout is the maximum duration for a user shell command.
const shellCommandTimeout = 120 * time.Second

// executeShellCommand runs a shell command asynchronously and returns the
// result as a shellCommandResultMsg. This is launched from Update() as a
// tea.Cmd so the TUI stays responsive during execution.
func (m *AppModel) executeShellCommand(msg shellCommandMsg) tea.Cmd {
	command := msg.Command
	excludeFromContext := msg.ExcludeFromContext
	cwd := m.cwd

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), shellCommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		if cwd != "" {
			cmd.Dir = cwd
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				// Non-zero exit is reported via exitCode, not as an error.
				err = nil
			} else if ctx.Err() == context.DeadlineExceeded {
				return shellCommandResultMsg{
					Command:            command,
					Output:             fmt.Sprintf("command timed out after %v", shellCommandTimeout),
					ExitCode:           -1,
					Err:                fmt.Errorf("command timed out after %v", shellCommandTimeout),
					ExcludeFromContext: excludeFromContext,
				}
			}
		}

		// Combine stdout + stderr.
		var combined strings.Builder
		if stdout.Len() > 0 {
			combined.WriteString(stdout.String())
		}
		if stderr.Len() > 0 {
			if combined.Len() > 0 {
				combined.WriteString("\n")
			}
			combined.WriteString(stderr.String())
		}

		return shellCommandResultMsg{
			Command:            command,
			Output:             combined.String(),
			ExitCode:           exitCode,
			Err:                err,
			ExcludeFromContext: excludeFromContext,
		}
	}
}

// handleShellCommandResult processes the result of a shell command execution.
// It prints the output to scrollback and optionally injects it into the
// conversation context (for ! commands) so the LLM can see it.
func (m *AppModel) handleShellCommandResult(msg shellCommandResultMsg) tea.Cmd {
	theme := GetTheme()

	// Build the display header.
	var header string
	if msg.ExcludeFromContext {
		header = fmt.Sprintf("$ %s  (excluded from context)", msg.Command)
	} else {
		header = fmt.Sprintf("$ %s", msg.Command)
	}

	// Build the output content.
	var content strings.Builder
	content.WriteString(header)

	// Display-level truncation: show first maxShellDisplayLines lines with a
	// "...(N more lines)" hint, matching the tool result renderer behavior.
	const maxShellDisplayLines = 20

	displayOutput := msg.Output
	var displayHiddenCount int
	if displayOutput != "" {
		lines := strings.Split(displayOutput, "\n")
		if len(lines) > maxShellDisplayLines {
			displayHiddenCount = len(lines) - maxShellDisplayLines
			displayOutput = strings.Join(lines[:maxShellDisplayLines], "\n")
		}
	}

	if msg.Err != nil {
		fmt.Fprintf(&content, "\n\nError: %v", msg.Err)
	} else if displayOutput != "" {
		content.WriteString("\n\n")
		content.WriteString(displayOutput)
		if displayHiddenCount > 0 {
			fmt.Fprintf(&content, "\n\n...(%d more lines)", displayHiddenCount)
		}
	} else {
		content.WriteString("\n\n(no output)")
	}

	if msg.ExitCode != 0 {
		fmt.Fprintf(&content, "\n\nExit code: %d", msg.ExitCode)
	}

	// Choose border color: dim for excluded, accent for included.
	borderClr := theme.Accent
	if msg.ExcludeFromContext {
		borderClr = theme.Muted
	}

	rendered := renderContentBlock(
		content.String(),
		m.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(borderClr),
		WithMarginBottom(1),
	)

	m.appendScrollback(rendered)

	// For ! (included in context): inject the command output into the
	// conversation as a user message so the LLM can reference it on the
	// next turn. This does NOT trigger an LLM response — it only adds
	// to the conversation history.
	if !msg.ExcludeFromContext && m.appCtrl != nil {
		// Truncate context output with the same limits as display.
		contextOutput := msg.Output
		if contextOutput != "" {
			tr := core.TruncateTail(contextOutput, core.DefaultMaxLines, core.DefaultMaxBytes)
			contextOutput = tr.Content
		} else {
			contextOutput = "(no output)"
		}
		contextMsg := fmt.Sprintf("<shell_command>\n<command>%s</command>\n<output>\n%s</output>\n<exit_code>%d</exit_code>\n</shell_command>",
			msg.Command, contextOutput, msg.ExitCode)
		m.appCtrl.AddContextMessage(contextMsg)
	}

	return nil
}
