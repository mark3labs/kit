// Package imagepreview renders low-resolution, in-terminal thumbnails of
// images using Unicode upper half-block characters (U+2580, "▀") combined
// with SGR foreground/background color codes.
//
// The technique stacks two vertical pixels into a single character cell: the
// foreground color paints the top pixel and the background color paints the
// bottom pixel. This produces pure styled text — no graphics escape sequences
// — so the output survives terminal multiplexers (tmux, zellij) untouched.
//
// The Kitty graphics protocol, Sixel, and iTerm2 inline images are
// deliberately NOT used: those are graphics escape-sequence protocols that
// tmux and zellij strip or mangle by default.
package imagepreview

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	// Register the standard image decoders so image.Decode can handle the
	// common clipboard / attachment formats.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	xdraw "golang.org/x/image/draw"
)

// upperHalfBlock is U+2580 ("▀"). The glyph fills the top half of a cell,
// letting the foreground color render the top pixel and the cell's background
// color render the bottom pixel.
const upperHalfBlock = "▀"

// reset is the SGR reset sequence appended after each rendered row.
const reset = "\x1b[0m"

// maxImageDimension is the largest width or height, in pixels, that Render will
// fully decode. Images larger than this in either axis are rejected before the
// expensive image.Decode call to guard against decompression bombs (small
// encoded payloads that expand to enormous pixel buffers).
const maxImageDimension = 20000

// Render returns a half-block ANSI thumbnail of the image, scaled to fit
// within maxCols x maxRows terminal cells while preserving aspect ratio.
//
// Each terminal cell encodes two vertically-stacked pixels, so the effective
// pixel resolution of the thumbnail is up to maxCols x (maxRows*2).
//
// Colors are emitted at the fidelity of the detected terminal color profile:
// truecolor (24-bit) when available, degrading to 256-color. When the
// terminal supports neither (no truecolor and no 256-color), Render returns
// an empty string and a nil error so the caller can fall back to a text
// indicator. A non-nil error is only returned when the image data cannot be
// decoded.
//
// bg is the color used to composite transparent pixels (typically the
// terminal background). A nil bg defaults to black.
func Render(data []byte, mediaType string, maxCols, maxRows int, bg color.Color) (string, error) {
	profile := colorprofile.Env(os.Environ())
	return renderWithProfile(data, maxCols, maxRows, bg, profile)
}

