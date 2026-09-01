//go:build !windows

package daemon

import "syscall"

// processExists reports whether a pid is live and signalable by this user.
// Signal 0 performs the permission and existence checks without delivering
// anything.
func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// signalTerm asks a process to exit.
func signalTerm(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }

// signalKill ends a process that ignored SIGTERM.
func signalKill(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }
