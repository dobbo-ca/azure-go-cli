package runcommand

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

func Create(ctx context.Context, cmd *cobra.Command, resourceGroup, vmssName, instanceID, name, location, script string, noWait bool) error {
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

	runCommand := armcompute.VirtualMachineRunCommand{
		Location: to.Ptr(location),
		Properties: &armcompute.VirtualMachineRunCommandProperties{
			Source: &armcompute.VirtualMachineRunCommandScriptSource{
				Script: to.Ptr(script),
			},
		},
	}

	fmt.Printf("Creating run command '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vmssName, instanceID, name, runCommand, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "create started"})
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("create failed: %w", err)
	}
	return output.PrintJSON(cmd, resp.VirtualMachineRunCommand)
}
