//go:build windows

package daemon

// watchResize is a no-op on Windows: there is no SIGWINCH. The initial
// size is still sent at attach time; live resizing lands later.
func watchResize(fd uintptr, onChange func(cols, rows int)) func() {
	return func() {}
}
