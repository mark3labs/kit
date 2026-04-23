package ui

import (
	"strings"
	"time"

	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/selection"
)

// MessageItem is the interface all scrollback messages must implement.
// This allows lazy rendering - messages are only rendered when visible.
type MessageItem interface {
	// Render returns the styled content for this message at the given width.
	// Implementations should cache the result to avoid re-rendering.
	Render(width int) string

	// Height returns the number of lines this message occupies when rendered.
	Height() int

	// ID returns a unique identifier for this message (for tracking).
	ID() string
}

// ScrollList manages a viewport over a list of MessageItems.
// It handles offset-based scrolling, lazy rendering, and character-level
// text selection (crush-style). Only visible items are rendered on each View() call.
type ScrollList struct {
	items      []MessageItem
	offsetIdx  int // Index of first visible item
	offsetLine int // Lines to skip from first visible item
	width      int
	height     int  // Viewport height in lines
	autoScroll bool // Whether to auto-scroll to bottom on new content
	itemGap    int  // Number of blank lines between items (0 = no gap)

	// heightCache maps item ID → rendered line count at current width.
	// Avoids redundant Render() calls in GotoBottom/clampOffset/AtBottom.
	// Invalidated on width change; individual entries are refreshed in
	// View() when an item is actually rendered.
	heightCache map[string]int

	// Character-level text selection (crush-style).
	sel selection.State
}

// NewScrollList creates a new ScrollList with the given dimensions.
func NewScrollList(width, height int) *ScrollList {
	return &ScrollList{
		items:       []MessageItem{},
		offsetIdx:   0,
		offsetLine:  0,
		width:       width,
		height:      height,
		autoScroll:  true,
		heightCache: make(map[string]int, 64),
		sel:         selection.NewState(),
	}
}

// SetItems replaces the items in the scroll list. If auto-scroll is enabled,
// the viewport will scroll to the bottom to show the latest content.
func (s *ScrollList) SetItems(items []MessageItem) {
	s.items = items
	if s.autoScroll {
		s.GotoBottom()
	}
}

// InvalidateItemHeight removes the cached height for the given item ID,
// forcing a re-render on the next height query. Call this after mutating
// an item's content (e.g. AppendChunk on a streaming message).
func (s *ScrollList) InvalidateItemHeight(id string) {
	delete(s.heightCache, id)
}

// SetHeight updates the viewport height. Called when the terminal is resized.
func (s *ScrollList) SetHeight(height int) {
	s.height = height
	s.clampOffset()
}

// SetWidth updates the viewport width. Called when the terminal is resized.
// This invalidates the height cache since rendered heights are width-dependent.
func (s *ScrollList) SetWidth(width int) {
	s.width = width
	// Width change invalidates all cached heights.
	clear(s.heightCache)
	s.clampOffset()
}

// SetItemGap sets the number of blank lines between items (0 = no gap).
func (s *ScrollList) SetItemGap(gap int) {
	s.itemGap = gap
}

// ItemGap returns the current gap between items.
func (s *ScrollList) ItemGap() int {
	return s.itemGap
}

// --------------------------------------------------------------------------
// Mouse event handling — character-level text selection (crush-style)
// --------------------------------------------------------------------------

// HandleMouseDown handles mouse button press. Detects single, double, and
// triple clicks for character, word, and line selection respectively.
// Returns true if the click was handled.
func (s *ScrollList) HandleMouseDown(x, y int) bool {
	if len(s.items) == 0 {
		return false
	}

	itemIdx, lineIdx := s.getItemAndLineAtY(y)
	if itemIdx < 0 {
		return false
	}

	// Multi-click detection (crush-style).
	now := time.Now()
	if now.Sub(s.sel.LastClickTime) <= selection.DoubleClickThreshold &&
		abs(x-s.sel.LastClickX) <= selection.ClickTolerance &&
		abs(y-s.sel.LastClickY) <= selection.ClickTolerance {
		s.sel.ClickCount++
	} else {
		s.sel.ClickCount = 1
	}
	s.sel.LastClickTime = now
	s.sel.LastClickX = x
	s.sel.LastClickY = y

	switch s.sel.ClickCount {
	case 1:
		// Single click: start character-level drag selection.
		s.sel.MouseDown = true
		s.sel.MouseDownItemIdx = itemIdx
		s.sel.MouseDownLineIdx = lineIdx
		s.sel.MouseDownCol = x
		s.sel.DragItemIdx = itemIdx
		s.sel.DragLineIdx = lineIdx
		s.sel.DragCol = x

	case 2:
		// Double click: select word at position.
		s.selectWord(itemIdx, lineIdx, x)

	case 3:
		// Triple click: select entire line.
		s.selectLine(itemIdx, lineIdx)
		s.sel.ClickCount = 0 // Reset after triple
	}

	return true
}

