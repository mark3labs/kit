//go:build darwin

package daemon

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// checkPeer rejects a local connection from another user.
//
// Darwin reports the peer's credentials through LOCAL_PEERCRED as an
// xucred, captured at connect(2) time.
func checkPeer(conn net.Conn) error {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("daemon: local connection is not a unix socket")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return fmt.Errorf("daemon: peer credentials: %w", err)
	}
	var cred *unix.Xucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
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
