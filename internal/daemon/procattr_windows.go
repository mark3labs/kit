//go:build windows

package daemon

import "syscall"

// detachedProcAttr is unused on Windows, where the local daemon is not
// supported yet.
func detachedProcAttr() *syscall.SysProcAttr { return nil }
