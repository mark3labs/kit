package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/session"
	"github.com/mark3labs/kit/internal/ui/style"
)

// SessionSelectedMsg is sent when the user selects a session from the picker.
type SessionSelectedMsg struct {
	Path string // absolute path to the JSONL session file
}

// SessionSelectorCancelledMsg is sent when the user cancels the picker.
type SessionSelectorCancelledMsg struct{}

// SessionDeletedMsg is sent after a session is deleted so the parent can
// react (e.g. print a message).
type SessionDeletedMsg struct {
	Name string
}

// SessionScopeMode controls which sessions are shown.
type SessionScopeMode int

const (
	SessionScopeCwd SessionScopeMode = iota // current folder only
	SessionScopeAll                         // all sessions across projects
)

func (m SessionScopeMode) String() string {
	if m == SessionScopeAll {
		return "All"
	}
	return "Current Folder"
}

// SessionFilterMode controls filtering of the session list.
type SessionFilterMode int

const (
	SessionFilterAll   SessionFilterMode = iota // show all sessions
	SessionFilterNamed                          // only named sessions
)

func (m SessionFilterMode) String() string {
	if m == SessionFilterNamed {
		return "Named"
	}
	return "All"
}

// controlCharsRe matches ASCII control characters for stripping from previews.
var controlCharsRe = regexp.MustCompile(`[\x00-\x1f\x7f]`)

// SessionSelectorComponent is a full-screen Bubble Tea component that lets
// the user browse and select from available sessions. Modeled after pi's
// session picker: right-aligned metadata, background-highlighted selection,
// scope/filter toggles, and inline search.
type SessionSelectorComponent struct {
	allSessions []session.SessionInfo
	cwdSessions []session.SessionInfo
	filtered    []session.SessionInfo

	cursor int
	search string

	scope  SessionScopeMode
	filter SessionFilterMode

	// currentPath is the active session file path for marking it in the list.
	currentPath string

	width  int
	height int
	active bool

	// confirmDelete is non-negative when a delete confirmation is pending.
	confirmDelete int
}

// NewSessionSelector creates a session selector. It loads sessions for the
// current working directory and all sessions across projects. If cwd is
// empty, only "All" scope is available.
func NewSessionSelector(cwd string, width, height int) *SessionSelectorComponent {
	ss := &SessionSelectorComponent{
		width:         width,
		height:        height,
		active:        true,
		confirmDelete: -1,
	}

	// Load sessions (errors are swallowed — empty list is fine).
	if cwd != "" {
		ss.cwdSessions, _ = session.ListSessions(cwd)
		ss.scope = SessionScopeCwd
	}
	ss.allSessions, _ = session.ListAllSessions()

	if cwd == "" || len(ss.cwdSessions) == 0 {
		ss.scope = SessionScopeAll
	}

	ss.rebuildFiltered()
	return ss
}

// SetCurrentPath sets the currently active session path so the picker can
// highlight it in the list.
func (ss *SessionSelectorComponent) SetCurrentPath(path string) {
	ss.currentPath = path
}

