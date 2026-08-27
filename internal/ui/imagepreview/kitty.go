package imagepreview

import (
	"bytes"
	"fmt"
	"image"
	"math/rand/v2"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
	xdraw "golang.org/x/image/draw"
)

// Thumbnail is a rendered image preview, split into the part that must be
// written straight to the terminal and the part that belongs in the view.
//
// The half-block renderer produces Cells only. The Kitty renderers produce
// both: Transmit carries the image data, and Cells reserves the screen area
// the image occupies.
type Thumbnail struct {
	// Transmit is an escape sequence that must reach the terminal unmodified,
	// outside the normal view text. It is empty for half-block thumbnails.
	//
	// Write it with tea.Raw, never by embedding it in a view: the frame
	// renderer parses view text into cells and would mangle or drop an APC
	// sequence hidden inside it.
	Transmit string

	// Cells is the text to place in the view. For a placeholder thumbnail this
	// is the Unicode placeholder grid that displays the image; for a direct
	// placement it is blank cells that reserve the area the image is drawn
	// over; for a half-block thumbnail it is the coloured art itself. Either
	// way it is ordinary text that occupies exactly the intended cells, so the
	// frame renderer can measure, pad, and diff it like any other content.
	Cells string

	// ImageID identifies the transmitted image so it can be deleted later. It
	// is zero when nothing was transmitted.
	ImageID uint32

	// Cols and Rows are the size of the preview in terminal cells.
	Cols, Rows int

	// PixelWidth and PixelHeight are the dimensions of the transmitted image.
	// They are what makes a partial placement possible: a slice of the image is
	// named in pixels, so a caller must be able to convert a count of cell rows
	// into the pixel band those rows cover. See PlaceRows.
	PixelWidth, PixelHeight int

	// NeedsPlacement reports that the image is drawn by a direct placement
	// anchored at the cursor, rather than by placeholder cells that travel with
	// the view text.
	//
	// A direct placement is not part of the view, so the caller must position
	// the cursor and emit Place after every frame that shows the preview, and
	// must emit DeleteImage when it goes away. See RenderKittyDirect.
	NeedsPlacement bool
}

// cellPixelWidth and cellPixelHeight are the fallback pixel dimensions of one
// terminal cell, used when the caller does not know the real ones.
//
// The exact cell size only decides how much detail is sent: the terminal
// rescales the image to the requested cell box regardless. The 1:2 ratio
// matches the assumption fitDimensions already makes, so the aspect ratio
// stays correct even when the guess is wrong.
const (
	cellPixelWidth  = 10
	cellPixelHeight = 20
)

// RenderKitty builds a Kitty graphics thumbnail: a virtual image placement
// plus the Unicode placeholder grid that displays it.
//
// Unicode placeholders let the image travel as ordinary text: the terminal
// draws it only where the placeholder cells appear, so it moves, scrolls, and
// disappears with the surrounding view. That is what a redrawing TUI wants,
// and it is the right choice wherever it works.
//
// It does not work everywhere. Zellij 0.45 forwards the graphics protocol but
// discards the combining marks that tell each placeholder cell which part of
// the image it holds, so nothing is drawn. Use RenderKittyDirect there.
//
// cellW and cellH are the pixel dimensions of a terminal cell, used to choose
// the resampling resolution. Pass zero for either to use a default.
//
// maxCols and maxRows bound the preview in cells, as with Render.
func RenderKitty(data []byte, maxCols, maxRows, cellW, cellH int) (Thumbnail, error) {
	scaled, cols, rows, err := scaleForCells(data, maxCols, maxRows, cellW, cellH)
	if err != nil || scaled == nil {
		return Thumbnail{}, err
	}

	id := newImageID()

	var buf bytes.Buffer
	opts := &kitty.Options{
		Action:       kitty.TransmitAndPut,
		ID:           int(id),
		Format:       kitty.PNG,
		Transmission: kitty.Direct,
		Columns:      cols,
		Rows:         rows,
		// A virtual placement transmits the image without drawing it; the
		// placeholder cells decide where it lands.
		VirtualPlacement: true,
		// Suppress both OK and error replies. Without this the terminal
		// answers every chunk, and those replies arrive on stdin where the
		// event loop would read them as input.
		Quiet: 2,
		// Chunk so no single escape sequence grows unbounded; terminals are
		// free to reject an overlong one.
		Chunk: true,
	}
	if err := kitty.EncodeGraphics(&buf, scaled, opts); err != nil {
		return Thumbnail{}, fmt.Errorf("encode kitty graphics: %w", err)
	}

	return Thumbnail{
		Transmit:    buf.String(),
		Cells:       placeholderGrid(id, cols, rows),
		ImageID:     id,
		Cols:        cols,
		Rows:        rows,
		PixelWidth:  scaled.Bounds().Dx(),
		PixelHeight: scaled.Bounds().Dy(),
	}, nil
}

