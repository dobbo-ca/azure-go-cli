package extension

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Upgrade(ctx context.Context, cmd *cobra.Command, resourceGroup, vmssName string, noWait bool) error {
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
		return fmt.Errorf("failed to create vmss rolling upgrades client: %w", err)
	}

	fmt.Printf("Starting extension upgrade on VMSS '%s'...\n", vmssName)
	poller, err := client.BeginStartExtensionUpgrade(ctx, resourceGroup, vmssName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin extension upgrade: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "extension upgrade started"})
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("extension upgrade failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("extension upgrade completed on VMSS '%s'.", vmssName)})
}
