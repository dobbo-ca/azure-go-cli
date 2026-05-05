package aks

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func OperationAbort(ctx context.Context, name, resourceGroup string, noWait bool) error {
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

	fmt.Printf("Aborting latest operation on AKS cluster '%s'...\n", name)
	poller, err := client.BeginAbortLatestOperation(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin abort: %w", err)
	}

	if noWait {
		fmt.Printf("Abort initiated for AKS cluster '%s' (running in background)\n", name)
		return nil
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to abort operation: %w", err)
	}

	fmt.Printf("Aborted latest operation on AKS cluster '%s'\n", name)
	return nil
}