// renderWithProfile is the testable core of Render. It accepts an explicit
// color profile instead of detecting one from the environment.
func renderWithProfile(data []byte, maxCols, maxRows int, bg color.Color, profile colorprofile.Profile) (string, error) {
	// Half-block fidelity needs at least 256-color support. Anything less
	// degrades to the caller's text fallback.
	if profile < colorprofile.ANSI256 {
		return "", nil
	}
	if maxCols < 1 || maxRows < 1 {
		return "", nil
	}
	if bg == nil {
		bg = color.Black
	}

	// Guard against decompression bombs: inspect the header dimensions before
	// fully decoding, so a small malicious payload cannot expand into an
	// enormous pixel buffer.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width > maxImageDimension || cfg.Height > maxImageDimension {
		return "", fmt.Errorf("decode image: dimensions %dx%d exceed limit %d", cfg.Width, cfg.Height, maxImageDimension)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	// Target pixel dimensions: one pixel per column horizontally and two
	// pixels per row vertically (the half-block trick).
	cols, rows := fitDimensions(img.Bounds().Dx(), img.Bounds().Dy(), maxCols, maxRows)
	if cols < 1 || rows < 1 {
		return "", nil
	}
	pxW, pxH := cols, rows*2

	scaled := image.NewRGBA(image.Rect(0, 0, pxW, pxH))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), xdraw.Over, nil)

	var b strings.Builder
	for y := 0; y < pxH; y += 2 {
		for x := range pxW {
			top := composite(scaled.At(x, y), bg)
			bottom := composite(scaled.At(x, y+1), bg)
			b.WriteString(sgr(top, bottom, profile))
			b.WriteString(upperHalfBlock)
		}
		b.WriteString(reset)
		if y+2 < pxH {
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}

// fitDimensions returns the largest cell dimensions (cols, rows) that fit a
// srcW x srcH image inside a maxCols x maxRows box while preserving aspect
// ratio. Because each cell stacks two vertical pixels, a terminal cell is
// treated as roughly twice as tall as it is wide, which keeps the thumbnail's
// aspect ratio visually correct.
func fitDimensions(srcW, srcH, maxCols, maxRows int) (cols, rows int) {
	if srcW <= 0 || srcH <= 0 {
		return 0, 0
	}
	// Work in pixel space: the box is maxCols wide and maxRows*2 tall.
	maxPxW := float64(maxCols)
	maxPxH := float64(maxRows * 2)
	scale := maxPxW / float64(srcW)
	if h := maxPxH / float64(srcH); h < scale {
		scale = h
	}
	if scale > 1 {
		scale = 1 // never upscale; keep the low-res look
	}
	pxW := int(float64(srcW) * scale)
	pxH := int(float64(srcH) * scale)
	if pxW < 1 {
		pxW = 1
	}
	if pxH < 2 {
		pxH = 2
	}
	// Convert back to cells; round the row count up to an even pixel height.
	cols = pxW
	rows = (pxH + 1) / 2
	if cols > maxCols {
		cols = maxCols
	}
	if rows > maxRows {
		rows = maxRows
	}
	return cols, rows
}

// composite blends a (possibly translucent) pixel over the background color,
// returning an opaque color. Fully opaque pixels are returned unchanged.
func composite(c, bg color.Color) color.Color {
	r, g, b, a := c.RGBA()
	if a == 0xffff {
		return c
	}
	br, bgc, bb, _ := bg.RGBA()
	// Standard "over" alpha compositing in 16-bit space.
	inv := 0xffff - a
	out := color.RGBA64{
		R: uint16(r + br*inv/0xffff),
		G: uint16(g + bgc*inv/0xffff),
		B: uint16(b + bb*inv/0xffff),
		A: 0xffff,
	}
	return out
}

// sgr builds the SGR escape sequence that sets the foreground (top pixel) and
// background (bottom pixel) colors at the fidelity of the given profile.
func sgr(fg, bg color.Color, profile colorprofile.Profile) string {
	if profile >= colorprofile.TrueColor {
		fr, fgc, fb := rgb8(fg)
		br, bgc, bb := rgb8(bg)
		return fmt.Sprintf("\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm", fr, fgc, fb, br, bgc, bb)
	}
	return fmt.Sprintf("\x1b[38;5;%d;48;5;%dm", index256(fg, profile), index256(bg, profile))
}

// rgb8 reduces a color to 8-bit RGB components.
func rgb8(c color.Color) (r, g, b uint8) {
	cr, cg, cb, _ := c.RGBA()
	return uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8)
}

// index256 converts a color to its nearest 256-color palette index using the
// supplied profile.
func index256(c color.Color, profile colorprofile.Profile) uint8 {
	cc := profile.Convert(c)
	if idx, ok := cc.(ansi.IndexedColor); ok {
		return uint8(idx)
	}
	if idx, ok := cc.(ansi.BasicColor); ok {
		return uint8(idx)
	}
	// Fallback: derive an index directly if conversion produced an
	// unexpected type.
	r, g, b := rgb8(c)
	return ansi256FromRGB(r, g, b)
}

// ansi256FromRGB maps an 8-bit RGB color to the xterm 256-color cube. It is a
// best-effort fallback used only when profile.Convert does not yield a known
// indexed color type.
func ansi256FromRGB(r, g, b uint8) uint8 {
	q := func(v uint8) int {
		switch {
		case v < 48:
			return 0
		case v < 115:
			return 1
		default:
			return int((v - 35) / 40)
		}
	}
	ri, gi, bi := q(r), q(g), q(b)
	return uint8(16 + 36*ri + 6*gi + bi)
}
