package auth

import (
	"io"
	"os"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/manifoldco/promptui"
	"golang.org/x/term"
)

// console is the terminal when stdout is captured, nil otherwise.
var console io.WriteCloser

// useConsole sends login progress and prompts to the terminal when stdout is
// captured. promptui redraws by moving the cursor, which a pipe ignores, and
// progress left on the pipe reaches the screen late, inside the prompt, so
// both must share one writer. The returned func undoes it.
func useConsole() func() {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return func() {}
	}
	c, err := openConsole()
	if err != nil {
		return func() {}
	}
	console = c
	prev, prevStdout := logger.Output, logger.Stdout
	logger.SetOutput(c)
	logger.Stdout = c
	return func() {
		logger.SetOutput(prev)
		logger.Stdout = prevStdout
		console = nil
		c.Close()
	}
}

func runSelect(p *promptui.Select) (int, error) {
	if console != nil {
		p.Stdout = console
	}
	idx, _, err := p.Run()
	return idx, err
}
