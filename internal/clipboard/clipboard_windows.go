//go:build windows

package clipboard

// ReadImage reads image data from the system clipboard on Windows.
// Windows clipboard image support is not yet implemented.
func readSystemImage() (*ImageData, error) {
	return nil, errNoClipboardTool
}
