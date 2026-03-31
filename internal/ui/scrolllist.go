package ui

import (
	"strings"
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
	totalHeight := 0
	for i, item := range s.items {
		totalHeight += item.Height()
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
		itemHeight := s.items[idx].Height()
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
	visibleHeight := 0
	for idx := s.offsetIdx; idx < len(s.items); idx++ {
		item := s.items[idx]
		itemHeight := item.Height()

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

			for i := startLine; i < len(contentLines) && remainingHeight > 0; i++ {
				lines = append(lines, contentLines[i])
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
// resizing or scrolling operations.
func (s *ScrollList) clampOffset() {
	if len(s.items) == 0 {
		s.offsetIdx = 0
		s.offsetLine = 0
		return
	}

	// Clamp offsetIdx
	if s.offsetIdx >= len(s.items) {
		s.offsetIdx = len(s.items) - 1
	}
	if s.offsetIdx < 0 {
		s.offsetIdx = 0
	}

	// Clamp offsetLine
	if s.offsetIdx < len(s.items) {
		itemHeight := s.items[s.offsetIdx].Height()
		if s.offsetLine >= itemHeight {
			s.offsetLine = max(0, itemHeight-1)
		}
	}
	if s.offsetLine < 0 {
		s.offsetLine = 0
	}
}
