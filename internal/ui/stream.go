package ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/indaco/herald"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/ui/style"
)

// knightRiderFrames generates a KITT-style scanning animation where a bright
// light bounces back and forth across a row of dots with a trailing glow.
// Colors are derived from the active theme. Used by StreamComponent (TUI
// inline spinner) and Spinner (stderr goroutine spinner).
func knightRiderFrames() []string {
	const numDots = 8
	const dot = "▪"

	cs := style.GetCachedStyles()
	bright := cs.SpinnerBright
	med := cs.SpinnerMed
	dim := cs.SpinnerDim
	off := cs.SpinnerOff

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

// streamFlushTickMsg fires when it's time to commit pending chunks to the
// main content builders and trigger a re-render. This coalesces rapid
// streaming chunks into fewer expensive markdown re-renders.
//
// generation ties the tick to the pending flush session that created it so
// stale ticks from a prior Reset() are discarded.
type streamFlushTickMsg struct {
	generation uint64
}

// streamFlushInterval is the coalescing window for stream chunks. Chunks
// arriving within this window are batched into a single render pass.
// 16ms ≈ 60 fps — fast enough to appear smooth, slow enough to coalesce
// bursts from the LLM provider.
const streamFlushInterval = 16 * time.Millisecond

// streamFlushTickCmd returns a tea.Cmd that fires streamFlushTickMsg after
// the coalescing interval.
func streamFlushTickCmd(generation uint64) tea.Cmd {
	return tea.Tick(streamFlushInterval, func(_ time.Time) tea.Msg {
		return streamFlushTickMsg{generation: generation}
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
// are added to the ScrollList by the parent AppModel. The StreamComponent
// only handles the live streaming text and spinner display.
//
// Lifecycle is managed entirely by the parent AppModel:
//   - Parent calls Reset() between agent steps to clear state.
//   - Content is displayed via StreamingMessageItem in the ScrollList.
//   - StreamComponent never calls tea.Quit.
//   - The spinner schedules nothing: it advances when the parent's shared
//     frame clock hands it a beat, and keeps that clock alive by reporting
//     IsSpinning.
//
// Events handled:
//   - app.SpinnerEvent{Show:true}  → raise the spinner
//   - app.StreamChunkEvent         → append text
//   - app.ToolExecutionEvent       → show execution label on spinner
//   - frameTickMsg                 → advance the spinner animation
type StreamComponent struct {
	// phase tracks whether the component is idle or active.
	phase streamPhase

	// spinning is true while the KITT animation is running. It is orthogonal
	// to whether streaming text is present: the spinner remains visible
	// alongside streaming text until Reset(). AppModel reads it, through
	// IsSpinning, to decide whether the shared frame clock should keep running.
	spinning bool

	// spinnerFrames are the pre-rendered KITT animation frames.
	spinnerFrames []string

	// spinnerFrame is the current frame index.
	spinnerFrame int

	// activeTools maps ToolCallID -> descriptor for currently running tools.
	activeTools map[string]activeTool

	// activeToolOrder preserves deterministic display order for active tools.
	activeToolOrder []string

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

	// flushGeneration is incremented when stream state resets so stale flush
	// ticks from a previous step can be discarded.
	flushGeneration uint64

	// thinkingVisible controls whether reasoning blocks are expanded or collapsed.
	thinkingVisible bool

	// reasoningStartTime records when the first reasoning chunk was received.
	reasoningStartTime time.Time

	// reasoningDuration holds the total reasoning time, frozen when streaming text begins.
	reasoningDuration time.Duration

	// renderer renders streaming assistant text.
	renderer Renderer

	// modelName is displayed in the streaming text header.
	modelName string

	// timestamp records when the current step started (used for message headers).
	timestamp time.Time

	// width is the current terminal column count.
	width int

	// height constrains the render output to at most this many lines.
	// 0 means unconstrained.
	height int

	// ty provides typography functions for rendering text.
	ty *herald.Typography
}

// NewStreamComponent creates a new StreamComponent ready to be embedded in AppModel.
func NewStreamComponent(width int, modelName string) *StreamComponent {
	if width == 0 {
		width = 80
	}

	renderer := newMessageRenderer(width, false)

	return &StreamComponent{
		spinnerFrames: knightRiderFrames(),
		modelName:     modelName,
		renderer:      renderer,
		width:         width,
		ty:            createTypography(GetTheme()),
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
	}
}

// Reset clears all accumulated state so the component is ready for the next
// agent step. Called by AppModel after a step completes or errors.
func (s *StreamComponent) Reset() {
	s.phase = streamPhaseIdle
	s.spinning = false
	s.spinnerFrame = 0
	s.activeTools = nil
	s.activeToolOrder = nil
	s.streamContent.Reset()
	s.reasoningContent.Reset()
	s.pendingStream.Reset()
	s.pendingReasoning.Reset()
	s.flushPending = false
	s.flushGeneration++
	s.timestamp = time.Time{}
	s.reasoningStartTime = time.Time{}
	s.reasoningDuration = 0
}

// ConsumeOverflow is a no-op in alt screen mode. Overflow is handled by the
// ScrollList viewport. Retained to satisfy streamComponentIface.
func (s *StreamComponent) ConsumeOverflow() string {
	return ""
}

// GetRenderedContent returns the rendered assistant message from the accumulated
// streaming text. Returns empty string if no text has been accumulated. Used by
// the parent AppModel to flush stream content before resetting.
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
		rendered := s.renderStreamingText(text)
		sections = append(sections, rendered)
	}

	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n")
}

// commitPending moves any pending chunks to the committed content builders.
// Called before reading content for output or on flush tick.
func (s *StreamComponent) commitPending() {
	if s.pendingStream.Len() > 0 {
		s.streamContent.WriteString(s.pendingStream.String())
		s.pendingStream.Reset()
	}
	if s.pendingReasoning.Len() > 0 {
		s.reasoningContent.WriteString(s.pendingReasoning.String())
		s.pendingReasoning.Reset()
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
		if s.renderer != nil {
			s.renderer.SetWidth(s.width)
		}

	case frameTickMsg:
		// One beat of the shared animation clock, subdivided to the spinner's
		// own rate. Nothing is scheduled here: AppModel owns the loop and keeps
		// it alive for as long as IsSpinning reports there is something to
		// animate.
		if s.spinning && msg.due(frameEverySpinner) {
			s.spinnerFrame++
		}
	// ── App-layer events ──────────────────────────────────────────────────

	case app.SpinnerEvent:
		if msg.Show && !s.spinning {
			s.phase = streamPhaseActive
			s.spinning = true
			s.spinnerFrame = 0
			if s.timestamp.IsZero() {
				s.timestamp = time.Now()
			}
		} else if !msg.Show && s.spinning {
			s.spinning = false
		}

	case streamFlushTickMsg:
		if msg.generation != s.flushGeneration {
			break
		}
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
			return s, streamFlushTickCmd(s.flushGeneration)
		}

	case app.ReasoningCompleteEvent:
		// Freeze reasoning duration when reasoning finishes (before text streaming starts).
		if s.reasoningDuration == 0 && !s.reasoningStartTime.IsZero() {
			s.reasoningDuration = time.Since(s.reasoningStartTime)
		}
		// Flush any remaining pending reasoning content.
		if s.pendingReasoning.Len() > 0 {
			s.reasoningContent.WriteString(s.pendingReasoning.String())
			s.pendingReasoning.Reset()
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

		// <think> tag filtering is handled at the agent layer — chunks here
		// are already clean text.
		s.pendingStream.WriteString(msg.Content)

		if !s.flushPending && s.pendingStream.Len() > 0 {
			s.flushPending = true
			return s, streamFlushTickCmd(s.flushGeneration)
		}

	case app.ToolExecutionEvent:
		toolID := msg.ToolCallID
		if toolID == "" {
			// Defensive fallback for older/third-party emitters that may omit
			// ToolCallID. Best-effort only: same-name+args concurrent calls can
			// still collide without a stable ID.
			toolID = fmt.Sprintf("%s|%s", msg.ToolName, msg.ToolArgs)
		}
		if msg.IsStarting {
			if s.activeTools == nil {
				s.activeTools = make(map[string]activeTool)
			}
			if _, exists := s.activeTools[toolID]; !exists {
				s.activeToolOrder = append(s.activeToolOrder, toolID)
			}
			agentName := ""
			if msg.ToolName == "subagent" {
				agentName = extractAgentNameFromArgs(msg.ToolArgs)
			}
			s.activeTools[toolID] = activeTool{
				name:      msg.ToolName,
				args:      msg.ToolArgs,
				agentName: agentName,
			}
			s.spinnerFrame = 0
			if !s.spinning {
				s.phase = streamPhaseActive
				s.spinning = true
			}
		} else {
			if s.activeTools != nil {
				delete(s.activeTools, toolID)
			}
			s.activeToolOrder = removeToolID(s.activeToolOrder, toolID)
		}
	}

	return s, nil
}

// View implements tea.Model. Returns an empty view since rendering is handled
// by StreamingMessageItem in the ScrollList. Retained to satisfy tea.Model.
func (s *StreamComponent) View() tea.View {
	return tea.NewView("")
}

// renderReasoningBlock renders the reasoning/thinking content using blockquote.
// When collapsed, shows the last 10 lines with a truncation hint. When
// expanded, shows all lines. Includes a "Thought for Xs" duration footer.
func (s *StreamComponent) renderReasoningBlock(reasoning string) string {
	lines := strings.Split(strings.TrimRight(reasoning, "\n"), "\n")

	var parts []string

	// When collapsed and content exceeds 10 lines, show only the last 10
	// with a truncation hint.
	const maxCollapsedLines = 10
	if !s.thinkingVisible && len(lines) > maxCollapsedLines {
		hidden := len(lines) - maxCollapsedLines
		parts = append(parts, s.ty.Italic(fmt.Sprintf("... (%d lines hidden)", hidden)))
		lines = lines[len(lines)-maxCollapsedLines:]
	}

	// Main content using Italic with Muted color for visual distinction.
	content := strings.TrimLeft(strings.Join(lines, "\n"), " \t\n")
	// Soft-wrap to the available width so long lines don't get cut off.
	if s.width > 4 {
		content = lipgloss.NewStyle().Width(s.width - 4).Render(content)
	}
	cs := style.GetCachedStyles()
	parts = append(parts, cs.Muted.Render(s.ty.Italic(content)))

	// Duration footer with VeryMuted label and Accent duration.
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
		label := cs.VeryMuted.Render("Thought for ")
		durationStyled := cs.Accent.Render(durationStr)
		parts = append(parts, label+durationStyled)
	}

	// Concatenate parts with newline between blockquote and footer
	var result string
	if len(parts) == 1 {
		result = parts[0]
	} else if len(parts) == 2 {
		result = parts[0] + "\n" + parts[1]
	} else {
		result = strings.Join(parts, "\n")
	}
	return styleMarginBottom1.Render(result)
}

// SetThinkingVisible sets whether reasoning blocks are shown or collapsed.
func (s *StreamComponent) SetThinkingVisible(visible bool) {
	if s.thinkingVisible != visible {
		s.thinkingVisible = visible
	}
}

// HasReasoning returns true if any reasoning content has been accumulated
// (committed or pending).
func (s *StreamComponent) HasReasoning() bool {
	return s.reasoningContent.Len() > 0 || s.pendingReasoning.Len() > 0
}

// ActivityDot returns just the animated indicator glyph, without any label.
// Returns "" when the spinner is not running.
func (s *StreamComponent) ActivityDot() string {
	if !s.spinning {
		return ""
	}
	return s.spinnerFrames[s.spinnerFrame%len(s.spinnerFrames)]
}

// ActivityPhrase returns a present-tense description of what the agent is
// currently doing, for the activity row. Concurrent tool calls collapse to a
// count so the row stays a single line; a turn with no tool in flight reports
// that the model is thinking.
func (s *StreamComponent) ActivityPhrase() string {
	tools := s.activeToolList()
	switch len(tools) {
	case 0:
		return "Thinking"
	case 1:
		t := tools[0]
		if t.name == "subagent" && t.agentName != "" {
			return "Delegating to " + t.agentName
		}
		return activityVerb(t.name, t.args)
	default:
		return fmt.Sprintf("Running %d tools", len(tools))
	}
}

// IsSpinning reports whether the stream is currently animating, i.e. whether
// there is live activity worth showing.
func (s *StreamComponent) IsSpinning() bool {
	return s.spinning
}

// renderStreamingText renders the accumulated streaming text as a live assistant
// message using the configured renderer.
func (s *StreamComponent) renderStreamingText(text string) string {
	ts := s.timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	if s.renderer == nil {
		return text
	}
	msg := s.renderer.RenderAssistantMessage(text, ts, s.modelName)
	return msg.Content
}

// activeTool describes a tool call that is currently in flight. The raw
// arguments are retained so the activity row can render a specific,
// present-tense phrase ("Reading internal/ui/model.go") rather than a bare
// tool name.
type activeTool struct {
	name      string
	args      string
	agentName string
}

func (s *StreamComponent) activeToolList() []activeTool {
	if len(s.activeTools) == 0 {
		return nil
	}
	out := make([]activeTool, 0, len(s.activeToolOrder))
	for _, id := range s.activeToolOrder {
		if t, ok := s.activeTools[id]; ok {
			out = append(out, t)
		}
	}
	return out
}

// removeToolID removes the first occurrence of a tool ID from a slice.
func removeToolID(ids []string, id string) []string {
	for i, v := range ids {
		if v == id {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

// agentNamePattern validates agent names: only alphanumeric, hyphens, underscores, and dots.
// Compiled once at package init time for reuse across function calls.
var agentNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// sanitizeAgentName validates the agent name for safe display in the spinner.
// Returns the sanitized agent name or empty string if invalid.
// Enforces: maximum 50 characters and only alphanumeric, hyphens, underscores, and dots.
// The regex pattern inherently rejects all control characters, newlines, and ANSI escapes.
func sanitizeAgentName(agent string) string {
	if agent == "" {
		return ""
	}

	const maxLen = 50

	// Enforce length limit (efficient early return)
	if len(agent) > maxLen {
		return ""
	}

	// Validate character set: only alphanumeric, hyphens, underscores, and dots.
	// The regex inherently rejects control characters, newlines, and ANSI escapes.
	if !agentNamePattern.MatchString(agent) {
		return ""
	}

	return agent
}

// extractAgentNameFromArgs parses the agent field from subagent ToolArgs JSON.
// Returns empty string if the agent field is not found, JSON parsing fails, or the agent name is invalid.
// The returned agent name is sanitized for safe spinner rendering.
func extractAgentNameFromArgs(toolArgs string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
		return ""
	}
	if agent, ok := args["agent"].(string); ok {
		return sanitizeAgentName(agent)
	}
	return ""
}

// UpdateTheme refreshes the component's typography instance and spinner
// animation frames with colors from the current theme. This is called when
// the user changes themes via /theme.
func (s *StreamComponent) UpdateTheme() {
	s.ty = createTypography(GetTheme())
	s.spinnerFrames = knightRiderFrames()
}
