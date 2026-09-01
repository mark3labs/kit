//go:build windows

package daemon

import (
	"os"

	"golang.org/x/sys/windows"
)

// processExists reports whether a pid is live.
//
// os.Process.Signal is no help here: on Windows it supports only os.Kill
// and returns EWINDOWS for anything else, so there is no signal-0
// equivalent. Wait on the process handle with a zero timeout instead.
func processExists(pid int) bool {
	// SYNCHRONIZE lets us wait on the handle; the process object becomes
	// signalled only once the process has terminated.
	h, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(h) }()

	// A zero timeout makes this a poll. WAIT_TIMEOUT means the handle is
	// still unsignalled, so the process is running.
	//
	// GetExitCodeProcess is not used for this: it reports STILL_ACTIVE
	// (259) for a live process, but a process that genuinely exited with
	// code 259 is indistinguishable from one that is running, and the
	// recovery sweep would then treat a dead pid as live.
	switch ev, werr := windows.WaitForSingleObject(h, 0); {
	case werr != nil:
		return false
	case ev == uint32(windows.WAIT_TIMEOUT):
		return true
	default:
		return false
	}
}

// signalTerm asks a process to exit. Windows has no SIGTERM, so this is
// the same abrupt stop as signalKill; the daemon hosts no local sessions
// there in any case.
func signalTerm(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

func signalKill(pid int) error { return signalTerm(pid) }
