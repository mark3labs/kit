package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	uicore "github.com/mark3labs/kit/internal/ui/core"
	"github.com/mark3labs/kit/internal/ui/imagepreview"
	"github.com/mark3labs/kit/internal/ui/style"
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

	got := m.computeGfxPlacement(parts, inputPartIndex, -1)
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

	want := xansi.CursorPosition(thumbPaddingLeft+1, wantRow)
	if !strings.Contains(got, want) {
		t.Errorf("placement does not position at row %d\n got: %q\nwant substring: %q", wantRow, got, want)
	}

	// The renderer owns the cursor, so the sequence must put it back.
	if !strings.HasPrefix(got, xansi.SaveCursor) || !strings.HasSuffix(got, xansi.RestoreCursor) {
		t.Error("placement does not save and restore the cursor")
	}
}

// With no pending images there is nothing to draw, and the placement must be
// empty so the flush path stays quiet.
func TestComputeGfxPlacementEmptyWithoutImages(t *testing.T) {
	m := &AppModel{height: 40, width: 80}
	m.input = NewInputComponent(80, nil)
	if got := m.computeGfxPlacement([]string{"a", "b", "c"}, 1, -1); got != "" {
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

	if got := m.computeGfxPlacement([]string{"scroll", ic.View().Content, "status"}, 1, -1); got != "" {
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

// A transcript preview must use the terminal's graphics protocol wherever the
// composer does.
//
// The transcript lives in the scrollback, which scrolls and clips its items,
// so an image painted over the screen has to be re-placed and clipped on every
// frame. That work happens in computeGfxPlacement; what this pins is that the
// preview is a real image — blank cells reserving the area plus a transmitted
// picture — rather than the half blocks it used to fall back to.
func TestTranscriptPreviewPlacesDirectly(t *testing.T) {
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
	if msg.transmit == "" {
		t.Error("transcript preview did not transmit the image")
	}
	if len(msg.previews) != 1 {
		t.Fatalf("got %d previews, want 1", len(msg.previews))
	}
	preview := msg.previews[0]
	if preview.image == nil {
		t.Fatal("preview carries no image to place; it fell back to text")
	}
	if !preview.image.NeedsPlacement || preview.image.ImageID == 0 {
		t.Errorf("preview image is not placeable: %+v", *preview.image)
	}
	if strings.Contains(preview.cells, "▀") {
		t.Error("transcript preview fell back to half blocks")
	}
	// The cells only reserve the area the picture is painted over, so they must
	// be blank and exactly as tall as the image.
	if strings.TrimSpace(xansi.Strip(preview.cells)) != "" {
		t.Errorf("reserved cells are not blank: %q", preview.cells)
	}
	if got := lipgloss.Height(preview.cells); got != preview.image.Rows {
		t.Errorf("reserved %d rows for a %d-row image", got, preview.image.Rows)
	}
}

// A directly-placed transcript image must be drawn at the row the scrollback
// gave its message, counted from the top of the transcript.
func TestTranscriptPlacementFollowsTheScrollback(t *testing.T) {
	const (
		screenH   = 40
		imageRows = 6
		imageCols = 20
		headerH   = 2
	)

	// Direct placements only ever exist in a terminal that needs them, which is
	// also what the placement path gates on.
	t.Cleanup(func() { termgfx.Set(termgfx.Capabilities{}) })
	termgfx.Set(termgfx.Capabilities{
		KittyGraphics:       true,
		CellWidth:           10,
		CellHeight:          20,
		TrueColor:           true,
		UnicodePlaceholders: false,
	})

	m := &AppModel{width: 80, height: screenH}
	m.scrollList = NewScrollList(80, 20)

	blank := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", style.ContentOffset+imageCols)+"\n", imageRows), "\n")
	item := NewStyledMessageItem("preview-1", "user", "", blank).
		WithDirectImage(imagepreview.Thumbnail{
			ImageID:        7,
			Cols:           imageCols,
			Rows:           imageRows,
			PixelWidth:     imageCols * 10,
			PixelHeight:    imageRows * 20,
			NeedsPlacement: true,
		})
	m.scrollList.SetItems([]MessageItem{
		NewStyledMessageItem("msg-1", "user", "", "hello"),
		item,
	})

	header := strings.TrimSuffix(strings.Repeat("header\n", headerH), "\n")
	parts := []string{header, m.scrollList.View(), "status"}

	got := m.computeGfxPlacement(parts, -1, 1)
	if got == "" {
		t.Fatal("no placement for a visible transcript image")
	}
	// The header takes the first rows, the "hello" message one more, so the
	// image starts on the row after that.
	wantRow := headerH + 1 + 1
	want := xansi.CursorPosition(style.ContentOffset+1, wantRow)
	if !strings.Contains(got, want) {
		t.Errorf("placement does not position at row %d\n got: %q\nwant substring: %q", wantRow, got, want)
	}
	if !strings.Contains(got, "i=7") {
		t.Errorf("placement does not name image 7: %q", got)
	}

	// Once the image is gone from the list its placement must be taken back:
	// the picture is painted over the screen and redrawing the cells beneath
	// does not remove it.
	m.scrollList.SetItems([]MessageItem{NewStyledMessageItem("msg-1", "user", "", "hello")})
	gone := m.computeGfxPlacement([]string{header, m.scrollList.View(), "status"}, -1, 1)
	if !strings.Contains(gone, "a=d") || !strings.Contains(gone, "i=7") {
		t.Errorf("image 7 was not deleted after leaving the transcript: %q", gone)
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
	if len(msg.previews) != 1 {
		t.Fatalf("got %d previews, want 1", len(msg.previews))
	}
	block := msg.previews[0].cells
	if msg.previews[0].image != nil {
		t.Error("placeholder cells draw themselves and need no placement")
	}
	if !strings.ContainsRune(block, '\U0010EEEE') {
		t.Error("transcript preview does not contain placeholder cells")
	}
	if strings.Contains(block, "▀") {
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

// An image whose top has scrolled out of the viewport must be drawn clipped,
// starting on the viewport's first row.
//
// A placed image is not part of the view, so nothing clips it: drawing the
// whole picture would paint the scrolled-away rows over whatever sits above
// the transcript.
func TestTranscriptPlacementClipsToTheViewport(t *testing.T) {
	const (
		imageRows = 6
		imageCols = 20
		visible   = 3
	)

	t.Cleanup(func() { termgfx.Set(termgfx.Capabilities{}) })
	termgfx.Set(termgfx.Capabilities{
		KittyGraphics:       true,
		CellWidth:           10,
		CellHeight:          20,
		TrueColor:           true,
		UnicodePlaceholders: false,
	})

	m := &AppModel{width: 80, height: 40}
	// A viewport shorter than the image, scrolled to the bottom, so the first
	// imageRows-visible rows of the picture are above the top edge.
	m.scrollList = NewScrollList(80, visible)

	blank := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", style.ContentOffset+imageCols)+"\n", imageRows), "\n")
	m.scrollList.SetItems([]MessageItem{
		NewStyledMessageItem("preview-1", "user", "", blank).
			WithDirectImage(imagepreview.Thumbnail{
				ImageID:        7,
				Cols:           imageCols,
				Rows:           imageRows,
				PixelWidth:     imageCols * 10,
				PixelHeight:    imageRows * 20,
				NeedsPlacement: true,
			}),
	})

	got := m.computeGfxPlacement([]string{m.scrollList.View()}, -1, 0)
	if got == "" {
		t.Fatal("no placement for a partially visible transcript image")
	}
	// Drawn from the very top of the screen, since the transcript is the only
	// section above it.
	if want := xansi.CursorPosition(style.ContentOffset+1, 1); !strings.Contains(got, want) {
		t.Errorf("clipped image is not drawn on the first row\n got: %q\nwant substring: %q", got, want)
	}
	if want := "r=" + strconv.Itoa(visible); !strings.Contains(got, want) {
		t.Errorf("placement does not draw %d rows: %q", visible, got)
	}
	// The hidden rows must be skipped in the source image, not squeezed into
	// the visible ones.
	if want := "y=" + strconv.Itoa((imageRows-visible)*20); !strings.Contains(got, want) {
		t.Errorf("placement does not skip the scrolled-away rows (%s): %q", want, got)
	}
}

// A modal covers the transcript, so the images behind it must be taken off the
// screen: a placed image is painted over the frame and would otherwise sit on
// top of the dialog.
func TestTranscriptPlacementYieldsToModals(t *testing.T) {
	t.Cleanup(func() { termgfx.Set(termgfx.Capabilities{}) })
	termgfx.Set(termgfx.Capabilities{
		KittyGraphics:       true,
		CellWidth:           10,
		CellHeight:          20,
		TrueColor:           true,
		UnicodePlaceholders: false,
	})

	m := &AppModel{width: 80, height: 40}
	m.scrollList = NewScrollList(80, 20)
	blank := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", style.ContentOffset+20)+"\n", 4), "\n")
	m.scrollList.SetItems([]MessageItem{
		NewStyledMessageItem("preview-1", "user", "", blank).
			WithDirectImage(imagepreview.Thumbnail{
				ImageID:        7,
				Cols:           20,
				Rows:           4,
				PixelWidth:     200,
				PixelHeight:    80,
				NeedsPlacement: true,
			}),
	})

	parts := []string{m.scrollList.View()}
	if got := m.computeGfxPlacement(parts, -1, 0); !strings.Contains(got, "a=p") {
		t.Fatalf("image not drawn before the modal opened: %q", got)
	}

	m.state = stateModelSelector
	got := m.computeGfxPlacement(parts, -1, 0)
	if strings.Contains(got, "a=p") {
		t.Errorf("image drawn over a modal: %q", got)
	}
	if !strings.Contains(got, "a=d") || !strings.Contains(got, "i=7") {
		t.Errorf("image 7 not removed while the modal is up: %q", got)
	}
}
