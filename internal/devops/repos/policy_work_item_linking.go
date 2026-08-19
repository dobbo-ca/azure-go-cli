package repos

import (
	"context"

	"github.com/spf13/cobra"
)

func newPolicyWorkItemLinkingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work-item-linking",
		Short: "Manage work item linking policies.",
	}
	cmd.AddCommand(policyNoSettingsCreateCmd("create", "Create work item linking policy.", func(ctx context.Context, cmd *cobra.Command) error {
		return policyRunNoSettingsCreate(ctx, cmd, policyTypeWorkItemLinking)
	}))
	cmd.AddCommand(policyNoSettingsUpdateCmd("update", "Update work item linking policy.", func(ctx context.Context, cmd *cobra.Command) error {
		return policyRunNoSettingsUpdate(ctx, cmd, policyTypeWorkItemLinking)
	}))
	return cmd
}