// RenderKittyDirect builds a Kitty graphics thumbnail that is drawn by a
// direct placement at the cursor rather than by placeholder cells.
//
// This exists for terminals that carry the graphics protocol but not the
// Unicode placeholder encoding — zellij 0.45 strips the combining marks that
// placeholders depend on, so RenderKitty draws nothing there while this works.
//
// The trade-off is that the image is not part of the view. It is painted over
// the screen at whatever position the cursor held when Place was written, and
// it stays there until deleted. The caller must therefore:
//
//   - reserve the area with Cells, which is blank text of exactly Cols x Rows,
//     so the layout accounts for the space the image covers;
//   - write Transmit once to load the image;
//   - position the cursor and write Place on every frame that shows it;
//   - write DeleteImage when the preview goes away, since overwriting the
//     cells does not remove it.
//
// cellW and cellH are the pixel dimensions of a terminal cell, used to choose
// the resampling resolution. Pass zero for either to use a default.
func RenderKittyDirect(data []byte, maxCols, maxRows, cellW, cellH int) (Thumbnail, error) {
	scaled, cols, rows, err := scaleForCells(data, maxCols, maxRows, cellW, cellH)
	if err != nil || scaled == nil {
		return Thumbnail{}, err
	}

	id := newImageID()

	var buf bytes.Buffer
	opts := &kitty.Options{
		// Transmit only. The placement is emitted separately by Place, so the
		// image can be redrawn on later frames without resending the data.
		Action:       kitty.Transmit,
		ID:           int(id),
		Format:       kitty.PNG,
		Transmission: kitty.Direct,
		Quiet:        2,
		Chunk:        true,
	}
	if err := kitty.EncodeGraphics(&buf, scaled, opts); err != nil {
		return Thumbnail{}, fmt.Errorf("encode kitty graphics: %w", err)
	}

	// Blank cells reserve the area. The image is painted over them, so their
	// only job is to make the layout leave room and to keep whatever was
	// underneath from showing through.
	blank := strings.Repeat(" ", cols)
	rowsText := make([]string, rows)
	for i := range rowsText {
		rowsText[i] = blank
	}

	return Thumbnail{
		Transmit:       buf.String(),
		Cells:          strings.Join(rowsText, "\n"),
		ImageID:        id,
		Cols:           cols,
		Rows:           rows,
		PixelWidth:     scaled.Bounds().Dx(),
		PixelHeight:    scaled.Bounds().Dy(),
		NeedsPlacement: true,
	}, nil
}

// Place returns the escape sequence that draws an already-transmitted image at
// the current cursor position, scaled into a Cols x Rows cell box.
//
// It returns an empty string for thumbnails that do not need a placement, so
// callers can emit it unconditionally.
func (t Thumbnail) Place() string {
	return t.PlaceRows(0, t.Rows)
}

// PlaceRows returns the escape sequence that draws a horizontal band of an
// already-transmitted image at the current cursor position: the first skip
// cell rows of the image are left out and the next rows rows are drawn.
//
// Clipping is what lets a directly-placed image live in a scrolling list. The
// image is painted over the screen instead of being part of the view, so one
// whose top has scrolled out of the viewport would otherwise be drawn over
// whatever sits above it. Naming only the visible band keeps the picture
// aligned with the cells the list gave it, and lets it slide under the edge of
// the viewport one row at a time.
//
// It returns an empty string when there is nothing left to draw, so callers
// can emit it unconditionally.
func (t Thumbnail) PlaceRows(skip, rows int) string {
	if !t.NeedsPlacement || t.ImageID == 0 || t.Cols < 1 || t.Rows < 1 {
		return ""
	}
	if skip < 0 {
		skip = 0
	}
	if rows > t.Rows-skip {
		rows = t.Rows - skip
	}
	if rows < 1 {
		return ""
	}

	// Drop any earlier placement of this image first. Each a=p adds another
	// placement rather than moving the existing one, so a redraw at a new row
	// would leave the previous copy stranded on screen — two thumbnails for one
	// attachment.
	//
	// Deleting placements keeps the transmitted data, so the redraw that
	// follows needs no retransmit. It does not depend on the cursor either, so
	// it is safe to emit before positioning.
	drop := DeletePlacements(t.ImageID)

	// C=1 keeps the cursor where it was: without it the terminal moves the
	// cursor past the image and the frame renderer loses its position.
	params := []string{
		"a=p",
		fmt.Sprintf("i=%d", t.ImageID),
		fmt.Sprintf("c=%d", t.Cols),
		fmt.Sprintf("r=%d", rows),
		"C=1",
		"q=2",
	}
	// A whole image needs no source rectangle. Naming one only for a partial
	// band keeps the common sequence exactly what it always was, and keeps a
	// thumbnail whose pixel size was never recorded working.
	if (skip > 0 || rows < t.Rows) && t.PixelWidth > 0 && t.PixelHeight > 0 {
		// The image was resampled to an exact multiple of its row count, so one
		// cell row is a whole number of pixels tall and every band lands on a
		// pixel boundary.
		rowPx := max(t.PixelHeight/t.Rows, 1)
		y := min(skip*rowPx, t.PixelHeight)
		h := min(rows*rowPx, t.PixelHeight-y)
		if h < 1 {
			return ""
		}
		params = append(params,
			"x=0",
			fmt.Sprintf("y=%d", y),
			fmt.Sprintf("w=%d", t.PixelWidth),
			fmt.Sprintf("h=%d", h),
		)
	}
	return drop + ansi.KittyGraphics(nil, params...)
}

