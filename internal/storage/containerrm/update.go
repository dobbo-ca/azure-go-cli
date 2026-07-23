package containerrm

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, resourceGroup, account, name, publicAccess string, metadata map[string]string) error {
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

	bc := buildBlobContainer(publicAccess, metadata)
	resp, err := client.Update(ctx, resourceGroup, account, name, bc, nil)
	if err != nil {
		return fmt.Errorf("failed to update container: %w", err)
	}
	return output.PrintJSON(cmd, resp.BlobContainer)
}
