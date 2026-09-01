//go:build !linux

package daemon

import "syscall"

// applyChildDeathSignal is a no-op where the kernel has no parent-death
// signal. The session registry sweep is the fallback there: the next
// daemon start kills anything a crashed run left behind.
func applyChildDeathSignal(attr *syscall.SysProcAttr) *syscall.SysProcAttr {
	return attr
}
