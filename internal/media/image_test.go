package media

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand"
	"testing"
)

// solidPNG returns a PNG of the given size filled with one colour. A solid
// fill compresses very well, so use it when the test cares about dimensions
// rather than payload size.
func solidPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 10, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// noisePNG returns a PNG of the given size filled with pseudo-random pixels.
// Noise defeats PNG compression, so the payload is large. Use it when the
// test needs to push against a byte budget.
func noisePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	rng := rand.New(rand.NewSource(1))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{
				R: uint8(rng.Intn(256)),
				G: uint8(rng.Intn(256)),
				B: uint8(rng.Intn(256)),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestDetectMediaType(t *testing.T) {
	small := solidPNG(t, 4, 4)

	var jpg bytes.Buffer
	img, err := png.Decode(bytes.NewReader(small))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}

	var g bytes.Buffer
	if err := gif.Encode(&g, img, nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}

	tests := []struct {
		name string
		data []byte
		file string
		want string
	}{
		{"png by signature", small, "x.bin", MediaTypePNG},
		{"jpeg by signature", jpg.Bytes(), "x.bin", MediaTypeJPEG},
		{"gif by signature", g.Bytes(), "x.bin", MediaTypeGIF},
		{"webp by riff container", []byte("RIFF\x00\x00\x00\x00WEBPVP8 rest"), "x.bin", MediaTypeWebP},
		{"plain text is not an image", []byte("package main\n"), "main.go", ""},
		{"empty data", nil, "x.png", MediaTypePNG},
		{"unknown extension", []byte{0x00, 0x01, 0x02, 0x03}, "x.dat", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectMediaType(tc.data, tc.file); got != tc.want {
				t.Errorf("DetectMediaType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsImageExt(t *testing.T) {
	yes := []string{"a.png", "a.PNG", "a.jpg", "a.jpeg", "a.gif", "a.webp", "/tmp/x/y.Png"}
	no := []string{"a.go", "a.txt", "a", "a.pngx", "a.svg"}
	for _, n := range yes {
		if !IsImageExt(n) {
			t.Errorf("IsImageExt(%q) = false, want true", n)
		}
	}
	for _, n := range no {
		if IsImageExt(n) {
			t.Errorf("IsImageExt(%q) = true, want false", n)
		}
	}
}

func TestNormalizePassesSmallImageThrough(t *testing.T) {
	data := solidPNG(t, 64, 48)
	res, err := Normalize(data, "small.png", Limits{})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if res.Resized {
		t.Error("Resized = true, want false for an image inside the budget")
	}
	if !bytes.Equal(res.Data, data) {
		t.Error("Data was re-encoded; an image inside the budget must pass through untouched")
	}
	if res.Width != 64 || res.Height != 48 {
		t.Errorf("size = %dx%d, want 64x48", res.Width, res.Height)
	}
	if res.MediaType != MediaTypePNG {
		t.Errorf("MediaType = %q, want %q", res.MediaType, MediaTypePNG)
	}
}

func TestNormalizeScalesDownToMaxEdge(t *testing.T) {
	data := solidPNG(t, 2400, 1200)
	res, err := Normalize(data, "wide.png", Limits{MaxEdge: 600})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if !res.Resized {
		t.Error("Resized = false, want true")
	}
	if res.Width != 600 {
		t.Errorf("Width = %d, want 600", res.Width)
	}
	if res.Height != 300 {
		t.Errorf("Height = %d, want 300 (aspect ratio must hold)", res.Height)
	}
	if res.OriginalWidth != 2400 || res.OriginalHeight != 1200 {
		t.Errorf("original size = %dx%d, want 2400x1200", res.OriginalWidth, res.OriginalHeight)
	}
	// The payload must really decode at the reported size.
	assertDecodesAt(t, res.Data, res.MediaType, 600, 300)
}

func TestNormalizeKeepsPortraitAspectRatio(t *testing.T) {
	data := solidPNG(t, 500, 2000)
	res, err := Normalize(data, "tall.png", Limits{MaxEdge: 200})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if res.Height != 200 {
		t.Errorf("Height = %d, want 200", res.Height)
	}
	if res.Width != 50 {
		t.Errorf("Width = %d, want 50", res.Width)
	}
}

func TestNormalizeFallsBackToJPEGForByteBudget(t *testing.T) {
	// Noise is incompressible, so PNG cannot meet a tight byte budget and
	// the encoder must switch to JPEG.
	data := noisePNG(t, 400, 400)
	res, err := Normalize(data, "noise.png", Limits{MaxEdge: 400, MaxEncodedBytes: 20_000})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if len(res.Data) > 20_000 {
		t.Errorf("len(Data) = %d, want at most 20000", len(res.Data))
	}
	if res.MediaType != MediaTypeJPEG {
		t.Errorf("MediaType = %q, want %q; PNG cannot meet this budget", res.MediaType, MediaTypeJPEG)
	}
	assertDecodesAt(t, res.Data, res.MediaType, res.Width, res.Height)
}

func TestNormalizeShrinksUntilByteBudgetIsMet(t *testing.T) {
	// A budget too small for any JPEG at full size forces the loop to step
	// the dimensions down as well.
	data := noisePNG(t, 800, 800)
	res, err := Normalize(data, "noise.png", Limits{MaxEdge: 800, MaxEncodedBytes: 4_000})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if len(res.Data) > 4_000 {
		t.Errorf("len(Data) = %d, want at most 4000", len(res.Data))
	}
	if res.Width >= 800 {
		t.Errorf("Width = %d, want below 800; the loop must scale down", res.Width)
	}
}

func TestNormalizeRejectsNonImage(t *testing.T) {
	_, err := Normalize([]byte("package main\n\nfunc main() {}\n"), "main.go", Limits{})
	if !errors.Is(err, ErrNotImage) {
		t.Errorf("error = %v, want ErrNotImage", err)
	}
}

func TestNormalizeRejectsCorruptImage(t *testing.T) {
	data := solidPNG(t, 32, 32)
	truncated := data[:len(data)/2]
	_, err := Normalize(truncated, "truncated.png", Limits{})
	if !errors.Is(err, ErrDecode) {
		t.Errorf("error = %v, want ErrDecode", err)
	}
}

func TestNormalizeRejectsAboveIngestLimit(t *testing.T) {
	// Build a payload above the ingest limit without allocating a real
	// image of that size: the limit is checked before any decode.
	data := make([]byte, IngestLimitBytes+1)
	copy(data, solidPNG(t, 8, 8))
	_, err := Normalize(data, "huge.png", Limits{})
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("error = %v, want ErrTooLarge", err)
	}
}

func TestNormalizeNoResizeRejectsOversized(t *testing.T) {
	data := solidPNG(t, 4000, 4000)
	_, err := Normalize(data, "big.png", Limits{MaxEdge: 100, NoResize: true})
	if !errors.Is(err, ErrIrreducible) {
		t.Errorf("error = %v, want ErrIrreducible", err)
	}
}

func TestNormalizeNoResizeAllowsImageInsideBudget(t *testing.T) {
	data := solidPNG(t, 50, 50)
	res, err := Normalize(data, "small.png", Limits{MaxEdge: 100, NoResize: true})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if !bytes.Equal(res.Data, data) {
		t.Error("Data changed; NoResize must still pass an image inside the budget through")
	}
}

func TestNormalizeAcceptsJPEGAndGIF(t *testing.T) {
	src, err := png.Decode(bytes.NewReader(solidPNG(t, 300, 150)))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	var jpgBuf bytes.Buffer
	if err := jpeg.Encode(&jpgBuf, src, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, src, nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}

	for _, tc := range []struct {
		name string
		data []byte
		file string
		want string
	}{
		{"jpeg", jpgBuf.Bytes(), "a.jpg", MediaTypeJPEG},
		{"gif", gifBuf.Bytes(), "a.gif", MediaTypeGIF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Normalize(tc.data, tc.file, Limits{})
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if res.MediaType != tc.want {
				t.Errorf("MediaType = %q, want %q", res.MediaType, tc.want)
			}
			if res.Width != 300 || res.Height != 150 {
				t.Errorf("size = %dx%d, want 300x150", res.Width, res.Height)
			}
		})
	}
}

func TestDefaultLimitsAreApplied(t *testing.T) {
	// A 3000 px image must come back at the default maximum edge.
	data := solidPNG(t, 3000, 3000)
	res, err := Normalize(data, "big.png", Limits{})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if res.Width != DefaultMaxEdge || res.Height != DefaultMaxEdge {
		t.Errorf("size = %dx%d, want %dx%d", res.Width, res.Height, DefaultMaxEdge, DefaultMaxEdge)
	}
	if len(res.Data) > DefaultMaxEncodedBytes {
		t.Errorf("len(Data) = %d, want at most %d", len(res.Data), DefaultMaxEncodedBytes)
	}
}

func TestFitWithin(t *testing.T) {
	tests := []struct {
		w, h, maxEdge int
		wantW, wantH  int
	}{
		{100, 100, 200, 100, 100}, // never scales up
		{400, 200, 200, 200, 100},
		{200, 400, 200, 100, 200},
		{400, 400, 200, 200, 200},
		{10000, 1, 100, 100, 1}, // extreme ratio keeps at least one pixel
	}
	for _, tc := range tests {
		gotW, gotH := fitWithin(tc.w, tc.h, tc.maxEdge)
		if gotW != tc.wantW || gotH != tc.wantH {
			t.Errorf("fitWithin(%d, %d, %d) = %d, %d; want %d, %d",
				tc.w, tc.h, tc.maxEdge, gotW, gotH, tc.wantW, tc.wantH)
		}
	}
}

// assertDecodesAt checks that data really is a decodable image of the given
// media type and size. It guards against a result that reports dimensions the
// payload does not have.
func assertDecodesAt(t *testing.T, data []byte, mediaType string, w, h int) {
	t.Helper()
	img, err := decode(data, mediaType)
	if err != nil {
		t.Fatalf("result does not decode as %s: %v", mediaType, err)
	}
	if got := img.Bounds(); got.Dx() != w || got.Dy() != h {
		t.Errorf("decoded size = %dx%d, want %dx%d", got.Dx(), got.Dy(), w, h)
	}
}
