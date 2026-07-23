package vm

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Deallocate(ctx context.Context, cmd *cobra.Command, resourceGroup, name string, noWait bool) error {
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
		return fmt.Errorf("failed to create VM client: %w", err)
	}

	fmt.Printf("Deallocating VM '%s'...\n", name)
	poller, err := client.BeginDeallocate(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin deallocate: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "deallocate started"})
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("deallocate failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("'%s' deallocated.", name)})
}