// Init implements tea.Model.
func (ss *SessionSelectorComponent) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (ss *SessionSelectorComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ss.width = msg.Width
		ss.height = msg.Height
		return ss, nil

	case tea.KeyPressMsg:
		// Delete confirmation mode.
		if ss.confirmDelete >= 0 {
			switch msg.String() {
			case "y", "Y":
				idx := ss.confirmDelete
				ss.confirmDelete = -1
				if idx < len(ss.filtered) {
					info := ss.filtered[idx]
					if err := session.DeleteSession(info.Path); err == nil {
						name := sessionDisplayName(info)
						ss.removeSession(info.Path)
						ss.rebuildFiltered()
						return ss, func() tea.Msg {
							return SessionDeletedMsg{Name: name}
						}
					}
				}
				return ss, nil
			default:
				ss.confirmDelete = -1
				return ss, nil
			}
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
			if ss.cursor > 0 {
				ss.cursor--
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
			if ss.cursor < len(ss.filtered)-1 {
				ss.cursor++
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("pgup"))):
			ss.cursor -= ss.visibleHeight()
			if ss.cursor < 0 {
				ss.cursor = 0
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("pgdown"))):
			ss.cursor += ss.visibleHeight()
			if ss.cursor >= len(ss.filtered) {
				ss.cursor = len(ss.filtered) - 1
			}
			if ss.cursor < 0 {
				ss.cursor = 0
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("home"))):
			ss.cursor = 0

		case key.Matches(msg, key.NewBinding(key.WithKeys("end"))):
			ss.cursor = max(len(ss.filtered)-1, 0)

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if ss.cursor < len(ss.filtered) {
				info := ss.filtered[ss.cursor]
				ss.active = false
				return ss, func() tea.Msg {
					return SessionSelectedMsg{Path: info.Path}
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if ss.search != "" {
				ss.search = ""
				ss.rebuildFiltered()
			} else {
				ss.active = false
				return ss, func() tea.Msg {
					return SessionSelectorCancelledMsg{}
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			if ss.scope == SessionScopeCwd {
				ss.scope = SessionScopeAll
			} else {
				ss.scope = SessionScopeCwd
			}
			ss.rebuildFiltered()

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+n"))):
			if ss.filter == SessionFilterAll {
				ss.filter = SessionFilterNamed
			} else {
				ss.filter = SessionFilterAll
			}
			ss.rebuildFiltered()

		case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
			if ss.cursor < len(ss.filtered) {
				ss.confirmDelete = ss.cursor
			}
			return ss, nil

		default:
			if msg.Text != "" && len(msg.Text) == 1 {
				ch := msg.Text[0]
				if ch >= 32 && ch < 127 {
					ss.search += string(ch)
					ss.rebuildFiltered()
				}
			}
			if key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))) && len(ss.search) > 0 {
				ss.search = ss.search[:len(ss.search)-1]
				ss.rebuildFiltered()
			}
		}
	}
	return ss, nil
}

