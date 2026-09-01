//go:build linux

package daemon

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// processCmdline returns a process's command line.
func processCmdline(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", err
	}
	// /proc cmdline is NUL-separated.
	return strings.ReplaceAll(string(data), "\x00", " "), nil
}

// processOwnedBy reports whether a process carries the given daemon-home
// marker in its environment.
//
// Reading another process's environment is only permitted for processes
// with the same uid, which is exactly the check we want: a pid belonging
// to another user is unreadable, so it is never claimed.
func processOwnedBy(pid int, marker string) (bool, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return false, err
	}
	want := []byte(sessionOwnerEnv + "=" + marker)
	for entry := range bytes.SplitSeq(data, []byte{0}) {
		if bytes.Equal(entry, want) {
			return true, nil
		}
	}
	return false, nil
}
