package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/style"
)

// helpModal is the /help reference sheet: every slash command and key
// binding, laid out in columns over the transcript.
//
// Its content is gathered when it renders, from the same sources the app
// dispatches on: the composer's live command list and the appKeyMap. Nothing
// here is a hand-written copy, so the sheet cannot drift from the app.
type helpModal struct {
	scrollOff int
	// pageSize and maxOff come from the last render; the key handler pages
	// and clamps with them.
	pageSize int
	maxOff   int
}

// Help modal geometry. The padding and gaps are generous on purpose: this is
// a reference sheet, and white space is what makes it quick to scan.
const (
	helpPadX   = 3
	helpPadY   = 1
	helpColGap = 5
	// Commands carry long descriptions, so they get fewer, wider columns
	// than key bindings.
	helpCmdMinColW = 56
	helpKeyMinColW = 40
	// helpMaxKeyFrac caps the key column at this fraction of the section
	// width. A longer key gets its own line instead of being cut.
	helpMaxKeyFrac = 3
	helpMaxCols    = 3
	helpMaxBoxW    = 150
	helpKeyDescGap = 2
	// helpMaxDescLines caps how many lines one description may take.
	helpMaxDescLines = 3
)

// helpModalKeys are the bindings the open modal answers to. The footer hint
// is rendered from them.
var helpModalKeys = struct {
	Up, Down, PageUp, PageDown, Top, Bottom, Close key.Binding
}{
	Up:       bind("↑/k", "up", "up", "k"),
	Down:     bind("↓/j", "down", "down", "j"),
	PageUp:   bind("pgup", "page up", "pgup", "shift+space"),
	PageDown: bind("pgdn/space", "page down", "pgdown", "space"),
	Top:      bind("g", "top", "home", "g"),
	Bottom:   bind("G", "bottom", "end", "G"),
	Close:    bind("esc/q", "close", "esc", "q", "enter", "ctrl+c"),
}

// openHelpModal shows the help sheet.
func (m *AppModel) openHelpModal() {
	m.helpModal = &helpModal{}
}

// updateHelpModal handles input while the help sheet is open. It owns the
// keyboard; every other message passes through to the app.
func (m *AppModel) updateHelpModal(msg tea.Msg) (handled bool) {
	h := m.helpModal
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, helpModalKeys.Close):
			m.helpModal = nil
		case key.Matches(msg, helpModalKeys.Up):
			h.scrollBy(-1)
		case key.Matches(msg, helpModalKeys.Down):
			h.scrollBy(1)
		case key.Matches(msg, helpModalKeys.PageUp):
			h.scrollBy(-max(1, h.pageSize-1))
		case key.Matches(msg, helpModalKeys.PageDown):
			h.scrollBy(max(1, h.pageSize-1))
		case key.Matches(msg, helpModalKeys.Top):
			h.scrollOff = 0
		case key.Matches(msg, helpModalKeys.Bottom):
			h.scrollOff = h.maxOff
		}
		return true
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			h.scrollBy(-3)
		case tea.MouseWheelDown:
			h.scrollBy(3)
		}
		return true
	case tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg:
		// Do not let clicks select transcript text under the sheet.
		return true
	}
	return false
}

func (h *helpModal) scrollBy(delta int) {
	h.scrollOff = max(0, min(h.maxOff, h.scrollOff+delta))
}

// helpCommandSections groups the live slash command list by category.
func (m *AppModel) helpCommandSections() []helpSection {
	cmds := commands.SlashCommands
	if ic, ok := m.input.(*InputComponent); ok {
		cmds = ic.commands
	}

	// Known categories in a fixed, logical order; unknown ones follow in
	// the order they first appear.
	order := []string{"Info", "Navigation", "System", "Extensions", "Prompts", "Skills", "MCP Prompts"}
	byCat := map[string][]helpEntry{}
	for _, c := range cmds {
		cat := c.Category
		if cat == "" {
			cat = "Other"
		}
		if !slices.Contains(order, cat) {
			order = append(order, cat)
		}
		byCat[cat] = append(byCat[cat], helpEntry{
			Key:  c.Name,
			Desc: c.Description,
			Note: strings.Join(c.Aliases, " "),
		})
	}

	var out []helpSection
	for _, cat := range order {
		if entries := byCat[cat]; len(entries) > 0 {
			out = append(out, helpSection{Title: cat, Entries: entries})
		}
	}
	return out
}

