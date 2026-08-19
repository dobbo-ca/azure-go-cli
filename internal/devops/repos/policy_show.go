package repos

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPolicyShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show policy details.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunShow(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunShow ports get_policy, policy.py:47-53.
func policyRunShow(ctx context.Context, cmd *cobra.Command) error {
	policyID, err := policyIDValue(cmd)
	if err != nil {
		return err
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
		Scope:      dctx.Project,
		Path:       "policy/configurations/" + url.PathEscape(policyID),
		APIVersion: "5.0",
	}, &result); err != nil {
		return fmt.Errorf("failed to get policy: %w", err)
	}

	return ado.Print(cmd, result, policyColumns...)
}
