package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/ui/style"
)

// PopupItem represents a single entry in a PopupList. The component renders
// Label as the primary text and Description as secondary text to its right.
// The Active flag renders a checkmark to indicate the currently-active item
// (e.g. the current model). Meta is opaque caller data returned on selection.
type PopupItem struct {
	Label       string // primary display text
	Description string // secondary text (shown right of label)
	Active      bool   // true → render checkmark indicator
	Meta        any    // opaque data returned on selection
}

// PopupList is a generic, themed, scrollable popup list used by every
// list-style popup in the TUI (slash commands, @file autocomplete, model
// picker, session picker, tree navigation, etc.).
//
// Two layout modes:
//   - Centered (default): bordered ~80-col box centered on the screen. Used
//     for the input-bar popups (/ and @) and the model picker.
//   - FullScreen: bordered panel filling almost the entire terminal. Used by
//     /tree, /fork, /sessions and other browse-many-items popups.
//
// Two usage modes:
//   - Internal state: caller creates the list with items, calls HandleKey for
//     navigation/search, and PopupList owns the cursor and search string.
//     Used by selectors like ModelSelector, TreeSelector, SessionSelector.
//   - External state: caller drives the items / cursor / search themselves
//     (e.g. InputComponent, where typing in the textarea filters the list).
//     Caller uses SetItems / SetCursor / SetSearch and only calls Render.
type PopupList struct {
	// Title shown at the top of the popup.
	Title string
	// Subtitle shown below the title (dimmed).
	Subtitle string
	// FooterHint overrides the default keyboard-hint footer.
	FooterHint string
	// ExtraFooter is appended to the footer line (after the default hint).
	// Used by selectors to surface mode info like the active filter.
	ExtraFooter string

	// FullScreen renders the popup at almost the full terminal size instead
	// of a centered ~80-col box. Used by tree/session/fork selectors.
	FullScreen bool

	// ShowSearch toggles the "> <query>" search input line. Default true.
	ShowSearch bool

	// HideCount suppresses the "(i/N)" count in the footer.
	HideCount bool

	// MaxVisible caps the number of items visible at once. 0 = derive from
	// available height.
	MaxVisible int

	// RenderItem optionally renders a single item row. When nil, the
	// built-in label + description + active-checkmark renderer is used.
	// innerWidth is the usable line width inside the popup (after border
	// and padding). The returned string must already be styled — the
	// shared selection-row background is applied by the popup only when
	// RenderItem is nil.
	RenderItem func(item PopupItem, innerWidth int, isCursor bool) string

	// FilterFunc is called with (query, allItems) and should return the
	// filtered+scored subset. When nil, a default substring + fuzzy match
	// is used. Only consulted in internal-state mode (via HandleKey).
	FilterFunc func(query string, items []PopupItem) []PopupItem

	allItems []PopupItem // full unfiltered list (internal-state mode)
	filtered []PopupItem // items currently rendered (driven by FilterFunc
	// in internal-state mode, or set directly via SetItems in external mode)
	cursor int
	search string

	width  int
	height int
}

// PopupResult is returned by HandleKey to tell the caller what happened.
type PopupResult struct {
	// Selected is non-nil when the user pressed Enter on an item.
	Selected *PopupItem
	// Cancelled is true when the user pressed Esc with no search text.
	Cancelled bool
	// Changed is true when the search or cursor moved (caller should re-render).
	Changed bool
}

// NewPopupList creates a new popup list with the given items and dimensions.
func NewPopupList(title string, items []PopupItem, width, height int) *PopupList {
	p := &PopupList{
		Title:      title,
		allItems:   items,
		filtered:   items,
		width:      width,
		height:     height,
		ShowSearch: true,
	}
	// Position cursor on the active item if one exists.
	for i, item := range p.filtered {
		if item.Active {
			p.cursor = i
			break
		}
	}
	return p
}

