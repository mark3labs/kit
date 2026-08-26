//go:build windows

package termgfx

// ptyPixelSize reports no pixel geometry on Windows.
//
// The console API exposes no per-pty pixel size, so image previews there fall
// back to half-block rendering. See the Unix implementation for why the report
// is required.
func ptyPixelSize(fd int) (width, height int) {
	return 0, 0
}
