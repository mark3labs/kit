//go:build !windows

package daemon

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/x/term"
)

// watchResize invokes onChange with the terminal size on every SIGWINCH.
// The returned function stops watching.
func watchResize(fd uintptr, onChange func(cols, rows int)) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
				if cols, rows, err := term.GetSize(fd); err == nil {
					onChange(cols, rows)
				}
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}
