//go:build !linux && !windows

package daemon

import "syscall"

// hasParentDeathSignal reports whether these attributes ask the kernel to
// signal the child when its parent dies.
//
// Only Linux has the mechanism. Everywhere else the answer is always no,
// which is why the portable session registry exists (see recovery.go) and
// why a supervisor on those platforms is safe from it by construction.
func hasParentDeathSignal(*syscall.SysProcAttr) bool { return false }
