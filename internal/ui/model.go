package ui

import (
	"fmt"
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
	// GetTreeSession returns the tree session manager, or nil if tree sessions
	// are not enabled. Used by slash commands like /tree, /fork, /session.
	GetTreeSession() *session.TreeManager
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
}

// AppModel is the root Bubble Tea model for the interactive TUI. It owns the
// state machine, routes events to child components, and manages the overall
// layout. It holds a reference to the app layer (AppController) for triggering
// agent work and queue operations.
//
// Layout (stacked, no alt screen):
//
//	┌─ stream region (variable height) ─────────────────┐
//	│                                                    │
//	├─ separator line (with optional queue count) ───────┤
//	│  queued  How do I fix the build?                   │
//	│  queued  Also check the tests                      │
//	└─ input region (fixed height from textarea) ────────┘
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

	// renderer renders completed assistant messages for tea.Println output.
	renderer *MessageRenderer

	// compactRdr renders in compact mode.
	compactRdr *CompactRenderer

	// compactMode selects which renderer to use.
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

	// treeSelector is the tree navigation overlay, active in stateTreeSelector.
	treeSelector *TreeSelectorComponent

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

	m := &AppModel{
		state:          stateInput,
		appCtrl:        appCtrl,
		renderer:       NewMessageRenderer(width, false),
		compactRdr:     NewCompactRenderer(width, false),
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

	// Wire up child components now that we have the concrete implementations.
	m.input = NewInputComponent(width, "Enter your prompt (Type /help for commands, Ctrl+C to quit)", appCtrl)
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

// PrintStartupInfo writes startup messages (model loaded, tool count) to
// stdout. Call this before program.Run() so the messages are visible above
// the Bubble Tea managed region.
func (m *AppModel) PrintStartupInfo() {
	render := func(text string) string {
		if m.compactMode {
			return m.compactRdr.RenderSystemMessage(text, time.Now()).Content
		}
		return m.renderer.RenderSystemMessage(text, time.Now()).Content
	}

	fmt.Println()

	if m.providerName != "" && m.modelName != "" {
		fmt.Println(render(fmt.Sprintf("Model loaded: %s (%s)", m.providerName, m.modelName)))
	}

	if m.loadingMessage != "" {
		fmt.Println(render(m.loadingMessage))
	}

	fmt.Println(render(fmt.Sprintf("Loaded %d tools from MCP servers", len(m.toolNames))))
}

// Update implements tea.Model. It is the heart of the state machine: it routes
// incoming messages to children and handles state transitions.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

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
		// freshly or from the queue after a previous step completed). Transition
		// to stateWorking so the TUI reflects the active state. This is
		// especially important for queued prompts: after StepCompleteEvent
		// resets state to stateInput, the next queued step fires SpinnerEvent
		// and we must go back to stateWorking.
		if msg.Show {
			m.state = stateWorking
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
		// Flush any remaining streamed text to scrollback, then reset stream
		// and return to input state. Token usage is rendered as a sticky
		// element in View() — the app layer has already updated the shared
		// UsageTracker before sending this event.
		cmds = append(cmds, m.flushStreamContent())
		if m.stream != nil {
			m.stream.Reset()
		}
		m.state = stateInput
		m.canceling = false

	case app.StepCancelledEvent:
		// User cancelled the step (double-ESC). Flush any partial content,
		// cut off the response where it was, and return to input with no error.
		cmds = append(cmds, m.flushStreamContent())
		if m.stream != nil {
			m.stream.Reset()
		}
		m.state = stateInput
		m.canceling = false

	case app.StepErrorEvent:
		// Flush streamed text, print the error, reset stream, return to input.
		cmds = append(cmds, m.flushStreamContent())
		if msg.Err != nil {
			cmds = append(cmds, m.printErrorResponse(msg))
		}
		if m.stream != nil {
			m.stream.Reset()
		}
		m.state = stateInput
		m.canceling = false

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
// stream region + [usage info] + separator + [queued messages] + input region.
// When the tree selector is active, it replaces the stream region.
func (m *AppModel) View() tea.View {
	// Tree selector overlay replaces the normal layout.
	if m.state == stateTreeSelector && m.treeSelector != nil {
		return m.treeSelector.View()
	}

	streamView := m.renderStream()
	separator := m.renderSeparator()
	inputView := m.renderInput()

	parts := []string{streamView}

	// Sticky usage info sits between the stream and separator so it is
	// always visible at the bottom of the messages area and updates in place.
	if usageView := m.renderUsageInfo(); usageView != "" {
		parts = append(parts, usageView)
	}

	parts = append(parts, separator)

	if queuedView := m.renderQueuedMessages(); queuedView != "" {
		parts = append(parts, queuedView)
	}

	parts = append(parts, inputView)

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

// renderUsageInfo returns the sticky token usage line (tokens + context% + cost).
// Returns an empty string when no usage data is available (no requests yet or
// tracker is nil), so the element is invisible until the first response arrives.
func (m *AppModel) renderUsageInfo() string {
	if m.usageTracker == nil {
		return ""
	}
	return m.usageTracker.RenderUsageInfo()
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
	var rendered string
	if m.compactMode {
		msg := m.compactRdr.RenderUserMessage(text, time.Now())
		rendered = msg.Content
	} else {
		msg := m.renderer.RenderUserMessage(text, time.Now())
		rendered = msg.Content
	}
	return tea.Println(rendered)
}

// printAssistantMessage renders an assistant message and emits it above the BT region.
func (m *AppModel) printAssistantMessage(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	var rendered string
	if m.compactMode {
		msg := m.compactRdr.RenderAssistantMessage(text, time.Now(), m.modelName)
		rendered = msg.Content
	} else {
		msg := m.renderer.RenderAssistantMessage(text, time.Now(), m.modelName)
		rendered = msg.Content
	}
	return tea.Println(rendered)
}

// printToolResult renders a tool result message and emits it above the BT region.
func (m *AppModel) printToolResult(evt app.ToolResultEvent) tea.Cmd {
	var rendered string
	if m.compactMode {
		msg := m.compactRdr.RenderToolMessage(evt.ToolName, evt.ToolArgs, evt.Result, evt.IsError)
		rendered = msg.Content
	} else {
		msg := m.renderer.RenderToolMessage(evt.ToolName, evt.ToolArgs, evt.Result, evt.IsError)
		rendered = msg.Content
	}
	return tea.Println(rendered)
}

// printErrorResponse renders an error message and emits it above the BT region.
func (m *AppModel) printErrorResponse(evt app.StepErrorEvent) tea.Cmd {
	if evt.Err == nil {
		return nil
	}
	var rendered string
	if m.compactMode {
		msg := m.compactRdr.RenderErrorMessage(evt.Err.Error(), time.Now())
		rendered = msg.Content
	} else {
		msg := m.renderer.RenderErrorMessage(evt.Err.Error(), time.Now())
		rendered = msg.Content
	}
	return tea.Println(rendered)
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
	var rendered string
	if m.compactMode {
		msg := m.compactRdr.RenderSystemMessage(text, time.Now())
		rendered = msg.Content
	} else {
		msg := m.renderer.RenderSystemMessage(text, time.Now())
		rendered = msg.Content
	}
	return tea.Println(rendered)
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
		"- `/clear`: Clear message history\n" +
		"- `/reset-usage`: Reset usage statistics\n" +
		"- `/quit`: Exit the application\n\n" +
		"**Keys:**\n" +
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

// flushStreamContent gets the rendered content from the stream component,
// emits it above the BT region via tea.Println, and resets the stream. This
// is called before printing tool calls (streaming completes before tools fire)
// and on step completion.
func (m *AppModel) flushStreamContent() tea.Cmd {
	if m.stream == nil {
		return nil
	}
	content := m.stream.GetRenderedContent()
	if content == "" {
		return nil
	}
	m.stream.Reset()
	return tea.Println(content)
}

// distributeHeight recalculates child component heights after a window resize
// or queue change, and propagates the computed stream height to the
// StreamComponent.
//
// Layout (line counts):
//
//	stream region  = total - usage(0-1) - separator(1) - queued(N*5) - input(5)
//	usage info     = 0 or 1 line (visible only after first response)
//	separator      = 1 line
//	queued msgs    = ~5 lines per message (padding + text + badge + padding)
//	input region   = 5 lines: title(1) + textarea(3) + help(1)
func (m *AppModel) distributeHeight() {
	const separatorLines = 1
	const inputLines = 5 // title (1) + textarea (3) + help (1)
	const linesPerQueuedMsg = 5
	queuedLines := len(m.queuedMessages) * linesPerQueuedMsg

	// Reserve space for the sticky usage line when the tracker has data.
	usageLines := 0
	if m.usageTracker != nil && m.usageTracker.GetSessionStats().RequestCount > 0 {
		usageLines = 1
	}

	streamHeight := max(m.height-usageLines-separatorLines-queuedLines-inputLines, 0)

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