// SetSize updates the popup dimensions (e.g. on window resize).
func (p *PopupList) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// SetItems replaces the displayed item list and clamps the cursor. Used by
// external-state callers (e.g. InputComponent) that filter items themselves.
// In internal-state mode, this also replaces the unfiltered backing list.
func (p *PopupList) SetItems(items []PopupItem) {
	p.allItems = items
	p.filtered = items
	if p.cursor >= len(p.filtered) {
		p.cursor = max(len(p.filtered)-1, 0)
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

// SetCursor moves the selection to the given index (clamped to range).
func (p *PopupList) SetCursor(i int) {
	if len(p.filtered) == 0 {
		p.cursor = 0
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(p.filtered) {
		i = len(p.filtered) - 1
	}
	p.cursor = i
}

// Cursor returns the current selection index.
func (p *PopupList) Cursor() int { return p.cursor }

// SetSearch replaces the search string without rebuilding the filtered list.
// Used by external-state callers that filter items themselves.
func (p *PopupList) SetSearch(s string) { p.search = s }

// Items returns the currently-visible (filtered) items.
func (p *PopupList) Items() []PopupItem { return p.filtered }

// Search returns the current search string.
func (p *PopupList) Search() string { return p.search }

// dimensions returns the (popupWidth, popupHeight, innerWidth, innerHeight)
// the popup will render at, given its current size and FullScreen flag.
func (p *PopupList) dimensions() (popupW, popupH, innerW, innerH int) {
	if p.FullScreen {
		// Leave a small margin so the border doesn't kiss the screen edge.
		popupW = max(p.width-2, 20)
		popupH = max(p.height-2, 10)
	} else {
		// Centered: cap at 80 cols, leave a 4-col margin.
		popupW = max(min(p.width-4, 80), 20)
		// Height is dynamic — let it grow with content within the screen.
		popupH = 0
	}
	// Border (2) + horizontal padding (4) = 6 chrome cols.
	innerW = max(popupW-6, 10)
	if popupH > 0 {
		// Border (2) + vertical padding (2) = 4 chrome rows.
		innerH = max(popupH-4, 6)
	}
	return
}

// visibleCount returns the number of items visible at once.
func (p *PopupList) visibleCount() int {
	if p.MaxVisible > 0 {
		return p.MaxVisible
	}
	if p.FullScreen {
		_, _, _, innerH := p.dimensions()
		// Reserve: title(1) + subtitle(0|1) + search(0|2) + sep(1) + footer(2)
		overhead := 4
		if p.Subtitle != "" {
			overhead++
		}
		if p.ShowSearch {
			overhead += 2
		}
		return max(innerH-overhead, 3)
	}
	// Centered: derive from terminal height (legacy behaviour).
	overhead := 8
	if p.Subtitle != "" {
		overhead++
	}
	if p.ShowSearch {
		overhead += 2
	}
	return max(p.height/2-overhead, 3)
}

// HandleKey processes a single key event and returns the result. The caller
// should inspect PopupResult to decide whether to re-render, close the popup,
// or act on a selection. Internal-state mode only — external-state callers
// drive cursor/search themselves and never call this.
//
// keyName is the Bubble Tea key string (e.g. "up", "down", "enter", "esc").
// keyText is the printable text for character keys (e.g. "a", "1").
func (p *PopupList) HandleKey(keyName, keyText string) PopupResult {
	switch keyName {
	case "up":
		if p.cursor > 0 {
			p.cursor--
			return PopupResult{Changed: true}
		}
		return PopupResult{}

	case "down":
		if p.cursor < len(p.filtered)-1 {
			p.cursor++
			return PopupResult{Changed: true}
		}
		return PopupResult{}

	case "pgup":
		p.cursor -= p.visibleCount()
		if p.cursor < 0 {
			p.cursor = 0
		}
		return PopupResult{Changed: true}

	case "pgdown":
		p.cursor += p.visibleCount()
		if p.cursor >= len(p.filtered) {
			p.cursor = max(len(p.filtered)-1, 0)
		}
		return PopupResult{Changed: true}

	case "home":
		p.cursor = 0
		return PopupResult{Changed: true}

	case "end":
		p.cursor = max(len(p.filtered)-1, 0)
		return PopupResult{Changed: true}

	case "enter":
		if p.cursor < len(p.filtered) {
			item := p.filtered[p.cursor]
			return PopupResult{Selected: &item}
		}
		return PopupResult{}

	case "esc":
		if p.search != "" {
			p.search = ""
			p.rebuildFiltered()
			return PopupResult{Changed: true}
		}
		return PopupResult{Cancelled: true}

	case "backspace":
		if len(p.search) > 0 {
			p.search = p.search[:len(p.search)-1]
			p.rebuildFiltered()
			return PopupResult{Changed: true}
		}
		return PopupResult{}

	default:
		// Printable character → append to search.
		if keyText != "" && len(keyText) == 1 {
			ch := keyText[0]
			if ch >= 32 && ch < 127 {
				p.search += string(ch)
				p.rebuildFiltered()
				return PopupResult{Changed: true}
			}
		}
		return PopupResult{}
	}
}

// Render returns the styled popup content (bordered box) ready to be placed
// as a centered overlay via lipgloss.Place + overlayContent.
func (p *PopupList) Render() string {
	theme := style.GetTheme()
	popupW, popupH, innerW, _ := p.dimensions()
	popupBg := theme.Background

	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Background(popupBg).
		Padding(1, 2).
		Width(popupW)
	if popupH > 0 {
		popupStyle = popupStyle.Height(popupH)
	} else {
		popupStyle = popupStyle.MarginBottom(1)
	}

	var b strings.Builder

	// Title.
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Accent).
		Background(popupBg).
		Width(innerW)
	b.WriteString(titleStyle.Render(p.Title))
	b.WriteString("\n")

	// Subtitle.
	if p.Subtitle != "" {
		subtitleStyle := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(popupBg).
			Width(innerW)
		b.WriteString(subtitleStyle.Render(p.Subtitle))
		b.WriteString("\n")
	}

	// Search input.
	if p.ShowSearch {
		searchStyle := lipgloss.NewStyle().
			Foreground(theme.Info).
			Background(popupBg).
			Width(innerW)
		if p.search != "" {
			b.WriteString(searchStyle.Render(fmt.Sprintf("> %s", p.search)))
		} else {
			b.WriteString(searchStyle.Render("> "))
		}
		b.WriteString("\n")

		// Separator.
		sepStyle := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(popupBg)
		b.WriteString(sepStyle.Render(strings.Repeat("─", innerW)))
		b.WriteString("\n")
	}

	// Item list.
	normalItemBg := lipgloss.NewStyle().
		Background(popupBg).
		Foreground(theme.Text).
		Width(innerW).
		Padding(0, 1)

	selectedItemBg := lipgloss.NewStyle().
		Background(theme.Primary).
		Foreground(theme.Background).
		Width(innerW).
		Padding(0, 1).
		Bold(true)

	scrollStyle := lipgloss.NewStyle().
		Background(popupBg).
		Foreground(theme.VeryMuted).
		Width(innerW).
		Padding(0, 1)

	vis := p.visibleCount()
	var items []string

	if len(p.filtered) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(popupBg).
			Width(innerW).
			Padding(0, 1)
		if p.search != "" {
			items = append(items, emptyStyle.Render("No matches for \""+p.search+"\""))
		} else {
			items = append(items, emptyStyle.Render("No items"))
		}
	} else {
		// Center the cursor in the visible window so the user always sees
		// context above and below. Clamp to bounds.
		startIdx := 0
		if len(p.filtered) > vis {
			startIdx = max(p.cursor-vis/2, 0)
			if startIdx+vis > len(p.filtered) {
				startIdx = len(p.filtered) - vis
			}
		}
		endIdx := min(startIdx+vis, len(p.filtered))

		if startIdx > 0 {
			items = append(items, scrollStyle.Render("  ↑ more above"))
		}

		// Account for the consumed padding (1 left + 1 right = 2 cols)
		// when rendering item content so RenderItem callbacks can match.
		itemContentWidth := max(innerW-2, 6)

		for i := startIdx; i < endIdx; i++ {
			entry := p.filtered[i]
			isCursor := i == p.cursor

			if p.RenderItem != nil {
				// Custom renderer: caller produces the inner text. We still
				// wrap it in a full-width row so the selection highlight
				// covers the line edge-to-edge.
				rowStyle := normalItemBg
				if isCursor {
					rowStyle = selectedItemBg
				}
				content := p.RenderItem(entry, itemContentWidth, isCursor)
				items = append(items, rowStyle.Render(content))
				continue
			}

			itemStyle := normalItemBg
			if isCursor {
				itemStyle = selectedItemBg
			}

			// Build indicator.
			var indicator string
			if isCursor {
				indicator = "> "
			} else {
				indicator = "  "
			}

			// Build content: indicator + label + description + active checkmark.
			content := p.renderItemContent(indicator, entry, itemContentWidth, isCursor)
			items = append(items, itemStyle.Render(content))
		}

		if endIdx < len(p.filtered) {
			items = append(items, scrollStyle.Render("  ↓ more below"))
		}
	}

	content := b.String() + strings.Join(items, "\n")

	// Footer with count and keyboard hints.
	var footerParts []string
	if !p.HideCount {
		footerParts = append(footerParts, fmt.Sprintf("(%d/%d)", p.cursor+1, len(p.filtered)))
	}

	footerHint := p.FooterHint
	if footerHint == "" {
		if innerW >= 50 {
			footerHint = "↑↓ navigate • enter select • esc cancel • type to filter"
		} else if innerW >= 30 {
			footerHint = "↑↓ nav • ↵ select • esc"
		} else {
			footerHint = "↑↓ ↵ esc"
		}
	}
	footerParts = append(footerParts, footerHint)
	if p.ExtraFooter != "" {
		footerParts = append(footerParts, p.ExtraFooter)
	}

	footer := lipgloss.NewStyle().
		Background(popupBg).
		Foreground(theme.VeryMuted).
		Italic(true).
		Render(strings.Join(footerParts, "  "))

	return popupStyle.Render(content + "\n\n" + footer)
}

// RenderCentered returns the popup placed at the center of a termWidth×termHeight
// canvas, ready to be composed with overlayContent().
func (p *PopupList) RenderCentered(termWidth, termHeight int) string {
	popupContent := p.Render()
	return lipgloss.Place(
		termWidth,
		termHeight,
		lipgloss.Center,
		lipgloss.Center,
		popupContent,
	)
}

// IsSearching returns true when the search input is non-empty.
func (p *PopupList) IsSearching() bool {
	return p.search != ""
}

// SelectedItem returns the item under the cursor, or nil if the list is empty.
func (p *PopupList) SelectedItem() *PopupItem {
	if p.cursor < len(p.filtered) {
		item := p.filtered[p.cursor]
		return &item
	}
	return nil
}

// --- Internal helpers ---

func (p *PopupList) rebuildFiltered() {
	if p.FilterFunc != nil {
		p.filtered = p.FilterFunc(p.search, p.allItems)
	} else {
		p.filtered = defaultFilter(p.search, p.allItems)
	}
	// Clamp cursor.
	if p.cursor >= len(p.filtered) {
		p.cursor = max(len(p.filtered)-1, 0)
	}
}

// defaultFilter is a simple case-insensitive substring + fuzzy character match.
func defaultFilter(query string, items []PopupItem) []PopupItem {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	type scored struct {
		item  PopupItem
		score int
	}
	var matches []scored
	for _, item := range items {
		label := strings.ToLower(item.Label)
		desc := strings.ToLower(item.Description)

		var s int
		switch {
		case label == q:
			s = 1000
		case strings.HasPrefix(label, q):
			s = 800 - len(label) + len(q)
		case strings.Contains(label, q):
			s = 600
		case strings.Contains(desc, q):
			s = 400
		default:
			s = fuzzyCharacterMatch(q, label)
		}
		if s > 0 {
			matches = append(matches, scored{item: item, score: s})
		}
	}

	// Sort by score descending, then alphabetically by label.
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].score > matches[i].score ||
				(matches[j].score == matches[i].score && matches[j].item.Label < matches[i].item.Label) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	result := make([]PopupItem, len(matches))
	for i, m := range matches {
		result[i] = m.item
	}
	return result
}