// HandleMouseDrag handles mouse motion while button is held.
// Updates the selection endpoint for character-level precision.
// Returns true if selection was updated.
func (s *ScrollList) HandleMouseDrag(x, y int) bool {
	if !s.sel.MouseDown {
		return false
	}

	if len(s.items) == 0 {
		return false
	}

	itemIdx, lineIdx := s.getItemAndLineAtY(y)
	if itemIdx < 0 {
		return false
	}

	s.sel.DragItemIdx = itemIdx
	s.sel.DragLineIdx = lineIdx
	s.sel.DragCol = x

	return true
}

// HandleMouseUp handles mouse button release.
// Returns true if there was an active selection.
func (s *ScrollList) HandleMouseUp() bool {
	if !s.sel.MouseDown {
		return false
	}
	s.sel.MouseDown = false
	return s.sel.HasSelection()
}

// HasSelection returns true if there is a non-empty active selection.
func (s *ScrollList) HasSelection() bool {
	return s.sel.HasSelection()
}

// ClearSelection clears the current text selection.
func (s *ScrollList) ClearSelection() {
	s.sel.Clear()
}

// ExtractSelectedText returns the plain text content of the current selection
// by walking through selected items and extracting text at the character level
// using the ultraviolet cell buffer (ANSI-aware).
func (s *ScrollList) ExtractSelectedText() string {
	r := s.sel.GetRange()
	if r.IsEmpty() {
		return ""
	}

	var sb strings.Builder

	for itemIdx := r.StartItemIdx; itemIdx <= r.EndItemIdx && itemIdx < len(s.items); itemIdx++ {
		item := s.items[itemIdx]
		content := item.Render(s.width)
		contentLines := strings.Split(content, "\n")

		for lineIdx, line := range contentLines {
			inRange, startCol, endCol := selection.IsLineInRange(r, itemIdx, lineIdx)
			if !inRange {
				continue
			}

			text := selection.ExtractText(line, startCol, endCol)
			if text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(text)
			}
		}
	}

	return sb.String()
}

// selectWord selects the word at the given position using UAX#29 word
// segmentation and display-width-aware column calculations.
func (s *ScrollList) selectWord(itemIdx, lineIdx, x int) {
	if itemIdx < 0 || itemIdx >= len(s.items) {
		return
	}

	item := s.items[itemIdx]
	content := item.Render(s.width)
	lines := strings.Split(content, "\n")
	if lineIdx < 0 || lineIdx >= len(lines) {
		return
	}

	// Strip ANSI codes for word boundary detection.
	plainLine := xansi.Strip(lines[lineIdx])
	startCol, endCol := selection.FindWordBoundaries(plainLine, x)

	if startCol == endCol {
		// No word at this position — set up single-click drag state.
		s.sel.MouseDown = true
		s.sel.MouseDownItemIdx = itemIdx
		s.sel.MouseDownLineIdx = lineIdx
		s.sel.MouseDownCol = x
		s.sel.DragItemIdx = itemIdx
		s.sel.DragLineIdx = lineIdx
		s.sel.DragCol = x
		return
	}

	// Set selection to the word boundaries.
	s.sel.MouseDown = true
	s.sel.MouseDownItemIdx = itemIdx
	s.sel.MouseDownLineIdx = lineIdx
	s.sel.MouseDownCol = startCol
	s.sel.DragItemIdx = itemIdx
	s.sel.DragLineIdx = lineIdx
	s.sel.DragCol = endCol
}

