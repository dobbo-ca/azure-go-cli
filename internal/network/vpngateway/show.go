package vpngateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, gatewayName, resourceGroup string) error {
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

	gateway, err := client.Get(ctx, resourceGroup, gatewayName, nil)
	if err != nil {
		return fmt.Errorf("failed to get virtual network gateway: %w", err)
	}

	data, err := json.MarshalIndent(gateway, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format virtual network gateway: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
