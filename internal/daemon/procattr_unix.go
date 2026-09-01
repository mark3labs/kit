//go:build !windows

package daemon

import "syscall"

// detachedProcAttr puts a spawned daemon in its own session, so it has no
// controlling terminal and does not receive the hangup that ends the shell
// which started it.
func detachedProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