// selectLine selects the entire line at the given position.
func (s *ScrollList) selectLine(itemIdx, lineIdx int) {
	if itemIdx < 0 || itemIdx >= len(s.items) {
		return
	}

	item := s.items[itemIdx]
	content := item.Render(s.width)
	lines := strings.Split(content, "\n")
	if lineIdx < 0 || lineIdx >= len(lines) {
		return
	}

	lineWidth := xansi.StringWidth(lines[lineIdx])

	s.sel.MouseDown = true
	s.sel.MouseDownItemIdx = itemIdx
	s.sel.MouseDownLineIdx = lineIdx
	s.sel.MouseDownCol = 0
	s.sel.DragItemIdx = itemIdx
	s.sel.DragLineIdx = lineIdx
	s.sel.DragCol = lineWidth
}

// getItemAndLineAtY converts a viewport-relative Y coordinate to item index
// and line index within that item. Accounts for scroll offset and item gaps.
// Returns (-1, -1) if Y is outside the viewport or beyond all items.
//
// IMPORTANT: Uses Render()+line counting (not Height()) to compute item height,
// because Height() on some MessageItem implementations (e.g. StreamingMessageItem
// for reasoning blocks) may return 0 when the render cache is empty.
func (s *ScrollList) getItemAndLineAtY(y int) (itemIdx, lineIdx int) {
	if y < 0 || y >= s.height || len(s.items) == 0 {
		return -1, -1
	}

	currentY := 0
	for idx := s.offsetIdx; idx < len(s.items); idx++ {
		item := s.items[idx]
		// Compute height the same way View() does: render, then count lines.
		itemHeight := s.renderedHeight(item)

		// Account for partial visibility of the first item.
		startLine := 0
		if idx == s.offsetIdx {
			startLine = s.offsetLine
			itemHeight -= s.offsetLine
		}

		if y >= currentY && y < currentY+itemHeight {
			return idx, (y - currentY) + startLine
		}

		currentY += itemHeight

		// Add gap after item (except last).
		if s.itemGap > 0 && idx < len(s.items)-1 {
			currentY += s.itemGap
		}

		if currentY >= s.height {
			break
		}
	}

	return -1, -1
}

// --------------------------------------------------------------------------
// Scrolling
// --------------------------------------------------------------------------

// ScrollBy scrolls the viewport by the given number of lines.
// Positive = scroll down, negative = scroll up.
func (s *ScrollList) ScrollBy(lines int) {
	if lines > 0 {
		// Scroll down
		for lines > 0 && s.offsetIdx < len(s.items) {
			if s.offsetIdx >= len(s.items) {
				break
			}
			ih := s.itemHeight(s.items[s.offsetIdx])
			remainingLines := ih - s.offsetLine

			if lines >= remainingLines {
				// Move to next item
				s.offsetIdx++
				s.offsetLine = 0
				lines -= remainingLines
				// Consume gap lines between items
				if s.itemGap > 0 && s.offsetIdx < len(s.items) {
					if lines >= s.itemGap {
						lines -= s.itemGap
					} else {
						lines = 0
					}
				}
			} else {
				// Stay on current item, skip more lines
				s.offsetLine += lines
				lines = 0
			}
		}
	} else if lines < 0 {
		// Scroll up
		lines = -lines
		for lines > 0 && (s.offsetIdx > 0 || s.offsetLine > 0) {
			if s.offsetLine > 0 {
				// Scroll within current item
				if lines >= s.offsetLine {
					lines -= s.offsetLine
					s.offsetLine = 0
				} else {
					s.offsetLine -= lines
					lines = 0
				}
			} else if s.offsetIdx > 0 {
				// Consume gap lines between items
				if s.itemGap > 0 {
					if lines > s.itemGap {
						lines -= s.itemGap
					} else {
						lines = 0
						continue
					}
				}
				// Move to previous item
				s.offsetIdx--
				if s.offsetIdx < len(s.items) {
					ih := s.itemHeight(s.items[s.offsetIdx])

					if lines >= ih {
						lines -= ih
						s.offsetLine = 0
					} else {
						s.offsetLine = ih - lines
						lines = 0
					}
				}
			}
		}
	}
	s.clampOffset()
}

