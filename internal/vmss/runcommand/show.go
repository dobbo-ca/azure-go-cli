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

func Show(ctx context.Context, cmd *cobra.Command, resourceGroup, vmssName, instanceID, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineScaleSetVMRunCommandsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create run commands client: %w", err)
	}

	resp, err := client.Get(ctx, resourceGroup, vmssName, instanceID, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get VMSS VM run command: %w", err)
	}
	return output.PrintJSON(cmd, resp.VirtualMachineRunCommand)
}
