package runcommand

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup, vmName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineRunCommandsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create run commands client: %w", err)
	}

	var items []*armcompute.VirtualMachineRunCommand
	pager := client.NewListByVirtualMachinePager(resourceGroup, vmName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list VM run commands: %w", err)
		}
		items = append(items, page.Value...)
	}
	return output.PrintJSON(cmd, items)
}
