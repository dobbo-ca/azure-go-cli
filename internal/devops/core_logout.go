package devops

import (
	"errors"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newCoreLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear the credential for all or a particular organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCoreLogout(cmd)
		},
	}

	// credential_clear has no `detect` parameter either — see core_login.go.
	cmd.Flags().String("organization", "", "Azure DevOps organization URL. Example: `https://dev.azure.com/MyOrganizationName/`. If no organization is specified, all organizations will be logged out.")
	cmd.Flags().String("org", "", "Alias for --organization.")

	return cmd
}

func runCoreLogout(cmd *cobra.Command) error {
	org, _ := cmd.Flags().GetString("organization")
	if org == "" {
		org, _ = cmd.Flags().GetString("org")
	}

	if err := ado.ClearPAT(org); err != nil {
		// DEVIATION: ado.ClearPAT returns "No credentials were found." for
		// both the bare-logout and the specific-org-not-found cases.
		// Python distinguishes them: a specific, never-set organization
		// raises "The credential was not found" (credential_store.py's
		// _CRDENTIAL_NOT_FOUND_MSG), while bare `devops logout` with
		// nothing at all raises "No credentials were found."
		// (_credentials.py:61). Rewritten here rather than in ado/auth.go,
		// which this task may not edit.
		if org != "" {
			return errors.New("The credential was not found")
		}
		return err
	}

	if org != "" {
		fmt.Println("The credential was successfully cleared.")
	} else {
		fmt.Println("Logged out of all Azure DevOps organizations.")
	}

	// _check_and_clear_default_organization (credentials.py:106-118): only
	// when --organization was given, and only if it equals the currently
	// configured default. Preserve any existing default project.
	if org != "" {
		cfgOrg, cfgProject, err := ado.ConfigDefaults()
		if err != nil {
			return err
		}
		if cfgOrg == org {
			if err := ado.SetConfigDefaults("", cfgProject); err != nil {
				return err
			}
		}
	}

	return nil
}
