package managementgroup

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/managementgroups/armmanagementgroups"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func newAPIClient() (*armmanagementgroups.APIClient, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	return armmanagementgroups.NewAPIClient(cred, nil)
}

// GetTenantBackfill returns the tenant backfill status.
func GetTenantBackfill(ctx context.Context, cmd *cobra.Command) error {
	client, err := newAPIClient()
	if err != nil {
		return err
	}
	resp, err := client.TenantBackfillStatus(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get tenant backfill status: %w", err)
	}
	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, resp.TenantBackfillStatusResult, format)
}

// StartTenantBackfill starts backfilling subscriptions for the tenant.
func StartTenantBackfill(ctx context.Context, cmd *cobra.Command) error {
	client, err := newAPIClient()
	if err != nil {
		return err
	}
	resp, err := client.StartTenantBackfill(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start tenant backfill: %w", err)
	}
	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, resp.TenantBackfillStatusResult, format)
}
