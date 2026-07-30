package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/ui/render"
	"github.com/mark3labs/kit/internal/ui/style"
)

// --------------------------------------------------------------------------
// MessageItem implementations for ScrollList
// --------------------------------------------------------------------------

// themeStamp records the theme generation a memoized render was produced at.
//
// Scrollback items cache their styled output because re-rendering a block on
// every frame is expensive. That cache holds baked-in ANSI color codes, so it
// outlives the theme it was built for unless something invalidates it. Any
// item that memoizes styled output embeds a themeStamp and consults stale()
// alongside its own cache-validity checks; a theme switch then costs one
// re-render per item, taken lazily when the item is next drawn.
type themeStamp struct {
	gen uint64
}

// stale reports whether the theme changed since the stamped render.
func (t *themeStamp) stale() bool {
	return t.gen != style.ThemeGeneration()
}

// stamp marks the cached render as current for the active theme.
func (t *themeStamp) stamp() {
	t.gen = style.ThemeGeneration()
}

// TextMessageItem represents a completed text message (user or assistant)
// in the scrollback.
//
// Items are created in one of two modes:
//
//   - Themed (NewThemedMessageItem): the item keeps the closure that produced
//     its styled content and re-runs it after a theme change, so historical
//     messages recolor along with the rest of the UI.
//   - Static (NewStyledMessageItem): the item holds a fixed string. This is
//     for content that is either theme-independent or already composed by a
//     caller that owns its own colors.
type TextMessageItem struct {
	id        string
	role      string // "user" or "assistant"
	content   string // Raw content (for the message inspector and re-rendering)
	timestamp time.Time

	// render regenerates styled content from the active theme. nil for static
	// items, whose rendered content is fixed at construction.
	render func() string

	// toolCall holds the structured parts of a tool call when this item
	// displays one. The inspector needs them to re-render the body at full
	// length; the flattened raw content cannot be taken apart again.
	toolCall *ToolCallInfo

	// rendered is the memoized styled content, valid for theme generation
	// themeStamp.gen when render is non-nil.
	rendered string
	themeStamp
}

// ToolCallInfo holds the structured parts of a tool call, as handed to the
// tool-body renderers.
type ToolCallInfo struct {
	Name    string
	Args    string
	Result  string
	IsError bool
}

// ToolInspectable is implemented by scrollback items that retain the structured
// parts of the tool call they display.
//
// The scrollback caps each tool body at a per-tool line limit so one large
// result cannot bury the rest of the transcript. The inspector opens precisely
// because something was capped, so it re-renders the body with the caps lifted
// rather than falling back to plain text and discarding the diff colouring,
// line gutters and panel fills. Reconstructing the call from the item's
// flattened raw content is not possible, so it is kept alongside.
type ToolInspectable interface {
	// ToolCall returns the call's parts, and false for items that do not
	// display a tool call.
	ToolCall() (ToolCallInfo, bool)
}

// NewStyledMessageItem creates a message item from fixed, pre-rendered content.
//
// The content will not follow subsequent theme changes. Use this only for
// output that is theme-independent or whose colors the caller owns
// deliberately; prefer NewThemedMessageItem for anything drawn from the theme
// palette.
func NewStyledMessageItem(id string, role string, rawContent string, preRendered string) *TextMessageItem {
	return &TextMessageItem{
		id:        id,
		role:      role,
		content:   rawContent,
		rendered:  preRendered,
		timestamp: time.Now(),
	}
}

// NewThemedMessageItem creates a message item that re-renders itself after a
// theme change. render must be a pure function of the raw content and the
// active theme — it is called lazily, once per theme generation, whenever the
// item is next drawn.
func NewThemedMessageItem(id string, role string, rawContent string, render func() string) *TextMessageItem {
	return &TextMessageItem{
		id:        id,
		role:      role,
		content:   rawContent,
		render:    render,
		timestamp: time.Now(),
	}
}

func (m *TextMessageItem) ID() string {
	return m.id
}

// RawContent returns the original, untruncated source text for this message.
// The scrollback stores a display-oriented (styled and possibly truncated)
// rendering; RawContent is what the message inspector shows so the user can
// read content that was elided for display.
func (m *TextMessageItem) RawContent() string {
	if m.content != "" {
		return m.content
	}
	return m.rendered
}

