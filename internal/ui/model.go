package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/core"
	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/prompts"
	"github.com/mark3labs/kit/internal/session"
	kit "github.com/mark3labs/kit/pkg/kit"
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

	// stateSessionSelector means the /resume session picker is active.
	stateSessionSelector
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
	// ReloadMessagesFromTree clears the in-memory message store and reloads
	// it from the tree session's current branch. Unlike ClearMessages, this
	// does NOT reset the tree session's leaf pointer. Used after Branch() to
	// sync the store with the new branch position.
	ReloadMessagesFromTree()
	// CompactConversation summarises older messages to free context space.
	// Runs asynchronously; results are delivered via CompactCompleteEvent or
	// CompactErrorEvent sent through the registered tea.Program. Returns an
	// error synchronously if compaction cannot be started (e.g. agent is busy).
	// customInstructions is optional text appended to the summary prompt.
	CompactConversation(customInstructions string) error
	// GetTreeSession returns the tree session manager, or nil if tree sessions
	// are not enabled. Used by slash commands like /tree, /fork, /session.
	GetTreeSession() *session.TreeManager
	// SwitchTreeSession replaces the active tree session with a new one,
	// closing the old session. Used by /new to create a completely fresh session.
	SwitchTreeSession(ts *session.TreeManager)
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
	RunWithFiles(prompt string, files []kit.LLMFilePart) int
	// Steer injects a steering message into the currently running agent
	// turn. If the agent is busy, the message is delivered between steps
	// (after current tool finishes, before next LLM call). If idle, the
	// message starts executing immediately. Returns 0 if started
	// immediately, >0 if injected/pending.
	Steer(prompt string) int
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

// noopCmd is a sentinel tea.Cmd returned by handlers that have consumed an
// event but produce no side-effects. It returns a nil Msg which BubbleTea
// discards, but its non-nil value lets callers distinguish "handled" from
// "not handled" (nil tea.Cmd).
var noopCmd tea.Cmd = func() tea.Msg { return nil }

// Package-level lipgloss styles that are invariant across frames (only depend
// on theme colors, which are updated via SetTheme). Defined at package level
// to avoid allocating new lipgloss.Style structs on every render call.
//
// Note: theme-sensitive styles (those using theme.Warning, theme.Muted, etc.)
// are rebuilt on theme change via ApplyTheme. The cancel warning style
// intentionally reads the theme at render time because themes can change at
// runtime; only truly static styles belong here.
var styleMarginBottom1 = lipgloss.NewStyle().MarginBottom(1)

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

	// PromptTemplates are user-defined prompt templates loaded from ~/.kit/prompts/,
	// .kit/prompts/, or explicit --prompt-template paths. They appear in autocomplete
	// and are expanded when submitted (e.g., /review → full prompt text).
	PromptTemplates []*prompts.PromptTemplate

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

	// SwitchSession opens a session by JSONL file path, replacing the
	// active tree session and reloading messages. Called when the user
	// picks a session from /resume. May be nil if session switching is
	// not supported.
	SwitchSession func(path string) error

	// ShowSessionPicker, when true, opens the session picker immediately
	// on startup (used by --resume flag).
	ShowSessionPicker bool

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

	// renderer renders completed messages for tea.Println output.
	renderer Renderer

	// modelName is the LLM model name shown in rendered messages.
	modelName string

	// queuedMessages stores the text of prompts that were queued (not yet
	// submitted to the agent). They are rendered with a "queued" badge above
	// the input and move to scrollback when the agent picks them up.
	queuedMessages []string

	// steeringMessages stores the text of prompts that were sent as steer
	// messages (injected mid-turn via Ctrl+S). Rendered with a "STEERING"
	// badge above the input. Cleared when the steer is consumed.
	steeringMessages []string

	// scrollList manages the in-memory message history with viewport scrolling.
	// Replaces the terminal scrollback (tea.Println) pattern with in-memory
	// scrollback for alt screen mode.
	scrollList *ScrollList

	// messages holds all completed messages in the conversation history.
	// The scrollList renders from this slice based on its viewport offset.
	messages []MessageItem

	// pendingUserPrints holds user messages that have been consumed from the
	// queue but not yet printed to scrollback. They are deferred until
	// SpinnerEvent{Show: true} so the previous assistant response can be
	// flushed first, preserving chronological order.
	// NOTE: With ScrollList, we add these directly to messages instead of printing.
	pendingUserPrints []string

	// scrollbackBuf is DEPRECATED in alt screen mode but kept for compatibility.
	// In alt screen mode, messages go directly to the scrollList.
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

	// promptTemplates are user-defined prompt templates for expansion.
	// They appear in autocomplete and are expanded when submitted.
	promptTemplates []*prompts.PromptTemplate

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

	// sessionSelector is the session picker overlay, active in stateSessionSelector.
	sessionSelector *SessionSelectorComponent

	// switchSession opens a session by JSONL path, replacing the active session.
	// Wired from cmd/root.go.
	switchSession func(path string) error

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

	// quitting signals that the app is shutting down. When true, View()
	// disables alt screen to restore the terminal properly.
	quitting bool

	// streamingBashOutput holds the current streaming bash output lines.
	// Lines are accumulated as they arrive and displayed in the stream region.
	streamingBashOutput []string
	// streamingBashStderr holds stderr lines separately (rendered differently).
	streamingBashStderr []string
	// streamingBashMaxLines caps how many lines to accumulate to prevent memory issues.
	streamingBashMaxLines int
	// streaming bash fields are only mutated/read from the Bubble Tea event loop
	// (Update/View), so no mutex is required here.
	// streamingBashCommand holds the command being executed for display as a header.
	streamingBashCommand string

	// ---------- Cached layout heights (invalidated by layoutDirty) ----------

	// layoutDirty marks that distributeHeight must recompute the stream height
	// on the next View() call. Set by any state change that affects sizing
	// (resize, queue changes, widget updates, visibility changes, etc.).
	// View() calls distributeHeight() when this is true and then clears it.
	layoutDirty bool
}

// --------------------------------------------------------------------------
// Child component interfaces
// --------------------------------------------------------------------------

// inputComponentIface is the interface the parent requires from InputComponent.
type inputComponentIface interface {
	tea.Model
}

