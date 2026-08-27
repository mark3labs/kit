package imagepreview

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi/kitty"
	"golang.org/x/term"
)

// TestKittyLive transmits a thumbnail to the real terminal and asserts that it
// accepts the image.
//
// The test is skipped unless KIT_LIVE_KITTY=1 and stdin is a terminal, because
// it needs a terminal that actually implements the graphics protocol. Run it
// inside kitty with:
//
//	KIT_LIVE_KITTY=1 go test -run TestKittyLive ./internal/ui/imagepreview/
//
// Unlike the offline tests, this one asks the terminal to confirm: it drops the
// quiet flag so the terminal reports whether the transmission was valid, which
// catches format, chunking, and base64 mistakes that a byte-level assertion
// cannot.
func TestKittyLive(t *testing.T) {
	if os.Getenv("KIT_LIVE_KITTY") != "1" {
		t.Skip("set KIT_LIVE_KITTY=1 and run inside a graphics-capable terminal")
	}
	// Talk to the controlling terminal directly. Under `go test` stdout is
	// usually a pipe or a file, and writing the transmission there would put
	// the escape sequence in the log instead of the terminal.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no controlling terminal: %v", err)
	}
	defer func() { _ = tty.Close() }()

	fd := int(tty.Fd())
	if !term.IsTerminal(fd) {
		t.Skip("/dev/tty is not a terminal")
	}

	thumb, err := RenderKitty(testPNG(t, 200, 100), 20, 5, 0, 0)
	if err != nil {
		t.Fatalf("RenderKitty: %v", err)
	}

	// Re-encode the transmission without the quiet flag so the terminal
	// answers. RenderKitty sets q=2 for production use, where an unsolicited
	// reply would land in the event loop's input stream.
	loud := strings.ReplaceAll(thumb.Transmit, ",q=2", "")

	state, err := term.MakeRaw(fd)
	if err != nil {
		t.Fatalf("raw mode: %v", err)
	}
	defer func() { _ = term.Restore(fd, state) }()

	if _, err := tty.WriteString(loud); err != nil {
		t.Fatalf("write transmission: %v", err)
	}

	reply := make(chan string, 1)
	go func() {
		buf := make([]byte, 1024)
		n, _ := tty.Read(buf)
		reply <- string(buf[:n])
	}()

	select {
	case got := <-reply:
		if !strings.Contains(got, ";OK") {
			t.Errorf("terminal rejected the image: %q", got)
		}
		t.Logf("terminal accepted the image: %q", got)
	case <-time.After(3 * time.Second):
		t.Error("terminal did not answer the transmission")
	}

	// Leave the placeholders on screen so `kitty @ get-text` can inspect them.
	_, _ = tty.WriteString("\r\n" + thumb.Cells + "\r\n")
	if os.Getenv("KIT_LIVE_KEEP") == "" {
		_, _ = tty.WriteString(DeleteImage(thumb.ImageID))
	}
}

// testPNG returns an encoded PNG of the given size with a colour gradient, so
// a rendered preview is visually distinguishable from a blank cell.
func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / max(w-1, 1)),
				G: uint8(y * 255 / max(h-1, 1)),
				B: 128,
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

// testNoisePNG returns an encoded PNG of random pixels. Noise resists PNG
// compression, so the payload is large enough to be split into chunks.
func testNoisePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{
				R: uint8(rand.IntN(256)), //nolint:gosec // test data, not security-sensitive
				G: uint8(rand.IntN(256)), //nolint:gosec
				B: uint8(rand.IntN(256)), //nolint:gosec
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode noise png: %v", err)
	}
	return buf.Bytes()
}

// TestPlaceholderGridStructure pins the placeholder encoding: one placeholder
// rune per cell, each followed by the row and column combining marks, with the
// image id carried as a 24-bit foreground colour.
func TestPlaceholderGridStructure(t *testing.T) {
	const id = 0x123456
	got := placeholderGrid(id, 3, 2)

	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d rows, want 2", len(lines))
	}

	for row, line := range lines {
		if !strings.HasPrefix(line, "\x1b[38;2;18;52;86m") {
			t.Errorf("row %d does not carry the image id as a foreground colour: %q", row, line)
		}
		if !strings.HasSuffix(line, reset) {
			t.Errorf("row %d does not reset its colour: %q", row, line)
		}

		runes := []rune(strings.TrimSuffix(strings.TrimPrefix(line, "\x1b[38;2;18;52;86m"), reset))
		if len(runes) != 3*3 { // 3 cells x (placeholder + row mark + column mark)
			t.Fatalf("row %d has %d runes, want 9", row, len(runes))
		}
		for col := range 3 {
			base := runes[col*3]
			rowMark := runes[col*3+1]
			colMark := runes[col*3+2]
			if base != kitty.Placeholder {
				t.Errorf("row %d col %d: base rune = %U, want %U", row, col, base, kitty.Placeholder)
			}
			if want := kitty.Diacritic(row); rowMark != want {
				t.Errorf("row %d col %d: row mark = %U, want %U", row, col, rowMark, want)
			}
			if want := kitty.Diacritic(col); colMark != want {
				t.Errorf("row %d col %d: column mark = %U, want %U", row, col, colMark, want)
			}
		}
	}
}

