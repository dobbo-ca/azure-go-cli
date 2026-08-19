package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newPolicyUpdateCmd is the generic/config-file form: `az repos policy
// update`. Unlike the typed sub-group update commands, this one PUTs the
// file's contents verbatim - no GET-then-merge (policy.py:79-93).
func newPolicyUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a policy using a configuration file.",
		Long: "Update a policy using a configuration file. Recommended when creating policies using " +
			"multiple scopes for a policy. See https://aka.ms/azure-devops-cli-docs-policy-file for more information.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunUpdateFile(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	policyAddConfigFlags(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunUpdateFile ports update_policy_configuration_file, policy.py:79-93.
func policyRunUpdateFile(ctx context.Context, cmd *cobra.Command) error {
	policyID, err := policyIDValue(cmd)
	if err != nil {
		return err
	}
	path, err := policyConfigValue(cmd)
	if err != nil {
		return err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read policy configuration file: %w", err)
	}
	var configuration any
	if err := json.Unmarshal(raw, &configuration); err != nil {
		return fmt.Errorf("failed to parse policy configuration file: %w", err)
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPut,
		Scope:      dctx.Project,
		Path:       "policy/configurations/" + url.PathEscape(policyID),
		APIVersion: "5.0",
		Body:       configuration,
	}, &result); err != nil {
		return fmt.Errorf("failed to update policy: %w", err)
	}

	return ado.Print(cmd, result, policyColumns...)
}