// streamComponentIface is the interface the parent requires from StreamComponent.
type streamComponentIface interface {
	tea.Model
	// Reset clears accumulated state between agent steps.
	Reset()
	// SetHeight constrains the render output to at most h lines (0 = unconstrained).
	SetHeight(h int)
	// GetRenderedContent returns the rendered assistant message from accumulated
	// streaming text, or empty string if nothing has been accumulated.
	GetRenderedContent() string
	// ConsumeOverflow returns lines from the top of the rendered content that
	// have overflowed the allocated height and haven't been pushed to the
	// terminal scrollback yet. Returns "" when no new overflow exists.
	ConsumeOverflow() string
	// SpinnerView returns the rendered spinner line (animation + optional label).
	// Returns "" when the spinner is not active. The parent renders this in the
	// status bar so the spinner never changes the view height.
	SpinnerView() string
	// SetThinkingVisible sets whether reasoning blocks are shown or collapsed.
	SetThinkingVisible(visible bool)
	// HasReasoning returns true if any reasoning content has been accumulated.
	HasReasoning() bool
	// UpdateTheme refreshes typography with colors from the current theme.
	UpdateTheme()
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

	mr := newMessageRenderer(width, false)
	mr.getToolRenderer = opts.GetToolRenderer
	rdr := mr

	m := &AppModel{
		state:          stateInput,
		appCtrl:        appCtrl,
		renderer:       rdr,
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
	m.promptTemplates = opts.PromptTemplates
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
	m.switchSession = opts.SwitchSession

	// Store context/skills metadata and tool counts for startup display.
	m.contextPaths = opts.ContextPaths
	m.skillItems = opts.SkillItems
	m.mcpToolCount = opts.MCPToolCount
	m.extensionToolCount = opts.ExtensionToolCount

	// Initialize streaming bash output buffer.
	m.streamingBashMaxLines = 50 // cap to prevent memory issues

	// Initialize ScrollList for in-memory message history (alt screen mode).
	// Height will be set properly by distributeHeight().
	m.scrollList = NewScrollList(width, height-10) // Placeholder height
	m.messages = []MessageItem{}

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

	// Merge prompt templates into the InputComponent's autocomplete source.
	if ic, ok := m.input.(*InputComponent); ok && len(opts.PromptTemplates) > 0 {
		for _, tpl := range opts.PromptTemplates {
			ic.commands = append(ic.commands, SlashCommand{
				Name:        "/" + tpl.Name,
				Description: tpl.Description,
				Category:    "Prompts",
			})
		}
	}

	m.stream = NewStreamComponent(width, opts.ModelName)
	m.stream.SetThinkingVisible(m.thinkingVisible)

	// If --resume was passed, open the session picker immediately.
	if opts.ShowSessionPicker {
		m.sessionSelector = NewSessionSelector(opts.Cwd, width, height)
		m.state = stateSessionSelector
	}

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
	// m.input is always set by NewAppModel; its Init starts the textarea cursor blink.
	// m.stream.Init() always returns nil, so there is nothing to batch.
	return m.input.Init()
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

	// Create typography instance for startup rendering
	ty := createTypography(GetTheme())

	fmt.Println()

	// Build key-value pairs for startup info
	var pairs [][2]string

	if m.providerName != "" && m.modelName != "" {
		pairs = append(pairs, [2]string{"Model", fmt.Sprintf("%s (%s)", m.providerName, m.modelName)})
	}

	if m.loadingMessage != "" {
		pairs = append(pairs, [2]string{"Status", m.loadingMessage})
	}

	// Context — loaded AGENTS.md files.
	if len(m.contextPaths) > 0 {
		contextStr := tildeHome(m.contextPaths[0])
		if len(m.contextPaths) > 1 {
			contextStr += fmt.Sprintf(" +%d more", len(m.contextPaths)-1)
		}
		pairs = append(pairs, [2]string{"Context", contextStr})
	}

	// Skills — listed by name.
	if len(m.skillItems) > 0 {
		names := make([]string, len(m.skillItems))
		for i, si := range m.skillItems {
			names[i] = si.Name
		}
		pairs = append(pairs, [2]string{"Skills", strings.Join(names, ", ")})
	}

	// Extension tool count (only shown when > 0).
	if m.extensionToolCount > 0 {
		pairs = append(pairs, [2]string{"Extensions", fmt.Sprintf("%d tools", m.extensionToolCount)})
	}

	// MCP tool count (only shown when > 0).
	if m.mcpToolCount > 0 {
		pairs = append(pairs, [2]string{"MCP", fmt.Sprintf("%d tools", m.mcpToolCount)})
	}

	if len(pairs) > 0 {
		rendered := ty.KVGroup(pairs)
		rendered = styleMarginBottom1.Render(rendered)
		fmt.Println(rendered)
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
				return m, noopCmd
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
				// Persist model selection for next launch.
				go func() { _ = SaveModelPreference(msg.ModelString) }()
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

	// ── Session selector events ──────────────────────────────────────────────
	case SessionSelectedMsg:
		m.sessionSelector = nil
		m.state = stateInput
		if m.switchSession != nil {
			if err := m.switchSession(msg.Path); err != nil {
				m.printSystemMessage(fmt.Sprintf("Failed to switch session: %v", err))
			} else {
				m.renderSessionHistory()
				m.printSystemMessage("Session loaded. Continue where you left off.")
			}
		} else {
			m.printSystemMessage("Session switching not available.")
		}
		cmds = append(cmds, m.drainScrollback())
		return m, tea.Batch(cmds...)

	case SessionSelectorCancelledMsg:
		m.sessionSelector = nil
		m.state = stateInput
		return m, nil

	case SessionDeletedMsg:
		// Session was deleted from picker — just show a message.
		m.printSystemMessage(fmt.Sprintf("Deleted session: %s", msg.Name))
		cmds = append(cmds, m.drainScrollback())
		return m, tea.Batch(cmds...)

	// ── Window resize ────────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layoutDirty = true
		// Update renderer width for proper message styling
		m.renderer.SetWidth(m.width)
		// Propagate to children.
		if m.input != nil {
			updated, cmd := m.input.Update(msg)
			m.input, _ = updated.(inputComponentIface)
			cmds = append(cmds, cmd)
		}
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

	// ── Mouse wheel scrolling ────────────────────────────────────────────────
	case tea.MouseWheelMsg:
		// Scroll the scrollback viewport with mouse wheel
		const scrollLines = 3
		switch msg.Button {
		case tea.MouseWheelUp:
			m.scrollList.ScrollBy(-scrollLines)
			m.scrollList.autoScroll = false
		case tea.MouseWheelDown:
			m.scrollList.ScrollBy(scrollLines)
			if m.scrollList.AtBottom() {
				m.scrollList.autoScroll = true
			}
		}

	// ── Mouse click selection ─────────────────────────────────────────────────
	// DISABLED: Selection/copy functionality is disabled for now but plumbing remains
	// case tea.MouseClickMsg:
	// 	// Handle mouse clicks in the scrollback area for item selection (crush-style)
	// 	// Only process left clicks in input state
	// 	if m.state == stateInput && msg.Button == tea.MouseLeft {
	// 		// Enable selection on the scrollList
	// 		m.scrollList.SetSelectable(true)
	// 		// Handle mouse down for selection tracking
	// 		if m.scrollList.HandleMouseDown(msg.X, msg.Y) {
	// 			// Disable auto-scroll so user can read
	// 			m.scrollList.autoScroll = false
	// 		}
	// 	}

	// ── Mouse motion/drag for selection ──────────────────────────────────────
	// DISABLED: Selection/copy functionality is disabled for now but plumbing remains
	// case tea.MouseMotionMsg:
	// 	// Handle mouse motion for text selection (crush-style)
	// 	// MouseMotionMsg is sent when mouse moves while button is held
	// 	if m.state == stateInput {
	// 		m.scrollList.HandleMouseDrag(msg.X, msg.Y)
	// 	}

	// ── Mouse release for copy ───────────────────────────────────────────────
	// DISABLED: Selection/copy functionality is disabled for now but plumbing remains
	// case tea.MouseReleaseMsg:
	// 	// Handle mouse release to finalize selection and copy (crush-style)
	// 	if m.state == stateInput {
	// 		if m.scrollList.HandleMouseUp(msg.X, msg.Y) {
	// 			// Selection was made - copy to clipboard
	// 			if m.scrollList.HasSelection() {
	// 				// Get selected content and copy
	// 				// For now, copy a placeholder - full implementation would extract text
	// 				cmd := CopyToClipboardWithMessage("Selected text", "Selection copied to clipboard")
	// 				cmds = append(cmds, cmd)
	// 			}
	// 		}
	// 	}

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
			// Set quitting flag so View() disables alt screen for clean exit.
			m.quitting = true
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

		// Scrollback keybindings (PgUp/PgDn/Home/End) for navigating message history.
		// Only active when not working (to avoid conflicts during streaming).
		if m.state == stateInput {
			switch msg.String() {
			// DISABLED: Copy shortcuts disabled for now but plumbing remains
			// case "c", "y":
			// 	// Copy current focused message or selection to clipboard (crush-style)
			// 	if m.scrollList.HasSelection() {
			// 		// Copy selection
			// 		cmd := CopyToClipboardWithMessage("Selected text", "Selection copied to clipboard")
			// 		cmds = append(cmds, cmd)
			// 	} else if m.scrollList.FocusedIdx() >= 0 {
			// 		// Copy focused message content
			// 		idx := m.scrollList.FocusedIdx()
			// 		if idx < len(m.messages) {
			// 			// Get the message content - would need to extract raw text
			// 			// For now, use a placeholder
			// 			cmd := CopyToClipboardWithMessage("Message content", "Message copied to clipboard")
			// 			cmds = append(cmds, cmd)
			// 		}
			// 	}
			// 	return m, tea.Batch(cmds...)
			case "pgup":
				m.scrollList.ScrollBy(-m.scrollList.height)
				m.scrollList.autoScroll = false
				return m, tea.Batch(cmds...)
			case "pgdown":
				m.scrollList.ScrollBy(m.scrollList.height)
				if m.scrollList.AtBottom() {
					m.scrollList.autoScroll = true
				}
				return m, tea.Batch(cmds...)
			case "alt+home":
				m.scrollList.GotoTop()
				m.scrollList.autoScroll = false
				return m, tea.Batch(cmds...)
			case "alt+end":
				m.scrollList.GotoBottom()
				m.scrollList.autoScroll = true
				return m, tea.Batch(cmds...)
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

		// Route to session selector when active.
		if m.state == stateSessionSelector && m.sessionSelector != nil {
			updated, cmd := m.sessionSelector.Update(msg)
			m.sessionSelector = updated.(*SessionSelectorComponent)
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

		case "ctrl+s":
			// Steer: inject the current input as a steering message into the
			// running agent turn. Only active during stateWorking — in input
			// state, Ctrl+S is passed through to children (no-op by default).
			if m.state == stateWorking && m.appCtrl != nil {
				var text string
				if ic, ok := m.input.(*InputComponent); ok {
					text = strings.TrimSpace(ic.textarea.Value())
				}
				if text != "" {
					// Clear the input and push to history.
					if ic, ok := m.input.(*InputComponent); ok {
						ic.pushHistory(text)
						ic.textarea.SetValue("")
					}

					// Preprocess @file references.
					processedText := text
					if m.cwd != "" {
						processedText = ProcessFileAttachments(text, m.cwd)
					}

					// Inject the steer message.
					sLen := m.appCtrl.Steer(processedText)
					if sLen > 0 {
						m.steeringMessages = append(m.steeringMessages, text)
						m.layoutDirty = true
					} else {
						// Started immediately (agent was idle).
						m.pendingUserPrints = append(m.pendingUserPrints, text)
						m.flushStreamAndPendingUserMessages()
						if m.state != stateWorking {
							m.state = stateWorking
						}
					}
				}
				return m, tea.Batch(cmds...)
			}
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
		// Parse once: split on the first space so argument-bearing commands
		// (e.g. "/model anthropic/foo", "/compact Focus on X") are matched by
		// their name and their args are passed through to the handler.
		if strings.HasPrefix(msg.Text, "/") {
			name, args, _ := strings.Cut(msg.Text, " ")
			if sc := GetCommandByName(name); sc != nil {
				if cmd := m.handleSlashCommand(sc, strings.TrimSpace(args)); cmd != nil {
					cmds = append(cmds, cmd)
				}
				cmds = append(cmds, m.drainScrollback())
				return m, tea.Batch(cmds...)
			}
		}

		// Check extension-registered slash commands. These support arguments
		// (e.g. "/sub list files"), so we split on the first space.
		if cmd := m.handleExtensionCommand(msg.Text); cmd != nil {
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		// Expand prompt templates. If the input matches a template name,
		// substitute arguments and use the expanded content as the prompt.
		if expanded, ok := m.expandPromptTemplate(msg.Text); ok {
			msg.Text = expanded
		}

		// Regular prompt — forward to the app layer.
		// Preprocess @file references: expand them into XML-wrapped file
		// content before sending to the agent. The display text (shown in
		// scrollback) uses the original user text so the UI stays clean.
		processedText := msg.Text
		if m.cwd != "" {
			processedText = ProcessFileAttachments(msg.Text, m.cwd)
		}

		// Convert image attachments to kit.LLMFilePart for the app layer.
		var fileParts []kit.LLMFilePart
		for _, img := range msg.Images {
			fileParts = append(fileParts, kit.LLMFilePart{
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
				m.layoutDirty = true
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
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: true})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
		// Execute the shell command asynchronously so the TUI stays responsive.
		cmds = append(cmds, m.executeShellCommand(msg))

	case shellCommandResultMsg:
		// Stop spinner now that the command has finished.
		if m.stream != nil {
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			m.stream, _ = updated.(streamComponentIface)
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
			m.layoutDirty = true
		}
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

	case app.ReasoningChunkEvent:
		// Forward to stream component for display rendering
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

		// Also update/create StreamingMessageItem in ScrollList for live display
		m.appendStreamingChunk("reasoning", msg.Delta)

	case app.StreamChunkEvent:
		// Forward to stream component for display rendering
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

		// Also update/create StreamingMessageItem in ScrollList for live display
		m.appendStreamingChunk("assistant", msg.Content)

	case app.ToolCallStartedEvent:
		// Flush any accumulated streaming text to scrollback first (streaming
		// always completes before tool calls fire). The tool call itself is
		// NOT printed here — a unified block (header + result) will be
		// rendered when the ToolResultEvent arrives.
		m.flushStreamContent()

		// For bash commands, extract and store the command for the streaming output header.
		if msg.ToolName == "bash" {
			var args struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(msg.ToolArgs), &args); err == nil && args.Command != "" {
				m.streamingBashCommand = args.Command
			}
		}

	case app.ToolExecutionEvent:
		// Pass to stream component for execution spinner display.
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

	case app.ToolResultEvent:
		// Remove streaming bash output item (if present) before adding the final tool result.
		// The tool result will contain the truncated output.
		if len(m.messages) > 0 {
			if _, ok := m.messages[len(m.messages)-1].(*StreamingBashOutputItem); ok {
				// Remove the streaming bash item
				m.messages = m.messages[:len(m.messages)-1]
			}
		}

		// Add the final tool result with truncated output.
		m.printToolResult(msg)

		// Clear legacy bash output state
		m.streamingBashOutput = nil
		m.streamingBashStderr = nil
		m.streamingBashCommand = ""

		// Start spinner again while waiting for the next LLM response.
		if m.stream != nil {
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: true})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

	case app.ToolOutputEvent:
		// Append bash output to streaming bash item in ScrollList.
		// Find or create the streaming bash output item.
		var bashItem *StreamingBashOutputItem
		if len(m.messages) > 0 {
			if item, ok := m.messages[len(m.messages)-1].(*StreamingBashOutputItem); ok {
				bashItem = item
			}
		}

		// Create new bash output item if needed
		if bashItem == nil {
			id := fmt.Sprintf("bash-%d", len(m.messages))
			bashItem = NewStreamingBashOutputItem(id, m.streamingBashCommand)
			m.messages = append(m.messages, bashItem)
		}

		// Append the chunk
		if msg.IsStderr {
			bashItem.AppendStderr(msg.Chunk)
		} else {
			bashItem.AppendStdout(msg.Chunk)
		}

		// Check height and cap if needed - we don't want streaming output to grow forever
		const maxStreamingBashHeight = 20 // Max lines to show during streaming
		if bashItem.Height() > maxStreamingBashHeight {
			// Stop showing new output once we hit the limit
			// The final tool result will show truncated output
			return m, nil
		}

		// Refresh ScrollList
		m.refreshContent()

		// Auto-scroll to bottom
		if m.scrollList != nil && m.scrollList.autoScroll {
			m.scrollList.GotoBottom()
		}

	case app.ToolCallContentEvent:
		// In streaming mode this text was already delivered via StreamChunkEvents
		// and will be flushed before the next tool call. Ignore to avoid
		// double-printing.

	case app.ResponseCompleteEvent:
		// This event fires for both streaming and non-streaming paths.
		// In streaming mode, mark the StreamingMessageItem as complete.
		// In non-streaming mode (no stream content accumulated), print the text.

		// Check if we have an active StreamingMessageItem
		hasStreamingItem := false
		if len(m.messages) > 0 {
			if streamMsg, ok := m.messages[len(m.messages)-1].(*StreamingMessageItem); ok {
				streamMsg.MarkComplete()
				hasStreamingItem = true
			}
		}

		// Reset stream component
		if m.stream != nil {
			m.stream.Reset()
		}

		// If no streaming item exists and we have content, print it as a regular message
		if !hasStreamingItem && strings.TrimSpace(msg.Content) != "" {
			m.printAssistantMessage(msg.Content)
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
		m.layoutDirty = true

	case app.SteerConsumedEvent:
		// Steering messages were consumed — either injected mid-turn via
		// PrepareStep, or drained into the queue after a text-only turn.
		//
		// Two cases:
		//
		//  1. Mid-turn (stateWorking, PrepareStep fired): no SpinnerEvent{Show:
		//     true} will follow within this turn, so we cannot rely on
		//     flushStreamAndPendingUserMessages() being called. Flush any live
		//     stream content first (assistant text up to the steer point), then
		//     render the steering user messages immediately to scrollback.
		//
		//  2. Post-turn (text-only response, drained after StepComplete): a
		//     SpinnerEvent{Show: true} for the next turn is already in flight.
		//     Defer to pendingUserPrints so the previous assistant response is
		//     flushed first, preserving chronological order.
		if m.state == stateWorking {
			// Case 1: mid-turn — flush + print immediately.
			m.flushStreamContent()
			for _, text := range m.steeringMessages {
				m.printUserMessage(text)
			}
			m.steeringMessages = m.steeringMessages[:0]
			m.layoutDirty = true
			cmds = append(cmds, m.drainScrollback())
		} else {
			// Case 2: post-turn — defer so SpinnerEvent orders correctly.
			m.pendingUserPrints = append(m.pendingUserPrints, m.steeringMessages...)
			m.steeringMessages = m.steeringMessages[:0]
			m.layoutDirty = true
		}

	case app.StepCompleteEvent:
		// Keep stream content visible in the view — don't flush to scrollback
		// yet. Flushing + resetting in the same frame would shrink the view
		// height, and bubbletea's inline renderer leaves blank lines at the
		// bottom for the orphaned rows. The content will be flushed to
		// scrollback when the next step starts (SpinnerEvent{Show: true}).
		// Just stop the spinner and return to input state.
		if m.stream != nil {
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
		m.state = stateInput
		m.canceling = false

	case app.StepCancelledEvent:
		// User cancelled the step (double-ESC). Keep partial stream content
		// visible (same reasoning as StepCompleteEvent). Just stop the spinner.
		if m.stream != nil {
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
		m.state = stateInput
		m.canceling = false

	case app.StepErrorEvent:
		// Keep partial stream content visible (same reasoning as
		// StepCompleteEvent). Print the error to scrollback — it appears
		// above the view, and the partial response stays visible below.
		if m.stream != nil {
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
		if msg.Err != nil {
			m.printErrorResponse(msg)
		}
		m.state = stateInput
		m.canceling = false

	case app.CompactCompleteEvent:
		// Finalize any streaming compaction content.
		if m.stream != nil {
			m.stream.Reset()
		}
		m.state = stateInput

		// Mark the last streaming message as complete in ScrollList.
		if len(m.messages) > 0 {
			if streamMsg, ok := m.messages[len(m.messages)-1].(*StreamingMessageItem); ok {
				streamMsg.MarkComplete()
			}
		}

		// Refresh content to show the finalized message.
		m.refreshContent()

		// Print stats as a separate system message.
		saved := msg.OriginalTokens - msg.CompactedTokens
		statsMsg := fmt.Sprintf(
			"Compaction complete: %d messages summarised, ~%dk tokens freed (%dk -> %dk)",
			msg.MessagesRemoved, saved/1000, msg.OriginalTokens/1000, msg.CompactedTokens/1000,
		)
		m.printSystemMessage(statsMsg)

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
		m.layoutDirty = true

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

	case shareResultMsg:
		if msg.err != nil {
			m.printSystemMessage(fmt.Sprintf("Share failed: %v", msg.err))
		} else {
			m.printSystemMessage(fmt.Sprintf("Session shared!\n\n  Viewer: %s\n  Gist:   %s", msg.viewerURL, msg.gistURL))
		}
		return m, m.drainScrollback()

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
			updated, cmd := m.input.Update(msg)
			m.input, _ = updated.(inputComponentIface)
			cmds = append(cmds, cmd)
		}
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
	}

	// Flush any stream overflow lines that have grown past the allocated
	// height into the terminal's real scrollback buffer. This ensures the
	// diagram's invariant: streaming text starts at the top of the viewable
	// terminal and overflows upward into the scrollback buffer rather than
	// silently discarding the older lines.
	//
	// IMPORTANT: overflow is emitted directly via tea.Println rather than
	// via appendScrollback. Using appendScrollback would cause drainScrollback
	// to see a non-empty scrollbackBuf and trigger its auto-flush, which calls
	// GetRenderedContent() + Reset() while the stream is still active —
	// causing duplication and premature resets.
	//
	// NOTE: In alt screen mode, overflow is handled differently - we don't use
	// tea.Println() since that writes to terminal scrollback, not alt screen.
	// The StreamingMessageItem dynamically renders the current stream content.
	// Overflow is not emitted - the full stream content is always rendered
	// via StreamingMessageItem in the ScrollList viewport.
	if m.stream != nil {
		// Consume and discard overflow in alt screen mode
		_ = m.stream.ConsumeOverflow()
	}

	cmds = append(cmds, m.drainScrollback())
	return m, tea.Batch(cmds...)
}

// View implements tea.Model. It renders the stacked layout:
// stream region + separator + [queued messages] + input region + status bar.
// The status bar is always present (1 line) to avoid layout shifts.
// When the tree selector is active, it replaces the stream region.
func (m *AppModel) View() tea.View {
	// When quitting, disable alt screen for clean terminal restoration.
	// This prevents terminal corruption issues on exit.
	if m.quitting {
		v := tea.NewView("")
		v.AltScreen = false
		v.MouseMode = tea.MouseModeNone
		return v
	}

	// Tree selector overlay replaces the normal layout.
	if m.state == stateTreeSelector && m.treeSelector != nil {
		return m.treeSelector.View()
	}

	// Model selector overlay replaces the normal layout.
	if m.state == stateModelSelector && m.modelSelector != nil {
		return m.modelSelector.View()
	}

	// Session selector overlay replaces the normal layout.
	if m.state == stateSessionSelector && m.sessionSelector != nil {
		return m.sessionSelector.View()
	}

	// Overlay dialog replaces the normal layout.
	if m.state == stateOverlay && m.overlay != nil {
		v := tea.NewView(m.overlay.Render())
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		v.ReportFocus = true
		v.KeyboardEnhancements = tea.KeyboardEnhancements{
			ReportEventTypes: true,
		}
		return v
	}

	// Recompute layout heights if any Update() changed state that affects
	// sizing. Deferring this to View() guarantees exactly one call per frame
	// regardless of how many events triggered a layout change in a single
	// Update() invocation.
	if m.layoutDirty {
		m.distributeHeight()
		m.layoutDirty = false
	}

	vis := m.uiVis()

	// Render scrollback content from ScrollList (replaces renderStream() in alt screen mode)
	scrollbackView := m.renderScrollback()

	// Propagate hint visibility to the input component before rendering.
	if ic, ok := m.input.(*InputComponent); ok {
		ic.hideHint = vis.HideInputHint
		ic.agentBusy = m.state == stateWorking
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

	// Only include the scrollback region when it has content. When idle the
	// scrollback renders "" which JoinVertical would pad to a full-width blank
	// line, inflating the view unnecessarily.
	if scrollbackView != "" {
		parts = append(parts, scrollbackView)
	}

	// Add canceling warning between scrollback and separator
	// (doesn't go inside scrollback viewport to avoid affecting scroll position)
	theme := GetTheme()
	if m.canceling {
		warning := lipgloss.NewStyle().
			Foreground(theme.Warning).
			Bold(true).
			Render("  ⚠ Press ESC again to cancel")
		parts = append(parts, warning)
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

	// Render slash command popup as centered overlay if active
	finalContent := content
	if ic, ok := m.input.(*InputComponent); ok {
		if popupContent := ic.RenderPopupCentered(m.width, m.height); popupContent != "" {
			// Overlay popup content on top of main content
			finalContent = overlayContent(content, popupContent, m.width, m.height)
		}
	}

	v := tea.NewView(finalContent)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.ReportFocus = true
	v.KeyboardEnhancements = tea.KeyboardEnhancements{
		ReportEventTypes: true,
	}
	return v
}

// --------------------------------------------------------------------------
// Rendering helpers
// --------------------------------------------------------------------------

// overlayContent overlays popup content on top of base content line-by-line.
// Both content strings should be full-screen (width x height).
func overlayContent(base, overlay string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure we have exactly height lines
	for len(baseLines) < height {
		baseLines = append(baseLines, strings.Repeat(" ", width))
	}
	for len(overlayLines) < height {
		overlayLines = append(overlayLines, strings.Repeat(" ", width))
	}

	// Merge lines - overlay takes precedence where non-empty
	result := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(overlayLines) && strings.TrimSpace(overlayLines[i]) != "" {
			result[i] = overlayLines[i]
		} else if i < len(baseLines) {
			result[i] = baseLines[i]
		} else {
			result[i] = strings.Repeat(" ", width)
		}
	}

	return strings.Join(result, "\n")
}

// renderStream returns the stream region content.
func (m *AppModel) renderStream() string {
	theme := GetTheme()

	var parts []string

	// Stream component content (LLM streaming text, reasoning, spinner placeholder).
	if m.stream != nil {
		if content := m.stream.View().Content; content != "" {
			parts = append(parts, content)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	// Show canceling warning if set.
	if m.canceling {
		warning := lipgloss.NewStyle().
			Foreground(theme.Warning).
			Bold(true).
			Render("  ⚠ Press ESC again to cancel")
		parts = append(parts, warning)
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// refreshContent updates the ScrollList with current messages.
// Called whenever messages change (new message, streaming update, etc.)
// ScrollList lazily renders only visible items on View() call.
func (m *AppModel) refreshContent() {
	if m.scrollList == nil {
		return
	}

	// MessageItem implements ScrollItem interface, so we can use copy
	m.scrollList.SetItems(m.messages)

	// Only adjust scroll position if auto-scroll is enabled
	if m.scrollList.autoScroll {
		m.scrollList.GotoBottom()
	}
}

// renderScrollback returns the scrollback content from ScrollList.
// This replaces renderStream() in alt screen mode.
func (m *AppModel) renderScrollback() string {
	// Content is refreshed via refreshContent() when messages change
	// ScrollList renders lazily on View() call
	return m.scrollList.View()
}

// renderStreamingBashOutput renders accumulated streaming bash output (stdout + stderr)
// below the LLM streaming text. Returns empty string if no bash output is present.
// Lines are truncated to the terminal width and capped to maxBashLines to prevent
// long-running commands from blowing up the TUI layout.
func (m *AppModel) renderStreamingBashOutput(theme Theme) string {
	stdoutLines := make([]string, len(m.streamingBashOutput))
	copy(stdoutLines, m.streamingBashOutput)
	stderrLines := make([]string, len(m.streamingBashStderr))
	copy(stderrLines, m.streamingBashStderr)
	command := m.streamingBashCommand

	if len(stdoutLines) == 0 && len(stderrLines) == 0 {
		return ""
	}

	const lineIndent = "  "
	lineWidth := max(m.width-2-len(lineIndent), 20)
	// Account for PaddingLeft(1) on the output/stderr styles.
	maxLineChars := lineWidth - 1

	outputStyle := lipgloss.NewStyle().
		Background(theme.CodeBg).
		PaddingLeft(1)

	stderrStyle := lipgloss.NewStyle().
		Foreground(theme.Error).
		Background(theme.CodeBg).
		PaddingLeft(1)

	// Header style for the command - muted text with a subtle indicator.
	headerStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		PaddingLeft(1)

	// Cap displayed lines to maxBashLines (show the tail, since streaming
	// output is most useful at the end). The buffer itself is larger to
	// preserve context, but we only render the last N lines.
	totalLines := len(stdoutLines) + len(stderrLines)
	var hiddenCount int
	if totalLines > maxBashLines {
		hiddenCount = totalLines - maxBashLines
		// Trim from stdout first (older output), then stderr.
		remaining := maxBashLines
		if len(stderrLines) >= remaining {
			stdoutLines = nil
			stderrLines = stderrLines[len(stderrLines)-remaining:]
		} else {
			remaining -= len(stderrLines)
			if len(stdoutLines) > remaining {
				stdoutLines = stdoutLines[len(stdoutLines)-remaining:]
			}
		}
	}

	var lines []string

	// Command header - show the bash command being executed.
	if command != "" {
		headerText := fmt.Sprintf("$ %s", command)
		headerContent := headerStyle.Width(lineWidth).Render(truncateLine(headerText, maxLineChars))
		lines = append(lines, lineIndent+headerContent)
	}

	// Truncation hint at the top.
	if hiddenCount > 0 {
		hint := fmt.Sprintf("...(%d more lines above)", hiddenCount)
		hintContent := outputStyle.Width(lineWidth).
			Foreground(theme.Muted).Italic(true).Render(hint)
		lines = append(lines, lineIndent+hintContent)
	}

	// Render stdout lines.
	for _, line := range stdoutLines {
		line = truncateLine(strings.TrimRight(line, "\n"), maxLineChars)
		styled := outputStyle.Width(lineWidth).Render(line)
		lines = append(lines, lineIndent+styled)
	}

	// Render stderr lines with error styling.
	for _, line := range stderrLines {
		line = truncateLine(strings.TrimRight(line, "\n"), maxLineChars)
		styled := stderrStyle.Width(lineWidth).Render(line)
		lines = append(lines, lineIndent+styled)
	}

	return strings.Join(lines, "\n")
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

	// Persist thinking level for next launch.
	go func() { _ = SaveThinkingLevelPreference(next) }()
}

// renderSeparator renders the separator line with an optional queue/steer count badge.
func (m *AppModel) renderSeparator() string {
	theme := GetTheme()
	lineStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	queueLen := len(m.queuedMessages)
	steerLen := len(m.steeringMessages)

	if steerLen > 0 || queueLen > 0 {
		var parts []string
		if steerLen > 0 {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(theme.Warning).
				Render(fmt.Sprintf("%d steering", steerLen)))
		}
		if queueLen > 0 {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(theme.Secondary).
				Render(fmt.Sprintf("%d queued", queueLen)))
		}
		badge := strings.Join(parts, " ")

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

// renderQueuedMessages renders queued and steering prompts as styled content
// blocks with badges, anchored between the separator and input. Steering
// messages use a distinct "STEERING" badge to differentiate from queued ones.
func (m *AppModel) renderQueuedMessages() string {
	if len(m.queuedMessages) == 0 && len(m.steeringMessages) == 0 {
		return ""
	}
	theme := GetTheme()

	var blocks []string

	// Render steering messages first (higher priority).
	if len(m.steeringMessages) > 0 {
		badge := CreateBadge("STEERING", theme.Warning)
		for _, msg := range m.steeringMessages {
			content := msg + "\n" + badge
			rendered := renderContentBlock(
				content,
				m.width,
				WithAlign(lipgloss.Left),
				WithBorderColor(theme.Warning),
			)
			blocks = append(blocks, rendered)
		}
	}

	// Render queued messages.
	if len(m.queuedMessages) > 0 {
		badge := CreateBadge("QUEUED", theme.Accent)
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
	}

	return strings.Join(blocks, "\n")
}

// --------------------------------------------------------------------------
// Print helpers — emit content to scrollback via tea.Println
// --------------------------------------------------------------------------

// printUserMessage renders a user message into the scrollback buffer.
func (m *AppModel) printUserMessage(text string) {
	// Check if this exact message was just added (prevents duplicates)
	if len(m.messages) > 0 {
		if lastMsg, ok := m.messages[len(m.messages)-1].(*TextMessageItem); ok {
			if lastMsg.role == "user" && lastMsg.content == text {
				return // Skip duplicate
			}
		}
	}

	// Render styled content using MessageRenderer
	styledMsg := m.renderer.RenderUserMessage(text, time.Now())

	// Add to in-memory scrollList with styled content
	msg := NewStyledMessageItem(generateMessageID(), "user", text, styledMsg.Content)
	m.messages = append(m.messages, msg)

	// Refresh ScrollList content and scroll to bottom
	m.refreshContent()

	// Also append to legacy buffer for compatibility
	m.appendScrollback(styledMsg.Content)
}

// printAssistantMessage renders an assistant message into the scrollback buffer.
func (m *AppModel) printAssistantMessage(text string) {
	if strings.TrimSpace(text) != "" {
		// Render styled content using MessageRenderer
		styledMsg := m.renderer.RenderAssistantMessage(text, time.Now(), m.modelName)

		// Add to in-memory scrollList with styled content
		msg := NewStyledMessageItem(generateMessageID(), "assistant", text, styledMsg.Content)
		m.messages = append(m.messages, msg)

		// Refresh ScrollList content and scroll to bottom
		m.refreshContent()

		// Also append to legacy buffer for compatibility
		m.appendScrollback(styledMsg.Content)
	}
}

// printToolResult renders a tool result message into the scrollback buffer.
func (m *AppModel) printToolResult(evt app.ToolResultEvent) {
	// Render styled tool message using MessageRenderer
	styledMsg := m.renderer.RenderToolMessage(evt.ToolName, evt.ToolArgs, evt.Result, evt.IsError)

	// Add to in-memory scrollList with styled content
	msg := NewStyledMessageItem(generateMessageID(), "tool", styledMsg.Content, styledMsg.Content)
	m.messages = append(m.messages, msg)

	// Refresh ScrollList content
	m.refreshContent()

	// Also append to legacy buffer for compatibility
	m.appendScrollback(styledMsg.Content)
}

// printErrorResponse renders an error message into the scrollback buffer.
func (m *AppModel) printErrorResponse(evt app.StepErrorEvent) {
	if evt.Err != nil {
		// Render styled error message using MessageRenderer
		styledMsg := m.renderer.RenderErrorMessage(evt.Err.Error(), time.Now())

		// Add to in-memory scrollList with styled content
		msg := NewStyledMessageItem(generateMessageID(), "error", styledMsg.Content, styledMsg.Content)
		m.messages = append(m.messages, msg)

		// Refresh ScrollList content
		m.refreshContent()

		// Also append to legacy buffer for compatibility
		m.appendScrollback(styledMsg.Content)
	}
}

// --------------------------------------------------------------------------
// Slash command handlers
// --------------------------------------------------------------------------

// handleSlashCommand executes a recognized slash command and returns a tea.Cmd.
// args contains any text after the command name (may be empty). Returns tea.Quit
// for /quit, nil for commands with no output, or a tea.Println cmd for display.
func (m *AppModel) handleSlashCommand(sc *SlashCommand, args string) tea.Cmd {
	switch sc.Name {
	case "/quit":
		m.quitting = true
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
		return m.handleModelCommand(args)
	case "/theme":
		return m.handleThemeCommand(args)
	case "/thinking":
		return m.handleThinkingCommand(args)
	case "/compact":
		return m.handleCompactCommand(args)
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
		m.steeringMessages = m.steeringMessages[:0]
		m.layoutDirty = true

	case "/tree":
		return m.handleTreeCommand()
	case "/fork":
		return m.handleForkCommand()
	case "/new":
		return m.handleNewCommand()
	case "/name":
		return m.handleNameCommand(args)
	case "/resume":
		return m.handleResumeCommand()
	case "/export":
		return m.handleExportCommand(args)
	case "/share":
		return m.handleShareCommand()
	case "/import":
		return m.handleImportCommand(args)
	case "/session":
		return m.handleSessionInfoCommand()

	default:
		m.printSystemMessage(fmt.Sprintf("Unknown command: %s", sc.Name))
	}
	return nil
}

// printSystemMessage renders a system-level message into the scrollback buffer.
func (m *AppModel) printSystemMessage(text string) {
	// Render styled system message using MessageRenderer
	styledMsg := m.renderer.RenderSystemMessage(text, time.Now())

	// Add to in-memory scrollList with styled content
	msg := NewStyledMessageItem(generateMessageID(), "system", styledMsg.Content, styledMsg.Content)
	m.messages = append(m.messages, msg)

	// Refresh ScrollList content
	m.refreshContent()

	// Also append to legacy buffer for compatibility
	m.appendScrollback(styledMsg.Content)
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

	// Add to in-memory scrollList with rendered content
	msg := NewStyledMessageItem(generateMessageID(), "extension", rendered, rendered)
	m.messages = append(m.messages, msg)

	// Refresh ScrollList content
	m.refreshContent()

	// Also append to legacy buffer for compatibility
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
	return noopCmd
}

// expandPromptTemplate checks if the submitted text matches a prompt template
// and returns the expanded content with arguments substituted.
// Returns (expanded, true) if a template was found and expanded, (text, false) otherwise.
func (m *AppModel) expandPromptTemplate(text string) (string, bool) {
	if len(m.promptTemplates) == 0 {
		return text, false
	}

	// Only consider inputs that look like slash commands.
	if !strings.HasPrefix(text, "/") {
		return text, false
	}

	// Split: "/templatename arg1 arg2" → name="/templatename", args="arg1 arg2"
	name, args, _ := strings.Cut(text, " ")
	name = strings.TrimPrefix(name, "/")

	// Find matching template
	for _, tpl := range m.promptTemplates {
		if tpl.Name == name {
			return tpl.Expand(args), true
		}
	}

	return text, false
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
		"- `/new`: Start a new session (discards context, saves old session)\n" +
		"- `/resume`: Open session picker to switch sessions\n" +
		"- `/name <name>`: Set a display name for this session\n\n" +
		"**System:**\n" +
		"- `/compact [instructions]`: Summarise older messages to free context space\n" +
		"- `/clear`: Clear message history\n" +
		"- `/export [path]`: Export session as JSONL\n" +
		"- `/import <path.jsonl>`: Import session from JSONL file\n" +
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
		"- `ESC` (x2): Cancel ongoing LLM generation\n" +
		"- `Ctrl+S`: Steer — redirect the agent mid-turn (injected between tool calls)\n" +
		"- `Enter` (while working): Queue message for after the agent finishes\n\n" +
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

	// Mark the existing StreamingMessageItem as complete instead of creating a new one.
	// The StreamingMessageItem already has the content from appendStreamingChunk().
	if len(m.messages) > 0 {
		if streamMsg, ok := m.messages[len(m.messages)-1].(*StreamingMessageItem); ok {
			streamMsg.MarkComplete()
			m.refreshContent()
			return
		}
	}

	// Fallback: If no StreamingMessageItem exists (shouldn't happen), create a new styled message.
	// This handles edge cases where flushStreamContent is called without streaming.
	styledMsg := m.renderer.RenderAssistantMessage(content, time.Now(), m.modelName)
	msg := NewStyledMessageItem(generateMessageID(), "assistant", content, styledMsg.Content)
	m.messages = append(m.messages, msg)
	m.refreshContent()

	// Also append to legacy buffer for compatibility
	m.appendScrollback(styledMsg.Content)
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

			// Render styled content using MessageRenderer
			styledMsg := m.renderer.RenderAssistantMessage(content, time.Now(), m.modelName)

			// Add to in-memory scrollList with styled content
			msg := NewStyledMessageItem(generateMessageID(), "assistant", content, styledMsg.Content)
			m.messages = append(m.messages, msg)

			// Also append to legacy buffer for compatibility
			m.appendScrollback(styledMsg.Content)
		}
	}

	// 2. Render pending user messages from the queue.
	for _, text := range m.pendingUserPrints {
		// Render styled content using MessageRenderer
		styledMsg := m.renderer.RenderUserMessage(text, time.Now())

		// Add to in-memory scrollList with styled content
		msg := NewStyledMessageItem(generateMessageID(), "user", text, styledMsg.Content)
		m.messages = append(m.messages, msg)

		// Also append to legacy buffer for compatibility
		m.appendScrollback(styledMsg.Content)
	}
	m.pendingUserPrints = nil

	// Refresh ScrollList content once after all messages are added
	m.refreshContent()
}

// appendStreamingChunk updates or creates a StreamingMessageItem in the ScrollList.
// This enables live streaming text display within the ScrollList viewport (iteratr-style).
func (m *AppModel) appendStreamingChunk(role, content string) {
	// Find the last message
	var lastMsg MessageItem
	if len(m.messages) > 0 {
		lastMsg = m.messages[len(m.messages)-1]
	}

	// If last message is a StreamingMessageItem with matching role, append to it
	if streamMsg, ok := lastMsg.(*StreamingMessageItem); ok && streamMsg.role == role {
		streamMsg.AppendChunk(content)
		// Auto-scroll to bottom if enabled
		if m.scrollList != nil && m.scrollList.autoScroll {
			m.scrollList.GotoBottom()
		}
		return
	}

	// Transition detected: mark previous reasoning message as complete when assistant text starts
	if streamMsg, ok := lastMsg.(*StreamingMessageItem); ok && streamMsg.role == "reasoning" && role == "assistant" {
		streamMsg.MarkComplete()
	}

	// Otherwise, create a new StreamingMessageItem
	id := fmt.Sprintf("streaming-%s-%d", role, len(m.messages))
	newMsg := NewStreamingMessageItem(id, role, m.modelName)
	newMsg.AppendChunk(content)
	m.messages = append(m.messages, newMsg)

	// Refresh ScrollList and scroll to bottom
	m.refreshContent()
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
//
// drainScrollback is a no-op in alt screen mode. Scrollback is managed
// in-memory by ScrollList and never printed via tea.Println().
// The scrollbackBuf is still populated for compatibility but cleared here
// to prevent memory leaks.
func (m *AppModel) drainScrollback() tea.Cmd {
	// In alt screen mode, all scrollback is managed in-memory by ScrollList.
	// Never use tea.Println() as it writes to terminal scrollback, not alt screen.
	m.scrollbackBuf = m.scrollbackBuf[:0] // Clear buffer to prevent memory leak
	return nil
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

	// In alt screen mode, give the calculated height to ScrollList instead of stream.
	// The stream component still exists but is embedded as the last item in scrollList.
	m.scrollList.SetHeight(streamHeight)
	m.scrollList.SetWidth(m.width)

	// Keep stream height in sync for rendering (even though it's embedded in scrollList)
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

	// Persist model selection for next launch.
	go func() { _ = SaveModelPreference(args) }()

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

	m.renderer.UpdateTheme()
	m.stream.UpdateTheme()
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
	// Persist thinking level for next launch.
	go func() { _ = SaveThinkingLevelPreference(string(level)) }()
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

// handleNewCommand starts a completely new session (Pi-style /new behavior).
// Creates a new session file, discarding all context from the previous conversation.
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
		return noopCmd
	}

	return m.performNewSession()
}

// performNewSession performs the actual session reset. Called either directly
// (when no before-hook exists) or after the async hook completes.
// Matches Pi behavior: creates a completely new session file, discarding all
// context from the previous conversation.
func (m *AppModel) performNewSession() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		// No tree session — just clear messages.
		if m.appCtrl != nil {
			m.appCtrl.ClearMessages()
		}
		// Reset usage statistics for fresh session
		if m.usageTracker != nil {
			m.usageTracker.Reset()
		}
		m.printSystemMessage("Conversation cleared. Starting fresh.")
		return nil
	}

	// Create a brand new session file (Pi-style /new behavior)
	newTs, err := session.CreateTreeSession(m.cwd)
	if err != nil {
		m.printSystemMessage(fmt.Sprintf("Failed to create new session: %v", err))
		return nil
	}

	// Switch to the new session, closing the old one
	m.appCtrl.SwitchTreeSession(newTs)
	// Reset usage statistics for the new session
	if m.usageTracker != nil {
		m.usageTracker.Reset()
	}
	m.printSystemMessage("New session started. Previous conversation saved.")
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

	// Branch the tree session to the target entry. We must NOT call
	// ClearMessages() here because it resets the leaf pointer back to "",
	// undoing the branch we just set. Instead, branch first and then
	// reload the in-memory store from the tree session's current branch.
	_ = ts.Branch(targetID)
	m.appCtrl.ReloadMessagesFromTree()

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
// Usage: /name <new name> — sets the session name.
//
//	/name             — shows the current name.
func (m *AppModel) handleNameCommand(args string) tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		m.printSystemMessage("No tree session active.")
		return nil
	}

	if args == "" {
		// No argument — show current name.
		currentName := ts.GetSessionName()
		if currentName != "" {
			m.printSystemMessage(fmt.Sprintf("Session name: %q\nTo rename: `/name <new name>`", currentName))
		} else {
			m.printSystemMessage("Session has no name. Set one with: `/name <new name>`")
		}
		return nil
	}

	// Set the session name.
	if _, err := ts.AppendSessionInfo(args); err != nil {
		m.printSystemMessage(fmt.Sprintf("Failed to set session name: %v", err))
		return nil
	}
	m.printSystemMessage(fmt.Sprintf("Session named %q", args))
	return nil
}

// handleExportCommand exports the current session to a file.
// Usage: /export          — copies the JSONL file to cwd with a descriptive name.
//
//	/export path.jsonl — copies to the specified path.
func (m *AppModel) handleExportCommand(args string) tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		m.printSystemMessage("No tree session active.")
		return nil
	}

	srcPath := ts.GetFilePath()
	if srcPath == "" {
		m.printSystemMessage("Session is in-memory (not persisted). Nothing to export.")
		return nil
	}

	// Determine destination path.
	dstPath := args
	if dstPath == "" {
		// Generate a name based on session name or ID.
		name := ts.GetSessionName()
		if name == "" {
			name = ts.GetSessionID()[:12]
		}
		// Sanitize for filename.
		name = strings.Map(func(r rune) rune {
			if r == '/' || r == '\\' || r == ':' || r == ' ' {
				return '_'
			}
			return r
		}, name)
		dstPath = fmt.Sprintf("session_%s.jsonl", name)
	}

	// Copy the file.
	data, err := os.ReadFile(srcPath)
	if err != nil {
		m.printSystemMessage(fmt.Sprintf("Failed to read session file: %v", err))
		return nil
	}

	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		m.printSystemMessage(fmt.Sprintf("Failed to write export file: %v", err))
		return nil
	}

	m.printSystemMessage(fmt.Sprintf("Session exported to: %s (%d bytes)", dstPath, len(data)))
	return nil
}

// handleShareCommand uploads the current session as a GitHub Gist and prints
// a shareable viewer URL. Requires the GitHub CLI (gh) to be installed and
// authenticated.
func (m *AppModel) handleShareCommand() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		m.printSystemMessage("No tree session active.")
		return nil
	}

	srcPath := ts.GetFilePath()
	if srcPath == "" {
		m.printSystemMessage("Session is in-memory (not persisted). Nothing to share.")
		return nil
	}

	// Check that gh CLI is available.
	if _, err := exec.LookPath("gh"); err != nil {
		m.printSystemMessage("GitHub CLI (gh) is not installed. Install it from https://cli.github.com/")
		return nil
	}

	// Check that gh is authenticated.
	authCheck := exec.Command("gh", "auth", "status")
	if err := authCheck.Run(); err != nil {
		m.printSystemMessage("GitHub CLI is not logged in. Run 'gh auth login' first.")
		return nil
	}

	// Copy session to a temp file with a clean name.
	data, err := os.ReadFile(srcPath)
	if err != nil {
		m.printSystemMessage(fmt.Sprintf("Failed to read session file: %v", err))
		return nil
	}

	name := ts.GetSessionName()
	if name == "" {
		name = "session"
	}
	// Sanitize for filename.
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == ' ' {
			return '_'
		}
		return r
	}, name)

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("kit-%s-*.jsonl", name))
	if err != nil {
		m.printSystemMessage(fmt.Sprintf("Failed to create temp file: %v", err))
		return nil
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		m.printSystemMessage(fmt.Sprintf("Failed to write temp file: %v", err))
		return nil
	}
	_ = tmpFile.Close()

	m.printSystemMessage("Uploading session to GitHub Gist...")

	// Run gh gist create in background to avoid blocking the UI.
	return func() tea.Msg {
		defer func() { _ = os.Remove(tmpPath) }()

		cmd := exec.Command("gh", "gist", "create", tmpPath, "--desc", "Kit session shared via /share")
		output, err := cmd.Output()
		if err != nil {
			return shareResultMsg{err: fmt.Errorf("failed to create gist: %w", err)}
		}

		// gh outputs the gist URL like: https://gist.github.com/username/abc123def456
		gistURL := strings.TrimSpace(string(output))

		// Extract gist ID (last path segment).
		parts := strings.Split(gistURL, "/")
		gistID := parts[len(parts)-1]

		viewerURL := fmt.Sprintf("https://go-kit.dev/session/#%s", gistID)
		return shareResultMsg{gistURL: gistURL, viewerURL: viewerURL}
	}
}

