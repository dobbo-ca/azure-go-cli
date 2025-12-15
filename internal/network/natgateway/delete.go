package natgateway

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewNatGatewaysClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create nat gateways client: %w", err)
	}

	fmt.Printf("Deleting NAT gateway '%s'...\n", name)
	poller, err := client.BeginDelete(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete NAT gateway: %w", err)
	}

	if noWait {
		fmt.Printf("Started deletion of NAT gateway '%s'\n", name)
		return nil
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete NAT gateway: %w", err)
	}

	fmt.Printf("Deleted NAT gateway '%s'\n", name)
	return nil
}
