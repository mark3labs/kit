//go:build linux

package daemon

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// checkPeer rejects a local connection from another user.
//
// SO_PEERCRED reports the credentials the kernel recorded when the peer
// called connect(2), so the check cannot be raced by a process that execs
// or changes uid afterwards.
func checkPeer(conn net.Conn) error {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("daemon: local connection is not a unix socket")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return fmt.Errorf("daemon: peer credentials: %w", err)
	}
	var cred *unix.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return fmt.Errorf("daemon: peer credentials: %w", err)
	}
	if credErr != nil {
		return fmt.Errorf("daemon: peer credentials: %w", credErr)
	}
	if uint32(os.Getuid()) != cred.Uid {
		return fmt.Errorf("daemon: peer uid %d is not the daemon's uid %d", cred.Uid, os.Getuid())
	}
	return nil
}
