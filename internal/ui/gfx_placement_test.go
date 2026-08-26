package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	uicore "github.com/mark3labs/kit/internal/ui/core"
)

// A directly-placed image must land inside the composer, immediately above the
// status bar, regardless of what the scrollback rendered.
//
// The placement is positioned by counting up from the bottom of the screen
// precisely so a scrollback that draws fewer lines than it was allotted cannot
// float the image away from the composer. This pins that: the scrollback part
// is deliberately shorter than the space above the input.
func TestComputeGfxPlacementAnchorsToComposer(t *testing.T) {
	const (
		screenH   = 40
		imageRows = 12
		imageCols = 20
	)

	m := &AppModel{height: screenH, width: 80}
	ic := NewInputComponent(80, nil)
	// One pending image whose thumbnail is already rendered, with a placement
	// sequence, as the direct renderer produces.
	ic.pendingImages = make([]uicore.ImageAttachment, 1)
	ic.imageThumbs = []string{strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", imageCols)+"\n", imageRows), "\n")}
	ic.imageIDs = []uint32{7}
	ic.imagePlace = []string{"\x1b_Ga=p,i=7\x1b\\"}
	m.input = ic

	// A layout whose scrollback is far shorter than the screen, which is the
	// case that broke the original top-down arithmetic.
	scrollback := strings.TrimSuffix(strings.Repeat("scrollback\n", 3), "\n")
	inputView := ic.View().Content
	statusBar := "status"
	parts := []string{scrollback, inputView, statusBar}
	const inputPartIndex = 1

	got := m.computeGfxPlacement(parts, inputPartIndex)
	if got == "" {
		t.Fatal("computeGfxPlacement returned nothing, want a placement")
	}

	// The image's top row: the composer's last row is the one above the status
	// bar, and the image sits at its own offset within the composer.
	inputTop := screenH - lipgloss.Height(statusBar) - lipgloss.Height(inputView) + 1
	offsets := ic.thumbRowOffsets()
	if len(offsets) != 1 || offsets[0] < 0 {
		t.Fatalf("thumbRowOffsets() = %v, want one non-negative offset", offsets)
	}
	wantRow := inputTop + offsets[0]

	// The image must end on the row just above the status bar. If this fails
	// the preview has drifted off the composer, which is the bug this guards.
	if wantEnd := screenH - lipgloss.Height(statusBar); wantRow+imageRows-1 != wantEnd {
		t.Errorf("image occupies rows %d-%d, want it to end at %d (just above the status bar)",
			wantRow, wantRow+imageRows-1, wantEnd)
	}

	want := ansi.CursorPosition(thumbPaddingLeft+1, wantRow)
	if !strings.Contains(got, want) {
		t.Errorf("placement does not position at row %d\n got: %q\nwant substring: %q", wantRow, got, want)
	}

	// The renderer owns the cursor, so the sequence must put it back.
	if !strings.HasPrefix(got, ansi.SaveCursor) || !strings.HasSuffix(got, ansi.RestoreCursor) {
		t.Error("placement does not save and restore the cursor")
	}
}

// With no pending images there is nothing to draw, and the placement must be
// empty so the flush path stays quiet.
func TestComputeGfxPlacementEmptyWithoutImages(t *testing.T) {
	m := &AppModel{height: 40, width: 80}
	m.input = NewInputComponent(80, nil)
	if got := m.computeGfxPlacement([]string{"a", "b", "c"}, 1); got != "" {
		t.Errorf("computeGfxPlacement() = %q, want empty", got)
	}
}

// Half-block and placeholder thumbnails draw themselves as text and must not
// produce a placement.
func TestComputeGfxPlacementIgnoresTextThumbnails(t *testing.T) {
	m := &AppModel{height: 40, width: 80}
	ic := NewInputComponent(80, nil)
	ic.pendingImages = make([]uicore.ImageAttachment, 1)
	ic.imageThumbs = []string{"▀▀▀▀"}
	ic.imageIDs = []uint32{0}
	ic.imagePlace = []string{""} // text thumbnails carry no placement
	m.input = ic

	if got := m.computeGfxPlacement([]string{"scroll", ic.View().Content, "status"}, 1); got != "" {
		t.Errorf("computeGfxPlacement() = %q, want empty for a text thumbnail", got)
	}
}

// A directly-placed image must be drawn again after any repaint, even when the
// row it belongs on has not changed.
//
// The renderer scrolls the screen to update it, and a terminal scrolls its
// images along with the text, so an image drifts upward while its computed row
// stays identical. Re-placing only on a changed row therefore leaves the image
// stranded, which is the bug this guards.
func TestFlushGfxPlacementRedrawsAfterRepaint(t *testing.T) {
	m := &AppModel{height: 40, width: 80}
	m.gfxPlacement = "\x1b_Ga=p,i=7\x1b\\"

	// A repaint happened.
	m.gfxDirty = true
	if cmd := m.flushGfxPlacement(); cmd == nil {
		t.Fatal("no placement written after a repaint")
	}
	if m.gfxDirty {
		t.Error("gfxDirty still set after the placement was written")
	}

	// Nothing has changed since, so nothing more should be written. Without
	// this the nudge would drive an endless render loop.
	if cmd := m.flushGfxPlacement(); cmd != nil {
		t.Error("placement rewritten when the screen had not changed")
	}

	// The next repaint must write it again, with the row unchanged.
	m.gfxDirty = true
	if cmd := m.flushGfxPlacement(); cmd == nil {
		t.Error("no placement written after a second repaint")
	}
}

// Once the images are gone there is nothing to draw, and the dirty flag must
// still clear so the nudge loop stops.
func TestFlushGfxPlacementStopsWhenImagesGo(t *testing.T) {
	m := &AppModel{height: 40, width: 80}
	m.gfxPlacement = ""
	m.gfxDirty = true

	if cmd := m.flushGfxPlacement(); cmd != nil {
		t.Error("placement written when there are no images")
	}
	if m.gfxDirty {
		t.Error("gfxDirty still set, which would spin the render loop")
	}
}
