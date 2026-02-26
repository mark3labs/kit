package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/session"
)

// TreeFilterMode controls which entries are visible in the tree selector.
type TreeFilterMode int

const (
	TreeFilterDefault   TreeFilterMode = iota // hide settings entries
	TreeFilterNoTools                         // hide tool results
	TreeFilterUserOnly                        // show only user messages
	TreeFilterLabelOnly                       // show only labeled entries
	TreeFilterAll                             // show everything
)

func (m TreeFilterMode) String() string {
	switch m {
	case TreeFilterDefault:
		return "default"
	case TreeFilterNoTools:
		return "no-tools"
	case TreeFilterUserOnly:
		return "user-only"
	case TreeFilterLabelOnly:
		return "labeled"
	case TreeFilterAll:
		return "all"
	default:
		return "unknown"
	}
}

// FlatNode is a tree entry flattened for list rendering with indentation info.
type FlatNode struct {
	Entry    any    // the underlying entry
	ID       string // entry ID
	ParentID string
	Depth    int    // indentation level
	IsLast   bool   // last child at this depth
	Prefix   string // computed prefix string (├─, └─, etc.)
	Label    string // user-defined label, if any
}

// TreeSelectorComponent is a Bubble Tea component that renders the session
// tree as an ASCII art list with navigation and selection. It follows pi's
// tree selector design.
type TreeSelectorComponent struct {
	tm         *session.TreeManager
	flatNodes  []FlatNode
	cursor     int
	filter     TreeFilterMode
	leafID     string // real leaf for "active" marker
	width      int
	height     int
	search     string
	active     bool
	selectedID string // set when user selects a node
	cancelled  bool
}

// NewTreeSelector creates a tree selector from a TreeManager.
func NewTreeSelector(tm *session.TreeManager, width, height int) *TreeSelectorComponent {
	ts := &TreeSelectorComponent{
		tm:     tm,
		filter: TreeFilterDefault,
		leafID: tm.GetLeafID(),
		width:  width,
		height: height,
		active: true,
	}
	ts.rebuildFlatList()
	// Position cursor at the active leaf.
	for i, node := range ts.flatNodes {
		if node.ID == ts.leafID {
			ts.cursor = i
			break
		}
	}
	return ts
}

// Init implements tea.Model.
func (ts *TreeSelectorComponent) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (ts *TreeSelectorComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ts.width = msg.Width
		ts.height = msg.Height
		return ts, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if ts.cursor > 0 {
				ts.cursor--
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if ts.cursor < len(ts.flatNodes)-1 {
				ts.cursor++
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "pgup"))):
			// Page up.
			ts.cursor -= ts.visibleHeight()
			if ts.cursor < 0 {
				ts.cursor = 0
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "pgdown"))):
			// Page down.
			ts.cursor += ts.visibleHeight()
			if ts.cursor >= len(ts.flatNodes) {
				ts.cursor = len(ts.flatNodes) - 1
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("home"))):
			ts.cursor = 0

		case key.Matches(msg, key.NewBinding(key.WithKeys("end"))):
			ts.cursor = len(ts.flatNodes) - 1

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if ts.cursor < len(ts.flatNodes) {
				ts.selectedID = ts.flatNodes[ts.cursor].ID
				ts.active = false
				return ts, func() tea.Msg {
					return TreeNodeSelectedMsg{
						ID:       ts.selectedID,
						Entry:    ts.flatNodes[ts.cursor].Entry,
						IsUser:   ts.isUserMessage(ts.flatNodes[ts.cursor].Entry),
						UserText: ts.extractUserText(ts.flatNodes[ts.cursor].Entry),
					}
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if ts.search != "" {
				ts.search = ""
				ts.rebuildFlatList()
			} else {
				ts.cancelled = true
				ts.active = false
				return ts, func() tea.Msg {
					return TreeCancelledMsg{}
				}
			}

		// Filter cycle with ctrl+o.
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+o"))):
			ts.filter = (ts.filter + 1) % 5
			ts.rebuildFlatList()

		// Direct filter shortcuts.
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+d"))):
			ts.filter = TreeFilterDefault
			ts.rebuildFlatList()
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+t"))):
			ts.filter = TreeFilterNoTools
			ts.rebuildFlatList()
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+u"))):
			ts.filter = TreeFilterUserOnly
			ts.rebuildFlatList()
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+l"))):
			ts.filter = TreeFilterLabelOnly
			ts.rebuildFlatList()
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+a"))):
			ts.filter = TreeFilterAll
			ts.rebuildFlatList()

		default:
			// Typing search.
			if msg.Text != "" && len(msg.Text) == 1 {
				ch := msg.Text[0]
				if ch >= 32 && ch < 127 {
					ts.search += string(ch)
					ts.rebuildFlatList()
				}
			}
			if key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))) && len(ts.search) > 0 {
				ts.search = ts.search[:len(ts.search)-1]
				ts.rebuildFlatList()
			}
		}
	}
	return ts, nil
}

