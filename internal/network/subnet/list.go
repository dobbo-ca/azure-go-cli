package subnet

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

	client, err := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create subnets client: %w", err)
	}

	pager := client.NewListPager(resourceGroup, vnetName, nil)
	var subnets []map[string]interface{}

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list subnets: %w", err)
		}

		for _, subnet := range page.Value {
			subnets = append(subnets, formatSubnet(subnet))
		}
	}

	data, err := json.MarshalIndent(subnets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format subnets: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func formatSubnet(subnet *armnetwork.Subnet) map[string]interface{} {
	result := map[string]interface{}{
		"name": azure.GetStringValue(subnet.Name),
	}

	if subnet.Properties != nil {
		if subnet.Properties.AddressPrefix != nil {
			result["addressPrefix"] = *subnet.Properties.AddressPrefix
		}
		if subnet.Properties.ProvisioningState != nil {
			result["provisioningState"] = string(*subnet.Properties.ProvisioningState)
		}
		if subnet.Properties.NetworkSecurityGroup != nil && subnet.Properties.NetworkSecurityGroup.ID != nil {
			result["networkSecurityGroup"] = *subnet.Properties.NetworkSecurityGroup.ID
		}
		if subnet.Properties.RouteTable != nil && subnet.Properties.RouteTable.ID != nil {
			result["routeTable"] = *subnet.Properties.RouteTable.ID
		}
		if subnet.Properties.NatGateway != nil && subnet.Properties.NatGateway.ID != nil {
			result["natGateway"] = *subnet.Properties.NatGateway.ID
		}
	}

	return result
}
