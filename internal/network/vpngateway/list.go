package vpngateway

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

	client, err := armnetwork.NewVirtualNetworkGatewaysClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual network gateways client: %w", err)
	}

	var gateways []map[string]interface{}

	if resourceGroup != "" {
		// List VPN gateways in specific resource group
		pager := client.NewListPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list virtual network gateways: %w", err)
			}

			for _, gw := range page.Value {
				gateways = append(gateways, formatVpnGateway(gw))
			}
		}
	} else {
		return fmt.Errorf("resource group is required for listing virtual network gateways")
	}

	return output.PrintJSON(cmd, gateways)
}

func formatVpnGateway(gw *armnetwork.VirtualNetworkGateway) map[string]interface{} {
	result := map[string]interface{}{
		"name":          azure.GetStringValue(gw.Name),
		"location":      azure.GetStringValue(gw.Location),
		"resourceGroup": getResourceGroupFromID(azure.GetStringValue(gw.ID)),
	}

	if gw.Properties != nil {
		if gw.Properties.GatewayType != nil {
			result["gatewayType"] = string(*gw.Properties.GatewayType)
		}
		if gw.Properties.VPNType != nil {
			result["vpnType"] = string(*gw.Properties.VPNType)
		}
		if gw.Properties.ProvisioningState != nil {
			result["provisioningState"] = string(*gw.Properties.ProvisioningState)
		}
		if gw.Properties.EnableBgp != nil {
			result["enableBgp"] = *gw.Properties.EnableBgp
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