// Role returns the message role ("user", "assistant", "tool", ...).
func (m *TextMessageItem) Role() string {
	return m.role
}

// WithToolCall attaches the structured parts of the tool call this item
// displays, so the inspector can re-render its body uncapped. Returns the item
// for chaining at the construction site.
func (m *TextMessageItem) WithToolCall(info ToolCallInfo) *TextMessageItem {
	m.toolCall = &info
	return m
}

// ToolCall implements ToolInspectable.
func (m *TextMessageItem) ToolCall() (ToolCallInfo, bool) {
	if m.toolCall == nil {
		return ToolCallInfo{}, false
	}
	return *m.toolCall, true
}

// Invalidate discards the memoized render so the next draw re-runs the render
// closure. Use it when the closure's inputs changed rather than the theme —
// an animation frame, for instance. No-op for static items.
func (m *TextMessageItem) Invalidate() {
	m.gen = 0 // never equal to a live generation, which starts at 1
}

func (m *TextMessageItem) Render(width int) string {
	// Themed items re-render once per theme generation. Static items
	// (render == nil) keep whatever they were constructed with.
	if m.render != nil && m.stale() {
		m.rendered = m.render()
		m.stamp()
	}

	if m.rendered != "" {
		return m.rendered
	}

	// Fallback to simple formatting if there is no rendered content.
	return m.renderContent(width)
}

func (m *TextMessageItem) Height() int {
	rendered := m.Render(0) // Width only matters for the bare fallback path
	if rendered == "" {
		return 0
	}
	return strings.Count(rendered, "\n") + 1
}

// renderContent is the fallback used when no pre-rendered content exists —
// for instance when a block renderer returns empty for whitespace-only input.
// It honours the same left-edge contract as every other block: a gutter glyph
// in column 0 for user messages, text at ContentOffset.
func (m *TextMessageItem) renderContent(width int) string {
	var parts []string

	gutter := ""
	if m.role == "user" {
		gutter = style.GutterGlyph
	}

	// lipgloss wraps by display width and is ANSI-aware; wrapping by byte
	// length would break multi-byte text and any styled content.
	wrapped := lipgloss.NewStyle().Width(style.ContentWidth(width)).Render(m.content)
	for line := range strings.SplitSeq(strings.TrimRight(wrapped, "\n"), "\n") {
		if gutter != "" {
			parts = append(parts, gutter+" "+line)
			continue
		}
		parts = append(parts, strings.Repeat(" ", style.ContentOffset)+line)
	}

	return strings.Join(parts, "\n")
}

// --------------------------------------------------------------------------
// StreamingMessageItem - Live streaming assistant/reasoning text
// --------------------------------------------------------------------------

// StreamingMessageItem represents actively streaming assistant or reasoning text.
// It accumulates content chunks and re-renders on each update for live display.
type StreamingMessageItem struct {
	id            string
	role          string          // "assistant" or "reasoning"
	content       strings.Builder // Accumulated streaming content
	timestamp     time.Time
	startTime     time.Time // When streaming started (for live duration counter)
	modelName     string
	streaming     bool          // true while actively streaming
	finalDuration time.Duration // Frozen duration when complete
	cachedRender  string
	cachedWidth   int

	// reasoningContent caches the expensive styled/wrapped content portion
	// of a reasoning block. While streaming, the live duration label changes
	// every frame but the content only changes when a chunk arrives — so the
	// content render is cached separately and composed with a fresh label.
	reasoningContent      string
	reasoningContentWidth int

	// Both caches above hold theme-colored output, so a theme change has to
	// invalidate them even though neither the content nor the width moved.
	themeStamp
}

// NewStreamingMessageItem creates a new streaming message item.
func NewStreamingMessageItem(id, role string, modelName string) *StreamingMessageItem {
	now := time.Now()
	return &StreamingMessageItem{
		id:                    id,
		role:                  role,
		timestamp:             now,
		startTime:             now,
		modelName:             modelName,
		streaming:             true,
		cachedWidth:           -1,
		reasoningContentWidth: -1,
	}
}

// ID returns the unique identifier.
func (s *StreamingMessageItem) ID() string {
	return s.id
}

