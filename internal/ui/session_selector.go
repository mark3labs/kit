package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

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

// SessionSelectorComponent is a Bubble Tea component that lets the user browse
// and select from available sessions. It wraps PopupList in FullScreen mode:
// PopupList owns the cursor/search/scroll math/chrome; this component owns
// the session list, scope/filter toggles, and delete-confirmation flow.
type SessionSelectorComponent struct {
	allSessions []session.SessionInfo
	cwdSessions []session.SessionInfo
	filtered    []session.SessionInfo // matches popup.Items() 1:1

	scope  SessionScopeMode
	filter SessionFilterMode

	// currentPath is the active session file path for marking it in the list.
	currentPath string

	popup  *PopupList
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

	ss.popup = NewPopupList("Resume Session", nil, width, height)
	ss.popup.FullScreen = true
	ss.popup.FooterHint = "↑↓ nav • ↵ open • esc cancel • tab scope • ^N named • d delete • type to search"
	ss.popup.RenderItem = ss.renderEntry

	ss.rebuild()
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
		ss.popup.SetSize(msg.Width, msg.Height)
		return ss, nil

	case tea.KeyPressMsg:
		// Delete confirmation mode swallows all keys until y/n.
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
						ss.rebuild()
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
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			if ss.scope == SessionScopeCwd {
				ss.scope = SessionScopeAll
			} else {
				ss.scope = SessionScopeCwd
			}
			ss.rebuild()
			return ss, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+n"))):
			if ss.filter == SessionFilterAll {
				ss.filter = SessionFilterNamed
			} else {
				ss.filter = SessionFilterAll
			}
			ss.rebuild()
			return ss, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+d"))):
			// Ctrl+D as an explicit delete shortcut. Plain "d" still works
			// below when the search field is empty so it doesn't conflict
			// with typing the letter 'd' into a query.
			if c := ss.popup.Cursor(); c < len(ss.filtered) {
				ss.confirmDelete = c
			}
			return ss, nil
		}

		// Plain 'd' triggers delete only when there's no active search
		// query (otherwise the user would never be able to type 'd' into
		// a search like "doc").
		if msg.String() == "d" && !ss.popup.IsSearching() {
			if c := ss.popup.Cursor(); c < len(ss.filtered) {
				ss.confirmDelete = c
				return ss, nil
			}
		}

		// Delegate everything else to the popup.
		result := ss.popup.HandleKey(msg.String(), msg.Text)
		if result.Changed {
			ss.syncFiltered()
		}
		if result.Selected != nil {
			cursor := ss.popup.Cursor()
			if cursor < len(ss.filtered) {
				info := ss.filtered[cursor]
				ss.active = false
				return ss, func() tea.Msg {
					return SessionSelectedMsg{Path: info.Path}
				}
			}
		}
		if result.Cancelled {
			ss.active = false
			return ss, func() tea.Msg {
				return SessionSelectorCancelledMsg{}
			}
		}
	}
	return ss, nil
}

// View implements tea.Model.
func (ss *SessionSelectorComponent) View() tea.View {
	// Compose dynamic footer extras: scope + filter + (delete confirm).
	extra := fmt.Sprintf("scope: %s • filter: %s", ss.scope, ss.filter)
	if ss.confirmDelete >= 0 && ss.confirmDelete < len(ss.filtered) {
		name := truncateRunes(sessionDisplayName(ss.filtered[ss.confirmDelete]), 30)
		extra = fmt.Sprintf("delete %q? y/N", name)
	}
	ss.popup.Title = fmt.Sprintf("Resume Session (%s)", ss.scope)
	ss.popup.ExtraFooter = extra

	rendered := ss.popup.RenderCentered(ss.width, ss.height)
	v := tea.NewView(rendered)
	v.AltScreen = true
	return v
}

// IsActive returns whether the selector is still accepting input.
func (ss *SessionSelectorComponent) IsActive() bool {
	return ss.active
}

// --- Internal helpers ---