// DeletePlacements returns the escape sequence that removes every placement of
// an image while keeping the transmitted data, so the image can be drawn again
// without being sent again.
//
// A directly-placed image is painted over the screen and stays there until it
// is told to go; redrawing the cells underneath does not remove it. Anything
// that takes such an image off screen — scrolling it out of the viewport,
// covering it with a modal, clearing the transcript — must therefore drop its
// placements explicitly.
func DeletePlacements(id uint32) string {
	if id == 0 {
		return ""
	}
	// The lowercase d=i deletes placements only; d=I would free the data too.
	return ansi.KittyGraphics(nil, "a=d", "d=i", fmt.Sprintf("i=%d", id), "q=2")
}

// scaleForCells decodes an image and resamples it to fill a cell box that fits
// within maxCols x maxRows while preserving aspect ratio. It returns a nil
// image when there is no room to draw anything.
func scaleForCells(data []byte, maxCols, maxRows, cellW, cellH int) (*image.RGBA, int, int, error) {
	if maxCols < 1 || maxRows < 1 {
		return nil, 0, 0, nil
	}
	if cellW < 1 {
		cellW = cellPixelWidth
	}
	if cellH < 1 {
		cellH = cellPixelHeight
	}

	// Guard against decompression bombs before decoding, exactly as Render
	// does: a small payload must not be able to expand into a huge buffer.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width > maxImageDimension || cfg.Height > maxImageDimension {
		return nil, 0, 0, fmt.Errorf("decode image: dimensions %dx%d exceed limit %d", cfg.Width, cfg.Height, maxImageDimension)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", err)
	}

	cols, rows := fitDimensions(img.Bounds().Dx(), img.Bounds().Dy(), maxCols, maxRows)
	if cols < 1 || rows < 1 {
		return nil, 0, 0, nil
	}

	// Resample before transmitting. The source may be a multi-megabyte
	// screenshot, and every byte would otherwise be base64-encoded into the
	// escape sequence for a thumbnail a few dozen cells wide.
	scaled := image.NewRGBA(image.Rect(0, 0, cols*cellW, rows*cellH))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return scaled, cols, rows, nil
}

// placeholderGrid builds the cell grid that displays a virtually placed image.
//
// Each cell holds U+10EEEE followed by two combining marks that encode its row
// and column, and the whole row carries a foreground colour whose 24 bits are
// the image id. The terminal reads that triple to decide which image, and
// which part of it, belongs in the cell.
func placeholderGrid(id uint32, cols, rows int) string {
	var b strings.Builder

	// The id travels as a 24-bit RGB foreground colour. newImageID keeps ids
	// inside 24 bits so no fourth byte (a third combining mark) is needed.
	r := (id >> 16) & 0xff
	g := (id >> 8) & 0xff
	bl := id & 0xff

	for row := range rows {
		fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", r, g, bl)
		rowMark := kitty.Diacritic(row)
		for col := range cols {
			b.WriteRune(kitty.Placeholder)
			b.WriteRune(rowMark)
			b.WriteRune(kitty.Diacritic(col))
		}
		b.WriteString(reset)
		if row+1 < rows {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// DeleteImage returns the escape sequence that removes a transmitted image and
// frees the memory the terminal holds for it.
//
// Every transmitted thumbnail must be deleted once its preview is gone.
// Terminals keep image data until told otherwise, so a session that pastes
// many images would otherwise accumulate all of them.
func DeleteImage(id uint32) string {
	if id == 0 {
		return ""
	}
	// d=I (not d=i) deletes the image data as well as its placements.
	return ansi.KittyGraphics(nil, "a=d", "d=I", fmt.Sprintf("i=%d", id), "q=2")
}

// newImageID returns a random image id in [0x010000, 0xFFFFFF].
//
// Ids are a terminal-wide namespace shared with every other program drawing to
// the same window, so they are randomised rather than counted from zero to
// avoid colliding with another program's images. Staying inside 24 bits lets
// the id fit in the placeholder foreground colour; keeping the high byte
// non-zero avoids ids that a terminal might read as a palette index.
func newImageID() uint32 {
	return uint32(rand.IntN(0xFFFFFF-0x010000+1) + 0x010000) //nolint:gosec // not security-sensitive
}
