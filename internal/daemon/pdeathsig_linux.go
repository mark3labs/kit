//go:build linux

package daemon

import "syscall"

// applyChildDeathSignal asks the kernel to SIGKILL this child the moment
// its parent dies, however the parent dies — including SIGKILL, a panic,
// or the OOM killer, none of which leave the daemon a chance to clean up.
//
// This is the only mechanism that cannot be skipped, so it is the primary
// guarantee that no session outlives its daemon. The session registry
// covers the platforms that have no equivalent.
//
// pty.StartWithSize supplies Setsid and Setctty; this adds to that struct
// rather than replacing it.
func applyChildDeathSignal(attr *syscall.SysProcAttr) *syscall.SysProcAttr {
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	attr.Pdeathsig = syscall.SIGKILL
	return attr
}
