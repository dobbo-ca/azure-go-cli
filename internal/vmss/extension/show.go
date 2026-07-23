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

func Show(ctx context.Context, cmd *cobra.Command, resourceGroup, vmssName, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineScaleSetExtensionsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create vmss extensions client: %w", err)
	}

	resp, err := client.Get(ctx, resourceGroup, vmssName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get vmss extension: %w", err)
	}
	return output.PrintJSON(cmd, resp.VirtualMachineScaleSetExtension)
}