// View implements tea.Model.
func (ts *TreeSelectorComponent) View() tea.View {
	theme := GetTheme()

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Accent).
		PaddingLeft(2)

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		PaddingLeft(2)

	var b strings.Builder

	// Header.
	b.WriteString(headerStyle.Render("Session Tree"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: move  ←/→: page  enter: select  esc: cancel  ^O: cycle filter"))
	b.WriteString("\n")

	if ts.search != "" {
		searchStyle := lipgloss.NewStyle().Foreground(theme.Info).PaddingLeft(2)
		b.WriteString(searchStyle.Render(fmt.Sprintf("Search: %s", ts.search)))
		b.WriteString("\n")
	}

	b.WriteString(lipgloss.NewStyle().Foreground(theme.Muted).Render(strings.Repeat("─", ts.width)))
	b.WriteString("\n")

	if len(ts.flatNodes) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(theme.Muted).PaddingLeft(2)
		b.WriteString(emptyStyle.Render("No entries in session"))
		b.WriteString("\n")
	} else {
		// Compute visible window.
		visH := ts.visibleHeight()
		startIdx := 0
		if ts.cursor >= visH {
			startIdx = ts.cursor - visH + 1
		}
		endIdx := min(startIdx+visH, len(ts.flatNodes))

		for i := startIdx; i < endIdx; i++ {
			node := ts.flatNodes[i]
			line := ts.renderNode(node, i == ts.cursor, node.ID == ts.leafID)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Footer.
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Muted).Render(strings.Repeat("─", ts.width)))
	b.WriteString("\n")

	footerStyle := lipgloss.NewStyle().Foreground(theme.Muted).PaddingLeft(2)
	footer := fmt.Sprintf("(%d/%d) [%s]", ts.cursor+1, len(ts.flatNodes), ts.filter)
	b.WriteString(footerStyle.Render(footer))

	return tea.NewView(b.String())
}

// IsActive returns whether the tree selector is still accepting input.
func (ts *TreeSelectorComponent) IsActive() bool {
	return ts.active
}

// --- Internal helpers ---

func (ts *TreeSelectorComponent) visibleHeight() int {
	// Reserve lines for header(3) + search(1) + separator(1) + footer(2).
	h := max(ts.height/2-7, 5)
	return h
}

func (ts *TreeSelectorComponent) rebuildFlatList() {
	tree := ts.tm.GetTree()
	ts.flatNodes = ts.flatNodes[:0]
	for i, root := range tree {
		isLast := i == len(tree)-1
		ts.flattenNode(root, 0, isLast, "")
	}

	// Apply search filter.
	if ts.search != "" {
		query := strings.ToLower(ts.search)
		filtered := make([]FlatNode, 0)
		for _, node := range ts.flatNodes {
			text := ts.entryDisplayText(node.Entry)
			if strings.Contains(strings.ToLower(text), query) {
				filtered = append(filtered, node)
			}
		}
		ts.flatNodes = filtered
	}

	// Clamp cursor.
	if ts.cursor >= len(ts.flatNodes) {
		ts.cursor = max(len(ts.flatNodes)-1, 0)
	}
}

func (ts *TreeSelectorComponent) flattenNode(node *session.TreeNode, depth int, isLast bool, gutterPrefix string) {
	if !ts.passesFilter(node) {
		// Still recurse into children in case they pass.
		for i, child := range node.Children {
			childIsLast := i == len(node.Children)-1
			ts.flattenNode(child, depth, childIsLast, gutterPrefix)
		}
		return
	}

	var prefix string
	if depth == 0 {
		prefix = ""
	} else if isLast {
		prefix = gutterPrefix + "└─ "
	} else {
		prefix = gutterPrefix + "├─ "
	}

	label := ts.tm.GetLabel(node.ID)

	ts.flatNodes = append(ts.flatNodes, FlatNode{
		Entry:    node.Entry,
		ID:       node.ID,
		ParentID: node.ParentID,
		Depth:    depth,
		IsLast:   isLast,
		Prefix:   prefix,
		Label:    label,
	})

	// Build gutter prefix for children.
	var childGutter string
	if depth == 0 {
		childGutter = ""
	} else if isLast {
		childGutter = gutterPrefix + "   "
	} else {
		childGutter = gutterPrefix + "│  "
	}

	for i, child := range node.Children {
		childIsLast := i == len(node.Children)-1
		ts.flattenNode(child, depth+1, childIsLast, childGutter)
	}
}

