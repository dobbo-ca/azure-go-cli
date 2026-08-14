package vnet

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, vnetName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual networks client: %w", err)
	}

	vnet, err := client.Get(ctx, resourceGroup, vnetName, nil)
	if err != nil {
		return fmt.Errorf("failed to get virtual network: %w", err)
	}

	return output.PrintJSON(cmd, vnet)
}
