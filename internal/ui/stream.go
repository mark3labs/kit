package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mark3labs/mcphost/internal/app"
)

// knightRiderFrames generates a KITT-style scanning animation where a bright
// red light bounces back and forth across a row of dots with a trailing glow.
// Used by StreamComponent (TUI inline spinner) and Spinner (stderr goroutine spinner).
func knightRiderFrames() []string {
	const numDots = 8
	const dot = "▪"

	bright := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	med := lipgloss.NewStyle().Foreground(lipgloss.Color("#990000"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#440000"))
	off := lipgloss.NewStyle().Foreground(lipgloss.Color("#222222"))

	// Scanner bounces: 0→7→0
	positions := make([]int, 0, 2*numDots-2)
	for i := 0; i < numDots; i++ {
		positions = append(positions, i)
	}
	for i := numDots - 2; i > 0; i-- {
		positions = append(positions, i)
	}

	frames := make([]string, len(positions))
	for f, pos := range positions {
		var b strings.Builder
		for i := 0; i < numDots; i++ {
			d := pos - i
			if d < 0 {
				d = -d
			}
			switch {
			case d == 0:
				b.WriteString(bright.Render(dot))
			case d == 1:
				b.WriteString(med.Render(dot))
			case d == 2:
				b.WriteString(dim.Render(dot))
			default:
				b.WriteString(off.Render(dot))
			}
		}
		frames[f] = b.String()
	}
	return frames
}

// streamSpinnerTickMsg is the internal tick message that drives the KITT-style
// spinner animation inside StreamComponent.
type streamSpinnerTickMsg struct{}

// streamSpinnerTickCmd returns a tea.Cmd that fires streamSpinnerTickMsg at the
// KITT animation frame rate (14 fps).
func streamSpinnerTickCmd() tea.Cmd {
	return tea.Tick(time.Second/14, func(_ time.Time) tea.Msg {
		return streamSpinnerTickMsg{}
	})
}

// streamPhase tracks what the StreamComponent is currently displaying.
type streamPhase int

const (
	// streamPhaseIdle is the initial state — nothing to display.
	streamPhaseIdle streamPhase = iota

	// streamPhaseSpinner shows the KITT-style animation while waiting for the
	// first streaming chunk or tool event.
	streamPhaseSpinner

	// streamPhaseStreaming shows the live streaming text as chunks arrive.
	streamPhaseStreaming
)

// StreamComponent is the Bubble Tea child model responsible for the stream
// region: it renders a KITT-style spinner when the agent is thinking, and
// switches to live text once StreamChunkEvents start arriving. It also renders
// intermediate tool-call events.
//
// Lifecycle is managed entirely by the parent AppModel:
//   - Parent calls Reset() between agent steps to clear state.
//   - Parent emits completed responses above the BT region via tea.Println()
//     (see AppModel.printCompletedResponse); StreamComponent never calls tea.Quit.
//
// Events handled:
//   - app.SpinnerEvent{Show:true}  → enter spinner phase, start tick loop
//   - app.SpinnerEvent{Show:false} → (unused — first chunk transitions automatically)
//   - app.StreamChunkEvent         → append text, enter streaming phase
//   - app.ToolCallStartedEvent     → record active tool call
//   - app.ToolExecutionEvent       → update tool execution status
//   - app.ToolResultEvent          → append rendered tool result line
//   - app.ToolCallContentEvent     → append assistant commentary text
//   - app.ResponseCompleteEvent    → no-op (parent handles completion)
//   - app.HookBlockedEvent         → append block message line
type StreamComponent struct {
	// phase tracks what we are currently showing.
	phase streamPhase

	// spinnerFrames are the pre-rendered KITT animation frames.
	spinnerFrames []string

	// spinnerFrame is the current frame index.
	spinnerFrame int

	// spinnerMsg is the label shown next to the KITT animation.
	spinnerMsg string

	// streamContent accumulates all streaming text chunks.
	streamContent strings.Builder

	// toolLines are rendered-and-finalized tool event lines appended above the
	// live streaming text.
	toolLines []string

	// activeToolName tracks the tool currently being executed (for status updates).
	activeToolName string

	// messageRenderer renders tool / content lines in standard mode.
	messageRenderer *MessageRenderer

	// compactRenderer renders tool / content lines in compact mode.
	compactRenderer *CompactRenderer

	// compactMode selects which renderer to use.
	compactMode bool

	// modelName is displayed in the streaming text header.
	modelName string

	// timestamp records when the current step started (used for message headers).
	timestamp time.Time

	// width is the current terminal column count.
	width int

	// height constrains the render output to at most this many lines.
	// 0 means unconstrained.
	height int
}

// NewStreamComponent creates a new StreamComponent ready to be embedded in AppModel.
func NewStreamComponent(compactMode bool, width int, modelName string) *StreamComponent {
	if width == 0 {
		width = 80
	}
	return &StreamComponent{
		spinnerFrames:   knightRiderFrames(),
		spinnerMsg:      "Thinking…",
		compactMode:     compactMode,
		modelName:       modelName,
		messageRenderer: NewMessageRenderer(width, false),
		compactRenderer: NewCompactRenderer(width, false),
		width:           width,
	}
}

// SetHeight constrains the stream region render height. When height > 0, the
// render output is clamped to that many lines (trailing lines are discarded).
// A value of 0 means unconstrained.
func (s *StreamComponent) SetHeight(h int) {
	if h < 0 {
		h = 0
	}
	s.height = h
}

// Reset clears all accumulated state so the component is ready for the next
// agent step. Called by AppModel after a step completes or errors.
func (s *StreamComponent) Reset() {
	s.phase = streamPhaseIdle
	s.spinnerFrame = 0
	s.streamContent.Reset()
	s.toolLines = nil
	s.activeToolName = ""
	s.timestamp = time.Time{}
}

// --------------------------------------------------------------------------
// tea.Model interface
// --------------------------------------------------------------------------

// Init implements tea.Model. No startup command needed; SpinnerEvent drives
// the ticker.
func (s *StreamComponent) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Routes app-layer events and internal tick msgs.
func (s *StreamComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		s.width = msg.Width - 4 // match existing padding convention
		s.messageRenderer.SetWidth(s.width)
		s.compactRenderer.SetWidth(s.width)

	case streamSpinnerTickMsg:
		if s.phase == streamPhaseSpinner {
			s.spinnerFrame++
			return s, streamSpinnerTickCmd()
		}
		// Phase changed; let the tick loop die naturally.

	// ── App-layer events ──────────────────────────────────────────────────

	case app.SpinnerEvent:
		if msg.Show {
			if s.phase == streamPhaseIdle {
				s.phase = streamPhaseSpinner
				s.timestamp = time.Now()
				return s, streamSpinnerTickCmd()
			}
		}
		// Show:false is a no-op; the first StreamChunkEvent transitions phase.

	case app.StreamChunkEvent:
		if s.phase != streamPhaseStreaming {
			s.phase = streamPhaseStreaming
			if s.timestamp.IsZero() {
				s.timestamp = time.Now()
			}
		}
		s.streamContent.WriteString(msg.Content)

	case app.ToolCallStartedEvent:
		s.activeToolName = msg.ToolName
		// Render a "starting" tool call line and append it.
		line := s.renderToolCallLine(msg.ToolName, msg.ToolArgs, time.Now())
		s.toolLines = append(s.toolLines, line)

	case app.ToolExecutionEvent:
		if msg.IsStarting {
			s.activeToolName = msg.ToolName
		} else {
			s.activeToolName = ""
		}

	case app.ToolResultEvent:
		line := s.renderToolResultLine(msg.ToolName, msg.ToolArgs, msg.Result, msg.IsError)
		s.toolLines = append(s.toolLines, line)

	case app.ToolCallContentEvent:
		// Assistant commentary that accompanies a tool call — treat as streamed text.
		if s.phase != streamPhaseStreaming {
			s.phase = streamPhaseStreaming
			if s.timestamp.IsZero() {
				s.timestamp = time.Now()
			}
		}
		s.streamContent.WriteString(msg.Content)

	case app.ResponseCompleteEvent:
		// No-op: parent handles completion via StepCompleteEvent.

	case app.HookBlockedEvent:
		// Append a styled notice line.
		line := s.renderHookBlockedLine(msg.Message)
		s.toolLines = append(s.toolLines, line)
	}

	return s, nil
}

// View implements tea.Model. Renders the current stream region content.
func (s *StreamComponent) View() tea.View {
	return tea.NewView(s.render())
}

// --------------------------------------------------------------------------
// Internal rendering
// --------------------------------------------------------------------------

// render builds the full content string for the stream region.
func (s *StreamComponent) render() string {
	var content string
	switch s.phase {
	case streamPhaseIdle:
		return ""

	case streamPhaseSpinner:
		content = s.renderSpinner()

	case streamPhaseStreaming:
		var parts []string

		// Tool event lines rendered above the live text.
		parts = append(parts, s.toolLines...)

		// Live streaming assistant text.
		text := s.streamContent.String()
		if text != "" {
			parts = append(parts, s.renderStreamingText(text))
		}

		// Show active tool status if a tool is still running.
		if s.activeToolName != "" {
			activeLine := s.renderActiveToolLine(s.activeToolName)
			parts = append(parts, activeLine)
		}

		content = strings.Join(parts, "\n")

	default:
		return ""
	}

	// Clamp to height if constrained: keep the last h lines so the most
	// recent output is always visible.
	if s.height > 0 && content != "" {
		lines := strings.Split(content, "\n")
		if len(lines) > s.height {
			lines = lines[len(lines)-s.height:]
			content = strings.Join(lines, "\n")
		}
	}

	return content
}

// renderSpinner renders the KITT-style scanning animation with a message label.
func (s *StreamComponent) renderSpinner() string {
	theme := GetTheme()

	frame := s.spinnerFrames[s.spinnerFrame%len(s.spinnerFrames)]
	msgStyle := lipgloss.NewStyle().
		Foreground(theme.Text).
		Italic(true)

	return "  " + frame + " " + msgStyle.Render(s.spinnerMsg)
}

// renderStreamingText renders the accumulated streaming text as a live assistant
// message using the configured renderer.
func (s *StreamComponent) renderStreamingText(text string) string {
	ts := s.timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	if s.compactMode {
		msg := s.compactRenderer.RenderAssistantMessage(text, ts, s.modelName)
		return msg.Content
	}
	msg := s.messageRenderer.RenderAssistantMessage(text, ts, s.modelName)
	return msg.Content
}

// renderToolCallLine renders a single "tool being called" line.
func (s *StreamComponent) renderToolCallLine(toolName, toolArgs string, ts time.Time) string {
	if s.compactMode {
		msg := s.compactRenderer.RenderToolCallMessage(toolName, toolArgs, ts)
		return msg.Content
	}
	msg := s.messageRenderer.RenderToolCallMessage(toolName, toolArgs, ts)
	return msg.Content
}

// renderToolResultLine renders a single "tool result" line.
func (s *StreamComponent) renderToolResultLine(toolName, toolArgs, result string, isError bool) string {
	if s.compactMode {
		msg := s.compactRenderer.RenderToolMessage(toolName, toolArgs, result, isError)
		return msg.Content
	}
	msg := s.messageRenderer.RenderToolMessage(toolName, toolArgs, result, isError)
	return msg.Content
}

// renderActiveToolLine renders a small inline spinner for a tool still executing.
func (s *StreamComponent) renderActiveToolLine(toolName string) string {
	theme := GetTheme()
	dot := lipgloss.NewStyle().Foreground(theme.Tool).Render("⠋")
	label := lipgloss.NewStyle().Foreground(theme.Tool).Italic(true).Render(toolName + "…")
	return "  " + dot + " " + label
}

// renderHookBlockedLine renders a notice that a hook blocked an action.
func (s *StreamComponent) renderHookBlockedLine(message string) string {
	theme := GetTheme()
	return lipgloss.NewStyle().
		Foreground(theme.Error).
		Bold(true).
		Render("  ⛔ Hook blocked: " + message)
}
