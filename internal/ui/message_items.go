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

// TextMessageItem represents a completed text message (user or assistant)
// in the scrollback. It uses pre-rendered styled content from MessageRenderer.
type TextMessageItem struct {
	id          string
	role        string // "user" or "assistant"
	content     string // Raw content (for re-rendering if needed)
	preRendered string // Pre-rendered styled content from MessageRenderer
	timestamp   time.Time
}

// NewStyledMessageItem creates a message item with pre-rendered styled content.
// This is the preferred way to create messages when you have styled content from MessageRenderer.
func NewStyledMessageItem(id string, role string, rawContent string, preRendered string) *TextMessageItem {
	return &TextMessageItem{
		id:          id,
		role:        role,
		content:     rawContent,
		preRendered: preRendered,
		timestamp:   time.Now(),
	}
}

func (m *TextMessageItem) ID() string {
	return m.id
}

func (m *TextMessageItem) Render(width int) string {
	// If we have pre-rendered styled content, return it
	if m.preRendered != "" {
		return m.preRendered
	}

	// Fallback to simple formatting if no pre-rendered content
	return m.renderContent(width)
}

func (m *TextMessageItem) Height() int {
	rendered := m.Render(0) // Width doesn't matter since we use pre-rendered
	if rendered == "" {
		return 0
	}
	return strings.Count(rendered, "\n") + 1
}

func (m *TextMessageItem) renderContent(width int) string {
	var parts []string

	// Role indicator
	if m.role == "user" {
		parts = append(parts, "│ ▸ You")
	} else {
		parts = append(parts, "") // Assistant messages start without role
	}

	// Content with simple wrapping
	contentWidth := max(width-4, 20)

	for line := range strings.SplitSeq(m.content, "\n") {
		if len(line) <= contentWidth {
			parts = append(parts, "│ "+line)
		} else {
			// Basic wrap
			for len(line) > contentWidth {
				parts = append(parts, "│ "+line[:contentWidth])
				line = line[contentWidth:]
			}
			if len(line) > 0 {
				parts = append(parts, "│ "+line)
			}
		}
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

// Render renders the streaming message with live content.
func (s *StreamingMessageItem) Render(width int) string {
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

func (m *StreamingBashOutputItem) Render(width int) string {
	// Return cached if width matches and complete
	if m.complete && m.cachedWidth == width && m.cachedRender != "" {
		return m.cachedRender
	}

	theme := style.GetTheme()
	var parts []string

	// Header with command
	if m.command != "" {
		headerStyle := style.GetCachedStyles().BashHeader
		parts = append(parts, headerStyle.Render(fmt.Sprintf("▸ %s", m.command)))
	}

	const lineIndent = "  "
	lineWidth := width - len(lineIndent)

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
