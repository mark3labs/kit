package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mark3labs/kit/internal/app"
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
}

// SkillItem holds display metadata about a loaded skill for the startup
// [Skills] section. Built by the CLI layer from the SDK's []*kit.Skill.
type SkillItem struct {
	Name   string // Skill name (e.g. "btca-cli").
	Path   string // Absolute path to the skill file.
	Source string // "project" or "user" (global).
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
	// Placeholder until InputComponent is implemented in TAS-15.
	input inputComponentIface

	// stream is the child streaming display component (spinner + streaming text).
	// Placeholder until StreamComponent is implemented in TAS-16.
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
		rdr = NewCompactRenderer(width, false)
	} else {
		rdr = NewMessageRenderer(width, false)
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
		width:          width,
		height:         height,
	}

	// Store extension commands for dispatch.
	m.extensionCommands = opts.ExtensionCommands
	m.getWidgets = opts.GetWidgets
	m.getHeader = opts.GetHeader
	m.getFooter = opts.GetFooter

	// Store context/skills metadata and tool counts for startup display.
	m.contextPaths = opts.ContextPaths
	m.skillItems = opts.SkillItems
	m.mcpToolCount = opts.MCPToolCount
	m.extensionToolCount = opts.ExtensionToolCount

	// Wire up child components now that we have the concrete implementations.
	m.input = NewInputComponent(width, "Enter your prompt (Type /help for commands, Ctrl+C to quit)", appCtrl)

	// Merge extension commands into the InputComponent's autocomplete source.
	if ic, ok := m.input.(*InputComponent); ok && len(opts.ExtensionCommands) > 0 {
		for _, ec := range opts.ExtensionCommands {
			ic.commands = append(ic.commands, SlashCommand{
				Name:        ec.Name,
				Description: ec.Description,
				Category:    "Extensions",
			})
		}
	}

	m.stream = NewStreamComponent(opts.CompactMode, width, opts.ModelName)

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