// The transmission must create a virtual placement sized to the placeholder
// grid, and must not provoke replies that the event loop would read as input.
func TestRenderKittyTransmission(t *testing.T) {
	thumb, err := RenderKitty(testPNG(t, 200, 100), 20, 5, 0, 0)
	if err != nil {
		t.Fatalf("RenderKitty: %v", err)
	}
	if thumb.ImageID == 0 {
		t.Fatal("ImageID = 0, want a real id")
	}
	if !strings.HasPrefix(thumb.Transmit, "\x1b_G") {
		t.Errorf("transmission is not an APC graphics sequence: %.32q", thumb.Transmit)
	}
	// t=d is omitted deliberately: direct transmission is the protocol default,
	// so the encoder leaves the key out.
	for _, want := range []string{"a=T", "U=1", "q=2", "f=100"} {
		if !strings.Contains(thumb.Transmit, want) {
			t.Errorf("transmission missing %q", want)
		}
	}

	// The declared cell box must match the placeholder grid exactly, or the
	// terminal scales the image into the wrong number of cells.
	rows := strings.Split(thumb.Cells, "\n")
	cols := strings.Count(rows[0], string(kitty.Placeholder))
	if !strings.Contains(thumb.Transmit, "c="+strconv.Itoa(cols)) {
		t.Errorf("transmission does not declare c=%d", cols)
	}
	if !strings.Contains(thumb.Transmit, "r="+strconv.Itoa(len(rows))) {
		t.Errorf("transmission does not declare r=%d", len(rows))
	}
}

// A 200x100 image is wider than it is tall, so the preview must be too. This
// guards the aspect-ratio handling shared with the half-block renderer.
func TestRenderKittyPreservesAspect(t *testing.T) {
	thumb, err := RenderKitty(testPNG(t, 200, 100), 20, 20, 0, 0)
	if err != nil {
		t.Fatalf("RenderKitty: %v", err)
	}
	rows := strings.Split(thumb.Cells, "\n")
	cols := strings.Count(rows[0], string(kitty.Placeholder))
	if cols <= len(rows) {
		t.Errorf("preview is %dx%d cells, want it wider than tall", cols, len(rows))
	}
}

// A large payload must be split across several escape sequences rather than
// sent as one unbounded sequence a terminal may reject.
func TestRenderKittyChunksLargePayload(t *testing.T) {
	// Noise, not a gradient: a smooth image compresses into a single chunk and
	// would not exercise the split at all.
	thumb, err := RenderKitty(testNoisePNG(t, 400, 400), 30, 15, 0, 0)
	if err != nil {
		t.Fatalf("RenderKitty: %v", err)
	}
	if n := strings.Count(thumb.Transmit, "\x1b_G"); n < 2 {
		t.Errorf("payload sent as %d sequence(s), want it split into several", n)
	}
}

func TestDeleteImage(t *testing.T) {
	got := DeleteImage(0x4242)
	for _, want := range []string{"a=d", "d=I", "i=16962"} {
		if !strings.Contains(got, want) {
			t.Errorf("DeleteImage() = %q, missing %q", got, want)
		}
	}
	if DeleteImage(0) != "" {
		t.Error("DeleteImage(0) should return no sequence")
	}
}

func TestRenderKittyRejectsGarbage(t *testing.T) {
	if _, err := RenderKitty([]byte("not an image"), 10, 5, 0, 0); err == nil {
		t.Error("RenderKitty accepted non-image data")
	}
}

func TestNewImageIDStaysIn24Bits(t *testing.T) {
	for range 1000 {
		id := newImageID()
		if id < 0x010000 || id > 0xFFFFFF {
			t.Fatalf("newImageID() = %#x, want it inside [0x010000, 0xFFFFFF]", id)
		}
	}
}

