package localgateway

import (
	"context"
	"fmt"

	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
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

	client, err := armnetwork.NewLocalNetworkGatewaysClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create local network gateway client: %w", err)
	}

	fmt.Printf("Deleting local network gateway '%s'...\n", name)
	poller, err := client.BeginDelete(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete local network gateway: %w", err)
	}

	if noWait {
		fmt.Printf("Delete operation started for local network gateway '%s'\n", name)
		return nil
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete local network gateway deletion: %w", err)
	}

	fmt.Printf("Deleted local network gateway '%s'\n", name)
	return nil
}
