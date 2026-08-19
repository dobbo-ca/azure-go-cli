package devops

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newServiceEndpointCommand returns the `devops service-endpoint` command
// tree: list, show, create, update, delete, and the azurerm/github create
// subgroups (dev/team/commands.py:110-119).
func newServiceEndpointCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service-endpoint",
		Short: "Manage service endpoints/connections",
	}

	cmd.AddCommand(serviceendpointNewListCmd())
	cmd.AddCommand(serviceendpointNewShowCmd())
	cmd.AddCommand(serviceendpointNewCreateCmd())
	cmd.AddCommand(serviceendpointNewUpdateCmd())
	cmd.AddCommand(serviceendpointNewDeleteCmd())

	azurerm := &cobra.Command{
		Use:   "azurerm",
		Short: "Manage Azure RM service endpoints/connections",
	}
	azurerm.AddCommand(serviceendpointNewAzureRMCreateCmd())
	cmd.AddCommand(azurerm)

	github := &cobra.Command{
		Use:   "github",
		Short: "Manage GitHub service endpoints/connections",
	}
	github.AddCommand(serviceendpointNewGitHubCreateCmd())
	cmd.AddCommand(github)

	return cmd
}

// serviceendpointSecretFromEnvOrPrompt returns envVar's value if set,
// otherwise verifies stdin is a TTY and prompts twice for label, requiring
// the two entries to match — knack's prompt_pass(label, confirm=True), used
// by both `azurerm create` (service_endpoint.py:121-134) and `github create`
// (service_endpoint.py:165-172) for their respective secrets. nonTTYMsg is
// the verbatim CLIError text each command raises via
// verify_is_a_tty_or_raise_error when neither the env var nor a TTY is
// available.
func serviceendpointSecretFromEnvOrPrompt(envVar, label, nonTTYMsg string) (string, error) {
	// service_endpoint.py:123,165 test `in os.environ`, not truthiness, so
	// an exported-but-empty var is used verbatim rather than falling
	// through to the prompt/non-TTY error.
	if v, ok := os.LookupEnv(envVar); ok {
		return v, nil
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New(nonTTYMsg)
	}

	for {
		fmt.Printf("%s: ", label)
		first, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", label, err)
		}

		fmt.Printf("%s: ", label)
		second, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", label, err)
		}

		if string(first) != string(second) {
			fmt.Println("The two entered values do not match.")
			continue
		}
		return string(first), nil
	}
}
