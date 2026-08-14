package container

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, accountName, containerName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armstorage.NewBlobContainersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob containers client: %w", err)
	}

	container, err := client.Get(ctx, resourceGroup, accountName, containerName, nil)
	if err != nil {
		return fmt.Errorf("failed to get blob container: %w", err)
	}

	return output.PrintJSON(cmd, container)
}
