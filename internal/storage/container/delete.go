package container

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, accountName, containerName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armstorage.NewBlobContainersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob containers client: %w", err)
	}

	fmt.Printf("Deleting blob container '%s'...\n", containerName)
	_, err = client.Delete(ctx, resourceGroup, accountName, containerName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete blob container: %w", err)
	}

	fmt.Printf("Deleted blob container '%s'\n", containerName)
	return nil
}
