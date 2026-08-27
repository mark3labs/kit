package ui

import (
	"slices"
	"strings"
	"time"

	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/selection"
	"github.com/mark3labs/kit/internal/ui/style"
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

	// selectedIdx is the index of the message currently framed by the
	// selection border, or -1 when message navigation is inactive. The
	// selected item renders two lines taller than normal, so changing this
	// must invalidate the affected height-cache entries (SetSelectedIndex).
	selectedIdx int

	// selectionLabel and selectionHint are spliced into the selected item's
	// frame — top edge and bottom edge respectively. Set by the owner on
	// selection change (SetSelectionFrame); they replace border characters, so
	// they never change the framed item's height.
	selectionLabel string
	selectionHint  string

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
		selectedIdx: -1,
		sel:         selection.NewState(),
	}
}

// SetItems replaces the items in the scroll list. If auto-scroll is enabled,
// the viewport will scroll to the bottom to show the latest content — EXCEPT
// when the user is actively selecting text (mouse button held), in which case
// the scroll position is locked so the highlighted content stays under the
// cursor. The pending bottom-scroll is deferred to MouseUp.
func (s *ScrollList) SetItems(items []MessageItem) {
	s.items = items
	s.pruneHeightCache()
	// A shrinking list (e.g. /clear or a session switch) can strand the
	// selection past the end, which would leave a selected index that no
	// longer names a message.
	if s.selectedIdx >= len(s.items) {
		s.selectedIdx = len(s.items) - 1
		// The item now under the selection may have a cached height measured
		// without the selection border, which would throw the scroll maths
		// off by selectionBorderOverhead until it is next rendered. Drop it
		// so the next measurement includes the frame, as SetSelectedIndex does.
		if s.selectedIdx >= 0 {
			delete(s.heightCache, s.items[s.selectedIdx].ID())
		}
	}
	if s.autoScroll && !s.sel.MouseDown {
		s.GotoBottom()
	}
}

// pruneHeightCache evicts height-cache entries for items that are no longer
// in the list. Message IDs are unique per item, so without pruning the cache
// grows without bound across /clear and session switches. To keep SetItems
// cheap during streaming, pruning only runs when the cache has grown well
// beyond the current item count (amortized O(1) per call).
func (s *ScrollList) pruneHeightCache() {
	if len(s.heightCache) <= 2*len(s.items)+64 {
		return
	}
	live := make(map[string]struct{}, len(s.items))
	for _, item := range s.items {
		live[item.ID()] = struct{}{}
	}
	for id := range s.heightCache {
		if _, ok := live[id]; !ok {
			delete(s.heightCache, id)
		}
	}
}

// InvalidateItemHeight removes the cached height for the given item ID,
// forcing a re-render on the next height query. Call this after mutating
// an item's content (e.g. AppendChunk on a streaming message).
func (s *ScrollList) InvalidateItemHeight(id string) {
	delete(s.heightCache, id)
}

// InvalidateHeights drops every cached height, forcing each item to be
// re-measured on the next query. Call this when something outside the list
// changes how items render — a theme switch, for instance, where a repainted
// block can occupy a different number of lines than the one it replaces.
func (s *ScrollList) InvalidateHeights() {
	clear(s.heightCache)
}

// SetSelectedIndex sets the message framed by the selection border, or -1 to
// clear the selection. The border adds two lines to the item's rendered
// height, so the cached heights of both the previously and newly selected
// items are invalidated to keep scroll math and mouse hit-testing accurate.
func (s *ScrollList) SetSelectedIndex(idx int) {
	if idx == s.selectedIdx {
		return
	}
	if s.selectedIdx >= 0 && s.selectedIdx < len(s.items) {
		delete(s.heightCache, s.items[s.selectedIdx].ID())
	}
	if idx >= 0 && idx < len(s.items) {
		delete(s.heightCache, s.items[idx].ID())
	}
	s.selectedIdx = idx
	s.clampOffset()
}

// SelectedIndex returns the index of the selected message, or -1 if none.
func (s *ScrollList) SelectedIndex() int {
	return s.selectedIdx
}

// Len returns the number of items in the list.
func (s *ScrollList) Len() int {
	return len(s.items)
}

