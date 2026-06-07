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
// tree as an ASCII art list with navigation and selection. It is a thin
// wrapper around PopupList (in FullScreen mode) — PopupList owns the cursor,
// search, scroll math, and chrome; TreeSelectorComponent supplies the
// filtered node list and a custom RenderItem that draws each tree node with
// its indentation prefix and role colors.
type TreeSelectorComponent struct {
	tm         *session.TreeManager
	flatNodes  []FlatNode // visible nodes (matches popup.Items() 1:1)
	filter     TreeFilterMode
	leafID     string // real leaf for "active" marker
	popup      *PopupList
	width      int
	height     int
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
	ts.initPopup()
	ts.rebuild()
	// Position cursor at the active leaf.
	for i, node := range ts.flatNodes {
		if node.ID == ts.leafID {
			ts.popup.SetCursor(i)
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
	ts.initPopup()
	ts.rebuild()
	// Position cursor at the last user message before the leaf.
	for i := len(ts.flatNodes) - 1; i >= 0; i-- {
		if ts.isUserMessage(ts.flatNodes[i].Entry) {
			ts.popup.SetCursor(i)
			break
		}
	}
	return ts
}

func (ts *TreeSelectorComponent) initPopup() {
	ts.popup = NewPopupList("Session Tree", nil, ts.width, ts.height)
	ts.popup.FullScreen = true
	ts.popup.FooterHint = "↑↓ nav • ←→ page • ↵ select • esc cancel • ^O filter • type to search"
	ts.popup.RenderItem = ts.renderNode
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
		ts.popup.SetSize(msg.Width, msg.Height)
		return ts, nil

	case tea.KeyPressMsg:
		// Tree-specific keys we handle ourselves before delegating to popup.
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "pgup"))):
			// Page up.
			result := ts.popup.HandleKey("pgup", "")
			_ = result
			return ts, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "pgdown"))):
			result := ts.popup.HandleKey("pgdown", "")
			_ = result
			return ts, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+o"))):
			ts.filter = (ts.filter + 1) % 5
			ts.rebuild()
			return ts, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+d"))):
			ts.filter = TreeFilterDefault
			ts.rebuild()
			return ts, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+t"))):
			ts.filter = TreeFilterNoTools
			ts.rebuild()
			return ts, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+u"))):
			ts.filter = TreeFilterUserOnly
			ts.rebuild()
			return ts, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+l"))):
			ts.filter = TreeFilterLabelOnly
			ts.rebuild()
			return ts, nil
		}

		// Delegate everything else (nav, search, enter, esc) to the popup.
		result := ts.popup.HandleKey(msg.String(), msg.Text)

		// Update our flatNodes view if popup filtered/changed search.
		if result.Changed {
			ts.syncFlatNodes()
		}

		if result.Selected != nil {
			cursor := ts.popup.Cursor()
			if cursor < len(ts.flatNodes) {
				node := ts.flatNodes[cursor]
				ts.selectedID = node.ID
				ts.active = false
				return ts, func() tea.Msg {
					return core.TreeNodeSelectedMsg{
						ID:       node.ID,
						Entry:    node.Entry,
						IsUser:   ts.isUserMessage(node.Entry),
						UserText: ts.extractUserText(node.Entry),
					}
				}
			}
		}
		if result.Cancelled {
			ts.cancelled = true
			ts.active = false
			return ts, func() tea.Msg {
				return core.TreeCancelledMsg{}
			}
		}
	}
	return ts, nil
}

// View implements tea.Model.
func (ts *TreeSelectorComponent) View() tea.View {
	// Update extra footer with current filter mode.
	ts.popup.ExtraFooter = fmt.Sprintf("[%s]", ts.filter)
	rendered := ts.popup.RenderCentered(ts.width, ts.height)
	v := tea.NewView(rendered)
	v.AltScreen = true
	return v
}

// IsActive returns whether the tree selector is still accepting input.
func (ts *TreeSelectorComponent) IsActive() bool {
	return ts.active
}

// --- Internal helpers ---

// rebuild reflattens the tree under the current filter and reseeds the popup
// with PopupItems. Called on initial load and whenever the filter changes.
func (ts *TreeSelectorComponent) rebuild() {
	ts.flatNodes = ts.flatNodes[:0]
	tree := ts.tm.GetTree()
	for i, root := range tree {
		isLast := i == len(tree)-1
		ts.flattenNode(root, 0, isLast, "")
	}
	ts.publishItems()
}

// syncFlatNodes refreshes flatNodes from the popup's current filtered view.
// Called after a search-driven HandleKey result so the cursor index matches.
func (ts *TreeSelectorComponent) syncFlatNodes() {
	items := ts.popup.Items()
	newFlat := make([]FlatNode, len(items))
	for i, it := range items {
		if fn, ok := it.Meta.(FlatNode); ok {
			newFlat[i] = fn
		}
	}
	ts.flatNodes = newFlat
}