// RawContent returns the accumulated streaming text without styling.
func (s *StreamingMessageItem) RawContent() string {
	return s.content.String()
}

// Role returns the message role ("assistant" or "reasoning").
func (s *StreamingMessageItem) Role() string {
	return s.role
}

// Render renders the streaming message with live content.
func (s *StreamingMessageItem) Render(width int) string {
	// A theme change invalidates every cached render below: they all hold
	// baked-in color codes from the theme they were produced under.
	if s.stale() {
		s.cachedRender = ""
		s.cachedWidth = -1
		s.reasoningContentWidth = -1
		s.stamp()
	}

	// Serve from cache when valid. Reasoning blocks are only cached once
	// complete (frozen duration); assistant blocks cache immediately.
	if s.cachedWidth == width && s.cachedRender != "" {
		return s.cachedRender
	}

	var rendered string
	if s.role == "reasoning" {
		// Calculate duration in milliseconds for render.ReasoningBlockFromContent
		var durationMs int64
		if s.finalDuration > 0 {
			durationMs = s.finalDuration.Milliseconds()
		} else if !s.startTime.IsZero() {
			durationMs = time.Since(s.startTime).Milliseconds()
		}
		// The styled/wrapped content is cached separately from the live
		// duration label: only the label changes per frame while streaming,
		// so the expensive part renders once per chunk instead of per frame.
		if s.reasoningContentWidth != width {
			s.reasoningContent = render.ReasoningContent(
				s.content.String(), width, createTypography(style.GetTheme()))
			s.reasoningContentWidth = width
		}
		rendered = render.ReasoningBlockFromContent(s.reasoningContent, durationMs, style.GetTheme())
	} else {
		// Render as assistant message
		rendered = render.AssistantBlock(s.content.String(), width, style.GetTheme())
	}

	// Cache the full render. A streaming reasoning block needs its live
	// duration label re-rendered every frame, so it is only cached once
	// MarkComplete freezes the duration.
	if s.role != "reasoning" || !s.streaming {
		s.cachedRender = rendered
		s.cachedWidth = width
	}
	return rendered
}

// Height returns the number of lines.
func (s *StreamingMessageItem) Height() int {
	// For actively streaming reasoning blocks, cachedRender is not populated
	// (the live duration label changes per frame). Fall back to Render(0)
	// so callers always get the correct height.
	rendered := s.cachedRender
	if rendered == "" {
		rendered = s.Render(0)
	}
	if rendered == "" {
		return 0
	}
	return strings.Count(rendered, "\n") + 1
}

// AppendChunk adds a content chunk and invalidates the render cache.
func (s *StreamingMessageItem) AppendChunk(chunk string) {
	s.content.WriteString(chunk)
	s.cachedRender = ""
	s.cachedWidth = -1 // Invalidate cache (0 is a legitimate width from Height())
	s.reasoningContentWidth = -1
}

// MarkComplete marks the streaming message as complete and freezes the duration.
func (s *StreamingMessageItem) MarkComplete() {
	s.streaming = false
	// Freeze the duration for reasoning blocks
	if s.role == "reasoning" && !s.startTime.IsZero() {
		s.finalDuration = time.Since(s.startTime)
		// Invalidate any full-render cache so the frozen duration label is
		// rendered (and from now on cached) on the next Render call.
		s.cachedRender = ""
		s.cachedWidth = -1
	}
}

// --------------------------------------------------------------------------
// StreamingBashOutputItem - Live bash command output
// --------------------------------------------------------------------------

// StreamingBashOutputItem represents live bash command output.
type StreamingBashOutputItem struct {
	id           string
	command      string
	stdoutLines  []string
	stderrLines  []string
	maxLines     int
	complete     bool
	cachedRender string
	cachedWidth  int

	// cachedRender holds theme-colored output, so a theme change has to
	// invalidate it even though neither the content nor the width moved.
	themeStamp
}

// NewStreamingBashOutputItem creates a new streaming bash output item.
func NewStreamingBashOutputItem(id string, command string) *StreamingBashOutputItem {
	return &StreamingBashOutputItem{
		id:          id,
		command:     command,
		stdoutLines: make([]string, 0),
		stderrLines: make([]string, 0),
		maxLines:    100, // Cap lines to prevent memory issues
		complete:    false,
	}
}

