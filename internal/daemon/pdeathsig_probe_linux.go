//go:build linux

package daemon

import "syscall"

// hasParentDeathSignal reports whether these attributes ask the kernel to
// signal the child when its parent dies. Linux is the platform that has
// the mechanism, so it is the platform where the tests can check both
// that a session child HAS it and that a supervisor does not.
func hasParentDeathSignal(attr *syscall.SysProcAttr) bool {
	return attr != nil && attr.Pdeathsig != 0
}
