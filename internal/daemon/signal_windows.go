//go:build windows

package daemon

import "os"

// processExists reports whether a pid is live. Windows has no signal 0, so
// this opens a handle instead.
func processExists(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 is implemented as a liveness probe on Windows too.
	return p.Signal(os.Signal(nil)) == nil
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
