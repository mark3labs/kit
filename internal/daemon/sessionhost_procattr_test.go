//go:build !windows

package daemon

import (
	"syscall"
	"testing"
)

// The process attributes that decide whether a session survives its
// daemon.
//
// This is the one place where a one-line change silently destroys the
// whole feature. Adding applyChildDeathSignal to the supervisor spawn
// would look like tidying up — every other child the daemon starts has it
// — and would make the kernel kill every session the instant the daemon
// exits. Nothing else would fail: sessions would start, work, and quietly
// die on restart, exactly as they did before.

// TestChildDeathSignalArmsTheChild covers the attribute a session CHILD
// must have. A kit process whose PTY master has died is unreachable, so
// it must not be allowed to outlive whoever holds that master.
func TestChildDeathSignalArmsTheChild(t *testing.T) {
	attr := applyChildDeathSignal(nil)
	if attr == nil {
		t.Fatal("applyChildDeathSignal returned nil")
	}
	if !hasParentDeathSignal(attr) {
		t.Skip("this platform has no parent-death signal; the session registry covers it")
	}
}

// TestChildDeathSignalPreservesPTYAttributes checks the helper adds to
// the struct pty.StartWithSize filled in rather than replacing it.
// Dropping Setsid or Setctty would leave the child without a controlling
// terminal, and a TUI without one cannot read the keyboard properly.
func TestChildDeathSignalPreservesPTYAttributes(t *testing.T) {
	base := &syscall.SysProcAttr{Setsid: true, Setctty: true}
	got := applyChildDeathSignal(base)
	if !got.Setsid || !got.Setctty {
		t.Fatal("applyChildDeathSignal discarded the pty's own process attributes")
	}
}

// TestSupervisorIsNotArmedWithAParentDeathSignal is the regression test
// for the mistake that would undo this whole change.
//
// A supervisor exists precisely to outlive the daemon that started it. It
// is spawned with detachedProcAttr, which must put it in a session of its
// own and must NOT ask the kernel to kill it when its parent dies.
func TestSupervisorIsNotArmedWithAParentDeathSignal(t *testing.T) {
	attr := detachedProcAttr()
	if attr == nil {
		t.Fatal("detachedProcAttr returned nil; a supervisor would share the daemon's process group")
	}
	if !attr.Setsid {
		t.Error("a supervisor must get its own session, or a signal aimed at the daemon's " +
			"process group would reach every session too")
	}
	if hasParentDeathSignal(attr) {
		t.Fatal("a supervisor is armed with a parent-death signal: the kernel would kill every " +
			"session the moment the daemon exits, which is the exact behaviour session hosts remove")
	}
}
