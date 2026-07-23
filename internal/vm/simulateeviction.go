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

func SimulateEviction(ctx context.Context, cmd *cobra.Command, resourceGroup, name string) error {
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

	if _, err := client.SimulateEviction(ctx, resourceGroup, name, nil); err != nil {
		return fmt.Errorf("failed to simulate eviction: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("eviction simulated for '%s'.", name)})
}
