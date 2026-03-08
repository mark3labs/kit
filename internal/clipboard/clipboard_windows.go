//go:build windows

package clipboard

// ReadImage reads image data from the system clipboard on Windows.
// Windows clipboard image support is not yet implemented.
func ReadImage() (*ImageData, error) {
	return nil, ErrNoClipboardTool
}
