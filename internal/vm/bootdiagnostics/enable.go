package bootdiagnostics

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

func Enable(ctx context.Context, cmd *cobra.Command, resourceGroup, name, storageURI string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual machines client: %w", err)
	}

	boot := &armcompute.BootDiagnostics{Enabled: to.Ptr(true)}
	if storageURI != "" {
		boot.StorageURI = to.Ptr(storageURI)
	}
	update := armcompute.VirtualMachineUpdate{
		Properties: &armcompute.VirtualMachineProperties{
			DiagnosticsProfile: &armcompute.DiagnosticsProfile{
				BootDiagnostics: boot,
			},
		},
	}

	fmt.Printf("Enabling boot diagnostics on '%s'...\n", name)
	poller, err := client.BeginUpdate(ctx, resourceGroup, name, update, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "boot diagnostics enable started"})
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("boot diagnostics enabled on '%s'.", name)})
}
