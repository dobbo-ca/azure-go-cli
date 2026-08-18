package vnet

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup string) error {
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

	var vnets []map[string]interface{}

	if resourceGroup != "" {
		// List VNets in specific resource group
		pager := client.NewListPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list virtual networks: %w", err)
			}

			for _, vnet := range page.Value {
				vnets = append(vnets, formatVNet(vnet))
			}
		}
	} else {
		// List all VNets in subscription
		pager := client.NewListAllPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list virtual networks: %w", err)
			}

			for _, vnet := range page.Value {
				vnets = append(vnets, formatVNet(vnet))
			}
		}
	}

	return output.PrintJSON(cmd, vnets)
}

func formatVNet(vnet *armnetwork.VirtualNetwork) map[string]interface{} {
	result := map[string]interface{}{
		"name":          azure.GetStringValue(vnet.Name),
		"location":      azure.GetStringValue(vnet.Location),
		"resourceGroup": getResourceGroupFromID(azure.GetStringValue(vnet.ID)),
	}

	if vnet.Properties != nil {
		if vnet.Properties.AddressSpace != nil && vnet.Properties.AddressSpace.AddressPrefixes != nil {
			result["addressPrefixes"] = vnet.Properties.AddressSpace.AddressPrefixes
		}
		if vnet.Properties.ProvisioningState != nil {
			result["provisioningState"] = string(*vnet.Properties.ProvisioningState)
		}
		if vnet.Properties.Subnets != nil {
			result["subnets"] = len(vnet.Properties.Subnets)
		}
	}

	return result
}

func getResourceGroupFromID(id string) string {
	parsed, err := arm.ParseResourceID(id)
	if err != nil {
		return ""
	}
	return parsed.ResourceGroupName
}