// ItemAt returns the item at idx, or nil when idx is out of range.
func (s *ScrollList) ItemAt(idx int) MessageItem {
	if idx < 0 || idx >= len(s.items) {
		return nil
	}
	return s.items[idx]
}

// SetHeight updates the viewport height. Called when the terminal is resized.
func (s *ScrollList) SetHeight(height int) {
	s.height = height
	s.clampOffset()
}

// SetWidth updates the viewport width. Called when the terminal is resized.
// This invalidates the height cache since rendered heights are width-dependent.
// A no-op when the width is unchanged — distributeHeight() calls this every
// layout pass, and wiping the cache on every pass would force full re-renders
// of all visible items purely to recompute known heights.
func (s *ScrollList) SetWidth(width int) {
	if width == s.width {
		return
	}
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
//
// Defensively disables auto-scroll on every drag update — even if the
// MouseDown handler missed (e.g. click landed in viewport padding), any
// active drag means the user is selecting and the viewport must not jump.
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

	// Hard-lock the viewport while dragging.
	s.autoScroll = false

	s.sel.DragItemIdx = itemIdx
	s.sel.DragLineIdx = lineIdx
	s.sel.DragCol = x

	return true
}

// IsMouseDown reports whether the user currently has the mouse button held
// (i.e. a selection drag is in progress). Used by the parent model to avoid
// re-enabling auto-scroll during streaming while the user is selecting.
func (s *ScrollList) IsMouseDown() bool {
	return s.sel.MouseDown
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
		content := s.renderItem(itemIdx)
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

	content := s.renderItem(itemIdx)
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

	content := s.renderItem(itemIdx)
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
		// Compute height the same way View() does: render, then count lines.
		itemHeight := s.renderedHeight(idx)

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
			ih := s.itemHeight(s.offsetIdx)
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
					ih := s.itemHeight(s.offsetIdx)

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
	s.offsetIdx, s.offsetLine = s.bottomOffset()
}

// bottomOffset computes the (offsetIdx, offsetLine) at which the last line
// of content sits at the bottom of the viewport — i.e. the maximum valid
// scroll offset. Walks backwards from the last item accumulating cached
// heights, so it is O(visible) instead of O(all items). Returns (0, 0)
// when all content fits in the viewport.
func (s *ScrollList) bottomOffset() (offsetIdx, offsetLine int) {
	if len(s.items) == 0 {
		return 0, 0
	}

	budget := s.height
	for idx := range slices.Backward(s.items) {
		ih := s.itemHeight(idx)

		// Account for gap *above* this item (gap between idx-1 and idx).
		gap := 0
		if s.itemGap > 0 && idx < len(s.items)-1 {
			gap = s.itemGap
		}

		if ih+gap >= budget {
			// This item (partially) fills the remaining budget.
			// When the gap consumed part of the budget, offsetLine would go
			// negative — clamp to 0 so the item is shown fully.
			return idx, max(0, ih-budget)
		}
		budget -= ih + gap
	}

	// All content fits in viewport — start at top.
	return 0, 0
}

// GotoTop scrolls to the beginning of the list.
func (s *ScrollList) GotoTop() {
	s.offsetIdx = 0
	s.offsetLine = 0
}

// EnsureVisible scrolls the minimum amount needed to bring the item at idx
// fully into the viewport.
//
// If the item sits above the viewport it is aligned to the top; if it
// extends past the bottom it is scrolled up just far enough to fit. An item
// taller than the viewport is aligned to its first line so navigation always
// lands on the start of a long message rather than its middle.
func (s *ScrollList) EnsureVisible(idx int) {
	if idx < 0 || idx >= len(s.items) {
		return
	}

	// Above the viewport (or partially scrolled off the top) — align to top.
	if idx < s.offsetIdx || (idx == s.offsetIdx && s.offsetLine > 0) {
		s.offsetIdx = idx
		s.offsetLine = 0
		s.clampOffset()
		return
	}

	// Measure from the top of the viewport to the end of the target item.
	distance := -s.offsetLine
	for i := s.offsetIdx; i <= idx; i++ {
		distance += s.itemHeight(i)
		if s.itemGap > 0 && i < idx {
			distance += s.itemGap
		}
	}

	// Already fully visible.
	if distance <= s.height {
		return
	}

	// Taller than the viewport — show it from its first line.
	if s.itemHeight(idx) >= s.height {
		s.offsetIdx = idx
		s.offsetLine = 0
		s.clampOffset()
		return
	}

	s.ScrollBy(distance - s.height)
}

