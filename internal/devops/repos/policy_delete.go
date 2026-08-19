package repos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPolicyDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunDelete(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	// Python (dev/repos/commands.py:63) registers this command with a knack
	// confirmation= prompt but no explicit --yes argument, unlike `repos
	// delete`. Per this port's convention every destructive command gets
	// --yes/-y so it can be scripted non-interactively.
	ado.AddYesFlag(cmd)

	return cmd
}

// policyRunDelete ports delete_policy, policy.py:56-62.
func policyRunDelete(ctx context.Context, cmd *cobra.Command) error {
	policyID, err := policyIDValue(cmd)
	if err != nil {
		return err
	}

	if err := ado.Confirm(cmd, "Are you sure you want to delete this policy?"); err != nil {
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

	var result any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Scope:      dctx.Project,
		Path:       "policy/configurations/" + url.PathEscape(policyID),
		APIVersion: "5.0",
	}, &result); err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	return ado.Print(cmd, result)
}