// View implements tea.Model.
func (ss *SessionSelectorComponent) View() tea.View {
	theme := style.GetTheme()

	// Full-screen bordered container - uses entire terminal width and height
	maxWidth := ss.width - 2 // Small margin on each side
	if maxWidth < 20 {
		maxWidth = ss.width
	}
	maxHeight := ss.height - 2 // Small margin top/bottom to prevent overflow
	if maxHeight < 10 {
		maxHeight = ss.height
	}
	horizontalPadding := 1
	innerWidth := maxWidth - 4   // Account for border (2) + padding (2)
	innerHeight := maxHeight - 4 // Account for border (2) + padding (2)

	// Container style with border - full width/height like a framed panel
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Background(theme.Background).
		Padding(1, horizontalPadding).
		Width(maxWidth).
		Height(maxHeight)

	var contentBuilder strings.Builder

	// ── Header: title + scope badges ─────────────────────────────
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Accent).
		Background(theme.Background)
	contentBuilder.WriteString(titleStyle.Render(fmt.Sprintf("Resume Session (%s)", ss.scope)))
	contentBuilder.WriteString("\n")

	// ── Help / keybindings ───────────────────────────────────────
	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Background(theme.Background)
	if innerWidth >= 75 {
		contentBuilder.WriteString(helpStyle.Render("tab: scope  N: named  D: delete  R: rename  type to search  esc: cancel"))
	} else if innerWidth >= 50 {
		contentBuilder.WriteString(helpStyle.Render("tab scope  N named  D del  type to search  esc"))
	} else {
		contentBuilder.WriteString(helpStyle.Render("tab N D esc"))
	}
	contentBuilder.WriteString("\n")

	// ── Search (only shown when active) ──────────────────────────
	if ss.search != "" {
		searchStyle := lipgloss.NewStyle().
			Foreground(theme.Info).
			Background(theme.Background)
		contentBuilder.WriteString(searchStyle.Render(fmt.Sprintf("> %s", ss.search)))
		contentBuilder.WriteString("\n")
	}

	// Separator line
	sepWidth := innerWidth
	contentBuilder.WriteString(
		lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(theme.Background).
			Render(strings.Repeat("─", sepWidth)))
	contentBuilder.WriteString("\n")

	// ── Delete confirmation ──────────────────────────────────────
	if ss.confirmDelete >= 0 && ss.confirmDelete < len(ss.filtered) {
		warnStyle := lipgloss.NewStyle().
			Foreground(theme.Error).
			Bold(true).
			Background(theme.Background)
		name := sessionDisplayName(ss.filtered[ss.confirmDelete])
		contentBuilder.WriteString(warnStyle.Render(fmt.Sprintf("Delete %q? (y/N)", truncateRunes(name, 40))))
		contentBuilder.WriteString("\n")
	}

	// ── Session list ─────────────────────────────────────────────
	if len(ss.filtered) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(theme.Background)
		if ss.search != "" {
			contentBuilder.WriteString(emptyStyle.Render(fmt.Sprintf("No sessions matching %q", ss.search)))
		} else if ss.filter == SessionFilterNamed {
			contentBuilder.WriteString(emptyStyle.Render("No named sessions. Press N to show all."))
		} else if ss.scope == SessionScopeCwd {
			contentBuilder.WriteString(emptyStyle.Render("No sessions in current folder. Press tab to view all."))
		} else {
			contentBuilder.WriteString(emptyStyle.Render("No sessions found"))
		}
		contentBuilder.WriteString("\n")
	} else {
		// Compute visible window based on inner container height
		// Chrome: header(2) + separator(1) + footer separator(1) + footer(1) = 5
		chromeLines := 5
		if ss.search != "" {
			chromeLines++
		}
		if ss.confirmDelete >= 0 {
			chromeLines++
		}
		visH := max(innerHeight-chromeLines, 3)

		// Center the cursor in the visible window.
		startIdx := max(0, min(ss.cursor-visH/2, len(ss.filtered)-visH))
		endIdx := min(startIdx+visH, len(ss.filtered))

		for i := startIdx; i < endIdx; i++ {
			info := ss.filtered[i]
			isCursor := i == ss.cursor
			isCurrent := info.Path == ss.currentPath
			isDeleting := i == ss.confirmDelete
			line := ss.renderEntry(info, isCursor, isCurrent, isDeleting, innerWidth)
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		}

		// Scroll position indicator.
		if len(ss.filtered) > visH {
			posStyle := lipgloss.NewStyle().
				Foreground(theme.Muted).
				Background(theme.Background)
			contentBuilder.WriteString(posStyle.Render(fmt.Sprintf("(%d/%d)", ss.cursor+1, len(ss.filtered))))
			contentBuilder.WriteString("\n")
		}
	}

	// Footer separator
	contentBuilder.WriteString(
		lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(theme.Background).
			Render(strings.Repeat("─", sepWidth)))
	contentBuilder.WriteString("\n")

	// Footer with filter info
	footerStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Background(theme.Background)
	contentBuilder.WriteString(footerStyle.Render(fmt.Sprintf("Filter: %s", ss.filter)))

	// Apply the bordered container
	content := contentBuilder.String()
	borderedContent := containerStyle.Render(content)

	v := tea.NewView(borderedContent)
	v.AltScreen = true
	return v
}

// IsActive returns whether the selector is still accepting input.
func (ss *SessionSelectorComponent) IsActive() bool {
	return ss.active
}

// --- Internal helpers ---

func (ss *SessionSelectorComponent) visibleHeight() int {
	// Reserve: title(1) + help(1) + blank(1) + scroll indicator(1) = 4.
	// Optional: search(1), delete confirm(1).
	chrome := 4
	if ss.search != "" {
		chrome++
	}
	if ss.confirmDelete >= 0 {
		chrome++
	}
	return max(ss.height-chrome, 3)
}

func (ss *SessionSelectorComponent) rebuildFiltered() {
	var source []session.SessionInfo
	if ss.scope == SessionScopeCwd {
		source = ss.cwdSessions
	} else {
		source = ss.allSessions
	}

	if ss.filter == SessionFilterNamed {
		var named []session.SessionInfo
		for _, s := range source {
			if s.Name != "" {
				named = append(named, s)
			}
		}
		source = named
	}

	if ss.search != "" {
		query := strings.ToLower(ss.search)
		var matches []session.SessionInfo
		for _, s := range source {
			haystack := strings.ToLower(s.Name + " " + s.FirstMessage + " " + s.Cwd)
			if strings.Contains(haystack, query) {
				matches = append(matches, s)
			}
		}
		ss.filtered = matches
	} else {
		ss.filtered = source
	}

	if ss.cursor >= len(ss.filtered) {
		ss.cursor = max(len(ss.filtered)-1, 0)
	}
}

