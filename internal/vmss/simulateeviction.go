package vmss

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func SimulateEviction(ctx context.Context, cmd *cobra.Command, resourceGroup, name, instanceID string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineScaleSetVMsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VMSS VM client: %w", err)
	}

	if _, err := client.SimulateEviction(ctx, resourceGroup, name, instanceID, nil); err != nil {
		return fmt.Errorf("failed to simulate eviction: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("eviction simulated for instance '%s' in '%s'.", instanceID, name)})
}
