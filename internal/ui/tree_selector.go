package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/session"
	"github.com/mark3labs/kit/internal/ui/core"
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
// tree as an ASCII art list with navigation and selection.
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

// NewTreeSelectorForFork creates a tree selector for the /fork command.
// It shows only user messages (flat list) matching Pi's fork behavior.
func NewTreeSelectorForFork(tm *session.TreeManager, width, height int) *TreeSelectorComponent {
	ts := &TreeSelectorComponent{
		tm:     tm,
		filter: TreeFilterUserOnly,
		leafID: tm.GetLeafID(),
		width:  width,
		height: height,
		active: true,
	}
	ts.rebuildFlatList()
	// Position cursor at the last user message before the leaf.
	for i := len(ts.flatNodes) - 1; i >= 0; i-- {
		if ts.isUserMessage(ts.flatNodes[i].Entry) {
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
		case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
			if ts.cursor > 0 {
				ts.cursor--
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
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
					return core.TreeNodeSelectedMsg{
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
					return core.TreeCancelledMsg{}
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

	// Full-screen bordered container - uses entire terminal width and height
	maxWidth := ts.width - 2 // Small margin on each side
	if maxWidth < 20 {
		maxWidth = ts.width
	}
	maxHeight := ts.height - 2 // Small margin top/bottom to prevent overflow
	if maxHeight < 10 {
		maxHeight = ts.height
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

	// Header style with background highlight (like PopupList title)
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Accent).
		Background(theme.Background)

	// Help text style
	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Background(theme.Background)

	var contentBuilder strings.Builder

	// Header row with title and help
	headerRow := headerStyle.Render("Session Tree")
	contentBuilder.WriteString(headerRow)
	contentBuilder.WriteString("\n")

	// Help text - adapt to terminal width
	var helpText string
	if ts.width >= 70 {
		helpText = "↑/↓: move  ←/→: page  enter: select  esc: cancel  ^O: cycle filter"
	} else if ts.width >= 45 {
		helpText = "↑↓ move  ↵ select  esc cancel  ^O filter"
	} else {
		helpText = "↑↓ ↵ esc ^O"
	}
	contentBuilder.WriteString(helpStyle.Render(helpText))
	contentBuilder.WriteString("\n")

	// Search display (if active)
	if ts.search != "" {
		searchStyle := lipgloss.NewStyle().
			Foreground(theme.Info).
			Background(theme.Background)
		contentBuilder.WriteString(searchStyle.Render(fmt.Sprintf("> %s", ts.search)))
		contentBuilder.WriteString("\n")
	}

	// Separator line - full width
	sepWidth := innerWidth
	contentBuilder.WriteString(
		lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(theme.Background).
			Render(strings.Repeat("─", sepWidth)))
	contentBuilder.WriteString("\n")

	// Tree content
	if len(ts.flatNodes) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(theme.Background)
		contentBuilder.WriteString(emptyStyle.Render("No entries in session"))
		contentBuilder.WriteString("\n")
	} else {
		// Compute visible window based on inner container height
		// Chrome: header(2) + separator(1) + footer separator(1) + footer(1) = 5
		chromeLines := 5
		if ts.search != "" {
			chromeLines++
		}
		visH := max(innerHeight-chromeLines, 3)

		startIdx := 0
		if ts.cursor >= visH {
			startIdx = ts.cursor - visH + 1
		}
		endIdx := min(startIdx+visH, len(ts.flatNodes))

		for i := startIdx; i < endIdx; i++ {
			node := ts.flatNodes[i]
			line := ts.renderNode(node, i == ts.cursor, node.ID == ts.leafID, innerWidth)
			contentBuilder.WriteString(line)
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

	// Footer with count and filter
	footerStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Background(theme.Background)
	footer := fmt.Sprintf("(%d/%d) [%s]", ts.cursor+1, len(ts.flatNodes), ts.filter)
	contentBuilder.WriteString(footerStyle.Render(footer))

	// Apply the bordered container - full width, no centering
	content := contentBuilder.String()
	borderedContent := containerStyle.Render(content)

	v := tea.NewView(borderedContent)
	v.AltScreen = true
	return v
}

// IsActive returns whether the tree selector is still accepting input.
func (ts *TreeSelectorComponent) IsActive() bool {
	return ts.active
}

// --- Internal helpers ---

func (ts *TreeSelectorComponent) visibleHeight() int {
	// Chrome: header(1) + help(1) + separator(1) + entries + separator(1) + footer(1) = 5 fixed.
	// Optional search line adds 1 more. Use 7 as a safe estimate.
	const chromeLines = 7
	return max(ts.height-chromeLines, 3)
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

func (ts *TreeSelectorComponent) renderNode(node FlatNode, isCursor, isLeaf bool, innerWidth int) string {
	theme := GetTheme()

	// Cursor indicator - use ">" for selected (like PopupList)
	var cursor string
	if isCursor {
		cursor = lipgloss.NewStyle().Foreground(theme.Accent).Render("> ")
	} else {
		cursor = "  "
	}

	// Role-colored content with background support for selection
	text := ts.entryDisplayText(node.Entry)

	// Calculate available width accounting for cursor, prefix, and markers
	prefixLen := len(node.Prefix)
	available := innerWidth - prefixLen - 4 // 4 for cursor and some padding
	if available > 3 && len(text) > available {
		trimLen := max(available-3, 1)
		if trimLen < len(text) {
			text = text[:trimLen] + "..."
		}
	}

	// Build the full line style
	var lineStyle lipgloss.Style
	var textStyle lipgloss.Style

	// Base text color based on role
	switch e := node.Entry.(type) {
	case *session.MessageEntry:
		switch e.Role {
		case "user":
			textStyle = lipgloss.NewStyle().Foreground(theme.Accent)
		case "assistant":
			textStyle = lipgloss.NewStyle().Foreground(theme.Success)
		default:
			textStyle = lipgloss.NewStyle().Foreground(theme.Muted)
		}
	case *session.BranchSummaryEntry:
		textStyle = lipgloss.NewStyle().Foreground(theme.Warning).Italic(true)
	case *session.CompactionEntry:
		textStyle = lipgloss.NewStyle().Foreground(theme.Info).Italic(true)
	default:
		textStyle = lipgloss.NewStyle().Foreground(theme.Muted)
	}

	// Apply selection highlighting (like PopupList)
	if isCursor {
		// Inverted colors for selected item - matches PopupList style
		lineStyle = lipgloss.NewStyle().
			Background(theme.Primary).
			Foreground(theme.Background).
			Bold(true)
		textStyle = lipgloss.NewStyle().
			Background(theme.Primary).
			Foreground(theme.Background).
			Bold(true)
	}

	// Render components
	content := textStyle.Render(text)

	// Label badge.
	var labelBadge string
	if node.Label != "" {
		labelStyle := lipgloss.NewStyle().Foreground(theme.Warning)
		if isCursor {
			labelStyle = lipgloss.NewStyle().
				Background(theme.Primary).
				Foreground(theme.Warning)
		}
		labelBadge = " " + labelStyle.Render("["+node.Label+"]")
	}

	// Active marker - use Success color for better visibility
	var activeMarker string
	if isLeaf {
		markerStyle := lipgloss.NewStyle().Foreground(theme.Success).Bold(true)
		if isCursor {
			markerStyle = lipgloss.NewStyle().
				Background(theme.Primary).
				Foreground(theme.Success).
				Bold(true)
		}
		activeMarker = markerStyle.Render(" ← active")
	}

	// Prefix (tree lines) - use MutedBorder for subtler appearance
	prefixStyle := lipgloss.NewStyle().Foreground(theme.MutedBorder)
	if isCursor {
		prefixStyle = lipgloss.NewStyle().
			Background(theme.Primary).
			Foreground(theme.MutedBorder)
	}
	renderedPrefix := prefixStyle.Render(node.Prefix)

	// Combine all parts
	line := cursor + renderedPrefix + content + labelBadge + activeMarker

	// If selected, apply the background to the entire line
	if isCursor {
		return lineStyle.Render(line)
	}

	return line
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

	case *session.CompactionEntry:
		summary := e.Summary
		if len(summary) > 60 {
			summary = summary[:60] + "..."
		}
		return fmt.Sprintf("compaction: %s", summary)

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