// helpKeySections returns built-in bindings, input syntax and the shortcuts
// that extensions registered, grouped by extension.
func (m *AppModel) helpKeySections() []helpSection {
	secs := keys.keyHelpSections()
	secs = append(secs, composerSyntaxSection())

	if m.getShortcutList == nil {
		return secs
	}
	var current *helpSection
	for _, sc := range m.getShortcutList() {
		title := "Extension: " + strings.TrimSuffix(filepath.Base(sc.Source), filepath.Ext(sc.Source))
		if current == nil || current.Title != title {
			secs = append(secs, helpSection{Title: title})
			current = &secs[len(secs)-1]
		}
		desc := sc.Description
		if desc == "" {
			desc = "(no description)"
		}
		current.Entries = append(current.Entries, helpEntry{Key: sc.Key, Desc: desc})
	}
	return secs
}

// helpStyles are the styles of the help sheet. Every style carries the
// sheet background: the sheet is composited over the transcript, and a span
// without its own background would show the terminal colour at each reset.
type helpStyles struct {
	bg, title, subtitle, group, rule, section, key, desc, note, hintKey, hintDesc lipgloss.Style
}

func newHelpStyles(theme style.Theme) helpStyles {
	base := lipgloss.NewStyle().Background(theme.Background)
	return helpStyles{
		bg:       base,
		title:    base.Foreground(theme.Primary).Bold(true),
		subtitle: base.Foreground(theme.Muted),
		group:    base.Foreground(theme.Primary).Bold(true),
		rule:     base.Foreground(theme.MutedBorder),
		section:  base.Foreground(theme.Accent).Bold(true),
		key:      base.Foreground(theme.Info),
		desc:     base.Foreground(theme.Text),
		note:     base.Foreground(theme.VeryMuted),
		hintKey:  base.Foreground(theme.Muted).Bold(true),
		hintDesc: base.Foreground(theme.VeryMuted),
	}
}

// padLine pads a rendered line with background-coloured spaces to width.
func (s helpStyles) padLine(line string, width int) string {
	if gap := width - lipgloss.Width(line); gap > 0 {
		return line + s.bg.Render(strings.Repeat(" ", gap))
	}
	return line
}

// renderHelpSection renders one titled section as lines exactly colW wide.
func renderHelpSection(sec helpSection, colW int, s helpStyles) []string {
	lines := []string{s.padLine(s.section.Render(xansi.Truncate(sec.Title, colW, "…")), colW)}

	// The key column fits the longest key that is not over the cap, so
	// one long name does not push every description to the right.
	keyCap := max(6, colW/helpMaxKeyFrac)
	keyW := 0
	for _, e := range sec.Entries {
		if w := lipgloss.Width(e.Key); w <= keyCap {
			keyW = max(keyW, w)
		}
	}
	descW := max(10, colW-keyW-helpKeyDescGap)
	indent := s.bg.Render(strings.Repeat(" ", keyW+helpKeyDescGap))

	for _, e := range sec.Entries {
		var keyCell string
		if lipgloss.Width(e.Key) > keyW {
			// Too long for the key column: the key takes a full line and
			// the description starts on the next, under the column.
			lines = append(lines, s.padLine(s.key.Render(xansi.Truncate(e.Key, colW, "…")), colW))
			keyCell = indent
		} else {
			keyCell = s.padLine(s.key.Render(e.Key), keyW) + s.bg.Render(strings.Repeat(" ", helpKeyDescGap))
		}

		// Wrap at spaces only, and hard-break a word longer than the line,
		// so no line can grow past the column.
		descLines := strings.Split(xansi.Wrap(e.Desc, descW, ""), "\n")
		// Some descriptions (skills especially) are written for the model
		// and run to a paragraph. The sheet is for scanning, so keep the
		// first lines; the / popup shows the rest.
		if len(descLines) > helpMaxDescLines {
			descLines = descLines[:helpMaxDescLines]
			last := helpMaxDescLines - 1
			descLines[last] = xansi.Truncate(descLines[last], descW-1, "") + "…"
		}
		rendered := make([]string, len(descLines))
		for i, d := range descLines {
			rendered[i] = s.desc.Render(d)
		}
		// Aliases go on the last line when they fit, else on their own.
		if e.Note != "" {
			last := len(descLines) - 1
			note := xansi.Truncate(e.Note, descW, "…")
			if lipgloss.Width(descLines[last])+2+lipgloss.Width(note) <= descW {
				rendered[last] += s.bg.Render("  ") + s.note.Render(note)
			} else {
				rendered = append(rendered, s.note.Render(note))
			}
		}
		for i, r := range rendered {
			prefix := keyCell
			if i > 0 {
				prefix = indent
			}
			lines = append(lines, s.padLine(prefix+r, colW))
		}
	}
	return lines
}

