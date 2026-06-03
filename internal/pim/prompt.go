package pim

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ErrNonInteractive is returned when a prompt is needed but the caller has
// disabled prompting (non-TTY or --no-input).
var ErrNonInteractive = errors.New("prompt required but running non-interactively")

// Prompter reads from In and writes to Out. IsTTY gates all prompting; when
// false, every prompt method returns ErrNonInteractive without touching In/Out.
type Prompter struct {
	In    io.Reader
	Out   io.Writer
	IsTTY bool
}

// NewPrompter returns a Prompter bound to stdin/stdout with IsTTY derived from
// the actual file descriptor (and overridden to false when noInput is true).
func NewPrompter(noInput bool) *Prompter {
	isTTY := !noInput && term.IsTerminal(int(os.Stdin.Fd()))
	return &Prompter{In: os.Stdin, Out: os.Stdout, IsTTY: isTTY}
}

// PromptString asks for a single line of input.
func (p *Prompter) PromptString(label string) (string, error) {
	if !p.IsTTY {
		return "", ErrNonInteractive
	}
	fmt.Fprintf(p.Out, "%s: ", label)
	line, err := bufio.NewReader(p.In).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// PickAssignment renders a numbered list and reads a 1-based selection.
func (p *Prompter) PickAssignment(label string, items []string) (int, error) {
	if !p.IsTTY {
		return 0, ErrNonInteractive
	}
	fmt.Fprintln(p.Out, label)
	for i, it := range items {
		fmt.Fprintf(p.Out, "  [%d] %s\n", i+1, it)
	}
	fmt.Fprint(p.Out, "Choose: ")
	line, err := bufio.NewReader(p.In).ReadString('\n')
	if err != nil && err != io.EOF {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return 0, fmt.Errorf("invalid selection %q: %w", strings.TrimSpace(line), err)
	}
	if n < 1 || n > len(items) {
		return 0, fmt.Errorf("selection %d out of range (1-%d)", n, len(items))
	}
	return n - 1, nil
}
