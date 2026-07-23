package actiongroup

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewActionGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create action group client: %w", err)
	}

	var items []*armmonitor.ActionGroupResource
	if resourceGroup != "" {
		pager := client.NewListByResourceGroupPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list action groups: %w", err)
			}
			items = append(items, page.Value...)
		}
	} else {
		pager := client.NewListBySubscriptionIDPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list action groups: %w", err)
			}
			items = append(items, page.Value...)
		}
	}
	return output.PrintJSON(cmd, items)
}