// PrintStartupInfo prints the startup banner (model name, context, skills,
// tool counts) to stdout. Call this before program.Run() so the messages are
// visible above the Bubble Tea managed region.
//
// All startup information is rendered inside a single system message block.
func (m *AppModel) PrintStartupInfo() {
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
			_ = ts.Branch(targetID)
			m.appCtrl.ClearMessages()

			// If it was a user message, populate the input with the text.
			if msg.IsUser && msg.UserText != "" {
				if ic, ok := m.input.(*InputComponent); ok {
					ic.textarea.SetValue(msg.UserText)
					ic.textarea.CursorEnd()
				}
			}

			cmds = append(cmds, m.printSystemMessage(
				fmt.Sprintf("Navigated to branch point. %s",
					func() string {
						if msg.IsUser {
							return "Edit and resubmit to create a new branch."
						}
						return "Continue from this point."
					}())))
		}
		m.treeSelector = nil
		m.state = stateInput
		return m, tea.Batch(cmds...)

	case TreeCancelledMsg:
		m.treeSelector = nil
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
			// Graceful quit: app.Close() is deferred in cmd/root.go.
			return m, tea.Quit
		}

		// Route to tree selector when active.
		if m.state == stateTreeSelector && m.treeSelector != nil {
			updated, cmd := m.treeSelector.Update(msg)
			m.treeSelector = updated.(*TreeSelectorComponent)
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

		// Route key events to the focused child.
		if m.input != nil {
			updated, cmd := m.input.Update(msg)
			m.input, _ = updated.(inputComponentIface)
			cmds = append(cmds, cmd)
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
			return m, tea.Batch(cmds...)
		}

		// /compact supports optional args: "/compact Focus on API decisions".
		// GetCommandByName won't match the full text, so check the prefix.
		if name, args, ok := strings.Cut(msg.Text, " "); ok {
			if sc := GetCommandByName(name); sc != nil && sc.Name == "/compact" {
				if cmd := m.handleCompactCommand(strings.TrimSpace(args)); cmd != nil {
					cmds = append(cmds, cmd)
				}
				return m, tea.Batch(cmds...)
			}
		}

		// Check extension-registered slash commands. These support arguments
		// (e.g. "/sub list files"), so we split on the first space.
		if cmd := m.handleExtensionCommand(msg.Text); cmd != nil {
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		// Regular prompt — forward to the app layer.
		if m.appCtrl != nil {
			// Run returns the queue depth: >0 means the prompt was queued
			// (agent is busy). We update queuedMessages directly here
			// instead of relying on an event from prog.Send(), which would
			// deadlock when called synchronously from within Update().
			if qLen := m.appCtrl.Run(msg.Text); qLen > 0 {
				// Queued: anchor the message text above the input with a
				// "queued" badge. It will be printed to scrollback when
				// the agent picks it up (on QueueUpdatedEvent).
				m.queuedMessages = append(m.queuedMessages, msg.Text)
				m.distributeHeight()
			} else {
				// Started immediately: print to scrollback now.
				cmds = append(cmds, m.printUserMessage(msg.Text))
			}
		} else {
			cmds = append(cmds, m.printUserMessage(msg.Text))
		}
		if m.state != stateWorking {
			m.state = stateWorking
		}

	// ── App layer events ─────────────────────────────────────────────────────

	case app.SpinnerEvent:
		// SpinnerEvent{Show: true} means a new agent step has started (either
		// freshly or from the queue after a previous step completed). Flush
		// any leftover stream content from the previous step to scrollback
		// before starting the new one. This deferred flush avoids shrinking
		// the view at step-completion time (which leaves blank lines).
		if msg.Show {
			cmds = append(cmds, m.flushStreamContent())
			m.state = stateWorking
			m.distributeHeight()
		}
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
		cmds = append(cmds, m.flushStreamContent())

	case app.ToolExecutionEvent:
		// Pass to stream component for execution spinner display.
		if m.stream != nil {
			_, cmd := m.stream.Update(msg)
			cmds = append(cmds, cmd)
		}

	case app.ToolResultEvent:
		// Print tool result immediately to scrollback.
		cmds = append(cmds, m.printToolResult(msg))
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
		// Non-streaming mode: this carries the full response text (StreamChunkEvents
		// never fire). Print it immediately.
		if msg.Content != "" {
			cmds = append(cmds, m.printAssistantMessage(msg.Content))
		}
		if m.stream != nil {
			m.stream.Reset() // stop spinner
		}

	case app.MessageCreatedEvent:
		// Informational — no action needed by parent.

	case app.QueueUpdatedEvent:
		// drainQueue popped item(s) from the queue. Move consumed messages
		// from the anchored display to scrollback (they are now being processed
		// or about to be).
		for len(m.queuedMessages) > msg.Length {
			text := m.queuedMessages[0]
			m.queuedMessages = m.queuedMessages[1:]
			cmds = append(cmds, m.printUserMessage(text))
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
			cmds = append(cmds, m.printErrorResponse(msg))
		}
		m.state = stateInput
		m.canceling = false

	case app.CompactCompleteEvent:
		if m.stream != nil {
			m.stream.Reset()
		}
		m.state = stateInput
		cmds = append(cmds, m.printCompactResult(msg))

	case app.CompactErrorEvent:
		if m.stream != nil {
			m.stream.Reset()
		}
		m.state = stateInput
		cmds = append(cmds, m.printSystemMessage(fmt.Sprintf("Compaction failed: %v", msg.Err)))

	case app.WidgetUpdateEvent:
		// Extension widget changed — recalculate height distribution so the
		// stream region accounts for widget space. View() will read the
		// latest widget state on the next render.
		m.distributeHeight()

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

	case extensionCmdResultMsg:
		// Async extension slash command completed. Render output/error.
		if msg.err != nil {
			cmds = append(cmds, m.printSystemMessage(
				fmt.Sprintf("Command %s error: %v", msg.name, msg.err)))
		} else if msg.output != "" {
			cmds = append(cmds, m.printSystemMessage(msg.output))
		}

	case app.ExtensionPrintEvent:
		// Extension output — route through styled renderers when a level is set.
		switch msg.Level {
		case "info":
			cmds = append(cmds, m.printSystemMessage(msg.Text))
		case "error":
			cmds = append(cmds, m.printErrorResponse(app.StepErrorEvent{
				Err: fmt.Errorf("%s", msg.Text),
			}))
		case "block":
			cmds = append(cmds, m.printExtensionBlock(msg))
		default:
			cmds = append(cmds, tea.Println(msg.Text))
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

	streamView := m.renderStream()
	separator := m.renderSeparator()

	// When a prompt is active, it replaces the input area for consistency
	// (appears below the separator, in the same position as the input).
	var inputView string
	if m.state == statePrompt && m.prompt != nil {
		inputView = m.prompt.Render()
	} else {
		inputView = m.renderInput()
	}
	statusBar := m.renderStatusBar()

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
	parts = append(parts, separator)

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

	parts = append(parts, statusBar)

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
		warning := lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
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
// Left side: spinner (when active). Right side: provider · model + usage stats.
// This bar is always present so its height is constant, eliminating layout
// shifts from spinner or usage info appearing/disappearing.
func (m *AppModel) renderStatusBar() string {
	theme := GetTheme()

	// Left side: spinner animation (when active).
	var leftSide string
	if m.stream != nil {
		leftSide = m.stream.SpinnerView()
	}
	leftWidth := lipgloss.Width(leftSide)

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
	rightWidth := lipgloss.Width(rightSide)

	// Fill the gap between left and right with spaces.
	gap := max(m.width-leftWidth-rightWidth, 1)

	return leftSide + strings.Repeat(" ", gap) + rightSide
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

// renderInput returns the input region content.
func (m *AppModel) renderInput() string {
	if m.input == nil {
		return ""
	}
	return m.input.View().Content
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

// printUserMessage renders a user message and emits it above the BT region.
func (m *AppModel) printUserMessage(text string) tea.Cmd {
	return tea.Println(m.renderer.RenderUserMessage(text, time.Now()).Content)
}

// printAssistantMessage renders an assistant message and emits it above the BT region.
func (m *AppModel) printAssistantMessage(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	return tea.Println(m.renderer.RenderAssistantMessage(text, time.Now(), m.modelName).Content)
}

// printToolResult renders a tool result message and emits it above the BT region.
func (m *AppModel) printToolResult(evt app.ToolResultEvent) tea.Cmd {
	return tea.Println(m.renderer.RenderToolMessage(evt.ToolName, evt.ToolArgs, evt.Result, evt.IsError).Content)
}

// printErrorResponse renders an error message and emits it above the BT region.
func (m *AppModel) printErrorResponse(evt app.StepErrorEvent) tea.Cmd {
	if evt.Err == nil {
		return nil
	}
	return tea.Println(m.renderer.RenderErrorMessage(evt.Err.Error(), time.Now()).Content)
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
		return m.printHelpMessage()
	case "/tools":
		return m.printToolsMessage()
	case "/servers":
		return m.printServersMessage()
	case "/usage":
		return m.printUsageMessage()
	case "/reset-usage":
		return m.printResetUsage()
	case "/compact":
		return m.handleCompactCommand("")
	case "/clear":
		if m.appCtrl != nil {
			m.appCtrl.ClearMessages()
		}
		return m.printSystemMessage("Conversation cleared. Starting fresh.")
	case "/clear-queue":
		if m.appCtrl != nil {
			m.appCtrl.ClearQueue()
		}
		m.queuedMessages = m.queuedMessages[:0]
		m.distributeHeight()
		return nil

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
		return m.printSystemMessage(fmt.Sprintf("Unknown command: %s", sc.Name))
	}
}

// printSystemMessage renders a system-level message and emits it above the BT region.
func (m *AppModel) printSystemMessage(text string) tea.Cmd {
	return tea.Println(m.renderer.RenderSystemMessage(text, time.Now()).Content)
}

// printExtensionBlock renders a custom styled block from an extension with
// caller-chosen border color and optional subtitle, then emits it to scrollback.
func (m *AppModel) printExtensionBlock(evt app.ExtensionPrintEvent) tea.Cmd {
	theme := GetTheme()

	// Resolve border color: use the extension's hex value, fall back to theme accent.
	var borderClr = lipgloss.Color("#89b4fa") // default blue
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
	return tea.Println(rendered)
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
func (m *AppModel) printHelpMessage() tea.Cmd {
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

	help += "**Keys:**\n" +
		"- `Ctrl+C`: Exit at any time\n" +
		"- `ESC` (x2): Cancel ongoing LLM generation\n\n" +
		"You can also just type your message to chat with the AI assistant."
	return m.printSystemMessage(help)
}

// printToolsMessage renders the list of available tools.
func (m *AppModel) printToolsMessage() tea.Cmd {
	var content string
	content = "## Available Tools\n\n"
	if len(m.toolNames) == 0 {
		content += "No tools are currently available."
	} else {
		for i, tool := range m.toolNames {
			content += fmt.Sprintf("%d. `%s`\n", i+1, tool)
		}
	}
	return m.printSystemMessage(content)
}

// printServersMessage renders the list of configured MCP servers.
func (m *AppModel) printServersMessage() tea.Cmd {
	var content string
	content = "## Configured MCP Servers\n\n"
	if len(m.serverNames) == 0 {
		content += "No MCP servers are currently configured."
	} else {
		for i, server := range m.serverNames {
			content += fmt.Sprintf("%d. `%s`\n", i+1, server)
		}
	}
	return m.printSystemMessage(content)
}

// printUsageMessage renders token usage statistics.
func (m *AppModel) printUsageMessage() tea.Cmd {
	if m.usageTracker == nil {
		return m.printSystemMessage("Usage tracking is not available for this model.")
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

	return m.printSystemMessage(content)
}

// printResetUsage resets usage statistics and prints a confirmation.
func (m *AppModel) printResetUsage() tea.Cmd {
	if m.usageTracker == nil {
		return m.printSystemMessage("Usage tracking is not available for this model.")
	}
	m.usageTracker.Reset()
	return m.printSystemMessage("Usage statistics have been reset.")
}

// handleCompactCommand starts an async compaction. It returns a tea.Cmd that
// prints a "compacting..." message and transitions to the working state. If
// the app controller rejects the request (busy, closed) it prints an error
// instead. customInstructions is optional text appended to the summary
// prompt (e.g. "Focus on the API design decisions").
func (m *AppModel) handleCompactCommand(customInstructions string) tea.Cmd {
	if m.appCtrl == nil {
		return m.printSystemMessage("Compaction is not available.")
	}
	if err := m.appCtrl.CompactConversation(customInstructions); err != nil {
		return m.printSystemMessage(fmt.Sprintf("Cannot compact: %v", err))
	}
	// Transition to working state so the spinner shows while compaction runs.
	m.state = stateWorking
	var spinnerCmd tea.Cmd
	if m.stream != nil {
		_, spinnerCmd = m.stream.Update(app.SpinnerEvent{Show: true})
	}
	return tea.Batch(m.printSystemMessage("Compacting conversation..."), spinnerCmd)
}

// printCompactResult renders the compaction summary in a styled block with
// a distinct border color and a stats subtitle.
func (m *AppModel) printCompactResult(evt app.CompactCompleteEvent) tea.Cmd {
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
	return tea.Println(rendered)
}

// flushStreamContent gets the rendered content from the stream component,
// emits it above the BT region via tea.Println, and resets the stream. This
// is called before printing tool calls (streaming completes before tools fire)
// and on step completion.
//
// After flushing, a ClearScreen is issued to force a full terminal redraw.
// This is the bubbletea equivalent of pi's "clearOnShrink" mechanism: when
// the stream content is moved to scrollback the view height shrinks, and
// bubbletea's inline renderer doesn't clear the orphaned terminal rows
// below the managed region. ClearScreen ensures a clean redraw.
func (m *AppModel) flushStreamContent() tea.Cmd {
	if m.stream == nil {
		return nil
	}
	content := m.stream.GetRenderedContent()
	if content == "" {
		return nil
	}
	m.stream.Reset()
	return tea.Sequence(
		tea.Println(content),
		func() tea.Msg { return tea.ClearScreen() },
	)
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
//	queued msgs    = ~5 lines per message (padding + text + badge + padding)
//	input region   = measured dynamically via lipgloss.Height()
//	below widgets  = measured dynamically
//	status bar     = 1 line (always present)
//	footer         = measured dynamically (0 if not set)
func (m *AppModel) distributeHeight() {
	const separatorLines = 1
	const statusBarLines = 1 // always-present status bar
	const linesPerQueuedMsg = 5
	queuedLines := len(m.queuedMessages) * linesPerQueuedMsg

	// Measure the actual rendered input (or prompt overlay) height so we
	// don't rely on a fragile constant that drifts when styling changes.
	inputLines := 9 // fallback: title(1)+margin(1)+nl(1)+textarea(3)+nl(1)+margin(1)+help(1)
	if m.state == statePrompt && m.prompt != nil {
		if rendered := m.prompt.Render(); rendered != "" {
			inputLines = lipgloss.Height(rendered)
		}
	} else if m.input != nil {
		if rendered := m.input.View().Content; rendered != "" {
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
// Tree session command handlers
// --------------------------------------------------------------------------

// handleTreeCommand opens the tree selector overlay.
func (m *AppModel) handleTreeCommand() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		return m.printSystemMessage("No tree session active. Start with `--continue` or `--resume` to enable tree sessions.")
	}
	if ts.EntryCount() == 0 {
		return m.printSystemMessage("No entries in session yet.")
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
		return m.printSystemMessage("No tree session active. Start with `--continue` or `--resume` to enable tree sessions.")
	}
	if ts.EntryCount() == 0 {
		return m.printSystemMessage("No entries to fork from.")
	}

	m.treeSelector = NewTreeSelector(ts, m.width, m.height)
	m.state = stateTreeSelector
	return nil
}

// handleNewCommand starts a fresh session by resetting the tree leaf.
func (m *AppModel) handleNewCommand() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		// No tree session — just clear messages.
		if m.appCtrl != nil {
			m.appCtrl.ClearMessages()
		}
		return m.printSystemMessage("Conversation cleared. Starting fresh.")
	}

	ts.ResetLeaf()
	if m.appCtrl != nil {
		m.appCtrl.ClearMessages()
	}
	return m.printSystemMessage("New branch started. Previous conversation is preserved in the tree.")
}

// handleNameCommand sets a display name for the current session.
func (m *AppModel) handleNameCommand() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		return m.printSystemMessage("No tree session active.")
	}
	// For now, prompt user to provide name via input. We print instructions
	// and the next non-command input starting with "name:" will be captured.
	// TODO: inline input dialog like pi's implementation.
	currentName := ts.GetSessionName()
	if currentName != "" {
		return m.printSystemMessage(fmt.Sprintf("Current session name: %q\nTo rename, type: `/name <new name>` (not yet implemented — use the session file directly).", currentName))
	}
	return m.printSystemMessage("To name this session, use: `/name <new name>` (not yet implemented — use the session file directly).")
}

// handleSessionInfoCommand shows session statistics.
func (m *AppModel) handleSessionInfoCommand() tea.Cmd {
	ts := m.appCtrl.GetTreeSession()
	if ts == nil {
		return m.printSystemMessage("No tree session active.")
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

	return m.printSystemMessage(info)
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
