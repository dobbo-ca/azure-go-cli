package rollingupgrade

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Cancel(ctx context.Context, cmd *cobra.Command, resourceGroup, vmssName string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineScaleSetRollingUpgradesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create rolling upgrades client: %w", err)
	}

	fmt.Printf("Cancelling rolling upgrade for '%s'...\n", vmssName)
	poller, err := client.BeginCancel(ctx, resourceGroup, vmssName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin cancel: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "cancel started"})
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("cancel failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("rolling upgrade for '%s' cancelled.", vmssName)})
}
