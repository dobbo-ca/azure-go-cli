package blob

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, accountName, containerName, blobName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	// Build blob service URL
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)

	client, err := azblob.NewClient(serviceURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob client: %w", err)
	}

	// Get blob properties
	blobClient := client.ServiceClient().NewContainerClient(containerName).NewBlobClient(blobName)
	props, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get blob properties: %w", err)
	}

	return output.PrintJSON(cmd, props)
}