// GotoBottom scrolls to the end of the list.
// Uses cached heights and walks backwards from the end to avoid rendering
// every item in the list.
func (s *ScrollList) GotoBottom() {
	if len(s.items) == 0 {
		s.offsetIdx = 0
		s.offsetLine = 0
		return
	}

	// Walk backwards from the last item, accumulating height until we
	// exceed the viewport. This is O(visible) instead of O(all items).
	budget := s.height
	for idx := len(s.items) - 1; idx >= 0; idx-- {
		ih := s.itemHeight(s.items[idx])

		// Account for gap *above* this item (gap between idx-1 and idx).
		gap := 0
		if s.itemGap > 0 && idx < len(s.items)-1 {
			gap = s.itemGap
		}

		if ih+gap >= budget {
			// This item (partially) fills the remaining budget.
			// When the gap consumed part of the budget, offsetLine would go
			// negative — clamp to 0 so the item is shown fully.
			s.offsetIdx = idx
			s.offsetLine = max(0, ih-budget)
			return
		}
		budget -= ih + gap
	}

	// All content fits in viewport — start at top.
	s.offsetIdx = 0
	s.offsetLine = 0
}

// GotoTop scrolls to the beginning of the list.
func (s *ScrollList) GotoTop() {
	s.offsetIdx = 0
	s.offsetLine = 0
}

// AtBottom returns true if the viewport is at the bottom of the list.
func (s *ScrollList) AtBottom() bool {
	if len(s.items) == 0 {
		return true
	}

	visibleHeight := 0
	for idx := s.offsetIdx; idx < len(s.items); idx++ {
		ih := s.itemHeight(s.items[idx])

		if idx == s.offsetIdx {
			visibleHeight += ih - s.offsetLine
		} else {
			visibleHeight += ih
		}

		if s.itemGap > 0 && idx < len(s.items)-1 {
			visibleHeight += s.itemGap
		}

		if visibleHeight >= s.height {
			return false
		}
	}

	return true
}

// AtTop returns true if the viewport is at the top of the list.
func (s *ScrollList) AtTop() bool {
	return s.offsetIdx == 0 && s.offsetLine == 0
}

// --------------------------------------------------------------------------
// Rendering
// --------------------------------------------------------------------------

// View renders the visible portion of the scrollback.
// Only items that fit within the viewport height are rendered.
// ALWAYS returns exactly s.height lines (padded with empty lines if needed)
// to ensure the input/footer stay fixed at the bottom.
//
// When an active selection exists, character-level highlighting is applied
// using ultraviolet ScreenBuffer for ANSI-aware cell manipulation.
func (s *ScrollList) View() string {
	if s.height <= 0 {
		return ""
	}

	selRange := s.sel.GetRange()
	hasSelection := !selRange.IsEmpty()

	var lines []string
	remainingHeight := s.height

	if len(s.items) > 0 {
		for idx := s.offsetIdx; idx < len(s.items) && remainingHeight > 0; idx++ {
			item := s.items[idx]
			content := item.Render(s.width)
			contentLines := strings.Split(content, "\n")

			// Refresh height cache from the actual render (authoritative).
			s.heightCache[item.ID()] = len(contentLines)

			startLine := 0
			if idx == s.offsetIdx {
				startLine = s.offsetLine
			}

			for i := startLine; i < len(contentLines) && remainingHeight > 0; i++ {
				line := contentLines[i]

				// Apply character-level selection highlighting.
				if hasSelection {
					inRange, startCol, endCol := selection.IsLineInRange(selRange, idx, i)
					if inRange {
						line = selection.HighlightLine(line, startCol, endCol)
					}
				}

				lines = append(lines, line)
				remainingHeight--
			}

			// Add gap lines between items.
			if remainingHeight > 0 && idx < len(s.items)-1 && s.itemGap > 0 {
				for g := 0; g < s.itemGap && remainingHeight > 0; g++ {
					lines = append(lines, "")
					remainingHeight--
				}
			}
		}
	}

	// Pad with empty lines to ensure exactly s.height lines.
	for remainingHeight > 0 {
		lines = append(lines, "")
		remainingHeight--
	}

	return strings.Join(lines, "\n")
}

