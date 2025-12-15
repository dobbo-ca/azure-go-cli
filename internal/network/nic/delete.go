package nic

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

	client, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create NIC client: %w", err)
	}

	fmt.Printf("Deleting network interface '%s'...\n", name)
	poller, err := client.BeginDelete(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete NIC: %w", err)
	}

	if noWait {
		fmt.Printf("Delete operation started for network interface '%s'\n", name)
		return nil
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete NIC deletion: %w", err)
	}

	fmt.Printf("Deleted network interface '%s'\n", name)
	return nil
}
