package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
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

// NewTextMessageItem creates a new text message for the scrollback.
// The content should be pre-rendered using MessageRenderer for proper styling.
func NewTextMessageItem(id string, role string, content string) *TextMessageItem {
	return &TextMessageItem{
		id:        id,
		role:      role,
		content:   content,
		timestamp: time.Now(),
	}
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
	contentWidth := width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	lines := strings.Split(m.content, "\n")
	for _, line := range lines {
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
	role          string // "assistant" or "reasoning"
	content       string // Accumulated streaming content
	timestamp     time.Time
	startTime     time.Time // When streaming started (for live duration counter)
	modelName     string
	streaming     bool          // true while actively streaming
	finalDuration time.Duration // Frozen duration when complete
	cachedRender  string
	cachedWidth   int
}

// NewStreamingMessageItem creates a new streaming message item.
func NewStreamingMessageItem(id, role string, modelName string) *StreamingMessageItem {
	now := time.Now()
	return &StreamingMessageItem{
		id:        id,
		role:      role,
		timestamp: now,
		startTime: now,
		modelName: modelName,
		streaming: true,
	}
}

// ID returns the unique identifier.
func (s *StreamingMessageItem) ID() string {
	return s.id
}

// Render renders the streaming message with live content.
func (s *StreamingMessageItem) Render(width int) string {
	// For reasoning, never cache - we need live duration updates
	// For assistant, cache is OK
	if s.role != "reasoning" && s.cachedWidth == width && s.cachedRender != "" {
		return s.cachedRender
	}

	// Get renderer from context
	renderer := newMessageRenderer(width, false)

	var rendered string
	if s.role == "reasoning" {
		// Render as reasoning/thinking block with live duration counter
		theme := GetTheme()
		mutedStyle := lipgloss.NewStyle().Foreground(theme.Muted)
		ty := createTypography(theme)
		content := strings.TrimLeft(s.content, " \t\n")

		var parts []string
		parts = append(parts, mutedStyle.Render(ty.Italic(content)))

		// Add live duration counter (updates on each render)
		var duration time.Duration
		if s.finalDuration > 0 {
			// Streaming complete, show frozen duration
			duration = s.finalDuration
		} else if !s.startTime.IsZero() {
			// Still streaming, show live duration
			duration = time.Since(s.startTime)
		}

		if duration > 0 {
			var durationStr string
			if duration < time.Second {
				durationStr = fmt.Sprintf("%dms", duration.Milliseconds())
			} else {
				durationStr = fmt.Sprintf("%.1fs", duration.Seconds())
			}
			label := lipgloss.NewStyle().Foreground(theme.VeryMuted).Render("Thought for ")
			durationStyled := lipgloss.NewStyle().Foreground(theme.Accent).Render(durationStr)
			parts = append(parts, label+durationStyled)
		}

		rendered = styleMarginBottom1.Render(strings.Join(parts, "\n"))
	} else {
		// Render as assistant message
		msg := renderer.RenderAssistantMessage(s.content, s.timestamp, s.modelName)
		rendered = msg.Content
	}

	// Cache and return (but reasoning is never cached due to live duration)
	if s.role != "reasoning" {
		s.cachedRender = rendered
		s.cachedWidth = width
	}
	return rendered
}

// Height returns the number of lines.
func (s *StreamingMessageItem) Height() int {
	if s.cachedRender == "" {
		return 0
	}
	return strings.Count(s.cachedRender, "\n") + 1
}

// AppendChunk adds a content chunk and invalidates the render cache.
func (s *StreamingMessageItem) AppendChunk(chunk string) {
	s.content += chunk
	s.cachedWidth = 0 // Invalidate cache
}

// MarkComplete marks the streaming message as complete and freezes the duration.
func (s *StreamingMessageItem) MarkComplete() {
	s.streaming = false
	// Freeze the duration for reasoning blocks
	if s.role == "reasoning" && !s.startTime.IsZero() {
		s.finalDuration = time.Since(s.startTime)
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

	theme := GetTheme()
	var parts []string

	// Header with command
	if m.command != "" {
		headerStyle := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Italic(true)
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
// SystemMessageItem - System messages (commands, info, errors)
// --------------------------------------------------------------------------

// SystemMessageItem represents a system message (commands, info, errors).
type SystemMessageItem struct {
	id           string
	content      string
	timestamp    time.Time
	cachedRender string
	cachedWidth  int
}

// NewSystemMessageItem creates a new system message for the scrollback.
func NewSystemMessageItem(id, content string) *SystemMessageItem {
	return &SystemMessageItem{
		id:        id,
		content:   content,
		timestamp: time.Now(),
	}
}

func (m *SystemMessageItem) ID() string {
	return m.id
}

func (m *SystemMessageItem) Render(width int) string {
	// Return cached render if width matches
	if m.cachedWidth == width && m.cachedRender != "" {
		return m.cachedRender
	}

	// Simple system message formatting
	rendered := "│ " + strings.ReplaceAll(m.content, "\n", "\n│ ")

	// Cache and return
	m.cachedRender = rendered
	m.cachedWidth = width
	return rendered
}

func (m *SystemMessageItem) Height() int {
	if m.cachedRender != "" {
		return strings.Count(m.cachedRender, "\n") + 1
	}
	// Estimate
	if m.cachedWidth > 0 {
		return (len(m.content) / max(m.cachedWidth-10, 40)) + 3
	}
	return 3
}

// --------------------------------------------------------------------------
// Helper: generateMessageID
// --------------------------------------------------------------------------

var messageCounter = 0

func generateMessageID() string {
	messageCounter++
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), messageCounter)
}
