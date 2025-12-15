package peering

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, vnetName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armnetwork.NewVirtualNetworkPeeringsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual network peerings client: %w", err)
	}

	pager := client.NewListPager(resourceGroup, vnetName, nil)
	var peerings []map[string]interface{}

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list virtual network peerings: %w", err)
		}

		for _, peering := range page.Value {
			peerings = append(peerings, formatPeering(peering))
		}
	}

	data, err := json.MarshalIndent(peerings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format virtual network peerings: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func formatPeering(peering *armnetwork.VirtualNetworkPeering) map[string]interface{} {
	result := map[string]interface{}{
		"name": azure.GetStringValue(peering.Name),
	}

	if peering.Properties != nil {
		if peering.Properties.PeeringState != nil {
			result["peeringState"] = string(*peering.Properties.PeeringState)
		}
		if peering.Properties.ProvisioningState != nil {
			result["provisioningState"] = string(*peering.Properties.ProvisioningState)
		}
		if peering.Properties.RemoteVirtualNetwork != nil && peering.Properties.RemoteVirtualNetwork.ID != nil {
			result["remoteVirtualNetwork"] = *peering.Properties.RemoteVirtualNetwork.ID
		}
		if peering.Properties.AllowVirtualNetworkAccess != nil {
			result["allowVirtualNetworkAccess"] = *peering.Properties.AllowVirtualNetworkAccess
		}
		if peering.Properties.AllowForwardedTraffic != nil {
			result["allowForwardedTraffic"] = *peering.Properties.AllowForwardedTraffic
		}
		if peering.Properties.AllowGatewayTransit != nil {
			result["allowGatewayTransit"] = *peering.Properties.AllowGatewayTransit
		}
		if peering.Properties.UseRemoteGateways != nil {
			result["useRemoteGateways"] = *peering.Properties.UseRemoteGateways
		}
	}

	return result
}
