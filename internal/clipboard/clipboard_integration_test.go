//go:build integration

package clipboard_test

import (
	"os"
	"testing"

	"github.com/mark3labs/kit/internal/clipboard"
)

// TestReadImageIntegration tests reading an image from the system clipboard.
// Run with: WAYLAND_DISPLAY=wayland-1 go test -tags integration -v -run TestReadImageIntegration ./internal/clipboard/
//
// Prerequisites: copy an image to the clipboard first, e.g.:
//
//	WAYLAND_DISPLAY=wayland-1 wl-copy --type image/png < ~/Pictures/Screenshots/some_screenshot.png
func TestReadImageIntegration(t *testing.T) {
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
		t.Skip("no display server available (set WAYLAND_DISPLAY or DISPLAY)")
	}

	img, err := clipboard.ReadImage()
	if err != nil {
		t.Fatalf("ReadImage() error: %v", err)
	}

	if img == nil {
		t.Fatal("ReadImage() returned nil without error")
	}

	t.Logf("Image data: %d bytes", len(img.Data))
	t.Logf("Media type: %s", img.MediaType)

	if len(img.Data) == 0 {
		t.Fatal("image data is empty")
	}

	if img.MediaType == "" {
		t.Fatal("media type is empty")
	}

	// Verify magic bytes match the declared media type.
	detected := clipboard.DetectMediaType(img.Data)
	if detected == "" {
		t.Fatal("could not detect image format from magic bytes")
	}
	t.Logf("Detected format: %s", detected)

	if detected != img.MediaType {
		t.Errorf("media type mismatch: declared=%s detected=%s", img.MediaType, detected)
	}
}

func TestDetectMediaType(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{"PNG", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}, "image/png"},
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49}, "image/jpeg"},
		{"GIF", []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x00, 0x00, 0x00}, "image/gif"},
		{"BMP", []byte{0x42, 0x4D, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "image/bmp"},
		{"WebP", []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}, "image/webp"},
		{"TIFF-LE", []byte{0x49, 0x49, 0x2A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "image/tiff"},
		{"TIFF-BE", []byte{0x4D, 0x4D, 0x00, 0x2A, 0x00, 0x00, 0x00, 0x00, 0x00}, "image/tiff"},
		{"unknown", []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, ""},
		{"too short", []byte{0x89, 0x50}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clipboard.DetectMediaType(tt.data)
			if got != tt.expected {
				t.Errorf("DetectMediaType() = %q, want %q", got, tt.expected)
			}
		})
	}
}
