package nic

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
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
	client, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create interfaces client: %w", err)
	}

	resp, err := client.GetVirtualMachineScaleSetNetworkInterface(ctx, resourceGroup, vmssName, instanceID, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get VMSS network interface: %w", err)
	}
	return output.PrintJSON(cmd, resp.Interface)
}
