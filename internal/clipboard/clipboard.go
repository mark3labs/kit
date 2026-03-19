// Package clipboard provides cross-platform clipboard image reading for Kit.
//
// Terminals cannot paste binary image data via bracketed paste — only text is
// supported. To read images we shell out to platform-specific clipboard tools:
//
//   - Linux X11:  xclip -selection clipboard -t image/png -o
//   - Linux Wayland: wl-paste --type image/png
//   - macOS: osascript + pbpaste (via a helper that reads NSPasteboard)
//   - Windows/WSL: powershell Get-Clipboard -Format Image (not yet supported)
//
// The ReadImage function returns the raw image bytes and detected MIME type,
// or an error if no image is available on the clipboard.
package clipboard

import (
	"fmt"
)

// ImageData holds the result of a clipboard image read.
type ImageData struct {
	// Data is the raw image bytes (PNG, JPEG, etc.).
	Data []byte
	// MediaType is the MIME type (e.g. "image/png", "image/jpeg").
	MediaType string
}

// DetectMediaType inspects the magic bytes of data to determine the image
// MIME type. Returns empty string if the format is not recognized.
func DetectMediaType(data []byte) string {
	if len(data) < 8 {
		return ""
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return "image/png"
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// GIF: 47 49 46 38
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return "image/gif"
	}

	// WebP: RIFF....WEBP
	if len(data) >= 12 &&
		data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}

	// BMP: 42 4D
	if data[0] == 0x42 && data[1] == 0x4D {
		return "image/bmp"
	}

	// TIFF: 49 49 2A 00 (little-endian) or 4D 4D 00 2A (big-endian)
	if (data[0] == 0x49 && data[1] == 0x49 && data[2] == 0x2A && data[3] == 0x00) ||
		(data[0] == 0x4D && data[1] == 0x4D && data[2] == 0x00 && data[3] == 0x2A) {
		return "image/tiff"
	}

	return ""
}

// ErrNoImage is returned when the clipboard does not contain image data.
var ErrNoImage = fmt.Errorf("no image data on clipboard")

// errNoClipboardTool is returned when no suitable clipboard tool is found.
var errNoClipboardTool = fmt.Errorf("no clipboard tool available (install xclip, wl-paste, or use macOS)")
