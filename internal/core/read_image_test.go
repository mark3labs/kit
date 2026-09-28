package core

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/media"
)

// writePNG writes a PNG of the given size to dir/name and returns its path.
// When noisy is true the pixels are pseudo-random, which defeats PNG
// compression and yields a large payload.
func writePNG(t *testing.T, dir, name string, w, h int, noisy bool) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(7))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 10, G: 120, B: 200, A: 255}
			if noisy {
				c = color.RGBA{
					R: uint8(rng.Intn(256)),
					G: uint8(rng.Intn(256)),
					B: uint8(rng.Intn(256)),
					A: 255,
				}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// callRead runs the read tool against path with the given image limits.
func callRead(t *testing.T, path string, limits media.Limits) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(readArgs{Path: path})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	resp, err := executeRead(context.Background(), fantasy.ToolCall{
		ID:    "call_1",
		Name:  "read",
		Input: string(input),
	}, "", limits)
	if err != nil {
		t.Fatalf("executeRead() error = %v", err)
	}
	return resp
}

func TestReadReturnsImageAsImageResponse(t *testing.T) {
	dir := t.TempDir()
	path := writePNG(t, dir, "shot.png", 120, 80, false)

	resp := callRead(t, path, media.Limits{})

	if resp.IsError {
		t.Fatalf("IsError = true, content = %q", resp.Content)
	}
	if resp.Type != "image" {
		t.Errorf("Type = %q, want \"image\"", resp.Type)
	}
	if resp.MediaType != media.MediaTypePNG {
		t.Errorf("MediaType = %q, want %q", resp.MediaType, media.MediaTypePNG)
	}
	if len(resp.Data) == 0 {
		t.Error("Data is empty; the image payload must be attached")
	}
	if !strings.Contains(resp.Content, "120x80") {
		t.Errorf("Content = %q, want the dimensions in the summary", resp.Content)
	}
	if strings.Contains(resp.Content, "1: ") {
		t.Error("Content has numbered lines; an image must not go through the text path")
	}
}

func TestReadImageIsNotLineNumbered(t *testing.T) {
	dir := t.TempDir()
	path := writePNG(t, dir, "a.png", 16, 16, false)

	resp := callRead(t, path, media.Limits{})

	// The summary must be a single line of plain prose, never raw bytes.
	if strings.Count(resp.Content, "\n") != 0 {
		t.Errorf("Content = %q, want a one line summary", resp.Content)
	}
	if !strings.HasPrefix(resp.Content, "Read image ") {
		t.Errorf("Content = %q, want it to start with \"Read image \"", resp.Content)
	}
}

func TestReadResizesOversizedImage(t *testing.T) {
	dir := t.TempDir()
	path := writePNG(t, dir, "big.png", 1000, 500, false)

	resp := callRead(t, path, media.Limits{MaxEdge: 200})

	if resp.Type != "image" {
		t.Fatalf("Type = %q, want \"image\"", resp.Type)
	}
	if !strings.Contains(resp.Content, "200x100") {
		t.Errorf("Content = %q, want the resized dimensions", resp.Content)
	}
	if !strings.Contains(resp.Content, "resized from 1000x500") {
		t.Errorf("Content = %q, want the original dimensions noted", resp.Content)
	}
	img, _, err := image.Decode(bytes.NewReader(resp.Data))
	if err != nil {
		t.Fatalf("attached payload does not decode: %v", err)
	}
	if img.Bounds().Dx() != 200 || img.Bounds().Dy() != 100 {
		t.Errorf("attached size = %dx%d, want 200x100", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestReadImageRespectsByteBudget(t *testing.T) {
	dir := t.TempDir()
	path := writePNG(t, dir, "noise.png", 600, 600, true)

	resp := callRead(t, path, media.Limits{MaxEdge: 600, MaxEncodedBytes: 30_000})

	if resp.IsError {
		t.Fatalf("IsError = true, content = %q", resp.Content)
	}
	if len(resp.Data) > 30_000 {
		t.Errorf("len(Data) = %d, want at most 30000", len(resp.Data))
	}
}

func TestReadTextFileIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	resp := callRead(t, path, media.Limits{})

	if resp.Type != "text" {
		t.Errorf("Type = %q, want \"text\"", resp.Type)
	}
	if len(resp.Data) != 0 {
		t.Error("Data is set for a text file; it must be empty")
	}
	if !strings.Contains(resp.Content, "1: package main") {
		t.Errorf("Content = %q, want numbered lines", resp.Content)
	}
}

func TestReadCorruptImageReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writePNG(t, dir, "broken.png", 64, 64, false)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(path, data[:len(data)/2], 0o644); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	resp := callRead(t, path, media.Limits{})

	if !resp.IsError {
		t.Fatalf("IsError = false, want true; content = %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "cannot read image") {
		t.Errorf("Content = %q, want a clear image error", resp.Content)
	}
	// The error must not carry binary noise into the context.
	if len(resp.Data) != 0 {
		t.Error("Data is set on an error response; it must be empty")
	}
}

func TestReadImageThatCannotFitBudgetReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writePNG(t, dir, "big.png", 800, 800, true)

	resp := callRead(t, path, media.Limits{MaxEdge: 800, NoResize: true, MaxEncodedBytes: 1000})

	if !resp.IsError {
		t.Fatalf("IsError = false, want true; content = %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "could not be resized") {
		t.Errorf("Content = %q, want the size limit explained", resp.Content)
	}
}

func TestReadDirectoryStillRejected(t *testing.T) {
	resp := callRead(t, t.TempDir(), media.Limits{})
	if !resp.IsError {
		t.Fatal("IsError = false, want true for a directory")
	}
	if !strings.Contains(resp.Content, "is a directory") {
		t.Errorf("Content = %q, want the directory message", resp.Content)
	}
}

func TestReadToolCarriesImageLimits(t *testing.T) {
	dir := t.TempDir()
	path := writePNG(t, dir, "big.png", 900, 900, false)

	tool := NewReadTool(WithImageLimits(media.Limits{MaxEdge: 64}))
	input, err := json.Marshal(readArgs{Path: path})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID: "c1", Name: "read", Input: string(input),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(resp.Content, "64x64") {
		t.Errorf("Content = %q, want WithImageLimits to apply", resp.Content)
	}
}
