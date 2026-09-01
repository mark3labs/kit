//go:build darwin || freebsd || netbsd || openbsd

package daemon

import (
	"os/exec"
	"strconv"
	"strings"
)

// processCmdline returns a process's command line. The BSDs have no
// /proc, so this asks ps, which reports the full argv for the calling
// user's own processes.
func processCmdline(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