// renderItemContent builds the display string for a single item row.
func (p *PopupList) renderItemContent(indicator string, entry PopupItem, innerWidth int, isCursor bool) string {
	theme := style.GetTheme()

	// Reserve space: indicator(2) + potential checkmark(2)
	activeWidth := 0
	if entry.Active {
		activeWidth = 2
	}
	available := max(innerWidth-2-activeWidth, 6) // 2 for indicator, already included

	label := entry.Label
	desc := entry.Description

	if desc != "" {
		// Two-column layout: label + description.
		descWidth := len([]rune(desc)) + 1 // 1 space gap
		labelMax := max(available-descWidth, available*2/3)
		if len([]rune(label)) > labelMax && labelMax > 3 {
			runes := []rune(label)
			label = string(runes[:labelMax-1]) + "…"
		}
		labelDisplayLen := len([]rune(label))

		// If label + desc don't fit, truncate or drop desc.
		if labelDisplayLen+1+len([]rune(desc)) > available {
			remaining := available - labelDisplayLen - 1
			if remaining >= 4 {
				runes := []rune(desc)
				if len(runes) > remaining {
					desc = string(runes[:remaining-1]) + "…"
				}
			} else {
				desc = ""
			}
		}
	} else {
		// Single column: just the label.
		if len([]rune(label)) > available && available > 3 {
			runes := []rune(label)
			label = string(runes[:available-1]) + "…"
		}
	}

	result := indicator + label
	if desc != "" {
		descStyle := lipgloss.NewStyle().Foreground(theme.Muted)
		if isCursor {
			// When selected, use a dimmer foreground that still contrasts with Primary bg.
			descStyle = lipgloss.NewStyle().Foreground(theme.Background)
		}
		result += " " + descStyle.Render(desc)
	}
	if entry.Active {
		checkStyle := lipgloss.NewStyle().Foreground(theme.Success)
		if isCursor {
			checkStyle = lipgloss.NewStyle().Foreground(theme.Background)
		}
		result += checkStyle.Render(" ✓")
	}
	return result
}
