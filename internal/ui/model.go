package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mark3labs/mcphost/internal/app"
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

	// stateApproval means a tool approval dialog is active. The user must
	// approve or deny before the agent can continue.
	stateApproval
)

// AppController is the interface the parent TUI model uses to interact with the
// app layer. It is satisfied by *app.App once that is created (TAS-4).
// Using an interface here keeps model.go compilable before app.App exists, and
// makes the parent model easily testable with a mock.
type AppController interface {
	// Run queues or immediately starts a new agent step with the given prompt.
	// If an agent step is already in progress the prompt is queued and a
	// QueueUpdatedEvent is sent to the program.
	Run(prompt string)
	// CancelCurrentStep cancels any in-progress agent step.
	CancelCurrentStep()
	// QueueLength returns the number of prompts currently waiting in the queue.
	QueueLength() int
	// ClearQueue discards all queued prompts and emits a QueueUpdatedEvent.
	ClearQueue()
	// ClearMessages clears the conversation history.
	ClearMessages()
}

// AppModelOptions holds configuration passed to NewAppModel.
type AppModelOptions struct {
	// CompactMode selects the compact renderer for message formatting.
	CompactMode bool

	// ModelName is the display name of the model (e.g. "claude-sonnet-4-5").
	ModelName string

	// Width is the initial terminal width in columns.
	Width int

	// Height is the initial terminal height in rows.
	Height int
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
//	├─ separator line (with optional queue badge) ───────┤
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

	// approval is the child tool approval dialog component.
	// Placeholder until ApprovalComponent is implemented in TAS-17.
	approval approvalComponentIface

	// renderer renders completed assistant messages for tea.Println output.
	renderer *MessageRenderer

	// compactRdr renders in compact mode.
	compactRdr *CompactRenderer

	// compactMode selects which renderer to use.
	compactMode bool

	// modelName is the LLM model name shown in rendered messages.
	modelName string

	// queueCount is cached from the last QueueUpdatedEvent for badge rendering.
	queueCount int

	// canceling tracks whether the user has pressed ESC once during stateWorking.
	// A second ESC within 2 seconds will cancel the current step.
	canceling bool

	// approvalChan is the response channel for the current tool approval.
	// Set when a ToolApprovalNeededEvent arrives; cleared after sending the result.
	approvalChan chan<- bool

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

// approvalComponentIface is the interface the parent requires from ApprovalComponent.
// It will be satisfied by the real ApprovalComponent created in TAS-17.
type approvalComponentIface interface {
	tea.Model
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
// using the provided options. ApprovalComponent is created dynamically per-step
// in Update when a ToolApprovalNeededEvent arrives.
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
		state:       stateInput,
		appCtrl:     appCtrl,
		renderer:    NewMessageRenderer(width, false),
		compactRdr:  NewCompactRenderer(width, false),
		compactMode: opts.CompactMode,
		modelName:   opts.ModelName,
		width:       width,
		height:      height,
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

// Init implements tea.Model. No startup commands needed; the app layer fires
// events via program.Send() once the agent starts.
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

// Update implements tea.Model. It is the heart of the state machine: it routes
// incoming messages to children and handles state transitions.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

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
		if m.state == stateApproval && m.approval != nil {
			updated, cmd := m.approval.Update(msg)
			m.approval, _ = updated.(approvalComponentIface)
			cmds = append(cmds, cmd)
		} else if m.input != nil {
			updated, cmd := m.input.Update(msg)
			m.input, _ = updated.(inputComponentIface)
			cmds = append(cmds, cmd)
		}

	// ── Cancel timer expired ─────────────────────────────────────────────────
	case cancelTimerExpiredMsg:
		m.canceling = false

	// ── Input submitted ──────────────────────────────────────────────────────
	case submitMsg:
		// Handle /quit (and its aliases) before sending to the app layer:
		// look up the command and check if it resolves to "/quit".
		if cmd := GetCommandByName(msg.Text); cmd != nil && cmd.Name == "/quit" {
			return m, tea.Quit
		}
		// Print user message immediately to scrollback.
		cmds = append(cmds, m.printUserMessage(msg.Text))
		if m.appCtrl != nil {
			// app.Run() handles queueing internally if a step is in progress.
			m.appCtrl.Run(msg.Text)
		}
		if m.state != stateWorking {
			m.state = stateWorking
		}

	// ── Approval result ──────────────────────────────────────────────────────
	case approvalResultMsg:
		if m.approvalChan != nil {
			m.approvalChan <- msg.Approved
			m.approvalChan = nil
		}
		m.state = stateWorking

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
		// always completes before tool calls fire), then print the tool call.
		cmds = append(cmds, m.flushStreamContent())
		cmds = append(cmds, m.printToolCall(msg))

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

	case app.HookBlockedEvent:
		// Print hook blocked message immediately to scrollback.
		cmds = append(cmds, m.printHookBlocked(msg))

	case app.MessageCreatedEvent:
		// Informational — no action needed by parent.

	case app.QueueUpdatedEvent:
		m.queueCount = msg.Length

	case app.ToolApprovalNeededEvent:
		// Store the response channel and transition to approval state.
		m.approvalChan = msg.ResponseChan
		m.state = stateApproval
		// Construct the ApprovalComponent and init it (returns nil cmd, but good practice).
		approvalComp := NewApprovalComponent(msg.ToolName, msg.ToolArgs, m.width)
		cmds = append(cmds, approvalComp.Init())
		m.approval = approvalComp

	case app.StepCompleteEvent:
		// Flush any remaining streamed text to scrollback, then reset stream
		// and return to input state.
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
		if m.state == stateApproval && m.approval != nil {
			_, cmd := m.approval.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model. It renders the stacked layout:
// stream region + separator + input region.
func (m *AppModel) View() tea.View {
	streamView := m.renderStream()
	separator := m.renderSeparator()
	inputView := m.renderInput()

	content := lipgloss.JoinVertical(lipgloss.Left,
		streamView,
		separator,
		inputView,
	)

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

	if m.state == stateApproval && m.approval != nil {
		// Show both stream context and the approval dialog stacked.
		return lipgloss.JoinVertical(lipgloss.Left,
			m.stream.View().Content,
			m.approval.View().Content,
		)
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

// renderSeparator renders the separator line with an optional queue badge.
func (m *AppModel) renderSeparator() string {
	theme := GetTheme()
	lineStyle := lipgloss.NewStyle().Foreground(theme.Muted)

	if m.queueCount > 0 {
		badge := lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Render(fmt.Sprintf("%d queued", m.queueCount))

		// Fill the separator with dashes up to the badge.
		dashWidth := m.width - lipgloss.Width(badge) - 1
		if dashWidth < 0 {
			dashWidth = 0
		}
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

// printToolCall renders a tool call message and emits it above the BT region.
func (m *AppModel) printToolCall(evt app.ToolCallStartedEvent) tea.Cmd {
	var rendered string
	if m.compactMode {
		msg := m.compactRdr.RenderToolCallMessage(evt.ToolName, evt.ToolArgs, time.Now())
		rendered = msg.Content
	} else {
		msg := m.renderer.RenderToolCallMessage(evt.ToolName, evt.ToolArgs, time.Now())
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

// printHookBlocked renders a hook-blocked notice and emits it above the BT region.
func (m *AppModel) printHookBlocked(evt app.HookBlockedEvent) tea.Cmd {
	theme := GetTheme()
	rendered := lipgloss.NewStyle().
		Foreground(theme.Error).
		Bold(true).
		Render("  ⛔ Hook blocked: " + evt.Message)
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
// and propagates the computed stream height to the StreamComponent.
//
// Layout (line counts):
//
//	stream region  = total - separator(1) - input(5)
//	separator      = 1 line
//	input region   = 5 lines: title(1) + textarea(3) + help(1)
func (m *AppModel) distributeHeight() {
	const separatorLines = 1
	const inputLines = 5 // title (1) + textarea (3) + help (1)

	streamHeight := m.height - separatorLines - inputLines
	if streamHeight < 0 {
		streamHeight = 0
	}

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
// Cancel timer command
// --------------------------------------------------------------------------

// cancelTimerCmd returns a tea.Cmd that fires cancelTimerExpiredMsg after 2s.
// This is used for the double-tap ESC cancel flow.
func cancelTimerCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return cancelTimerExpiredMsg{}
	})
}