func (ss *SessionSelectorComponent) removeSession(path string) {
	ss.cwdSessions = removeByPath(ss.cwdSessions, path)
	ss.allSessions = removeByPath(ss.allSessions, path)
}

func removeByPath(sessions []session.SessionInfo, path string) []session.SessionInfo {
	result := make([]session.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		if s.Path != path {
			result = append(result, s)
		}
	}
	return result
}

// renderEntry renders a single session line with right-aligned metadata.
// Layout: [cursor 2] [message ...variable...] [padding] [count age] [cwd?]
func (ss *SessionSelectorComponent) renderEntry(info session.SessionInfo, isCursor, isCurrent, isDeleting bool, width int) string {
	theme := style.GetTheme()

	// ── Cursor indicator (2 chars) ───────────────────────────────
	cursorStr := "  "
	if isCursor {
		cursorStr = lipgloss.NewStyle().Foreground(theme.Accent).Render("> ")
	}
	const cursorW = 2

	// ── Right part: message count + relative time (+ optional cwd) ──
	age := relativeTime(info.Modified)
	msgCount := fmt.Sprintf("%d", info.MessageCount)
	rightPart := msgCount + " " + age
	if ss.scope == SessionScopeAll && info.Cwd != "" {
		shortCwd := shortenPath(info.Cwd)
		if len(shortCwd) > 25 {
			shortCwd = "..." + shortCwd[len(shortCwd)-22:]
		}
		rightPart = shortCwd + " " + rightPart
	}
	rightW := utf8.RuneCountInString(rightPart)

	// ── Message text ─────────────────────────────────────────────
	displayText := sessionDisplayName(info)
	// Strip control characters and collapse whitespace.
	displayText = controlCharsRe.ReplaceAllString(displayText, " ")
	displayText = strings.Join(strings.Fields(displayText), " ")

	availableForMsg := max(width-cursorW-rightW-2, 10) // 2 for min spacing
	displayText = truncateRunes(displayText, availableForMsg)
	msgW := utf8.RuneCountInString(displayText)

	// ── Style the message ────────────────────────────────────────
	var msgStyle lipgloss.Style
	switch {
	case isDeleting:
		msgStyle = lipgloss.NewStyle().Foreground(theme.Error)
	case isCurrent:
		msgStyle = lipgloss.NewStyle().Foreground(theme.Accent)
	case info.Name != "":
		msgStyle = lipgloss.NewStyle().Foreground(theme.Warning)
	default:
		msgStyle = lipgloss.NewStyle().Foreground(theme.Text)
	}

	// ── Style the right part ─────────────────────────────────────
	rightColor := theme.Muted
	if isDeleting {
		rightColor = theme.Error
	}
	var styledRight string

	// ── Assemble with spacing ────────────────────────────────────
	spacing := max(width-cursorW-msgW-rightW, 1)

	// If selected, use inverted colors like PopupList
	if isCursor {
		// Inverted colors for selected item
		msgStyle = lipgloss.NewStyle().
			Background(theme.Primary).
			Foreground(theme.Background).
			Bold(true)
		styledRight = lipgloss.NewStyle().
			Background(theme.Primary).
			Foreground(rightColor).
			Render(rightPart)
		cursorStr = lipgloss.NewStyle().
			Background(theme.Primary).
			Foreground(theme.Accent).
			Render("> ")
	} else {
		styledRight = lipgloss.NewStyle().Foreground(rightColor).Render(rightPart)
	}

	styledMsg := msgStyle.Render(displayText)
	line := cursorStr + styledMsg + strings.Repeat(" ", spacing) + styledRight

	return line
}

// --- Package helpers ---

// sessionDisplayName returns the best display string for a session:
// the name if set, the first message, or a fallback.
func sessionDisplayName(info session.SessionInfo) string {
	if info.Name != "" {
		return info.Name
	}
	if info.FirstMessage != "" {
		return info.FirstMessage
	}
	return "(empty session)"
}

// truncateRunes truncates a string to at most maxRunes runes, appending "..."
// if truncated.
func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-1]) + "…"
}

// shortenPath replaces the user's home directory prefix with ~.
func shortenPath(path string) string {
	return tildeHome(path)
}

// relativeTime formats a time as a short relative string like "5m", "2h", "3d".
func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/(24*365)))
	}
}
