//go:build windows

package daemon

import (
	"os/exec"
	"testing"
	"time"
)

// TestProcessExistsWithAmbiguousExitCode pins the one case that makes
// GetExitCodeProcess unusable for liveness: a process whose real exit code
// is 259, the same value Windows reports as STILL_ACTIVE.
//
// Reading the exit code would call this dead process live, and the
// recovery sweep would then leave a stale registry entry forever.
func TestProcessExistsWithAmbiguousExitCode(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit", "259")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a helper process: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err == nil {
		t.Fatal("expected a non-zero exit from the helper")
	}
	// The handle is signalled as soon as the process ends; allow a moment
	// for the kernel to settle before polling.
	time.Sleep(100 * time.Millisecond)

	if processExists(pid) {
		t.Fatal("a process that exited with code 259 was reported as live")
	}
}

// TestProcessExistsReportsALiveProcess is the positive case.
func TestProcessExistsReportsALiveProcess(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "ping", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a helper process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	if !processExists(cmd.Process.Pid) {
		t.Fatal("a running process was reported as dead")
	}
}