func (ts *TreeSelectorComponent) passesFilter(node *session.TreeNode) bool {
	switch ts.filter {
	case TreeFilterAll:
		return true

	case TreeFilterDefault:
		// Hide settings entries.
		switch node.Entry.(type) {
		case *session.ModelChangeEntry, *session.LabelEntry, *session.SessionInfoEntry:
			return false
		}
		// Hide tool messages unless they're the leaf.
		if me, ok := node.Entry.(*session.MessageEntry); ok {
			if me.Role == "tool" && node.ID != ts.leafID {
				return false
			}
		}
		return true

	case TreeFilterNoTools:
		if me, ok := node.Entry.(*session.MessageEntry); ok {
			return me.Role != "tool"
		}
		return true

	case TreeFilterUserOnly:
		if me, ok := node.Entry.(*session.MessageEntry); ok {
			return me.Role == "user"
		}
		return false

	case TreeFilterLabelOnly:
		return ts.tm.GetLabel(node.ID) != ""

	default:
		return true
	}
}

func (ts *TreeSelectorComponent) renderNode(node FlatNode, isCursor, isLeaf bool) string {
	theme := GetTheme()
	maxWidth := ts.width - 4

	// Cursor indicator.
	var cursor string
	if isCursor {
		cursor = lipgloss.NewStyle().Foreground(theme.Accent).Render("› ")
	} else {
		cursor = "  "
	}

	// Role-colored content.
	text := ts.entryDisplayText(node.Entry)
	if len(text) > maxWidth-len(node.Prefix)-10 {
		trimLen := maxWidth - len(node.Prefix) - 13
		if trimLen > 0 && trimLen < len(text) {
			text = text[:trimLen] + "..."
		}
	}

	var style lipgloss.Style
	switch e := node.Entry.(type) {
	case *session.MessageEntry:
		switch e.Role {
		case "user":
			style = lipgloss.NewStyle().Foreground(theme.Accent)
		case "assistant":
			style = lipgloss.NewStyle().Foreground(theme.Success)
		default:
			style = lipgloss.NewStyle().Foreground(theme.Muted)
		}
	case *session.BranchSummaryEntry:
		style = lipgloss.NewStyle().Foreground(theme.Warning).Italic(true)
	default:
		style = lipgloss.NewStyle().Foreground(theme.Muted)
	}

	if isCursor {
		style = style.Bold(true)
	}

	content := style.Render(text)

	// Label badge.
	var labelBadge string
	if node.Label != "" {
		labelBadge = " " + lipgloss.NewStyle().Foreground(theme.Warning).Render("["+node.Label+"]")
	}

	// Active marker.
	var activeMarker string
	if isLeaf {
		activeMarker = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render(" ← active")
	}

	// Prefix (tree lines).
	prefixStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	renderedPrefix := prefixStyle.Render(node.Prefix)

	return cursor + renderedPrefix + content + labelBadge + activeMarker
}

func (ts *TreeSelectorComponent) entryDisplayText(entry any) string {
	switch e := entry.(type) {
	case *session.MessageEntry:
		role := e.Role
		text := extractTextFromParts(e.Parts)
		if len(text) > 80 {
			text = text[:80] + "..."
		}
		if text == "" {
			// Tool call messages may not have text.
			text = "(tool interaction)"
		}
		return fmt.Sprintf("%s: %s", role, text)

	case *session.ModelChangeEntry:
		return fmt.Sprintf("model: %s/%s", e.Provider, e.ModelID)

	case *session.BranchSummaryEntry:
		summary := e.Summary
		if len(summary) > 60 {
			summary = summary[:60] + "..."
		}
		return fmt.Sprintf("branch summary: %s", summary)

	case *session.LabelEntry:
		return fmt.Sprintf("label: %s", e.Label)

	case *session.SessionInfoEntry:
		return fmt.Sprintf("name: %s", e.Name)

	default:
		return "(unknown entry)"
	}
}

func (ts *TreeSelectorComponent) isUserMessage(entry any) bool {
	if me, ok := entry.(*session.MessageEntry); ok {
		return me.Role == "user"
	}
	return false
}

func (ts *TreeSelectorComponent) extractUserText(entry any) string {
	if me, ok := entry.(*session.MessageEntry); ok && me.Role == "user" {
		return extractTextFromParts(me.Parts)
	}
	return ""
}

// extractTextFromParts extracts text content from type-tagged parts JSON.
func extractTextFromParts(partsJSON []byte) string {
	// Quick extraction without full unmarshal.
	var parts []struct {
		Type string `json:"type"`
		Data struct {
			Text string `json:"text"`
		} `json:"data"`
	}
	if err := json.Unmarshal(partsJSON, &parts); err != nil {
		return ""
	}
	var texts []string
	for _, p := range parts {
		if p.Type == "text" && p.Data.Text != "" {
			texts = append(texts, p.Data.Text)
		}
	}
	return strings.Join(texts, "\n")
}
