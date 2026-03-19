//go:build linux

package clipboard

import (
	"os/exec"
)

// ReadImage reads image data from the system clipboard on Linux.
// It tries xclip first (X11), then falls back to wl-paste (Wayland).
func ReadImage() (*ImageData, error) {
	// Try xclip first (X11).
	if path, err := exec.LookPath("xclip"); err == nil {
		data, err := readWithXclip(path)
		if err == nil && len(data) > 0 {
			mediaType := DetectMediaType(data)
			if mediaType == "" {
				mediaType = "image/png" // xclip was asked for image/png
			}
			return &ImageData{Data: data, MediaType: mediaType}, nil
		}
	}

	// Fallback to wl-paste (Wayland).
	if path, err := exec.LookPath("wl-paste"); err == nil {
		data, err := readWithWlPaste(path)
		if err == nil && len(data) > 0 {
			mediaType := DetectMediaType(data)
			if mediaType == "" {
				mediaType = "image/png"
			}
			return &ImageData{Data: data, MediaType: mediaType}, nil
		}
	}

	// Check if either tool exists but just had no image.
	if _, err := exec.LookPath("xclip"); err == nil {
		return nil, ErrNoImage
	}
	if _, err := exec.LookPath("wl-paste"); err == nil {
		return nil, ErrNoImage
	}

	return nil, errNoClipboardTool
}

// readWithXclip reads image data using xclip.
func readWithXclip(xclipPath string) ([]byte, error) {
	cmd := exec.Command(xclipPath, "-selection", "clipboard", "-t", "image/png", "-o")
	return cmd.Output()
}

// readWithWlPaste reads image data using wl-paste.
func readWithWlPaste(wlPastePath string) ([]byte, error) {
	cmd := exec.Command(wlPastePath, "--type", "image/png")
	return cmd.Output()
}