// AtBottom returns true if the viewport is at the bottom of the list.
func (s *ScrollList) AtBottom() bool {
	if len(s.items) == 0 {
		return true
	}

	visibleHeight := 0
	for idx := s.offsetIdx; idx < len(s.items); idx++ {
		ih := s.itemHeight(idx)

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
			content := s.renderItem(idx)

			// Items that render to an empty string contribute zero height to
			// the viewport. This MUST match renderedHeight()'s semantics —
			// otherwise getItemAndLineAtY (which uses renderedHeight) treats
			// the item as 0 lines while View() emits one blank line via
			// strings.Split("", "\n") = [""], producing a 1-row downward
			// drift in mouse hit-testing per empty item between offsetIdx
			// and the cursor (most visibly streaming-reasoning items before
			// any reasoning has streamed, which extension widgets surface by
			// shrinking the scrollback).
			if content == "" {
				s.heightCache[item.ID()] = 0
				continue
			}

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

// VisibleItem locates one item inside the rendered viewport.
//
// It exists for content that is drawn beside the view rather than inside it —
// a directly-placed terminal image, which is painted at an absolute screen
// position and must therefore be told which rows of the viewport its item
// currently owns. See AppModel.computeGfxPlacement.
type VisibleItem struct {
	// Index is the item's position in the list, and Item the item itself.
	Index int
	Item  MessageItem

	// Row is the viewport row, counted from the top of the list's own view,
	// that the item's first drawn line lands on.
	Row int

	// SkipTop is how many of the item's leading lines are scrolled off above
	// the viewport, and Height how many of its lines are drawn.
	SkipTop int
	Height  int

	// Framed reports that the item is drawn inside the selection border, which
	// shifts its content one row down and one column right.
	Framed bool
}

// VisibleItems returns the position of every item the next View will draw.
//
// It walks items exactly as View does — same render path, same empty-item and
// gap handling — so the two cannot disagree about which rows an item occupies.
// Anything else would drift a placed image away from the cells reserved for it.
func (s *ScrollList) VisibleItems() []VisibleItem {
	if s.height <= 0 || len(s.items) == 0 {
		return nil
	}

	var out []VisibleItem
	row := 0
	remainingHeight := s.height
	for idx := s.offsetIdx; idx < len(s.items) && remainingHeight > 0; idx++ {
		content := s.renderItem(idx)
		// An item that renders to nothing contributes no rows, matching View.
		if content == "" {
			continue
		}
		lines := strings.Count(content, "\n") + 1

		skip := 0
		if idx == s.offsetIdx {
			skip = s.offsetLine
		}
		drawn := min(lines-skip, remainingHeight)
		if drawn > 0 {
			out = append(out, VisibleItem{
				Index:   idx,
				Item:    s.items[idx],
				Row:     row,
				SkipTop: skip,
				Height:  drawn,
				Framed:  idx == s.selectedIdx,
			})
			row += drawn
			remainingHeight -= drawn
		}

		if remainingHeight > 0 && idx < len(s.items)-1 && s.itemGap > 0 {
			gap := min(s.itemGap, remainingHeight)
			row += gap
			remainingHeight -= gap
		}
	}
	return out
}

// ScrollPercent returns the current scroll position as a percentage (0.0-1.0).
// 0.0 = at top, 1.0 = at bottom. Useful for scroll indicators.
func (s *ScrollList) ScrollPercent() float64 {
	if len(s.items) == 0 {
		return 0.0
	}

	totalHeight := 0
	for idx := range s.items {
		totalHeight += s.itemHeight(idx)
	}

	if totalHeight <= s.height {
		return 1.0
	}

	linesAbove := 0
	for i := 0; i < s.offsetIdx && i < len(s.items); i++ {
		linesAbove += s.itemHeight(i)
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
//
// The past-the-bottom check computes the maximum valid offset via an
// O(visible) backward walk (bottomOffset) instead of summing the heights
// of every item in the list — clampOffset runs on every mouse-wheel tick,
// so an O(all items) walk makes scrolling cost grow with session length.
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
		ih := s.itemHeight(s.offsetIdx)
		if s.offsetLine >= ih {
			s.offsetLine = max(0, ih-1)
		}
	}
	if s.offsetLine < 0 {
		s.offsetLine = 0
	}

	// Prevent scrolling past the bottom: the maximum valid offset places the
	// last content line at the bottom of the viewport. bottomOffset returns
	// (0, 0) when all content fits, which also forces start-at-top.
	maxIdx, maxLine := s.bottomOffset()
	if s.offsetIdx > maxIdx || (s.offsetIdx == maxIdx && s.offsetLine > maxLine) {
		s.offsetIdx = maxIdx
		s.offsetLine = maxLine
	}
}

// itemHeight returns the cached rendered height for the item at idx,
// computing and caching it on first access. This avoids calling Render()
// purely to count lines — the most common source of redundant work in the
// scroll list (GotoBottom, clampOffset, AtBottom, ScrollBy all need heights
// but never use the rendered content).
//
// The cache is invalidated wholesale on width changes (SetWidth), per-item
// when the selection moves (SetSelectedIndex), and individual entries are
// refreshed in View() after an item is actually rendered, so stale entries
// are self-correcting within one frame.
func (s *ScrollList) itemHeight(idx int) int {
	if idx < 0 || idx >= len(s.items) {
		return 0
	}
	id := s.items[idx].ID()
	if h, ok := s.heightCache[id]; ok {
		return h
	}
	// Cache miss — render to measure.
	h := s.renderedHeight(idx)
	s.heightCache[id] = h
	return h
}

// renderedHeight returns the height of the item at idx in lines by actually
// rendering it. This is the single source of truth for item height — it
// matches exactly what View() produces, unlike item.Height() which may
// return stale/zero values for uncached items (e.g. reasoning blocks) and
// which is unaware of the selection border.
func (s *ScrollList) renderedHeight(idx int) int {
	rendered := s.renderItem(idx)
	if rendered == "" {
		return 0
	}
	return strings.Count(rendered, "\n") + 1
}

// renderItem renders the item at idx exactly as View() paints it, including
// the selection border when idx is the selected message.
//
// Every consumer of item geometry (View, height measurement, mouse
// hit-testing, text extraction) funnels through this method so they can
// never disagree about how many lines an item occupies or which line is
// which — a mismatch would offset mouse selection and corrupt the layout.
func (s *ScrollList) renderItem(idx int) string {
	if idx < 0 || idx >= len(s.items) {
		return ""
	}

	if idx != s.selectedIdx {
		return s.items[idx].Render(s.width)
	}

	// The border consumes one column on each side, so the content is asked
	// to render narrower and the framed result still spans exactly s.width.
	content := s.items[idx].Render(max(s.width-2, 1))
	if content == "" {
		// Nothing to frame — an empty item stays empty (and zero-height) so
		// the selection can't conjure a box out of blank space.
		return ""
	}

	// theme.Border is a near-invisible panel edge on a dark terminal, which is
	// the wrong weight for a cursor: the status bar announces MESSAGE NAV in
	// Accent, so the thing the mode is pointing at gets the same colour.
	return applySelectionBorder(
		content, s.width, style.GetTheme().Accent,
		s.selectionLabel, s.selectionHint,
	)
}

// SetSelectionFrame sets the text spliced into the selected item's frame:
// label into the top edge, hint into the bottom.
//
// The labels are pushed in on selection change rather than derived here per
// frame. Working out which message the selection is on, and what its role is,
// means walking the item list; doing that inside the render path would repeat
// the walk on every frame for an answer that only changes on a keypress.
func (s *ScrollList) SetSelectionFrame(label, hint string) {
	if label == s.selectionLabel && hint == s.selectionHint {
		return
	}
	s.selectionLabel = label
	s.selectionHint = hint
	// The frame's height is unchanged by its labels, so no height cache entry
	// needs dropping here.
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