// rebuild applies the scope and filter selections, then publishes the
// resulting session list to the popup.
func (ss *SessionSelectorComponent) rebuild() {
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

	// Build PopupItems. The Label holds a haystack string (name + first
	// message + cwd) so PopupList's default filter can match against any
	// of those fields. We render each row with a custom RenderItem.
	items := make([]PopupItem, len(source))
	for i, s := range source {
		haystack := strings.TrimSpace(s.Name + " " + s.FirstMessage + " " + s.Cwd)
		items[i] = PopupItem{
			Label:  haystack,
			Active: s.Path == ss.currentPath,
			Meta:   s,
		}
	}
	ss.popup.SetItems(items)
	ss.syncFiltered()
}

// syncFiltered refreshes the filtered slice from popup.Items() so cursor
// indices map back to session.SessionInfo for the parent.
func (ss *SessionSelectorComponent) syncFiltered() {
	items := ss.popup.Items()
	out := make([]session.SessionInfo, 0, len(items))
	for _, it := range items {
		if s, ok := it.Meta.(session.SessionInfo); ok {
			out = append(out, s)
		}
	}
	ss.filtered = out
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

// renderEntry is the RenderItem callback handed to PopupList. It produces a
// single-line entry with left-aligned message text and right-aligned
// metadata (message count + relative time, plus optional cwd in "All" scope).
//
// When isCursor we return a plain (unstyled) string so PopupList's outer
// row style can paint one continuous fg+bg span. Mixing inner lipgloss
// Render calls with an outer Background() breaks the highlight into bars,
// because each inner Render emits an ANSI reset that drops the background.
func (ss *SessionSelectorComponent) renderEntry(item PopupItem, innerWidth int, isCursor bool) string {
	theme := style.GetTheme()
	info, ok := item.Meta.(session.SessionInfo)
	if !ok {
		return item.Label
	}
	isCurrent := info.Path == ss.currentPath
	isDeleting := ss.confirmDelete >= 0 && ss.confirmDelete < len(ss.filtered) &&
		ss.filtered[ss.confirmDelete].Path == info.Path

	// Cursor indicator (2 cells).
	indicator := "  "
	if isCursor {
		indicator = "> "
	}

	// Right-hand metadata.
	age := relativeTime(info.Modified)
	right := fmt.Sprintf("%d %s", info.MessageCount, age)
	if ss.scope == SessionScopeAll && info.Cwd != "" {
		shortCwd := truncateRunes(shortenPath(info.Cwd), 25)
		right = shortCwd + " " + right
	}
	rightW := lipgloss.Width(right)

	// Message text width: innerWidth minus indicator(2) minus right minus gap(2).
	availForMsg := max(innerWidth-2-rightW-2, 10)

	displayText := sessionDisplayName(info)
	displayText = controlCharsRe.ReplaceAllString(displayText, " ")
	displayText = strings.Join(strings.Fields(displayText), " ")
	displayText = truncateRunes(displayText, availForMsg)

	msgW := lipgloss.Width(displayText)
	spacing := max(innerWidth-2-msgW-rightW, 1)

	// Selected row: raw string, outer row style paints it.
	if isCursor {
		return indicator + displayText + strings.Repeat(" ", spacing) + right
	}

	// Color the message text by state.
	var msgStyle, rightStyle lipgloss.Style
	switch {
	case isDeleting:
		msgStyle = lipgloss.NewStyle().Foreground(theme.Error)
	case isCurrent:
		msgStyle = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	case info.Name != "":
		msgStyle = lipgloss.NewStyle().Foreground(theme.Warning)
	default:
		msgStyle = lipgloss.NewStyle().Foreground(theme.Text)
	}
	if isDeleting {
		rightStyle = lipgloss.NewStyle().Foreground(theme.Error)
	} else {
		rightStyle = lipgloss.NewStyle().Foreground(theme.Muted)
	}

	return indicator + msgStyle.Render(displayText) + strings.Repeat(" ", spacing) + rightStyle.Render(right)
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

// truncateRunes truncates a string to at most maxRunes runes, appending "…"
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
