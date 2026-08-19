package ado

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// OpenBrowser opens rawURL in the user's default browser. Callers check
// --open *after* the API call succeeds and never let a browser failure fail
// the command — only warn.
func OpenBrowser(rawURL string) error {
	return browser.OpenURL(rawURL)
}

// AddYesFlag registers --yes/-y ("Do not prompt for confirmation.") on cmd.
func AddYesFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("yes", "y", false, "Do not prompt for confirmation.")
}

// Confirm prints msg + " (y/n): " and requires y/yes on stdin. It returns nil
// immediately when --yes is set, and an error when stdin is not a TTY and
// --yes was not passed.
func Confirm(cmd *cobra.Command, msg string) error {
	yes, _ := cmd.Flags().GetBool("yes")
	if yes {
		return nil
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("%s Use --yes to confirm non-interactively", msg)
	}

	fmt.Printf("%s (y/n): ", msg)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("operation cancelled")
	}
	return nil
}

// PromptSecret reads a secret with echo disabled when stdin is a TTY, and
// reads one line from stdin otherwise (this is what `echo $PAT | az devops
// login` relies on). Matches credentials.py:73-86: prompts "<label>: ",
// re-prompting while the entered value has length <= 1.
func PromptSecret(label string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("failed to read %s: %w", label, err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	for {
		fmt.Printf("%s: ", label)
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", label, err)
		}
		if len(b) > 1 {
			return string(b), nil
		}
		logger.Println("Please provide a PAT token.")
	}
}