// ScrollPercent returns the current scroll position as a percentage (0.0-1.0).
// 0.0 = at top, 1.0 = at bottom. Useful for scroll indicators.
func (s *ScrollList) ScrollPercent() float64 {
	if len(s.items) == 0 {
		return 0.0
	}

	totalHeight := 0
	for _, item := range s.items {
		totalHeight += s.itemHeight(item)
	}

	if totalHeight <= s.height {
		return 1.0
	}

	linesAbove := 0
	for i := 0; i < s.offsetIdx && i < len(s.items); i++ {
		linesAbove += s.itemHeight(s.items[i])
	}
	linesAbove += s.offsetLine

	scrollableHeight := totalHeight - s.height
	if scrollableHeight <= 0 {
		return 1.0
	}

	percent := float64(linesAbove) / float64(scrollableHeight)
	if percent > 1.0 {
		percent = 1.0
	}
	if percent < 0.0 {
		percent = 0.0
	}
	return percent
}

// clampOffset ensures the offset values are within valid bounds after
// resizing or scrolling operations. Uses cached heights to avoid
// redundant Render() calls.
func (s *ScrollList) clampOffset() {
	if len(s.items) == 0 {
		s.offsetIdx = 0
		s.offsetLine = 0
		return
	}

	// Clamp offsetIdx to valid item range.
	if s.offsetIdx >= len(s.items) {
		s.offsetIdx = len(s.items) - 1
	}
	if s.offsetIdx < 0 {
		s.offsetIdx = 0
	}

	// Clamp offsetLine within current item.
	if s.offsetIdx < len(s.items) {
		ih := s.itemHeight(s.items[s.offsetIdx])
		if s.offsetLine >= ih {
			s.offsetLine = max(0, ih-1)
		}
	}
	if s.offsetLine < 0 {
		s.offsetLine = 0
	}

	// Prevent scrolling past the bottom — compute total height and check
	// whether remaining content from the current offset fills the viewport.
	totalHeight := 0
	for i, item := range s.items {
		totalHeight += s.itemHeight(item)
		if s.itemGap > 0 && i < len(s.items)-1 {
			totalHeight += s.itemGap
		}
	}

	// If content fits in viewport, force start at top.
	if totalHeight <= s.height {
		s.offsetIdx = 0
		s.offsetLine = 0
		return
	}

	// Compute lines above the viewport.
	linesAbove := 0
	for i := 0; i < s.offsetIdx; i++ {
		linesAbove += s.itemHeight(s.items[i])
		if s.itemGap > 0 && i < len(s.items)-1 {
			linesAbove += s.itemGap
		}
	}
	linesAbove += s.offsetLine

	linesFromCurrentToEnd := totalHeight - linesAbove
	if linesFromCurrentToEnd < s.height {
		// We've scrolled past the bottom — reposition so the last line
		// of content sits at the bottom of the viewport.
		targetLine := totalHeight - s.height
		currentLine := 0

		for idx := 0; idx < len(s.items); idx++ {
			ih := s.itemHeight(s.items[idx])

			if currentLine+ih > targetLine {
				s.offsetIdx = idx
				s.offsetLine = targetLine - currentLine
				return
			}

			currentLine += ih
			if s.itemGap > 0 && idx < len(s.items)-1 {
				currentLine += s.itemGap
			}
		}
	}
}

// itemHeight returns the cached rendered height for an item, computing and
// caching it on first access. This avoids calling Render() purely to
// count lines — the most common source of redundant work in the scroll
// list (GotoBottom, clampOffset, AtBottom, ScrollBy all need heights but
// never use the rendered content).
//
// The cache is invalidated wholesale on width changes (SetWidth) and
// individual entries are refreshed in View() after an item is actually
// rendered, so stale entries are self-correcting within one frame.
func (s *ScrollList) itemHeight(item MessageItem) int {
	id := item.ID()
	if h, ok := s.heightCache[id]; ok {
		return h
	}
	// Cache miss — render to measure.
	h := s.renderedHeight(item)
	s.heightCache[id] = h
	return h
}

// renderedHeight returns the height of a message item in lines by actually
// rendering it. This is the single source of truth for item height — it
// matches exactly what View() produces, unlike item.Height() which may
// return stale/zero values for uncached items (e.g. reasoning blocks).
func (s *ScrollList) renderedHeight(item MessageItem) int {
	rendered := item.Render(s.width)
	if rendered == "" {
		return 0
	}
	return strings.Count(rendered, "\n") + 1
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
