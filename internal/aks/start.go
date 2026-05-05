package aks

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Start(ctx context.Context, name, resourceGroup string, noWait bool) error {
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

	fmt.Printf("Starting AKS cluster '%s'...\n", name)
	poller, err := client.BeginStart(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin start: %w", err)
	}

	if noWait {
		fmt.Printf("Started AKS cluster '%s' (running in background)\n", name)
		return nil
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to start AKS cluster: %w", err)
	}

	fmt.Printf("Started AKS cluster '%s'\n", name)
	return nil
}
