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
// equivalent. Ask the OS for the process's exit code instead — a live
// process reports STILL_ACTIVE.
// stillActive is the exit code Windows reports for a running process
// (STILL_ACTIVE / STATUS_PENDING).
const stillActive = 259

func processExists(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
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