func (m *StreamingBashOutputItem) ID() string {
	return m.id
}

// RawContent returns the captured command output as plain text, prefixed with
// the command itself. Streaming bash output is capped to maxLines for display,
// so this returns whatever the item still retains.
func (m *StreamingBashOutputItem) RawContent() string {
	var b strings.Builder
	if m.command != "" {
		b.WriteString("$ ")
		b.WriteString(m.command)
		b.WriteString("\n\n")
	}
	for _, line := range m.stdoutLines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, line := range m.stderrLines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// Role returns the message role.
func (m *StreamingBashOutputItem) Role() string {
	return "bash"
}

func (m *StreamingBashOutputItem) Render(width int) string {
	// A theme change invalidates the cache: it holds baked-in color codes
	// from the theme it was produced under.
	if m.stale() {
		m.cachedRender = ""
		m.stamp()
	}

	// Return cached if width matches and complete
	if m.complete && m.cachedWidth == width && m.cachedRender != "" {
		return m.cachedRender
	}

	theme := style.GetTheme()
	var parts []string

	// Header with command. The bullet is the same marker the finished tool
	// block uses, so live output and its settled form read as the same thing.
	if m.command != "" {
		headerStyle := style.GetCachedStyles().BashHeader
		parts = append(parts, headerStyle.Render("· "+m.command))
	}

	lineIndent := strings.Repeat(" ", style.ContentOffset)
	// The output panel pays for its own indent and its interior padding, so
	// it is one content offset narrower than the block. Clamping matters:
	// an unclamped subtraction goes negative on a very narrow terminal and
	// lipgloss renders nothing at all.
	lineWidth := max(style.BodyWidth(width)-1, style.MinContentWidth)

	// Stdout lines
	if len(m.stdoutLines) > 0 {
		outputStyle := lipgloss.NewStyle().
			Foreground(theme.Text).
			Background(theme.CodeBg).
			PaddingLeft(1).
			Width(lineWidth)
		for _, line := range m.stdoutLines {
			parts = append(parts, lineIndent+outputStyle.Render(line))
		}
	}

	// Stderr lines
	if len(m.stderrLines) > 0 {
		stderrStyle := lipgloss.NewStyle().
			Foreground(theme.Error).
			Background(theme.CodeBg).
			PaddingLeft(1).
			Width(lineWidth)
		for _, line := range m.stderrLines {
			parts = append(parts, lineIndent+stderrStyle.Render(line))
		}
	}

	result := strings.Join(parts, "\n")
	if m.complete {
		m.cachedRender = result
		m.cachedWidth = width
	}
	return result
}

func (m *StreamingBashOutputItem) Height() int {
	if m.cachedRender != "" {
		return strings.Count(m.cachedRender, "\n") + 1
	}
	// Estimate: command header + stdout + stderr
	return 1 + len(m.stdoutLines) + len(m.stderrLines)
}

// AppendStdout adds a stdout line to the output.
func (m *StreamingBashOutputItem) AppendStdout(line string) {
	m.stdoutLines = append(m.stdoutLines, line)
	// Cap lines
	if len(m.stdoutLines) > m.maxLines {
		m.stdoutLines = m.stdoutLines[len(m.stdoutLines)-m.maxLines:]
	}
	m.cachedWidth = 0 // Invalidate cache
}

// AppendStderr adds a stderr line to the output.
func (m *StreamingBashOutputItem) AppendStderr(line string) {
	m.stderrLines = append(m.stderrLines, line)
	// Cap lines
	if len(m.stderrLines) > m.maxLines {
		m.stderrLines = m.stderrLines[len(m.stderrLines)-m.maxLines:]
	}
	m.cachedWidth = 0 // Invalidate cache
}

// MarkComplete marks the bash output as complete.
func (m *StreamingBashOutputItem) MarkComplete() {
	m.complete = true
}

// --------------------------------------------------------------------------
// --------------------------------------------------------------------------
// Helper: generateMessageID
// --------------------------------------------------------------------------

var messageCounter = 0

func generateMessageID() string {
	messageCounter++
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), messageCounter)
}
