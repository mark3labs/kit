package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mark3labs/kit/internal/app"
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
// spinner animation inside StreamComponent. The generation field ties each tick
// to the spinner session that created it so that stale ticks from a previous
// start/stop cycle are silently discarded instead of creating a second
// concurrent tick loop (which doubles the animation speed).
type streamSpinnerTickMsg struct {
	generation uint64
}

// streamSpinnerTickCmd returns a tea.Cmd that fires streamSpinnerTickMsg at the
// KITT animation frame rate (14 fps). The generation parameter is embedded in
// the message so the receiver can verify it matches the current spinner session.
func streamSpinnerTickCmd(generation uint64) tea.Cmd {
	return tea.Tick(time.Second/14, func(_ time.Time) tea.Msg {
		return streamSpinnerTickMsg{generation: generation}
	})
}

// streamFlushTickMsg fires when it's time to commit pending chunks to the
// main content builders and trigger a re-render. This coalesces rapid
// streaming chunks into fewer expensive markdown re-renders.
type streamFlushTickMsg struct{}

// streamFlushInterval is the coalescing window for stream chunks. Chunks
// arriving within this window are batched into a single render pass.
// 16ms ≈ 60 fps — fast enough to appear smooth, slow enough to coalesce
// bursts from the LLM provider.
const streamFlushInterval = 16 * time.Millisecond