// handleImportCommand imports a session from a JSONL file.
// Usage: /import path.jsonl
func (m *AppModel) handleImportCommand(args string) tea.Cmd {
	if args == "" {
		m.printSystemMessage("Usage: `/import <path.jsonl>`")
		return nil
	}

	if m.switchSession == nil {
		m.printSystemMessage("Session switching is not available.")
		return nil
	}

	// Verify file exists before attempting to switch.
	if _, err := os.Stat(args); err != nil {
		m.printSystemMessage(fmt.Sprintf("File not found: %s", args))
		return nil
	}

	if err := m.switchSession(args); err != nil {
		m.printSystemMessage(fmt.Sprintf("Failed to import session: %v", err))
		return nil
	}

	m.renderSessionHistory()
	m.printSystemMessage(fmt.Sprintf("Session imported from: %s", args))
	return nil
}

// handleResumeCommand opens the session picker so the user can switch sessions.
func (m *AppModel) handleResumeCommand() tea.Cmd {
	if m.switchSession == nil {
		m.printSystemMessage("Session switching is not available.")
		return nil
	}

	m.sessionSelector = NewSessionSelector(m.cwd, m.width, m.height)
	m.state = stateSessionSelector
	return nil
}

// renderSessionHistory walks the current session branch and renders all
// messages (user, assistant, tool calls/results) into the scrollback buffer.
// This gives the user visual context of the conversation when resuming or
// importing a session. Call this after switchSession succeeds.
func (m *AppModel) renderSessionHistory() {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		return
	}

	branch := ts.GetBranch("")
	if len(branch) == 0 {
		return
	}

	// First pass: build a map of tool call ID → {name, args} from assistant
	// messages so we can pair them with tool results.
	type toolCallInfo struct {
		Name string
		Args string
	}
	toolCallMap := make(map[string]toolCallInfo)
	for _, entry := range branch {
		me, ok := entry.(*session.MessageEntry)
		if !ok {
			continue
		}
		if me.Role != "assistant" {
			continue
		}
		msg, err := me.ToMessage()
		if err != nil {
			continue
		}
		for _, tc := range msg.ToolCalls() {
			toolCallMap[tc.ID] = toolCallInfo{Name: tc.Name, Args: tc.Input}
		}
	}

	// Second pass: render each message in order.
	for _, entry := range branch {
		me, ok := entry.(*session.MessageEntry)
		if !ok {
			continue
		}
		msg, err := me.ToMessage()
		if err != nil {
			continue
		}

		switch msg.Role {
		case message.RoleUser:
			text := msg.Content()
			if text != "" {
				m.appendScrollback(m.renderer.RenderUserMessage(text, msg.CreatedAt).Content)
			}

		case message.RoleAssistant:
			// First render any reasoning/thinking content
			reasoning := msg.Reasoning()
			if reasoning.Thinking != "" {
				m.appendScrollback(m.renderer.RenderReasoningBlock(reasoning.Thinking, msg.CreatedAt).Content)
			}
			// Then render the text content
			text := msg.Content()
			if text != "" {
				modelName := m.modelName
				if msg.Model != "" {
					modelName = msg.Model
				}
				m.appendScrollback(m.renderer.RenderAssistantMessage(text, msg.CreatedAt, modelName).Content)
			}
			// Tool calls from assistant messages are rendered when we
			// encounter their corresponding tool results below.

		case message.RoleTool:
			for _, tr := range msg.ToolResults() {
				toolName := tr.Name
				toolArgs := ""
				if info, ok := toolCallMap[tr.ToolCallID]; ok {
					if toolName == "" {
						toolName = info.Name
					}
					toolArgs = info.Args
				}
				m.appendScrollback(m.renderer.RenderToolMessage(toolName, toolArgs, tr.Content, tr.IsError).Content)
			}
		}
	}
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

// shareResultMsg carries the result of an async gist upload.
type shareResultMsg struct {
	err       error
	gistURL   string
	viewerURL string
}

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

		// Ensure SHELL is set to bash so child processes (e.g. tmux) use bash
		// rather than the user's login shell (which may be nushell, fish, etc.).
		bashPath, _ := exec.LookPath("bash")
		if bashPath == "" {
			bashPath = "/bin/bash"
		}
		cmd.Env = append(os.Environ(), "SHELL="+bashPath)

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
		// Cap individual line length to prevent long lines from wrapping
		// into excessive visual rows.
		maxLineChars := max(m.width*3, 200)
		for i, line := range lines {
			if len(line) > maxLineChars {
				lines[i] = line[:maxLineChars] + "…"
			}
		}
		if len(lines) > maxShellDisplayLines {
			displayHiddenCount = len(lines) - maxShellDisplayLines
			displayOutput = strings.Join(lines[:maxShellDisplayLines], "\n")
		} else {
			displayOutput = strings.Join(lines, "\n")
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
