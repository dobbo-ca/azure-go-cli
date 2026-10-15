package auth

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// consoleFile restores the console mode the parent shell had.
type consoleFile struct {
	*os.File
	mode uint32
}

func (c consoleFile) Close() error {
	windows.SetConsoleMode(windows.Handle(c.Fd()), c.mode)
	return c.File.Close()
}

func openConsole() (io.WriteCloser, error) {
	f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	// readline only emulates ANSI on stdout; the console must parse it.
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		f.Close()
		return nil, err
	}
	if err := windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		f.Close()
		return nil, err
	}
	return consoleFile{f, mode}, nil
}
