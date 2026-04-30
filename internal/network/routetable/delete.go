package routetable

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

	client, err := armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create route tables client: %w", err)
	}

	poller, err := client.BeginDelete(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete route table: %w", err)
	}

	if noWait {
		fmt.Printf("Started deletion of route table '%s'\n", name)
		return nil
	}

	fmt.Printf("Deleting route table '%s'...\n", name)
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to delete route table: %w", err)
	}

	fmt.Printf("Deleted route table '%s'\n", name)
	return nil
}