// publishItems converts flatNodes → PopupItems and seeds the popup. We rely
// on PopupList's default substring filter against item.Label (which holds
// the display text) for search.
func (ts *TreeSelectorComponent) publishItems() {
	items := make([]PopupItem, len(ts.flatNodes))
	for i, n := range ts.flatNodes {
		items[i] = PopupItem{
			Label:  ts.entryDisplayText(n.Entry),
			Active: n.ID == ts.leafID,
			Meta:   n,
		}
	}
	ts.popup.SetItems(items)
	// Mirror the popup's current view in flatNodes so cursor lookups work.
	ts.syncFlatNodes()
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

// renderNode is the RenderItem callback handed to PopupList. PopupList wraps
// the returned string with a full-width row style.
//
// When isCursor we return a plain (unstyled) string so the outer row style
// can paint a single continuous fg+bg span across the line. Composing inner
// lipgloss.Render calls emits ANSI resets mid-string which knock the
// background back out, breaking the highlight into disjoint bars (issue
// observed with deep tool-interaction branches).
func (ts *TreeSelectorComponent) renderNode(item PopupItem, innerWidth int, isCursor bool) string {
	theme := GetTheme()
	node, ok := item.Meta.(FlatNode)
	if !ok {
		return item.Label
	}
	isLeaf := node.ID == ts.leafID

	// Indicator (2 cells).
	indicator := "  "
	if isCursor {
		indicator = "> "
	}

	// Prefix (tree art) — width measured in display cells via lipgloss.
	prefix := node.Prefix
	prefixW := lipgloss.Width(prefix)

	// Compute right-side fixed parts: label badge + active marker.
	var labelBadgeRaw, activeMarkerRaw string
	if node.Label != "" {
		labelBadgeRaw = " [" + node.Label + "]"
	}
	if isLeaf {
		activeMarkerRaw = " ← active"
	}
	rightW := lipgloss.Width(labelBadgeRaw) + lipgloss.Width(activeMarkerRaw)

	// If the tree prefix is so deep it would push the text off the row,
	// truncate the prefix from the LEFT and prepend an ellipsis. Keeping
	// the right-most segment preserves the most recent depth indicator
	// (└─ / ├─) so the user can still see this row's connection to its
	// parent. We reserve at least 20 cells for the actual entry text.
	const minTextWidth = 20
	budget := innerWidth - 2 - rightW - minTextWidth
	if prefixW > budget && budget > 2 {
		runes := []rune(prefix)
		// Strip from the left until lipgloss.Width fits the budget.
		for len(runes) > 0 && lipgloss.Width(string(runes)) > budget-1 {
			runes = runes[1:]
		}
		prefix = "…" + string(runes)
		prefixW = lipgloss.Width(prefix)
	}

	// Reserve space for indicator(2) + prefix + right parts.
	available := max(innerWidth-2-prefixW-rightW, 4)

	text := ts.entryDisplayText(node.Entry)
	text = truncateRunes(text, available)

	// Selected row: emit raw text. The outer row style applies fg+bg in one
	// uninterrupted span, keeping the highlight solid edge-to-edge.
	if isCursor {
		return indicator + prefix + text + labelBadgeRaw + activeMarkerRaw
	}

	// Role-based text color.
	var textStyle lipgloss.Style
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

	prefixStyle := lipgloss.NewStyle().Foreground(theme.MutedBorder)
	labelStyle := lipgloss.NewStyle().Foreground(theme.Warning)
	markerStyle := lipgloss.NewStyle().Foreground(theme.Success).Bold(true)

	parts := indicator + prefixStyle.Render(prefix) + textStyle.Render(text)
	if labelBadgeRaw != "" {
		parts += labelStyle.Render(labelBadgeRaw)
	}
	if activeMarkerRaw != "" {
		parts += markerStyle.Render(activeMarkerRaw)
	}
	return parts
}

func (ts *TreeSelectorComponent) entryDisplayText(entry any) string {
	switch e := entry.(type) {
	case *session.MessageEntry:
		role := e.Role
		text := collapseToLine(extractTextFromParts(e.Parts))
		text = truncateRunes(text, 200)
		if text == "" {
			text = "(tool interaction)"
		}
		return fmt.Sprintf("%s: %s", role, text)

	case *session.ModelChangeEntry:
		return fmt.Sprintf("model: %s/%s", e.Provider, e.ModelID)

	case *session.BranchSummaryEntry:
		return fmt.Sprintf("branch summary: %s", truncateRunes(collapseToLine(e.Summary), 200))

	case *session.CompactionEntry:
		return fmt.Sprintf("compaction: %s", truncateRunes(collapseToLine(e.Summary), 200))

	case *session.LabelEntry:
		return fmt.Sprintf("label: %s", e.Label)

	case *session.SessionInfoEntry:
		return fmt.Sprintf("name: %s", e.Name)

	default:
		return "(unknown entry)"
	}
}

// collapseToLine flattens any multi-line string into a single line by
// replacing whitespace runs (including newlines and tabs) with single
// spaces. Used so popup rows never wrap and break the layout.
func collapseToLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
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
