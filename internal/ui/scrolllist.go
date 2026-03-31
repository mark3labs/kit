package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// highlightStyle is lazily initialized to avoid creating it on every render
var highlightStyle lipgloss.Style

// initHighlightStyle creates the highlight style with proper colors
func initHighlightStyle() lipgloss.Style {
	if highlightStyle.String() == "" {
		theme := GetTheme()
		highlightStyle = lipgloss.NewStyle().
			Background(theme.Secondary).
			Foreground(theme.Background).
			Bold(true)
	}
	return highlightStyle
}

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
// It handles offset-based scrolling and lazy rendering. Only visible
// items are rendered on each View() call.
type ScrollList struct {
	items      []MessageItem
	offsetIdx  int // Index of first visible item
	offsetLine int // Lines to skip from first visible item
	width      int
	height     int  // Viewport height in lines
	autoScroll bool // Whether to auto-scroll to bottom on new content
	itemGap    int  // Number of blank lines between items (0 = no gap)
	focusedIdx int  // Index of focused/selected item (-1 = none)
	selectable bool // Whether items can be selected via mouse/keyboard

	// Selection tracking for copy+paste (crush-style)
	selection     CopySelection // Current text selection
	mouseDown     bool          // Whether mouse button is currently down
	mouseDownX    int           // X coordinate where mouse was pressed
	mouseDownY    int           // Y coordinate where mouse was pressed
	mouseDownItem int           // Item index where mouse was pressed
}

