package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// SessionEntry describes one live session on a remote host, as reported by
// the daemon's session list.
type SessionEntry struct {
	ID      uint64
	Clients int
	Started time.Time
	Cwd     string
}

// sessionPickerModel lists live sessions plus a "start a new session"
// entry. Enter picks; Esc cancels the whole connect.
type sessionPickerModel struct {
	items     []SessionEntry
	cursor    int
	quitting  bool
	cancelled bool
}

func (m *sessionPickerModel) Init() tea.Cmd { return nil }

func (m *sessionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items) {
				m.cursor++
			}
		case "enter":
			m.quitting = true
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *sessionPickerModel) View() tea.View {
	if m.cancelled || m.quitting {
		// Leave alt screen on the final render so the terminal returns to
		// the normal buffer cleanly (see dirPickerModel.View).
		v := tea.NewView("")
		v.AltScreen = false
		return v
	}
	var b strings.Builder
	b.WriteString("  Live sessions on this host\n\n")
	for i, e := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}
		b.WriteString(cursor + m.rowLabel(e) + "\n")
	}
	b.WriteString("  " + cursorMark() + " start a new session\n\n")
	b.WriteString("  ↑↓ navigate · ↵ select · esc cancel")
	return tea.NewView(b.String())
}

func cursorMark() string { return " " }

func (m sessionPickerModel) rowLabel(e SessionEntry) string {
	state := "detached"
	if e.Clients > 0 {
		state = fmt.Sprintf("%d client(s) attached", e.Clients)
	}
	age := time.Since(e.Started).Round(time.Second)
	label := fmt.Sprintf("session %d — %s · started %s ago", e.ID, state, age)
	if e.Cwd != "" {
		label += " · " + e.Cwd
	}
	return label
}

// RunSessionPicker shows the live sessions of a host and lets the user
// attach to one or start a new session. Returns the chosen entry's index,
// or -1 for a new session, or -2 when the user cancelled.
func RunSessionPicker(entries []SessionEntry) (int, error) {
	m := &sessionPickerModel{items: entries}
	prog := tea.NewProgram(m)
	final, err := prog.Run()
	if err != nil {
		return -2, fmt.Errorf("session picker: %w", err)
	}
	if sp, ok := final.(*sessionPickerModel); ok {
		if sp.cancelled {
			return -2, nil // user cancelled the connect
		}
		if sp.cursor < len(sp.items) {
			return sp.cursor, nil // attach to this session
		}
		return -1, nil // "start a new session" row
	}
	return -2, fmt.Errorf("session picker: unexpected state")
}
