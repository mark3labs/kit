package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// streamContentMsg carries updated content to the streaming display.
type streamContentMsg string

// streamDoneMsg signals the streaming display to quit cleanly.
type streamDoneMsg struct{}

// StreamingDisplay is a Bubble Tea model that renders streamed AI responses
// using BT's synchronized output and cursor management for flicker-free
// in-place updates. It replaces the manual ANSI escape sequence approach
// previously used in displayContainer().
type StreamingDisplay struct {
	content         string
	messageRenderer *MessageRenderer
	compactRenderer *CompactRenderer
	compactMode     bool
	width           int
	modelName       string
	timestamp       time.Time
}

// newStreamingDisplay creates a StreamingDisplay configured for the given
// display mode and terminal width.
func newStreamingDisplay(compactMode bool, width int, modelName string) *StreamingDisplay {
	return &StreamingDisplay{
		messageRenderer: NewMessageRenderer(width, false),
		compactRenderer: NewCompactRenderer(width, false),
		compactMode:     compactMode,
		width:           width,
		modelName:       modelName,
		timestamp:       time.Now(),
	}
}

// Init implements tea.Model. No initial command is needed.
func (m *StreamingDisplay) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It handles content updates and quit signals.
func (m *StreamingDisplay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case streamContentMsg:
		m.content = string(msg)
		return m, nil
	case streamDoneMsg:
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.width = msg.Width - 4 // Match CLI padding
		m.messageRenderer.SetWidth(m.width)
		m.compactRenderer.SetWidth(m.width)
		return m, nil
	}
	return m, nil
}

// View implements tea.Model. It renders the current streaming content as a
// fully styled assistant message. Bubble Tea handles the cursor management
// and synchronized output to prevent flicker.
func (m *StreamingDisplay) View() tea.View {
	var msg UIMessage
	if m.compactMode {
		msg = m.compactRenderer.RenderAssistantMessage(m.content, m.timestamp, m.modelName)
	} else {
		msg = m.messageRenderer.RenderAssistantMessage(m.content, m.timestamp, m.modelName)
	}

	paddedContent := lipgloss.NewStyle().
		PaddingLeft(2).
		Width(m.width).
		Render(msg.Content)

	return tea.NewView(paddedContent)
}
