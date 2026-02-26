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
// switches to live text once StreamChunkEvents start arriving.
//
// Tool calls, tool results, user messages, and other non-streaming content
// are printed immediately by the parent AppModel via tea.Println(). The
// StreamComponent only handles the live streaming text and spinner display.
//
// Lifecycle is managed entirely by the parent AppModel:
//   - Parent calls Reset() between agent steps to clear state.
//   - Parent emits completed responses above the BT region via tea.Println()
//     then calls Reset(); StreamComponent never calls tea.Quit.
//
// Events handled:
//   - app.SpinnerEvent{Show:true}  → enter spinner phase, start tick loop
//   - app.SpinnerEvent{Show:false} → (unused — first chunk transitions automatically)
//   - app.StreamChunkEvent         → append text, enter streaming phase
//   - app.ToolExecutionEvent       → show execution spinner during tool run
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

	// messageRenderer renders assistant messages in standard mode.
	messageRenderer *MessageRenderer

	// compactRenderer renders assistant messages in compact mode.
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
	s.spinnerMsg = "Thinking…"
	s.streamContent.Reset()
	s.timestamp = time.Time{}
}

// GetRenderedContent returns the rendered assistant message from the accumulated
// streaming text. Returns empty string if no text has been accumulated. Used by
// the parent AppModel to flush content via tea.Println() before resetting.
func (s *StreamComponent) GetRenderedContent() string {
	text := s.streamContent.String()
	if text == "" {
		return ""
	}
	return s.renderStreamingText(text)
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

	case app.ToolExecutionEvent:
		if msg.IsStarting {
			// Show a KITT spinner with the tool name while the tool executes.
			s.phase = streamPhaseSpinner
			s.spinnerMsg = "Executing " + msg.ToolName + "…"
			s.spinnerFrame = 0
			return s, streamSpinnerTickCmd()
		}
		// Tool finished — go idle. Parent will trigger a new spinner for
		// the next LLM call if needed.
		s.phase = streamPhaseIdle
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
		// Live streaming assistant text only. Tool calls, results, and
		// other messages are printed immediately by the parent via tea.Println.
		text := s.streamContent.String()
		if text != "" {
			content = s.renderStreamingText(text)
		}

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
