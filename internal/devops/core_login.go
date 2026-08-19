package devops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// coreAnonymousUserID is the sentinel authenticatedUser.id Azure DevOps
// returns for a request accepted by an org with public/anonymous access
// enabled, rather than an actual auth failure (credentials.py:121).
const coreAnonymousUserID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

func newCoreLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Set the credential (PAT) to use for a particular Azure DevOps organization, or as the fallback credential",
		Long:  "Refer https://aka.ms/azure-devops-cli-auth for more information on providing PAT as input.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCoreLogin(context.Background(), cmd)
		},
	}

	// login only declares --organization/--org (credential_set has no
	// `detect` parameter, so it gets no --detect flag — see
	// arguments.py:44-46 and foundation-spec.md §4.1).
	cmd.Flags().String("organization", "", "Azure DevOps organization URL. Example: `https://dev.azure.com/MyOrganizationName`")
	cmd.Flags().String("org", "", "Alias for --organization.")

	return cmd
}

func runCoreLogin(ctx context.Context, cmd *cobra.Command) error {
	org, _ := cmd.Flags().GetString("organization")
	if org == "" {
		org, _ = cmd.Flags().GetString("org")
	}

	pat, err := ado.PromptSecret("Token")
	if err != nil {
		return err
	}

	// Login with no --organization performs zero REST calls — the PAT is
	// stashed as the fallback credential (credentials.py:26-29).
	if org != "" {
		if err := coreVerifyToken(ctx, org, pat); err != nil {
			return errors.New("Failed to authenticate using the supplied token.")
		}
	}

	if err := ado.SetPAT(org, pat); err != nil {
		return err
	}

	// _check_and_set_default_organization (credentials.py:90-102): only
	// when --organization was given, and only if no default org is
	// currently configured. Preserve any existing default project — this
	// mirrors configure(defaults=['organization=...']), which merges into
	// the existing config rather than overwriting it.
	if org != "" {
		cfgOrg, cfgProject, err := ado.ConfigDefaults()
		if err != nil {
			return err
		}
		if cfgOrg == "" {
			if err := ado.SetConfigDefaults(org, cfgProject); err != nil {
				return err
			}
		}
	}

	return nil
}

// coreVerifyToken probes org with pat via the connectionData endpoint
// (credentials.py:57-70). It builds its own one-off Basic-auth request
// rather than going through ado.NewClient/resolveAuth, because the PAT under
// test here is not yet stored anywhere ado's auth precedence would find it —
// this is validating a brand new, in-memory candidate credential.
func coreVerifyToken(ctx context.Context, org, pat string) error {
	url := strings.TrimRight(org, "/") + "/_apis/connectionData?api-version=5.0-preview.1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+pat)))
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("connectionData returned status %d", resp.StatusCode)
	}

	var data struct {
		AuthenticatedUser struct {
			ID string `json:"id"`
		} `json:"authenticatedUser"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	if data.AuthenticatedUser.ID == coreAnonymousUserID {
		return errors.New("anonymous user")
	}
	return nil
}
