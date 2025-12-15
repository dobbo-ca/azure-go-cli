package vnet

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, resourceGroup string) error {
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

	data, err := json.MarshalIndent(vnets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format virtual networks: %w", err)
	}

	fmt.Println(string(data))
	return nil
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
	// ID format: /subscriptions/{sub}/resourceGroups/{rg}/providers/...
	parts := make([]string, 0)
	for _, part := range []rune(id) {
		if part == '/' {
			parts = append(parts, "")
		} else if len(parts) > 0 {
			parts[len(parts)-1] += string(part)
		}
	}

	for i, part := range parts {
		if part == "resourceGroups" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
