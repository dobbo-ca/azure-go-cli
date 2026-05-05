package aks

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Stop(ctx context.Context, name, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create AKS client: %w", err)
	}

	fmt.Printf("Stopping AKS cluster '%s'...\n", name)
	poller, err := client.BeginStop(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin stop: %w", err)
	}

	if noWait {
		fmt.Printf("Stopping AKS cluster '%s' (running in background)\n", name)
		return nil
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to stop AKS cluster: %w", err)
	}

	fmt.Printf("Stopped AKS cluster '%s'\n", name)
	return nil
}
