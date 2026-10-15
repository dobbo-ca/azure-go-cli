//go:build !windows

package auth

import (
	"io"
	"os"
)

func openConsole() (io.WriteCloser, error) {
	return os.OpenFile("/dev/tty", os.O_WRONLY, 0)
}
