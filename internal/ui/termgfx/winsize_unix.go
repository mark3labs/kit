//go:build !windows

package termgfx

import "golang.org/x/sys/unix"

// ptyPixelSize returns the pixel dimensions the terminal reports for the whole
// pty, or zeros when it reports none.
//
// A terminal emulator that draws images knows its own pixel geometry and fills
// these fields in. A multiplexer that only forwards text leaves them at zero,
// even while it forwards graphics queries to the terminal behind it and lets
// that terminal answer them. That makes this the honest test of whether the
// program on the other end of the pty is the one drawing.
//
// Kitty's own icat gates on the same report, and refuses to display an image
// when it is missing.
func ptyPixelSize(fd int) (width, height int) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil || ws == nil {
		return 0, 0
	}
	return int(ws.Xpixel), int(ws.Ypixel)
}
