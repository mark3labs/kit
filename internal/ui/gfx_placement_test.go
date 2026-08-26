package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	uicore "github.com/mark3labs/kit/internal/ui/core"
	"github.com/mark3labs/kit/internal/ui/termgfx"
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

// A transcript preview must never use a direct placement.
//
// The transcript lives in the scrollback, which scrolls and clips its items.
// Placeholder cells are text and move with the message they belong to; a
// directly placed image is painted at a fixed screen position and would sit
// still while the transcript scrolled underneath it, and would outlive the
// message scrolling away entirely. Terminals that need direct placement
// therefore keep half blocks here, even though the composer uses graphics.
func TestTranscriptPreviewNeverPlacesDirectly(t *testing.T) {
	t.Cleanup(func() {
		termgfx.Set(termgfx.Capabilities{})
	})
	// A terminal that draws graphics but strips the combining marks that
	// placeholders are built from, so it needs a direct placement. Stated as
	// capabilities rather than environment variables: the decision is a pure
	// function of them, and an env-based setup passed only where TERM happened
	// to be set.
	termgfx.Set(termgfx.Capabilities{
		KittyGraphics:       true,
		CellWidth:           10,
		CellHeight:          20,
		TrueColor:           true,
		UnicodePlaceholders: false,
	})
	// The half-block renderer reads the colour profile from the environment
	// and draws nothing below 256 colours, which is what a bare CI environment
	// reports. Give it a terminal so the fallback actually produces art to
	// assert on.
	t.Setenv("TERM", "xterm-256color")

	if termgfx.PreviewMode() != termgfx.ModeDirect {
		t.Fatalf("PreviewMode() = %v, want %v; the test premise no longer holds",
			termgfx.PreviewMode(), termgfx.ModeDirect)
	}

	m := &AppModel{width: 100, height: 40}
	cmd := m.transcriptPreviewCmd([]uicore.ImageAttachment{
		{Data: testGradientPNG(t), MediaType: "image/png"},
	}, "anchor-1")
	if cmd == nil {
		t.Fatal("transcriptPreviewCmd returned nil, want a preview")
	}
	msg, ok := cmd().(imagePreviewReadyMsg)
	if !ok {
		t.Fatalf("got %T, want imagePreviewReadyMsg", cmd())
	}
	if msg.transmit != "" {
		t.Error("transcript preview transmitted an image in a direct-placement terminal")
	}
	if !strings.Contains(msg.block, "▀") {
		t.Error("transcript preview is not half-block art")
	}
}

// Where placeholders work the transcript must use them, so a submitted image
// looks the same as it did in the composer.
func TestTranscriptPreviewUsesPlaceholders(t *testing.T) {
	t.Cleanup(func() {
		termgfx.Set(termgfx.Capabilities{})
	})
	termgfx.Set(termgfx.Capabilities{
		KittyGraphics:       true,
		CellWidth:           10,
		CellHeight:          20,
		TrueColor:           true,
		UnicodePlaceholders: true,
	})

	if termgfx.PreviewMode() != termgfx.ModePlaceholder {
		t.Fatalf("PreviewMode() = %v, want %v; the test premise no longer holds",
			termgfx.PreviewMode(), termgfx.ModePlaceholder)
	}

	m := &AppModel{width: 100, height: 40}
	cmd := m.transcriptPreviewCmd([]uicore.ImageAttachment{
		{Data: testGradientPNG(t), MediaType: "image/png"},
	}, "anchor-1")
	if cmd == nil {
		t.Fatal("transcriptPreviewCmd returned nil, want a preview")
	}
	msg, ok := cmd().(imagePreviewReadyMsg)
	if !ok {
		t.Fatalf("got %T, want imagePreviewReadyMsg", cmd())
	}
	if msg.transmit == "" {
		t.Error("transcript preview did not transmit the image")
	}
	if !strings.ContainsRune(msg.block, '\U0010EEEE') {
		t.Error("transcript preview does not contain placeholder cells")
	}
	if strings.Contains(msg.block, "▀") {
		t.Error("transcript preview fell back to half blocks")
	}
}

// testGradientPNG returns a small encoded PNG.
func testGradientPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for y := range 32 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 8), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

// ClearPendingImages must hand back the cleanup command.
//
// A terminal holds transmitted image data until told to drop it, and the ids
// are discarded when the pending set is cleared, so a caller that cannot run
// the cleanup leaks every preview image for the rest of the session. The
// function used to free the images internally and throw the command away,
// which meant the delete sequence never reached the terminal.
func TestClearPendingImagesReturnsCleanup(t *testing.T) {
	ic := NewInputComponent(80, nil)
	ic.pendingImages = make([]uicore.ImageAttachment, 1)
	ic.imageThumbs = []string{"thumb"}
	ic.imageIDs = []uint32{99} // a transmitted image the terminal is holding
	ic.imagePlace = []string{"\x1b_Ga=p,i=99\x1b\\"}

	images, cleanup := ic.ClearPendingImages()
	if len(images) != 1 {
		t.Fatalf("got %d attachments, want 1", len(images))
	}
	if cleanup == nil {
		t.Fatal("no cleanup command returned; the transmitted image would leak")
	}

	raw, ok := cleanup().(tea.RawMsg)
	if !ok {
		t.Fatalf("cleanup produced %T, want tea.RawMsg", cleanup())
	}
	seq, _ := raw.Msg.(string)
	if !strings.Contains(seq, "a=d") || !strings.Contains(seq, "i=99") {
		t.Errorf("cleanup does not delete image 99: %q", seq)
	}

	if len(ic.pendingImages) != 0 || len(ic.imageIDs) != 0 {
		t.Error("pending image state was not cleared")
	}
}

// With nothing transmitted there is nothing to free, so callers get no command
// to run rather than an empty escape sequence.
func TestClearPendingImagesNoCleanupForTextThumbnails(t *testing.T) {
	ic := NewInputComponent(80, nil)
	ic.pendingImages = make([]uicore.ImageAttachment, 1)
	ic.imageThumbs = []string{"▀▀▀▀"}
	ic.imageIDs = []uint32{0} // half blocks own no terminal resource
	ic.imagePlace = []string{""}

	images, cleanup := ic.ClearPendingImages()
	if len(images) != 1 {
		t.Fatalf("got %d attachments, want 1", len(images))
	}
	if cleanup != nil {
		t.Error("cleanup command returned when no image was transmitted")
	}
}
