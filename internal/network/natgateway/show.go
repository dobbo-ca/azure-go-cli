package natgateway

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, gatewayName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armnetwork.NewNatGatewaysClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create nat gateways client: %w", err)
	}

	gateway, err := client.Get(ctx, resourceGroup, gatewayName, nil)
	if err != nil {
		return fmt.Errorf("failed to get nat gateway: %w", err)
	}

	return output.PrintJSON(cmd, gateway)
}
