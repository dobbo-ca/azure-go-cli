package repos

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPolicyListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List policies.",
		Long:  "List all policies in a project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunList(context.Background(), cmd)
		},
	}

	cmd.Flags().String("repository-id", "", "ID of the repository to filter results by exact match of the repository ID. For example --repository-ID e556f204-53c9-4153-9cd9-ef41a11e3345")
	cmd.Flags().String("branch", "", "Branch name to filter results by exact match of branch name. The --repository-id parameter is required to use the branch filter. For example: --branch master")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunList ports list_policy, policy.py:20-44.
func policyRunList(ctx context.Context, cmd *cobra.Command) error {
	repositoryID, _ := cmd.Flags().GetString("repository-id")
	branch, _ := cmd.Flags().GetString("branch")

	if branch != "" && repositoryID == "" {
		return fmt.Errorf("--repository-id is required with --branch")
	}

	var scope string
	if repositoryID != "" {
		scope = strings.ReplaceAll(repositoryID, "-", "")
		if branch != "" {
			scope = scope + ":" + policyResolveRefHeads(branch)
		}
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	req := ado.Request{
		Scope:      dctx.Project,
		Path:       "policy/configurations",
		APIVersion: "5.0",
	}
	if scope != "" {
		req.Query = url.Values{"scope": {scope}}
	}

	var policies []map[string]any
	if err := client.List(ctx, req, &policies); err != nil {
		return fmt.Errorf("failed to list policies: %w", err)
	}

	return ado.Print(cmd, policies, policyColumns...)
}
