//go:build !linux

package daemon

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// processCmdline returns a process's command line. Without /proc this asks
// ps, which reports argv for the calling user's own processes.
func processCmdline(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// processOwnedBy cannot verify process ownership without /proc.
//
// Returning an error is deliberate: the sweep treats an unverifiable
// process as "not ours" and leaves it alone. Leaking a session is
// recoverable; killing another daemon's session, or another user's
// process, is not.
func processOwnedBy(int, string) (bool, error) {
	return false, errors.New("daemon: cannot verify process ownership on this platform")
}
