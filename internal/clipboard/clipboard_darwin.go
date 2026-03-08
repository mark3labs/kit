//go:build darwin

package clipboard

import (
	"os/exec"
)

// ReadImage reads image data from the system clipboard on macOS.
// It uses osascript to check if the clipboard contains an image and then
// reads the data using a temporary approach. If the clipboard contains
// an image, it writes it to stdout as PNG data.
func ReadImage() (*ImageData, error) {
	// Use osascript to write clipboard image to stdout via a pipe.
	// The script checks if the clipboard has a «class PNGf» item.
	script := `use framework "AppKit"
set pb to current application's NSPasteboard's generalPasteboard()
set imgData to pb's dataForType:(current application's NSPasteboardTypePNG)
if imgData is missing value then
	set tiffData to pb's dataForType:(current application's NSPasteboardTypeTIFF)
	if tiffData is missing value then
		error "No image on clipboard"
	end if
	set bitmapRep to current application's NSBitmapImageRep's imageRepWithData:tiffData
	set imgData to bitmapRep's representationUsingType:(current application's NSPNGFileType) |properties|:(missing value)
end if
imgData's writeToFile:"/dev/stdout" atomically:false`

	cmd := exec.Command("osascript", "-l", "AppleScript", "-e", script)
	data, err := cmd.Output()
	if err != nil {
		return nil, ErrNoImage
	}

	if len(data) == 0 {
		return nil, ErrNoImage
	}

	mediaType := DetectMediaType(data)
	if mediaType == "" {
		mediaType = "image/png" // osascript converts to PNG
	}

	return &ImageData{Data: data, MediaType: mediaType}, nil
}
