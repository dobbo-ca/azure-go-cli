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

func Delete(ctx context.Context, cmd *cobra.Command, resourceGroup, vmssName, instanceID, name string, noWait bool) error {
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

	fmt.Printf("Deleting run command '%s'...\n", name)
	poller, err := client.BeginDelete(ctx, resourceGroup, vmssName, instanceID, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "delete started"})
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("'%s' deleted.", name)})
}
