package auth

import (
	"os"

	"github.com/manifoldco/promptui"
	"golang.org/x/term"
)

// runSelect draws the prompt on the console even when stdout is captured.
// promptui redraws by moving the cursor, which a pipe ignores, so a script
// that captures 'az login' output saw the list repeat on every keypress.
func runSelect(p *promptui.Select) (int, error) {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		if f, err := openConsole(); err == nil {
			defer f.Close()
			p.Stdout = f
		}
	}
	idx, _, err := p.Run()
	return idx, err
}
