//go:build windows

package bastion

import (
	"syscall"
)

// setTCPNoDelay sets TCP_NODELAY on a socket handle (Windows)
func setTCPNoDelay(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
}
