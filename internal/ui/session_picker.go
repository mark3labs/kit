package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// SessionEntry describes one live session on a daemon, as reported by the
// session list.
type SessionEntry struct {
	ID      uint64
	Clients int
	Started time.Time
	Cwd     string
	Name    string
	// Host names the daemon the session belongs to. Empty means the
	// daemon the client is connected to; the hub picker sets it so
	// sessions from several machines can be listed together.
	Host string
}

// SessionPick is the outcome of the picker.
type SessionPick struct {
	// Index is the chosen entry, or -1 for "start a new session".
	Index int
	// Cancelled reports that the user dismissed the picker.
	Cancelled bool
}

// pickerRow is one rendered line: either a session or a group header.
type pickerRow struct {
	entry      SessionEntry
	index      int    // index into the original entries slice, -1 for a header
	header     string // group label when this row is a header
	isNew      bool   // the trailing "start a new session" row
	selectable bool
}

// sessionPickerModel lists live sessions plus a "start a new session"
// entry. Enter picks; Esc cancels.
type sessionPickerModel struct {
	rows      []pickerRow
	cursor    int
	quitting  bool
	cancelled bool
	title     string
	width     int
	height    int
}

func (m *sessionPickerModel) Init() tea.Cmd { return nil }

// move steps the cursor over selectable rows only, so group headers are
// skipped rather than landed on.
func (m *sessionPickerModel) move(delta int) {
	next := m.cursor
	for {
		next += delta
		if next < 0 || next >= len(m.rows) {
			return // ran off the end: leave the cursor where it was
		}
		if m.rows[next].selectable {
			m.cursor = next
			return
		}
	}
}

func (m *sessionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
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
		// Leave the alternate screen on the final render so the terminal
		// returns to the normal buffer cleanly (see dirPickerModel.View).
		//
		// A caller that owns the alt screen itself does not get to keep it
		// either way: Bubble Tea restores whatever screen state it entered
		// when the program shuts down, so holding the alt screen for this
		// last frame only moves the exit sequence later. The attach client
		// re-enters the alt screen after the picker returns.
		v := tea.NewView("")
		v.AltScreen = false
		v.MouseMode = tea.MouseModeNone
		return v
	}
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(m.title)
	b.WriteString("\n\n")
	for i, row := range m.rows {
		if !row.selectable {
			b.WriteString("  ")
			b.WriteString(row.header)
			b.WriteString("\n")
			continue
		}
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}
		b.WriteString(cursor)
		if row.isNew {
			b.WriteString("start a new session")
		} else {
			b.WriteString(rowLabel(row.entry))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n  ↑↓ navigate · ↵ select · esc cancel")

	// The picker runs between two attached sessions, so it must own the
	// alt screen: drawn inline it would smear over whatever the previous
	// session left on the terminal. The frame is placed against the full
	// terminal size so every cell is written each render — a partial
	// frame leaves the previous session's pixels showing through.
	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	v := tea.NewView(lipgloss.Place(width, height,
		lipgloss.Left, lipgloss.Top, b.String()))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeNone
	return v
}

// rowLabel renders one session. A named session leads with its name; an
// unnamed one falls back to its id, so every row is identifiable.
func rowLabel(e SessionEntry) string {
	state := "detached"
	if e.Clients > 0 {
		state = fmt.Sprintf("%d client(s) attached", e.Clients)
	}
	age := time.Since(e.Started).Round(time.Second)
	head := fmt.Sprintf("session %d", e.ID)
	if e.Name != "" {
		head = fmt.Sprintf("%s (%d)", e.Name, e.ID)
	}
	label := fmt.Sprintf("%s — %s · started %s ago", head, state, age)
	if e.Cwd != "" {
		label += " · " + e.Cwd
	}
	return label
}

// buildRows lays out the entries, grouping by host when more than one host
// is represented. The "start a new session" row is always last.
func buildRows(entries []SessionEntry) []pickerRow {
	hosts := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, e := range entries {
		if !seen[e.Host] {
			seen[e.Host] = true
			hosts = append(hosts, e.Host)
		}
	}
	grouped := len(hosts) > 1

	rows := make([]pickerRow, 0, len(entries)+len(hosts)+1)
	for _, host := range hosts {
		if grouped {
			label := host
			if label == "" {
				label = "this machine"
			}
			rows = append(rows, pickerRow{header: label, index: -1})
		}
		for i, e := range entries {
			if e.Host != host {
				continue
			}
			rows = append(rows, pickerRow{entry: e, index: i, selectable: true})
		}
	}
	rows = append(rows, pickerRow{index: -1, isNew: true, selectable: true})
	return rows
}

// RunSessionPicker shows the live sessions and lets the user attach to one
// or start a new session.
//
// ctx cancels the picker: it blocks in the Bubble Tea run loop, which the
// caller cannot interrupt any other way. Cancellation ends it as if the
// user had pressed Esc, so the terminal is restored on the way out.
//
// The picker owns the alternate screen while it runs and leaves it when it
// exits. A caller that was in the alt screen itself must re-enter it
// afterwards; Bubble Tea restores the screen state on shutdown, so the
// picker cannot hand it over.
func RunSessionPicker(ctx context.Context, entries []SessionEntry, input *os.File, title string) (SessionPick, error) {
	if title == "" {
		title = "Live sessions"
	}
	rows := buildRows(entries)
	m := &sessionPickerModel{rows: rows, title: title}
	// Start on the first selectable row, which is a header when grouped.
	for i, r := range rows {
		if r.selectable {
			m.cursor = i
			break
		}
	}

	opts := []tea.ProgramOption{}
	if input != nil {
		// Read from the caller's stream rather than opening os.Stdin
		// again: the attach client keeps one reader on the terminal and a
		// second would race it for keystrokes.
		opts = append(opts, tea.WithInput(input))
	}
	prog := tea.NewProgram(m, opts...)

	// Cancellation quits the program the same way Esc does, rather than
	// through tea.WithContext. WithContext kills the program: it skips the
	// final render, so the alt screen and the terminal modes are left as
	// the picker had them — the very state this picker is careful to
	// restore. (It also races its own input reader on that path.) Quit
	// runs the ordinary shutdown, so the caller gets its terminal back.
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	go func() {
		select {
		case <-ctx.Done():
			prog.Quit()
		case <-stopWatch:
		}
	}()

	final, err := prog.Run()
	if err != nil {
		// A cancelled context is the caller shutting the client down, not
		// a picker failure: report it as the cancellation it is so the
		// caller does not print a picker error on its way out.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return SessionPick{Cancelled: true}, ctxErr
		}
		return SessionPick{Cancelled: true}, fmt.Errorf("session picker: %w", err)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		// Quit on cancellation ends the program cleanly, so Run returns no
		// error. Report the cancellation rather than the empty choice it
		// would otherwise look like.
		return SessionPick{Cancelled: true}, ctxErr
	}
	sp, ok := final.(*sessionPickerModel)
	if !ok {
		return SessionPick{Cancelled: true}, fmt.Errorf("session picker: unexpected state")
	}
	// Only an explicit enter is a selection. A program that ends any other
	// way — an input stream at EOF, say — leaves both flags false, and
	// treating that as a choice would attach a session the user never
	// picked.
	if sp.cancelled || !sp.quitting {
		return SessionPick{Cancelled: true}, nil
	}
	row := sp.rows[sp.cursor]
	if row.isNew {
		return SessionPick{Index: -1}, nil
	}
	return SessionPick{Index: row.index}, nil
}
