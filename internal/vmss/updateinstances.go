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

func UpdateInstances(ctx context.Context, cmd *cobra.Command, resourceGroup, name string, instanceIDs []string, noWait bool) error {
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

	fmt.Printf("Updating instances in scale set '%s'...\n", name)
	poller, err := client.BeginUpdateInstances(ctx, resourceGroup, name, armcompute.VirtualMachineScaleSetVMInstanceRequiredIDs{
		InstanceIDs: to.SliceOfPtrs(instanceIDs...),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update instances: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "update instances started"})
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("update instances failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("instances updated in '%s'.", name)})
}
