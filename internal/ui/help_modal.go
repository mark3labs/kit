package ui

import (
	"path/filepath"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/style"
)

// The /help reference sheet lists every slash command and key binding, laid
// out in columns over the transcript. It is a sheetModal.
//
// Its content is gathered when it renders, from the same sources the app
// dispatches on: the composer's live command list and the appKeyMap. Nothing
// here is a hand-written copy, so the sheet cannot drift from the app.

// Help sheet geometry. The padding and gaps are generous on purpose: this is
// a reference sheet, and white space is what makes it quick to scan.
const (
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
	// helpMaxDescLines caps how many lines one description may take in the
	// help sheet. Other sheets pass 0, for no cap.
	helpMaxDescLines = 3
)

// openHelpModal shows the help sheet.
func (m *AppModel) openHelpModal() {
	m.openSheet("◆ Help", "Chat with the agent, or type / to find a command.", helpMaxBoxW, m.helpBody)
}

// helpBody renders the help sheet body: the commands, then the keys.
func (m *AppModel) helpBody(width int, s helpStyles) []string {
	blank := s.bg.Render(strings.Repeat(" ", width))
	var body []string
	body = append(body, helpGroupHeading("Commands", width, s), blank)
	body = append(body, layoutHelpGrid(m.helpCommandSections(), width, helpCmdMinColW, helpMaxDescLines, s)...)
	body = append(body, blank, blank)
	body = append(body, helpGroupHeading("Keyboard", width, s), blank)
	body = append(body, layoutHelpGrid(m.helpKeySections(), width, helpKeyMinColW, helpMaxDescLines, s)...)
	return body
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
	return append(secs, m.shortcutSections()...)
}

// shortcutSections returns the shortcuts that extensions registered, one
// section per extension.
func (m *AppModel) shortcutSections() []helpSection {
	if m.getShortcutList == nil {
		return nil
	}
	var secs []helpSection
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

// cleanCell makes text from outside the app (extension descriptions, job
// output) safe for one cell: escape sequences are removed, and tabs and
// newlines become spaces, since any of them would break the line widths.
func cleanCell(text string) string {
	text = xansi.Strip(text)
	text = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, text)
	return strings.TrimSpace(text)
}

// renderHelpSection renders one titled section as lines exactly colW wide.
// maxDesc caps the lines of one description; 0 means no cap.
func renderHelpSection(sec helpSection, colW, maxDesc int, s helpStyles) []string {
	lines := []string{s.padLine(s.section.Render(xansi.Truncate(sec.Title, colW, "…")), colW)}

	// The key column fits the longest key that is not over the cap, so
	// one long name does not push every description to the right.
	keyCap := max(6, colW/helpMaxKeyFrac)
	keyW := 0
	for _, e := range sec.Entries {
		if w := lipgloss.Width(cleanCell(e.Key)); w <= keyCap {
			keyW = max(keyW, w)
		}
	}
	descW := max(10, colW-keyW-helpKeyDescGap)
	indent := s.bg.Render(strings.Repeat(" ", keyW+helpKeyDescGap))

	for _, e := range sec.Entries {
		e.Key, e.Desc, e.Note = cleanCell(e.Key), cleanCell(e.Desc), cleanCell(e.Note)
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
		if maxDesc > 0 && len(descLines) > maxDesc {
			descLines = descLines[:maxDesc]
			last := maxDesc - 1
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
func layoutHelpGrid(secs []helpSection, width, minColW, maxDesc int, s helpStyles) []string {
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
		stacks[idx] = append(stacks[idx], renderHelpSection(sec, colW, maxDesc, s)...)
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