// NewScrollList creates a new ScrollList with the given dimensions.
func NewScrollList(width, height int) *ScrollList {
	return &ScrollList{
		items:      []MessageItem{},
		offsetIdx:  0,
		offsetLine: 0,
		width:      width,
		height:     height,
		autoScroll: true, // Start with auto-scroll enabled
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

// SetHeight updates the viewport height. Called when the terminal is resized.
func (s *ScrollList) SetHeight(height int) {
	s.height = height
	s.clampOffset()
}

// SetWidth updates the viewport width. Called when the terminal is resized.
// This may invalidate cached renders in MessageItems.
func (s *ScrollList) SetWidth(width int) {
	s.width = width
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

// SetSelectable enables or disables item selection.
func (s *ScrollList) SetSelectable(selectable bool) {
	s.selectable = selectable
}

// FocusedIdx returns the currently focused item index (-1 if none).
func (s *ScrollList) FocusedIdx() int {
	return s.focusedIdx
}

// SetFocused sets the focused item by index.
func (s *ScrollList) SetFocused(idx int) {
	if idx < -1 {
		s.focusedIdx = -1
	} else if idx >= len(s.items) {
		s.focusedIdx = len(s.items) - 1
	} else {
		s.focusedIdx = idx
	}
}

// SelectItemAtY selects the item at the given Y coordinate (relative to viewport).
// Returns the selected item index or -1 if no item at that position.
func (s *ScrollList) SelectItemAtY(y int) int {
	if !s.selectable || len(s.items) == 0 || y < 0 || y >= s.height {
		return -1
	}

	// Calculate which item is at the given Y position
	currentY := 0
	for idx := s.offsetIdx; idx < len(s.items); idx++ {
		item := s.items[idx]
		itemHeight := item.Height()

		// Check if y falls within this item
		if y >= currentY && y < currentY+itemHeight {
			s.focusedIdx = idx
			return idx
		}

		currentY += itemHeight

		// Add gap after item (except last)
		if s.itemGap > 0 && idx < len(s.items)-1 {
			currentY += s.itemGap
		}

		// Stop if we've passed the viewport
		if currentY >= s.height {
			break
		}
	}

	return -1
}

// HandleMouseDown handles mouse button press for selection (crush-style).
// Returns true if the click was handled.
func (s *ScrollList) HandleMouseDown(x, y int) bool {
	if !s.selectable || len(s.items) == 0 {
		return false
	}

	s.mouseDown = true
	s.mouseDownX = x
	s.mouseDownY = y

	// Find which item and line was clicked
	itemIdx, lineIdx := s.getItemAndLineAtY(y)
	s.mouseDownItem = itemIdx

	// Start a new selection at click position
	if itemIdx >= 0 {
		s.selection = CopySelection{
			StartItemIdx: itemIdx,
			StartLine:    lineIdx,
			StartCol:     x,
			EndItemIdx:   itemIdx,
			EndLine:      lineIdx,
			EndCol:       x,
			Active:       true,
		}
		return true
	}

	return false
}

// HandleMouseDrag handles mouse drag for selection (crush-style).
// Updates the selection end point. Returns true if selection changed.
func (s *ScrollList) HandleMouseDrag(x, y int) bool {
	if !s.mouseDown || !s.selectable {
		return false
	}

	// Find which item and line we're dragging over
	itemIdx, lineIdx := s.getItemAndLineAtY(y)
	if itemIdx < 0 {
		return false
	}

	// Update selection end point
	s.selection.EndItemIdx = itemIdx
	s.selection.EndLine = lineIdx
	s.selection.EndCol = x
	s.selection.Active = true

	return true
}

// getItemAndLineAtY converts a Y coordinate to item index and line index within that item.
// Returns (-1, -1) if Y is outside the viewport or beyond all items.
func (s *ScrollList) getItemAndLineAtY(y int) (itemIdx, lineIdx int) {
	if y < 0 || y >= s.height || len(s.items) == 0 {
		return -1, -1
	}

	currentY := 0
	for idx := s.offsetIdx; idx < len(s.items); idx++ {
		item := s.items[idx]
		itemHeight := item.Height()

		// Check if y falls within this item
		if y >= currentY && y < currentY+itemHeight {
			return idx, y - currentY
		}

		currentY += itemHeight

		// Add gap after item (except last)
		if s.itemGap > 0 && idx < len(s.items)-1 {
			currentY += s.itemGap
		}

		// Stop if we've passed the viewport
		if currentY >= s.height {
			break
		}
	}

	return -1, -1
}

// HandleMouseUp handles mouse button release (crush-style).
// Finalizes selection and returns true if there was an active selection.
func (s *ScrollList) HandleMouseUp(x, y int) bool {
	if !s.mouseDown {
		return false
	}

	s.mouseDown = false

	// Check if we have a valid selection
	if s.selection.Active && !s.selection.IsEmpty() {
		return true
	}

	return false
}

// GetSelection returns the current text selection.
func (s *ScrollList) GetSelection() CopySelection {
	return s.selection
}

// ClearSelection clears the current text selection.
func (s *ScrollList) ClearSelection() {
	s.selection = CopySelection{}
	s.mouseDown = false
}

// HasSelection returns true if there is an active non-empty selection.
func (s *ScrollList) HasSelection() bool {
	return s.selection.Active && !s.selection.IsEmpty()
}

// ScrollBy scrolls the viewport by the given number of lines.
// Positive = scroll down, negative = scroll up.
func (s *ScrollList) ScrollBy(lines int) {
	if lines > 0 {
		// Scroll down
		for lines > 0 && s.offsetIdx < len(s.items) {
			if s.offsetIdx >= len(s.items) {
				break
			}
			currentItem := s.items[s.offsetIdx]
			itemHeight := currentItem.Height()
			remainingLines := itemHeight - s.offsetLine

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
					currentItem := s.items[s.offsetIdx]
					itemHeight := currentItem.Height()

					if lines >= itemHeight {
						lines -= itemHeight
						s.offsetLine = 0
					} else {
						s.offsetLine = itemHeight - lines
						lines = 0
					}
				}
			}
		}
	}
	s.clampOffset()
}

// GotoBottom scrolls to the end of the list.
func (s *ScrollList) GotoBottom() {
	if len(s.items) == 0 {
		s.offsetIdx = 0
		s.offsetLine = 0
		return
	}

	// Calculate total height including gaps
	// Ensure items are rendered before checking height (iteratr pattern)
	totalHeight := 0
	for i, item := range s.items {
		// Render to get actual content (handles non-cached items like reasoning blocks)
		rendered := item.Render(s.width)
		itemHeight := strings.Count(rendered, "\n") + 1
		totalHeight += itemHeight
		// Add gap after each item except the last
		if s.itemGap > 0 && i < len(s.items)-1 {
			totalHeight += s.itemGap
		}
	}

	// If content fits in viewport, start at top
	if totalHeight <= s.height {
		s.offsetIdx = 0
		s.offsetLine = 0
		return
	}

	// Otherwise, position viewport at bottom
	remaining := totalHeight - s.height
	for idx := 0; idx < len(s.items); idx++ {
		// Render to get actual content
		rendered := s.items[idx].Render(s.width)
		itemHeight := strings.Count(rendered, "\n") + 1
		if remaining < itemHeight {
			s.offsetIdx = idx
			s.offsetLine = remaining
			return
		}
		remaining -= itemHeight
		// Subtract gap after item (except last)
		if s.itemGap > 0 && idx < len(s.items)-1 {
			remaining -= s.itemGap
		}
	}

	// Fallback: show last item
	s.offsetIdx = max(0, len(s.items)-1)
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

	// Calculate visible height from current position including gaps
	// Calculate height directly from rendered content (handles non-cached items)
	visibleHeight := 0
	for idx := s.offsetIdx; idx < len(s.items); idx++ {
		item := s.items[idx]
		// Render to get actual content
		rendered := item.Render(s.width)
		itemHeight := strings.Count(rendered, "\n") + 1

		if idx == s.offsetIdx {
			visibleHeight += itemHeight - s.offsetLine
		} else {
			visibleHeight += itemHeight
		}

		// Add gap after item (except last)
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

// View renders the visible portion of the scrollback.
// Only items that fit within the viewport height are rendered.
// ALWAYS returns exactly s.height lines (padded with empty lines if needed)
// to ensure the input/footer stay fixed at the bottom.
func (s *ScrollList) View() string {
	if s.height <= 0 {
		return ""
	}

	var lines []string
	remainingHeight := s.height

	// Render visible items
	if len(s.items) > 0 {
		for idx := s.offsetIdx; idx < len(s.items) && remainingHeight > 0; idx++ {
			item := s.items[idx]
			content := item.Render(s.width)
			contentLines := strings.Split(content, "\n")

			startLine := 0
			if idx == s.offsetIdx {
				startLine = s.offsetLine
			}

			// Check if this item is focused (for visual indicator)
			isFocused := idx == s.focusedIdx

			for i := startLine; i < len(contentLines) && remainingHeight > 0; i++ {
				line := contentLines[i]

				// Apply selection highlighting if this line is within selection
				if s.selection.Active && s.isLineInSelection(idx, i) {
					line = s.applyHighlight(line)
				} else if isFocused && s.selectable {
					// Apply subtle focus indicator when item is focused but not in selection
					line = s.applyFocusIndicator(line)
				}

				lines = append(lines, line)
				remainingHeight--
			}

			// Add gap lines between items (but not after the last visible item)
			if remainingHeight > 0 && idx < len(s.items)-1 && s.itemGap > 0 {
				for g := 0; g < s.itemGap && remainingHeight > 0; g++ {
					lines = append(lines, "")
					remainingHeight--
				}
			}
		}
	}

	// Pad with empty lines to ensure exactly s.height lines
	// This keeps the input/footer fixed at the bottom of the screen
	for remainingHeight > 0 {
		lines = append(lines, "")
		remainingHeight--
	}

	return strings.Join(lines, "\n")
}

// isLineInSelection checks if a specific line within an item is part of the current selection.
func (s *ScrollList) isLineInSelection(itemIdx, lineIdx int) bool {
	if !s.selection.Active {
		return false
	}

	// Normalize selection (start <= end)
	startItem := s.selection.StartItemIdx
	startLine := s.selection.StartLine
	endItem := s.selection.EndItemIdx
	endLine := s.selection.EndLine

	if startItem > endItem || (startItem == endItem && startLine > endLine) {
		startItem, endItem = endItem, startItem
		startLine, endLine = endLine, startLine
	}

	// Check if item is within selection range
	if itemIdx < startItem || itemIdx > endItem {
		return false
	}

	// For single item selection
	if startItem == endItem {
		return itemIdx == startItem && lineIdx >= startLine && lineIdx <= endLine
	}

	// For multi-item selection
	if itemIdx == startItem {
		return lineIdx >= startLine
	}
	if itemIdx == endItem {
		return lineIdx <= endLine
	}
	// Middle items are fully selected
	return itemIdx > startItem && itemIdx < endItem
}

// applyHighlight applies the highlight style to a line.
// Uses the theme's Highlight color for the background.
func (s *ScrollList) applyHighlight(line string) string {
	if line == "" {
		return line
	}
	// Apply background/foreground color change for selection
	style := initHighlightStyle()
	return style.Render(line)
}

// applyFocusIndicator applies a subtle visual indicator for focused items.
func (s *ScrollList) applyFocusIndicator(line string) string {
	if line == "" {
		return line
	}
	// Just return the line as-is - no visual indicator for focus
	// The selection highlighting is enough
	return line
}

// ScrollPercent returns the current scroll position as a percentage (0.0-1.0).
// 0.0 = at top, 1.0 = at bottom. Useful for scroll indicators.
func (s *ScrollList) ScrollPercent() float64 {
	if len(s.items) == 0 {
		return 0.0
	}

	totalHeight := 0
	for _, item := range s.items {
		totalHeight += item.Height()
	}

	if totalHeight <= s.height {
		return 1.0 // All content fits, consider it "at bottom"
	}

	// Calculate how many lines are above the viewport
	linesAbove := 0
	for i := 0; i < s.offsetIdx && i < len(s.items); i++ {
		linesAbove += s.items[i].Height()
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
// resizing or scrolling operations. Prevents scrolling past the bottom
// of content (showing empty space when there's content above).
func (s *ScrollList) clampOffset() {
	if len(s.items) == 0 {
		s.offsetIdx = 0
		s.offsetLine = 0
		return
	}

	// First, clamp offsetIdx to valid item range
	if s.offsetIdx >= len(s.items) {
		s.offsetIdx = len(s.items) - 1
	}
	if s.offsetIdx < 0 {
		s.offsetIdx = 0
	}

	// Clamp offsetLine within current item
	if s.offsetIdx < len(s.items) {
		// Calculate height from rendered content (handles non-cached items)
		rendered := s.items[s.offsetIdx].Render(s.width)
		itemHeight := strings.Count(rendered, "\n") + 1
		if s.offsetLine >= itemHeight {
			s.offsetLine = max(0, itemHeight-1)
		}
	}
	if s.offsetLine < 0 {
		s.offsetLine = 0
	}

	// Prevent scrolling past the bottom (showing empty space at bottom when there's content above)
	// Calculate total content height
	totalHeight := 0
	for i, item := range s.items {
		rendered := item.Render(s.width)
		totalHeight += strings.Count(rendered, "\n") + 1
		if s.itemGap > 0 && i < len(s.items)-1 {
			totalHeight += s.itemGap
		}
	}

	// If content fits in viewport, force start at top
	if totalHeight <= s.height {
		s.offsetIdx = 0
		s.offsetLine = 0
		return
	}

	// Calculate how many lines are currently above the viewport
	linesAbove := 0
	for i := 0; i < s.offsetIdx; i++ {
		rendered := s.items[i].Render(s.width)
		linesAbove += strings.Count(rendered, "\n") + 1
		if s.itemGap > 0 && i < len(s.items)-1 {
			linesAbove += s.itemGap
		}
	}
	linesAbove += s.offsetLine

	// Calculate how many lines are visible from current position to end
	linesFromCurrentToEnd := totalHeight - linesAbove

	// If there's less content remaining than the viewport height,
	// we've scrolled past the bottom - need to back up
	if linesFromCurrentToEnd < s.height {
		// Position viewport so the last line of content is at the bottom
		targetLine := totalHeight - s.height
		currentLine := 0

		for idx := 0; idx < len(s.items); idx++ {
			rendered := s.items[idx].Render(s.width)
			itemHeight := strings.Count(rendered, "\n") + 1

			if currentLine+itemHeight > targetLine {
				// This item contains the target line
				s.offsetIdx = idx
				s.offsetLine = targetLine - currentLine
				return
			}

			currentLine += itemHeight
			if s.itemGap > 0 && idx < len(s.items)-1 {
				currentLine += s.itemGap
			}
		}
	}
}