// layoutHelpGrid places sections in columns. Each section goes to the
// shortest column so far, which keeps the columns close in height without
// reordering the sections.
func layoutHelpGrid(secs []helpSection, width, minColW int, s helpStyles) []string {
	if len(secs) == 0 {
		return nil
	}
	cols := (width + helpColGap) / (minColW + helpColGap)
	cols = max(1, min(cols, helpMaxCols, len(secs)))
	colW := (width - helpColGap*(cols-1)) / cols

	stacks := make([][]string, cols)
	blank := s.bg.Render(strings.Repeat(" ", colW))
	for _, sec := range secs {
		idx := 0
		for j := 1; j < cols; j++ {
			if len(stacks[j]) < len(stacks[idx]) {
				idx = j
			}
		}
		if len(stacks[idx]) > 0 {
			stacks[idx] = append(stacks[idx], blank)
		}
		stacks[idx] = append(stacks[idx], renderHelpSection(sec, colW, s)...)
	}

	height := 0
	for _, st := range stacks {
		height = max(height, len(st))
	}
	gap := s.bg.Render(strings.Repeat(" ", helpColGap))
	out := make([]string, height)
	for row := range height {
		var b strings.Builder
		for c, st := range stacks {
			if c > 0 {
				b.WriteString(gap)
			}
			if row < len(st) {
				b.WriteString(st[row])
			} else {
				b.WriteString(blank)
			}
		}
		out[row] = s.padLine(b.String(), width)
	}
	return out
}

// helpGroupHeading renders a group name followed by a thin rule.
func helpGroupHeading(name string, width int, s helpStyles) string {
	label := s.group.Render(strings.ToUpper(name))
	rule := max(0, width-lipgloss.Width(label)-1)
	return s.padLine(label+s.bg.Render(" ")+s.rule.Render(strings.Repeat("─", rule)), width)
}

// renderHelpModal draws the help sheet, scrolled to the current offset.
func (m *AppModel) renderHelpModal() string {
	h := m.helpModal
	theme := style.GetTheme()
	s := newHelpStyles(theme)

	boxW := max(20, min(m.width-4, helpMaxBoxW))
	innerW := max(10, boxW-2-2*helpPadX)

	blank := s.bg.Render(strings.Repeat(" ", innerW))
	header := []string{
		s.padLine(s.title.Render("◆ Help"), innerW),
		s.padLine(s.subtitle.Render(xansi.Truncate("Chat with the agent, or type / to find a command.", innerW, "…")), innerW),
		blank,
	}

	var body []string
	body = append(body, helpGroupHeading("Commands", innerW, s), blank)
	body = append(body, layoutHelpGrid(m.helpCommandSections(), innerW, helpCmdMinColW, s)...)
	body = append(body, blank, blank)
	body = append(body, helpGroupHeading("Keyboard", innerW, s), blank)
	body = append(body, layoutHelpGrid(m.helpKeySections(), innerW, helpKeyMinColW, s)...)

	// The box may use the full height less a one-row margin top and bottom.
	maxBoxH := max(8, m.height-2)
	footerH := 2
	viewH := max(1, maxBoxH-2-2*helpPadY-len(header)-footerH)
	scrollable := len(body) > viewH
	if !scrollable {
		viewH = len(body)
	}
	h.pageSize = viewH
	h.maxOff = max(0, len(body)-viewH)
	h.scrollOff = min(h.scrollOff, h.maxOff)
	visible := body[h.scrollOff : h.scrollOff+viewH]

	lines := append([]string{}, header...)
	lines = append(lines, visible...)
	lines = append(lines, blank, m.helpFooter(scrollable, len(body), viewH, innerW, s))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		BorderBackground(theme.Background).
		Background(theme.Background).
		Padding(helpPadY, helpPadX).
		Render(strings.Join(lines, "\n"))
}

// helpFooter renders the key hints and, when the sheet scrolls, the position.
func (m *AppModel) helpFooter(scrollable bool, total, viewH, width int, s helpStyles) string {
	hint := func(b key.Binding) string {
		hb := b.Help()
		return s.hintKey.Render(hb.Key) + s.hintDesc.Render(" "+hb.Desc)
	}
	sep := s.hintDesc.Render("   ")
	var parts []string
	if scrollable {
		parts = append(parts, hint(helpModalKeys.Up), hint(helpModalKeys.Down), hint(helpModalKeys.PageDown))
	}
	parts = append(parts, hint(helpModalKeys.Close))
	left := strings.Join(parts, sep)

	right := ""
	if scrollable {
		h := m.helpModal
		pct := 100
		if h.maxOff > 0 {
			pct = h.scrollOff * 100 / h.maxOff
		}
		right = s.hintDesc.Render(fmt.Sprintf("%d–%d of %d · %d%%", h.scrollOff+1, h.scrollOff+viewH, total, pct))
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return s.padLine(xansi.Truncate(left, width, "…"), width)
	}
	return left + s.bg.Render(strings.Repeat(" ", gap)) + right
}
