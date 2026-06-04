package imagepreview

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// makePNG builds a simple w x h PNG filled with the given color and returns
// its encoded bytes.
func makePNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestRenderTrueColor(t *testing.T) {
	data := makePNG(t, 20, 20, color.RGBA{R: 255, A: 255})
	out, err := renderWithProfile(data, 10, 5, color.Black, colorprofile.TrueColor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty thumbnail for truecolor profile")
	}
	if !strings.Contains(out, upperHalfBlock) {
		t.Error("output should contain upper half block glyphs")
	}
	if !strings.Contains(out, "\x1b[38;2;") || !strings.Contains(out, "48;2;") {
		t.Errorf("expected truecolor SGR sequences, got %q", out)
	}
	// Red fill should appear as 255;0;0 somewhere.
	if !strings.Contains(out, "255;0;0") {
		t.Errorf("expected red color in output, got %q", out)
	}
}

func TestRenderANSI256(t *testing.T) {
	data := makePNG(t, 20, 20, color.RGBA{G: 255, A: 255})
	out, err := renderWithProfile(data, 8, 4, color.Black, colorprofile.ANSI256)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty thumbnail for ANSI256 profile")
	}
	if !strings.Contains(out, "\x1b[38;5;") || !strings.Contains(out, "48;5;") {
		t.Errorf("expected 256-color SGR sequences, got %q", out)
	}
	if strings.Contains(out, "38;2;") {
		t.Errorf("ANSI256 output should not contain truecolor sequences, got %q", out)
	}
}

func TestRenderDegradesBelowANSI256(t *testing.T) {
	data := makePNG(t, 20, 20, color.RGBA{B: 255, A: 255})
	for _, p := range []colorprofile.Profile{colorprofile.ANSI, colorprofile.ASCII, colorprofile.NoTTY} {
		out, err := renderWithProfile(data, 10, 5, color.Black, p)
		if err != nil {
			t.Fatalf("profile %v: unexpected error: %v", p, err)
		}
		if out != "" {
			t.Errorf("profile %v: expected empty fallback, got %q", p, out)
		}
	}
}

func TestRenderInvalidImage(t *testing.T) {
	out, err := renderWithProfile([]byte("not an image"), 10, 5, color.Black, colorprofile.TrueColor)
	if err == nil {
		t.Fatal("expected error for invalid image data")
	}
	if out != "" {
		t.Errorf("expected empty output on decode error, got %q", out)
	}
}

func TestRenderZeroBox(t *testing.T) {
	data := makePNG(t, 20, 20, color.White)
	out, err := renderWithProfile(data, 0, 0, color.Black, colorprofile.TrueColor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output for zero-sized box, got %q", out)
	}
}

func TestRenderNilBackgroundDefaults(t *testing.T) {
	data := makePNG(t, 10, 10, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	out, err := renderWithProfile(data, 6, 3, nil, colorprofile.TrueColor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected output with nil background (defaults to black)")
	}
}

func TestRowCountWithinBounds(t *testing.T) {
	// A tall image should be capped at maxRows cells.
	data := makePNG(t, 10, 100, color.White)
	out, err := renderWithProfile(data, 20, 6, color.Black, colorprofile.TrueColor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows := strings.Count(out, "\n") + 1
	if rows > 6 {
		t.Errorf("expected at most 6 rows, got %d", rows)
	}
}

func TestColumnCountWithinBounds(t *testing.T) {
	// A wide image should be capped at maxCols cells per row.
	data := makePNG(t, 100, 10, color.White)
	out, err := renderWithProfile(data, 8, 20, color.Black, colorprofile.TrueColor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	firstRow := strings.SplitN(out, "\n", 2)[0]
	cols := strings.Count(firstRow, upperHalfBlock)
	if cols > 8 {
		t.Errorf("expected at most 8 columns, got %d", cols)
	}
	if cols == 0 {
		t.Error("expected at least one column")
	}
}

func TestFitDimensionsPreservesAspect(t *testing.T) {
	// 2:1 (wide) image into a 40x20 box. Pixel box is 40x40; width-bound.
	cols, rows := fitDimensions(200, 100, 40, 20)
	if cols != 40 {
		t.Errorf("expected 40 cols, got %d", cols)
	}
	// pxH = 100 * (40/200) = 20 → 10 rows.
	if rows != 10 {
		t.Errorf("expected 10 rows, got %d", rows)
	}
}

func TestFitDimensionsNeverUpscales(t *testing.T) {
	cols, rows := fitDimensions(4, 4, 40, 20)
	if cols != 4 || rows != 2 {
		t.Errorf("expected 4x2 (no upscale), got %dx%d", cols, rows)
	}
}

func TestCompositeOpaquePassthrough(t *testing.T) {
	c := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	got := composite(c, color.White)
	if got != color.Color(c) {
		t.Errorf("opaque color should pass through unchanged, got %v", got)
	}
}

func TestCompositeTransparentOverBackground(t *testing.T) {
	// Fully transparent pixel over red background should yield red.
	got := composite(color.RGBA{}, color.RGBA{R: 255, A: 255})
	r, g, b, a := got.RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a != 0xffff {
		t.Errorf("expected opaque red, got r=%d g=%d b=%d a=%d", r>>8, g>>8, b>>8, a)
	}
}
