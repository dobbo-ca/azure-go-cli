package vmss

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Scale(ctx context.Context, cmd *cobra.Command, resourceGroup, name string, newCapacity int64, noWait bool) error {
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

	fmt.Printf("Scaling scale set '%s' to %d instances...\n", name, newCapacity)
	poller, err := client.BeginUpdate(ctx, resourceGroup, name, armcompute.VirtualMachineScaleSetUpdate{
		SKU: &armcompute.SKU{Capacity: to.Ptr(newCapacity)},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin scale: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "scale started"})
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("scale failed: %w", err)
	}
	return output.PrintJSON(cmd, resp.VirtualMachineScaleSet)
}
