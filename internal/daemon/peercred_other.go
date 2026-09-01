//go:build !windows && !linux && !darwin

package daemon

import "net"

// checkPeer is a no-op on platforms without a portable peer-credential
// API. The socket's 0600 mode inside a 0700 directory is what enforces
// single-user access there.
func checkPeer(net.Conn) error { return nil }