// A direct thumbnail transmits without placing, and reserves its area with
// blank cells that the image is painted over.
func TestRenderKittyDirect(t *testing.T) {
	thumb, err := RenderKittyDirect(testPNG(t, 200, 100), 20, 10, 10, 20)
	if err != nil {
		t.Fatalf("RenderKittyDirect: %v", err)
	}
	if !thumb.NeedsPlacement {
		t.Error("NeedsPlacement = false, want true")
	}
	if thumb.ImageID == 0 {
		t.Fatal("ImageID = 0, want a real id")
	}
	// The transmission must not display anything by itself: a=T would draw the
	// image at wherever the cursor happens to be, before the caller has
	// positioned it. Transmit-only (a=t) is the protocol default, so the
	// encoder omits the key entirely — its absence is what must hold.
	if strings.Contains(thumb.Transmit, "a=T") {
		t.Errorf("transmission draws immediately: %.48q", thumb.Transmit)
	}
	if strings.Contains(thumb.Transmit, "U=1") {
		t.Error("transmission requests a virtual placement, which needs placeholders")
	}
	if strings.Contains(thumb.Cells, string(kitty.Placeholder)) {
		t.Error("reserved cells contain placeholders; they should be blank")
	}

	// The reserved area must match the declared size exactly, or the layout
	// will not leave the right amount of room for the image.
	lines := strings.Split(thumb.Cells, "\n")
	if len(lines) != thumb.Rows {
		t.Errorf("reserved %d rows, want %d", len(lines), thumb.Rows)
	}
	for i, l := range lines {
		if len([]rune(l)) != thumb.Cols {
			t.Errorf("reserved row %d is %d cells, want %d", i, len([]rune(l)), thumb.Cols)
		}
		if strings.TrimSpace(l) != "" {
			t.Errorf("reserved row %d is not blank: %q", i, l)
		}
	}
}

// The placement must draw the image into exactly the reserved box and must not
// move the cursor, which the frame renderer owns.
func TestThumbnailPlace(t *testing.T) {
	thumb, err := RenderKittyDirect(testPNG(t, 200, 100), 20, 10, 10, 20)
	if err != nil {
		t.Fatalf("RenderKittyDirect: %v", err)
	}
	place := thumb.Place()
	for _, want := range []string{
		"a=p",
		"i=" + strconv.Itoa(int(thumb.ImageID)),
		"c=" + strconv.Itoa(thumb.Cols),
		"r=" + strconv.Itoa(thumb.Rows),
		"C=1", // do not move the cursor
		"q=2", // no replies into the input stream
	} {
		if !strings.Contains(place, want) {
			t.Errorf("Place() = %q, missing %q", place, want)
		}
	}
}

// Placeholder thumbnails draw themselves as text, so they must report no
// placement; callers emit Place unconditionally.
func TestPlaceholderThumbnailNeedsNoPlacement(t *testing.T) {
	thumb, err := RenderKitty(testPNG(t, 200, 100), 20, 10, 10, 20)
	if err != nil {
		t.Fatalf("RenderKitty: %v", err)
	}
	if thumb.NeedsPlacement {
		t.Error("NeedsPlacement = true for a placeholder thumbnail")
	}
	if got := thumb.Place(); got != "" {
		t.Errorf("Place() = %q, want empty", got)
	}
}

// Both renderers must agree on the cell box they report, since the layout uses
// it to reserve space either way.
func TestRenderersAgreeOnSize(t *testing.T) {
	data := testPNG(t, 200, 100)
	ph, err := RenderKitty(data, 20, 10, 10, 20)
	if err != nil {
		t.Fatalf("RenderKitty: %v", err)
	}
	dir, err := RenderKittyDirect(data, 20, 10, 10, 20)
	if err != nil {
		t.Fatalf("RenderKittyDirect: %v", err)
	}
	if ph.Cols != dir.Cols || ph.Rows != dir.Rows {
		t.Errorf("placeholder %dx%d, direct %dx%d; want identical",
			ph.Cols, ph.Rows, dir.Cols, dir.Rows)
	}
}

