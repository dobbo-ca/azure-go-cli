package vnet

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, addressPrefixes []string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual networks client: %w", err)
	}

	current, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get virtual network: %w", err)
	}

	flags := cmd.Flags()

	if flags.Changed("address-prefixes") {
		if current.Properties == nil {
			current.Properties = &armnetwork.VirtualNetworkPropertiesFormat{}
		}
		if current.Properties.AddressSpace == nil {
			current.Properties.AddressSpace = &armnetwork.AddressSpace{}
		}
		azurePrefixes := make([]*string, 0, len(addressPrefixes))
		for _, prefix := range addressPrefixes {
			azurePrefixes = append(azurePrefixes, to.Ptr(prefix))
		}
		current.Properties.AddressSpace.AddressPrefixes = azurePrefixes
	}

	if flags.Changed("tags") {
		tags, _ := flags.GetStringToString("tags")
		azureTags := make(map[string]*string)
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		current.Tags = azureTags
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, current.VirtualNetwork, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update virtual network: %w", err)
	}

	if noWait {
		fmt.Printf("Started update of virtual network '%s'\n", name)
		return nil
	}

	fmt.Printf("Updating virtual network '%s'...\n", name)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update virtual network: %w", err)
	}

	return output.PrintJSON(cmd, result.VirtualNetwork)
}
