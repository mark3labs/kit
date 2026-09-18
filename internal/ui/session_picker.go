package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/ui/style"
)

// newSessionLabel is the trailing row that starts a session instead of
// attaching to one. It mirrors the directory picker's "Use this
// directory" row: the action that ends the picker sits last in the list.
const newSessionLabel = "✚ Start a new session"

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
//
// The list itself is a PopupList, the same component behind the directory
// picker and every in-session selector, so the two pickers a user meets
// before a session starts look and behave alike: same bordered box, same
// search line, same keys. What this model adds on top is the group
// headers, which PopupList has no notion of — they are ordinary items the
// cursor is steered around.
type sessionPickerModel struct {
	popup     *PopupList
	rows      []pickerRow
	quitting  bool
	cancelled bool
	choice    pickerRow
	width     int
	height    int
}

// newSessionPickerModel builds the picker for entries.
func newSessionPickerModel(entries []SessionEntry, title string) *sessionPickerModel {
	if title == "" {
		title = "Live sessions"
	}
	m := &sessionPickerModel{rows: buildRows(entries)}
	m.popup = &PopupList{
		Title:      title,
		Subtitle:   sessionCountLabel(len(entries)),
		ShowSearch: true,
		HideCount:  true, // headers would make "(i/N)" count rows, not sessions
		RenderItem: m.renderRow,
		FilterFunc: filterSessionItems,
	}
	m.popup.SetItems(m.buildItems())
	m.snap(1)
	return m
}

// sessionCountLabel is the subtitle: what the list holds, in one line.
func sessionCountLabel(n int) string {
	switch n {
	case 0:
		return "no live sessions"
	case 1:
		return "1 live session"
	default:
		return fmt.Sprintf("%d live sessions", n)
	}
}

// buildItems converts the rows into popup items. Label doubles as the
// filter haystack (name, id and working directory), since the visible text
// is produced by renderRow.
func (m *sessionPickerModel) buildItems() []PopupItem {
	items := make([]PopupItem, 0, len(m.rows))
	for _, row := range m.rows {
		switch {
		case !row.selectable:
			items = append(items, PopupItem{Label: row.header, Meta: row})
		case row.isNew:
			items = append(items, PopupItem{Label: newSessionLabel, Meta: row})
		default:
			haystack := strings.TrimSpace(sessionHead(row.entry) + " " + row.entry.Cwd)
			items = append(items, PopupItem{Label: haystack, Meta: row})
		}
	}
	return items
}

func (m *sessionPickerModel) Init() tea.Cmd { return nil }

func (m *sessionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.popup.SetSize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *sessionPickerModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		m.cancelled = true
		return m, tea.Quit
	}

	search := m.popup.Search()
	result := m.popup.HandleKey(key, msg.Text)
	switch {
	case result.Cancelled:
		m.cancelled = true
		return m, tea.Quit
	case result.Selected != nil:
		row, ok := result.Selected.Meta.(pickerRow)
		if !ok || !row.selectable {
			return m, nil
		}
		m.choice = row
		m.quitting = true
		return m, tea.Quit
	}

	if m.popup.Search() != search {
		// A new query is a new list: go back to the top of it. Leaving
		// the cursor where it was only clamps it, which after a narrowing
		// search parks it on the last row — "start a new session". Enter
		// would then start a session instead of attaching to the one the
		// user just searched for.
		m.popup.SetCursor(0)
		m.snap(1)
		return m, nil
	}

	// PopupList does not know about headers, so it can leave the cursor on
	// one. Push it off in the direction the key was heading.
	m.snap(searchDirection(key))
	return m, nil
}

// searchDirection reports which way a navigation key moves the cursor, so
// a header landed on is left behind rather than stepped back onto.
func searchDirection(key string) int {
	switch key {
	case "up", "pgup", "end":
		return -1
	default:
		return 1
	}
}

// snap moves the cursor off a header row, preferring direction dir and
// falling back to the other way when the list ends first.
func (m *sessionPickerModel) snap(dir int) {
	if dir == 0 {
		dir = 1
	}
	if i := m.seek(m.popup.Cursor(), dir); i >= 0 {
		m.popup.SetCursor(i)
		return
	}
	if i := m.seek(m.popup.Cursor(), -dir); i >= 0 {
		m.popup.SetCursor(i)
	}
}

// seek returns the first selectable row at or after i, stepping by dir, or
// -1 when the list runs out.
func (m *sessionPickerModel) seek(i, dir int) int {
	items := m.popup.Items()
	for ; i >= 0 && i < len(items); i += dir {
		if row, ok := items[i].Meta.(pickerRow); ok && row.selectable {
			return i
		}
	}
	return -1
}

// filterSessionItems narrows the list to sessions matching query, keeping
// the host grouping intact: rows stay in their original order (PopupList's
// default filter sorts by score, which would scatter them across their
// headers), a header survives only while it still has a session under it,
// and "start a new session" is always reachable.
func filterSessionItems(query string, items []PopupItem) []PopupItem {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	kept := make([]PopupItem, 0, len(items))
	for _, item := range items {
		row, ok := item.Meta.(pickerRow)
		switch {
		case !ok:
			continue
		case row.isNew, !row.selectable:
			kept = append(kept, item) // headers are pruned below
		case strings.Contains(strings.ToLower(item.Label), q):
			kept = append(kept, item)
		}
	}
	return dropEmptyHeaders(kept)
}