// Re-placing an image must not stack copies on screen.
//
// Each a=p adds a placement rather than moving the existing one, so a redraw
// at a new row leaves the previous copy behind — one attachment, two visible
// thumbnails. Place must therefore drop the old placement first.
func TestPlaceReplacesRatherThanStacks(t *testing.T) {
	thumb, err := RenderKittyDirect(testPNG(t, 200, 100), 20, 10, 10, 20)
	if err != nil {
		t.Fatalf("RenderKittyDirect: %v", err)
	}
	place := thumb.Place()

	id := "i=" + strconv.Itoa(int(thumb.ImageID))
	drop := strings.Index(place, "a=d")
	put := strings.Index(place, "a=p")
	if drop < 0 {
		t.Fatalf("Place() does not remove the previous placement: %q", place)
	}
	if put < 0 {
		t.Fatalf("Place() does not draw the image: %q", place)
	}
	if drop > put {
		t.Error("Place() draws before removing the previous placement, which stacks copies")
	}

	// Lowercase d=i drops placements but keeps the image data, so redrawing
	// needs no retransmit. Uppercase d=I would discard the data and leave
	// later redraws with nothing to place.
	if !strings.Contains(place, "d=i") {
		t.Errorf("Place() does not drop placements with d=i: %q", place)
	}
	if strings.Contains(place, "d=I") {
		t.Error("Place() discards the image data, so redraws would have nothing to draw")
	}
	if strings.Count(place, id) != 2 {
		t.Errorf("Place() should scope both the drop and the draw to %s: %q", id, place)
	}
}

// A whole-image placement must carry no source rectangle.
//
// The rectangle is only meaningful for a clipped band, and adding it to every
// placement would change a sequence that terminals already accept for no gain.
func TestPlaceSendsNoSourceRectangle(t *testing.T) {
	thumb, err := RenderKittyDirect(testPNG(t, 200, 100), 20, 10, 10, 20)
	if err != nil {
		t.Fatalf("RenderKittyDirect: %v", err)
	}
	for _, key := range []string{"x=", "y=", "w=", "h="} {
		if strings.Contains(thumb.Place(), key) {
			t.Errorf("Place() names a source rectangle (%s): %q", key, thumb.Place())
		}
	}
}

// A clipped placement must draw only the band it was asked for, named as the
// matching slice of the source image.
//
// This is what lets a placed image live in a scrolling transcript: the rows
// that have left the viewport must not be drawn, or they would land on top of
// whatever is above the list.
func TestPlaceRowsClipsToTheVisibleBand(t *testing.T) {
	thumb, err := RenderKittyDirect(testPNG(t, 200, 100), 20, 10, 10, 20)
	if err != nil {
		t.Fatalf("RenderKittyDirect: %v", err)
	}
	if thumb.Rows < 4 {
		t.Fatalf("thumbnail is %d rows; the test needs at least 4", thumb.Rows)
	}
	rowPx := thumb.PixelHeight / thumb.Rows

	const skip = 2
	visible := thumb.Rows - skip
	place := thumb.PlaceRows(skip, visible)

	for _, want := range []string{
		"r=" + strconv.Itoa(visible),       // only the visible rows are drawn
		"y=" + strconv.Itoa(skip*rowPx),    // starting at the matching pixel row
		"h=" + strconv.Itoa(visible*rowPx), // for the matching pixel height
		"w=" + strconv.Itoa(thumb.PixelWidth),
		"x=0",
		"C=1",
	} {
		if !strings.Contains(place, want) {
			t.Errorf("PlaceRows(%d, %d) = %q, missing %q", skip, visible, place, want)
		}
	}
	// The full row count would draw the hidden rows as well.
	if strings.Contains(place, "r="+strconv.Itoa(thumb.Rows)) {
		t.Errorf("PlaceRows drew all %d rows: %q", thumb.Rows, place)
	}
}

// Asking for a band that has scrolled entirely out of view must draw nothing,
// so callers can compute a band and emit the result unconditionally.
func TestPlaceRowsEmptyBandDrawsNothing(t *testing.T) {
	thumb, err := RenderKittyDirect(testPNG(t, 200, 100), 20, 10, 10, 20)
	if err != nil {
		t.Fatalf("RenderKittyDirect: %v", err)
	}
	if got := thumb.PlaceRows(thumb.Rows, 3); got != "" {
		t.Errorf("PlaceRows past the end = %q, want empty", got)
	}
	if got := thumb.PlaceRows(0, 0); got != "" {
		t.Errorf("PlaceRows(0, 0) = %q, want empty", got)
	}
}

// Dropping placements must keep the image data, so the picture can be drawn
// again without being sent again.
func TestDeletePlacementsKeepsTheData(t *testing.T) {
	got := DeletePlacements(42)
	for _, want := range []string{"a=d", "d=i", "i=42", "q=2"} {
		if !strings.Contains(got, want) {
			t.Errorf("DeletePlacements(42) = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "d=I") {
		t.Error("DeletePlacements discards the image data")
	}
	if got := DeletePlacements(0); got != "" {
		t.Errorf("DeletePlacements(0) = %q, want empty", got)
	}
}
