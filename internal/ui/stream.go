package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mark3labs/mcphost/internal/app"
)

// knightRiderFrames generates a KITT-style scanning animation where a bright
// light bounces back and forth across a row of dots with a trailing glow.
// Colors are derived from the active theme. Used by StreamComponent (TUI
// inline spinner) and Spinner (stderr goroutine spinner).
func knightRiderFrames() []string {
	const numDots = 8
	const dot = "▪"

	theme := GetTheme()

	bright := lipgloss.NewStyle().Foreground(theme.Primary)
	med := lipgloss.NewStyle().Foreground(theme.Muted)
	dim := lipgloss.NewStyle().Foreground(theme.VeryMuted)
	off := lipgloss.NewStyle().Foreground(theme.MutedBorder)

	// Scanner bounces: 0→7→0
	positions := make([]int, 0, 2*numDots-2)
	for i := range numDots {
		positions = append(positions, i)
	}
	for i := numDots - 2; i > 0; i-- {
		positions = append(positions, i)
	}

	frames := make([]string, len(positions))
	for f, pos := range positions {
		var b strings.Builder
		for i := range numDots {
			d := pos - i
			if d < 0 {
				d = -d
			}
			switch d {
			case 0:
				b.WriteString(bright.Render(dot))
			case 1:
				b.WriteString(med.Render(dot))
			case 2:
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

	// streamPhaseActive means content is being displayed (streaming text
	// and/or spinner animation).
	streamPhaseActive
)

// StreamComponent is the Bubble Tea child model responsible for the stream
// region: it renders a KITT-style spinner when the agent is working, and
// displays live text as StreamChunkEvents arrive. The spinner remains visible
// alongside streaming text until the step completes and Reset() is called.
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
//   - app.SpinnerEvent{Show:true}  → start spinner tick loop
//   - app.StreamChunkEvent         → append text
//   - app.ToolExecutionEvent       → show execution label on spinner
type StreamComponent struct {
	// phase tracks whether the component is idle or active.
	phase streamPhase

	// spinning is true while the KITT animation tick loop is running.
	// It is orthogonal to whether streaming text is present: the spinner
	// remains visible alongside streaming text until Reset().
	spinning bool

	// spinnerFrames are the pre-rendered KITT animation frames.
	spinnerFrames []string

	// spinnerFrame is the current frame index.
	spinnerFrame int

	// spinnerMsg is the label shown next to the KITT animation (e.g.
	// "Executing tool_name…"). Empty string means no label.
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
	s.spinning = false
	s.spinnerFrame = 0
	s.spinnerMsg = ""
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
		if s.spinning {
			s.spinnerFrame++
			return s, streamSpinnerTickCmd()
		}
		// Spinning stopped; let the tick loop die naturally.

	// ── App-layer events ──────────────────────────────────────────────────

	case app.SpinnerEvent:
		if msg.Show && !s.spinning {
			s.phase = streamPhaseActive
			s.spinning = true
			s.spinnerFrame = 0
			if s.timestamp.IsZero() {
				s.timestamp = time.Now()
			}
			return s, streamSpinnerTickCmd()
		}

	case app.StreamChunkEvent:
		s.phase = streamPhaseActive
		if s.timestamp.IsZero() {
			s.timestamp = time.Now()
		}
		s.streamContent.WriteString(msg.Content)

	case app.ToolExecutionEvent:
		if msg.IsStarting {
			// Show the tool name on the spinner while the tool executes.
			s.spinnerMsg = "Executing " + msg.ToolName + "…"
			s.spinnerFrame = 0
			if !s.spinning {
				s.phase = streamPhaseActive
				s.spinning = true
				return s, streamSpinnerTickCmd()
			}
		} else {
			// Tool finished — clear execution label but keep spinning.
			s.spinnerMsg = ""
		}
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
	if s.phase == streamPhaseIdle {
		return ""
	}

	var parts []string

	// Render streaming text if present.
	if text := s.streamContent.String(); text != "" {
		parts = append(parts, s.renderStreamingText(text))
	}

	// Render spinner below streaming text (or alone if no text yet).
	if s.spinning {
		parts = append(parts, s.renderSpinner())
	}

	if len(parts) == 0 {
		return ""
	}

	content := strings.Join(parts, "\n")

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

// renderSpinner renders the KITT-style scanning animation with an optional label.
func (s *StreamComponent) renderSpinner() string {
	frame := s.spinnerFrames[s.spinnerFrame%len(s.spinnerFrames)]
	if s.spinnerMsg == "" {
		return "  " + frame
	}
	theme := GetTheme()
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