// dropEmptyHeaders removes group headers no session follows.
func dropEmptyHeaders(items []PopupItem) []PopupItem {
	out := make([]PopupItem, 0, len(items))
	for i, item := range items {
		if row, ok := item.Meta.(pickerRow); ok && !row.selectable {
			next, found := pickerRowAt(items, i+1)
			if !found || next.isNew || !next.selectable {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

func pickerRowAt(items []PopupItem, i int) (pickerRow, bool) {
	if i < 0 || i >= len(items) {
		return pickerRow{}, false
	}
	row, ok := items[i].Meta.(pickerRow)
	return row, ok
}

// renderRow draws one list line for PopupList: a dimmed group header, the
// new-session action, or a session with its working directory on the left
// and its state on the right.
//
// The cursor row is returned unstyled so the popup can paint one
// continuous highlight over it — an inner Render would reset the
// background mid-line and break the bar into pieces.
func (m *sessionPickerModel) renderRow(item PopupItem, innerWidth int, isCursor bool) string {
	theme := style.GetTheme()
	row, ok := item.Meta.(pickerRow)
	if !ok {
		return item.Label
	}

	if !row.selectable {
		return lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Background(theme.Background).
			Bold(true).
			Render(truncateRunes(row.header, max(innerWidth, 1)))
	}

	indicator := "  "
	if isCursor {
		indicator = "> "
	}

	if row.isNew {
		if isCursor {
			return indicator + newSessionLabel
		}
		return indicator + lipgloss.NewStyle().
			Foreground(theme.Accent).
			Background(theme.Background).
			Render(newSessionLabel)
	}

	e := row.entry
	right := sessionState(e)
	if !e.Started.IsZero() {
		right += " · " + relativeTime(e.Started)
	}
	rightW := lipgloss.Width(right)

	// indicator(2) + head + gap(2) + path ... gap + right
	avail := max(innerWidth-2-rightW-2, 10)
	head := truncateRunes(sessionHead(e), avail)
	path := ""
	if e.Cwd != "" {
		room := avail - lipgloss.Width(head) - 2
		if room >= 6 {
			path = truncateRunes(shortenPath(e.Cwd), room)
		}
	}

	left := head
	if path != "" {
		left += "  " + path
	}
	spacing := max(innerWidth-2-lipgloss.Width(left)-rightW, 1)

	if isCursor {
		return indicator + left + strings.Repeat(" ", spacing) + right
	}

	headStyle := lipgloss.NewStyle().Foreground(theme.Text).Background(theme.Background)
	if e.Name != "" {
		headStyle = headStyle.Foreground(theme.Warning)
	}
	mutedStyle := lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.Background)

	out := indicator + headStyle.Render(head)
	if path != "" {
		out += mutedStyle.Render("  " + path)
	}
	return out + strings.Repeat(" ", spacing) + mutedStyle.Render(right)
}

// sessionHead identifies one session. A named session leads with its name;
// an unnamed one falls back to its id, so every row is identifiable. The
// leading dot reports at a glance whether anybody is attached.
func sessionHead(e SessionEntry) string {
	head := fmt.Sprintf("session %d", e.ID)
	if e.Name != "" {
		head = fmt.Sprintf("%s (%d)", e.Name, e.ID)
	}
	glyph := "○"
	if e.Clients > 0 {
		glyph = "●"
	}
	return glyph + " " + head
}

// sessionState says who is on the session right now.
func sessionState(e SessionEntry) string {
	switch {
	case e.Clients == 1:
		return "1 client"
	case e.Clients > 1:
		return fmt.Sprintf("%d clients", e.Clients)
	default:
		return "detached"
	}
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
	m.popup.SetSize(width, height)

	v := tea.NewView(lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center, m.popup.Render()))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeNone
	return v
}

// buildRows lays out the entries, grouping by host when the rows do not
// all belong to this machine. The "start a new session" row is always
// last.
//
// A header is what tells the user WHERE a session is. It can only be left
// out when the answer is already known, and that is exactly one case:
// every row is on this machine. A single remote host needs the header as
// much as several do — an ungrouped list of somebody else's sessions reads
// as a list of local ones, and attaching to the wrong machine is not a
// mistake the user can see they are making.
func buildRows(entries []SessionEntry) []pickerRow {
	hosts := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, e := range entries {
		if !seen[e.Host] {
			seen[e.Host] = true
			hosts = append(hosts, e.Host)
		}
	}
	grouped := len(hosts) > 1 || (len(hosts) == 1 && hosts[0] != "")

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
	m := newSessionPickerModel(entries, title)

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
	if sp.choice.isNew {
		return SessionPick{Index: -1}, nil
	}
	return SessionPick{Index: sp.choice.index}, nil
}
