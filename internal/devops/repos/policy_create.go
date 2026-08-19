package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newPolicyCreateCmd is the generic/config-file form: `az repos policy
// create`. The typed sub-group create commands (approver-count, build, ...)
// live in their own files.
func newPolicyCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a policy using a configuration file.",
		Long: "Create a policy using a configuration file. Recommended when creating policies using " +
			"multiple scopes for a policy. See https://aka.ms/azure-devops-cli-docs-policy-file for more information.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunCreateFile(context.Background(), cmd)
		},
	}

	policyAddConfigFlags(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunCreateFile ports create_policy_configuration_file, policy.py:65-76.
func policyRunCreateFile(ctx context.Context, cmd *cobra.Command) error {
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
		Method:     http.MethodPost,
		Scope:      dctx.Project,
		Path:       "policy/configurations",
		APIVersion: "5.0",
		Body:       configuration,
	}, &result); err != nil {
		return fmt.Errorf("failed to create policy: %w", err)
	}

	return ado.Print(cmd, result, policyColumns...)
}
