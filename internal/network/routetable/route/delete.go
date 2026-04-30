package route

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name, routeTableName, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewRoutesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create routes client: %w", err)
	}

	poller, err := client.BeginDelete(ctx, resourceGroup, routeTableName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete route: %w", err)
	}

	if noWait {
		fmt.Printf("Started deletion of route '%s'\n", name)
		return nil
	}

	fmt.Printf("Deleting route '%s' from route table '%s'...\n", name, routeTableName)
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to delete route: %w", err)
	}

	fmt.Printf("Deleted route '%s'\n", name)
	return nil
}
