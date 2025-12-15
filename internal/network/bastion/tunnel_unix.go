//go:build unix

package bastion

import (
	"syscall"
)

// setTCPNoDelay sets TCP_NODELAY on a socket file descriptor (Unix/Linux/macOS)
func setTCPNoDelay(fd uintptr) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
}