// streamFlushTickCmd returns a tea.Cmd that fires streamFlushTickMsg after
// the coalescing interval.
func streamFlushTickCmd() tea.Cmd {
	return tea.Tick(streamFlushInterval, func(_ time.Time) tea.Msg {
		return streamFlushTickMsg{}
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

	// spinnerGeneration is incremented each time a new spinner tick loop
	// is started. Tick messages carry the generation they were created for;
	// if a tick's generation doesn't match the current one, it is a stale
	// tick from a previous start/stop cycle and is silently discarded.
	// This prevents multiple concurrent tick loops from accumulating when
	// the spinner is rapidly stopped and restarted (e.g. SpinnerEvent
	// hide → ToolExecutionEvent start before the old tick fires).
	spinnerGeneration uint64

	// spinnerFrames are the pre-rendered KITT animation frames.
	spinnerFrames []string

	// spinnerFrame is the current frame index.
	spinnerFrame int

	// activeTools tracks the names of tools currently executing in parallel.
	// When multiple tools run concurrently, all are displayed in the spinner.
	activeTools []string

	// streamContent holds committed streaming text (flushed from pending).
	streamContent strings.Builder

	// reasoningContent holds committed reasoning text (flushed from pending).
	reasoningContent strings.Builder

	// pendingStream accumulates streaming text chunks between flush ticks.
	// Chunks are written here immediately on arrival, then moved to
	// streamContent when the flush tick fires.
	pendingStream strings.Builder

	// pendingReasoning accumulates reasoning chunks between flush ticks.
	pendingReasoning strings.Builder

	// flushPending is true while a flush tick is in-flight. Prevents
	// scheduling duplicate ticks when multiple chunks arrive within
	// the same coalescing window.
	flushPending bool

	// renderCache holds the last rendered output string. Reused by View()
	// between flush ticks to avoid redundant markdown re-parsing.
	renderCache string

	// renderDirty is true when committed content has changed since the
	// last render. Set on flush tick; cleared after render() rebuilds
	// the cache.
	renderDirty bool

	// thinkingVisible controls whether reasoning blocks are expanded or collapsed.
	thinkingVisible bool

	// reasoningStartTime records when the first reasoning chunk was received.
	reasoningStartTime time.Time

	// reasoningDuration holds the total reasoning time, frozen when streaming text begins.
	reasoningDuration time.Duration

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
		messageRenderer: newMessageRenderer(width, false),
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
	if s.height != h {
		s.height = h
		// Invalidate cache — height clamp affects output.
		s.renderCache = ""
		s.renderDirty = true
	}
}

// Reset clears all accumulated state so the component is ready for the next
// agent step. Called by AppModel after a step completes or errors.
func (s *StreamComponent) Reset() {
	s.phase = streamPhaseIdle
	s.spinning = false
	s.spinnerGeneration++ // invalidate any in-flight tick commands
	s.spinnerFrame = 0
	s.activeTools = nil
	s.streamContent.Reset()
	s.reasoningContent.Reset()
	s.pendingStream.Reset()
	s.pendingReasoning.Reset()
	s.flushPending = false
	s.renderCache = ""
	s.renderDirty = false
	s.timestamp = time.Time{}
	s.reasoningStartTime = time.Time{}
	s.reasoningDuration = 0
}

// GetRenderedContent returns the rendered assistant message from the accumulated
// streaming text. Returns empty string if no text has been accumulated. Used by
// the parent AppModel to flush content via tea.Println() before resetting.
//
// This commits any pending chunks first so the output includes all received
// content, not just what has been flushed by the tick.
func (s *StreamComponent) GetRenderedContent() string {
	// Commit any pending chunks so the final output is complete.
	s.commitPending()

	var sections []string

	// Include rendered reasoning block if present.
	if reasoning := s.reasoningContent.String(); reasoning != "" {
		sections = append(sections, s.renderReasoningBlock(reasoning))
	}

	text := s.streamContent.String()
	if text != "" {
		sections = append(sections, s.renderStreamingText(text))
	}

	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n")
}

// commitPending moves any pending chunks to the committed content builders.
// Called before reading content for scrollback output or on flush tick.
func (s *StreamComponent) commitPending() {
	if s.pendingStream.Len() > 0 {
		s.streamContent.WriteString(s.pendingStream.String())
		s.pendingStream.Reset()
		s.renderDirty = true
	}
	if s.pendingReasoning.Len() > 0 {
		s.reasoningContent.WriteString(s.pendingReasoning.String())
		s.pendingReasoning.Reset()
		s.renderDirty = true
	}
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
		s.width = msg.Width
		s.messageRenderer.SetWidth(s.width)
		s.compactRenderer.SetWidth(s.width)
		// Invalidate render cache — width change affects wrapping/styling.
		s.renderCache = ""
		s.renderDirty = true

	case streamSpinnerTickMsg:
		// Only continue the tick loop if this tick belongs to the current
		// spinner session. Stale ticks from a previous start/stop cycle
		// are silently dropped, preventing duplicate concurrent tick loops
		// that would double (or worse) the animation speed.
		if s.spinning && msg.generation == s.spinnerGeneration {
			s.spinnerFrame++
			return s, streamSpinnerTickCmd(s.spinnerGeneration)
		}
		// Spinning stopped or generation mismatch; let the tick loop die.

	// ── App-layer events ──────────────────────────────────────────────────

	case app.SpinnerEvent:
		if msg.Show && !s.spinning {
			s.phase = streamPhaseActive
			s.spinning = true
			s.spinnerGeneration++ // new session; invalidate any stale ticks
			s.spinnerFrame = 0
			if s.timestamp.IsZero() {
				s.timestamp = time.Now()
			}
			return s, streamSpinnerTickCmd(s.spinnerGeneration)
		} else if !msg.Show && s.spinning {
			s.spinning = false
			// Bump generation so any in-flight tick from this session is
			// discarded if spinning is restarted before it fires.
			s.spinnerGeneration++
		}

	case streamFlushTickMsg:
		s.flushPending = false
		s.commitPending()

	case app.ReasoningChunkEvent:
		s.phase = streamPhaseActive
		if s.timestamp.IsZero() {
			s.timestamp = time.Now()
		}
		if s.reasoningStartTime.IsZero() {
			s.reasoningStartTime = time.Now()
		}
		s.pendingReasoning.WriteString(msg.Delta)
		if !s.flushPending {
			s.flushPending = true
			return s, streamFlushTickCmd()
		}

	case app.StreamChunkEvent:
		s.phase = streamPhaseActive
		if s.timestamp.IsZero() {
			s.timestamp = time.Now()
		}
		// Freeze reasoning duration on transition from reasoning to streaming.
		if s.reasoningDuration == 0 && !s.reasoningStartTime.IsZero() {
			s.reasoningDuration = time.Since(s.reasoningStartTime)
		}
		s.pendingStream.WriteString(msg.Content)
		if !s.flushPending {
			s.flushPending = true
			return s, streamFlushTickCmd()
		}

	case app.ToolExecutionEvent:
		if msg.IsStarting {
			// Add tool to active list for parallel execution display.
			toolDisplay := formatToolExecutionMessage(msg.ToolName, msg.ToolArgs)
			s.activeTools = append(s.activeTools, toolDisplay)
			s.spinnerFrame = 0
			if !s.spinning {
				s.phase = streamPhaseActive
				s.spinning = true
				s.spinnerGeneration++ // new session; invalidate stale ticks
				return s, streamSpinnerTickCmd(s.spinnerGeneration)
			}
		} else {
			// Tool finished — remove from active list but keep spinning if others remain.
			toolDisplay := formatToolExecutionMessage(msg.ToolName, msg.ToolArgs)
			s.activeTools = removeFromSlice(s.activeTools, toolDisplay)
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

// render builds the full content string for the stream region. Uses a render
// cache to avoid redundant markdown re-parsing between flush ticks. The cache
// is invalidated when committed content changes (flush tick), terminal width
// changes, or height/thinking visibility changes.
func (s *StreamComponent) render() string {
	if s.phase == streamPhaseIdle {
		return ""
	}

	// Return cached render if committed content hasn't changed.
	if !s.renderDirty {
		return s.renderCache
	}

	var sections []string

	// Render reasoning/thinking block above the main text if present.
	if reasoning := s.reasoningContent.String(); reasoning != "" {
		sections = append(sections, s.renderReasoningBlock(reasoning))
	}

	// Render streaming text only. The spinner is rendered in the status bar
	// by the parent so it never changes the stream region height.
	text := s.streamContent.String()
	if text != "" {
		sections = append(sections, s.renderStreamingText(text))
	}

	if len(sections) == 0 {
		s.renderCache = ""
		s.renderDirty = false
		return ""
	}

	content := strings.Join(sections, "\n")

	// Clamp to height if constrained: keep the last h lines so the most
	// recent output is always visible.
	if s.height > 0 && content != "" {
		lines := strings.Split(content, "\n")
		if len(lines) > s.height {
			lines = lines[len(lines)-s.height:]
			content = strings.Join(lines, "\n")
		}
	}

	s.renderCache = content
	s.renderDirty = false
	return content
}

// renderReasoningBlock renders the reasoning/thinking content in a surface-tinted
// box. When collapsed, shows the last 10 lines with a truncation hint. When
// expanded, shows all lines. Includes a "Thought for Xs" duration footer.
func (s *StreamComponent) renderReasoningBlock(reasoning string) string {
	theme := GetTheme()
	maxWidth := max(s.width-4, 20)

	lines := strings.Split(strings.TrimRight(reasoning, "\n"), "\n")

	contentStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Background(theme.MutedBorder).
		Italic(true)

	var parts []string

	// When collapsed and content exceeds 10 lines, show only the last 10
	// with a truncation hint (matching iteratr's thinking block pattern).
	const maxCollapsedLines = 10
	if !s.thinkingVisible && len(lines) > maxCollapsedLines {
		hidden := len(lines) - maxCollapsedLines
		hintStyle := lipgloss.NewStyle().
			Foreground(theme.VeryMuted).
			Background(theme.MutedBorder).
			Italic(true)
		parts = append(parts, hintStyle.Render(fmt.Sprintf("... (%d lines hidden)", hidden)))
		lines = lines[len(lines)-maxCollapsedLines:]
	}

	// Render reasoning text.
	parts = append(parts, contentStyle.Width(maxWidth).Render(strings.Join(lines, "\n")))

	// Duration footer.
	var duration time.Duration
	if s.reasoningDuration > 0 {
		duration = s.reasoningDuration
	} else if !s.reasoningStartTime.IsZero() {
		duration = time.Since(s.reasoningStartTime)
	}
	if duration > 0 {
		var durationStr string
		if duration < time.Second {
			durationStr = fmt.Sprintf("%dms", duration.Milliseconds())
		} else {
			durationStr = fmt.Sprintf("%.1fs", duration.Seconds())
		}
		footer := lipgloss.NewStyle().Foreground(theme.VeryMuted).Background(theme.MutedBorder).Render("Thought for ") +
			lipgloss.NewStyle().Foreground(theme.Info).Background(theme.MutedBorder).Render(durationStr)
		parts = append(parts, footer)
	}

	innerContent := strings.Join(parts, "\n")

	// Wrap in box with surface background for visual distinction.
	boxStyle := lipgloss.NewStyle().
		Background(theme.MutedBorder). // Surface0 (#313244)
		PaddingLeft(1).
		Width(maxWidth + 2).
		MarginBottom(1)

	return boxStyle.Render(innerContent)
}

// SetThinkingVisible sets whether reasoning blocks are shown or collapsed.
func (s *StreamComponent) SetThinkingVisible(visible bool) {
	if s.thinkingVisible != visible {
		s.thinkingVisible = visible
		// Invalidate cache — thinking visibility affects rendered output.
		s.renderCache = ""
		s.renderDirty = true
	}
}

// HasReasoning returns true if any reasoning content has been accumulated
// (committed or pending).
func (s *StreamComponent) HasReasoning() bool {
	return s.reasoningContent.Len() > 0 || s.pendingReasoning.Len() > 0
}

// SpinnerView returns the rendered spinner line for the parent to embed in the
// status bar. Returns "" when the spinner is not active.
func (s *StreamComponent) SpinnerView() string {
	if !s.spinning {
		return ""
	}
	frame := s.spinnerFrames[s.spinnerFrame%len(s.spinnerFrames)]
	if len(s.activeTools) == 0 {
		return "  " + frame
	}
	theme := GetTheme()
	msgStyle := lipgloss.NewStyle().
		Foreground(theme.Text).
		Italic(true)

	// Format active tools list
	var toolsMsg string
	if len(s.activeTools) == 1 {
		toolsMsg = s.activeTools[0]
	} else {
		toolsMsg = "Running: " + strings.Join(s.activeTools, ", ")
	}
	return "  " + frame + " " + msgStyle.Render(toolsMsg)
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

// removeFromSlice removes the first occurrence of a string from a slice.
func removeFromSlice(slice []string, s string) []string {
	for i, v := range slice {
		if v == s {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// formatToolExecutionMessage creates a descriptive spinner message for tool execution.
// For spawn_subagent, it shows simply as "Subagent" with optional task preview.
func formatToolExecutionMessage(toolName, toolArgs string) string {
	if toolName == "spawn_subagent" {
		return "Subagent"
	}
	return toolName
}
