package vmss

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ListInstancePublicIPs(ctx context.Context, cmd *cobra.Command, resourceGroup, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP client: %w", err)
	}

	var items []*armnetwork.PublicIPAddress
	pager := client.NewListVirtualMachineScaleSetPublicIPAddressesPager(resourceGroup, name, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list instance public IPs: %w", err)
		}
		items = append(items, page.Value...)
	}
	return output.PrintJSON(cmd, items)
}
