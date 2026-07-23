package vmss

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func GetOSUpgradeHistory(ctx context.Context, cmd *cobra.Command, resourceGroup, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineScaleSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VMSS client: %w", err)
	}

	var items []*armcompute.UpgradeOperationHistoricalStatusInfo
	pager := client.NewGetOSUpgradeHistoryPager(resourceGroup, name, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get OS upgrade history: %w", err)
		}
		items = append(items, page.Value...)
	}
	return output.PrintJSON(cmd, items)
}
