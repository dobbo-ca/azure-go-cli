package blob

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, containerName, blobName string) error {
	client, err := newClient(ctx, cmd)
	if err != nil {
		return err
	}

	// Get blob properties
	blobClient := client.ServiceClient().NewContainerClient(containerName).NewBlobClient(blobName)
	props, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get blob properties: %w", storageError(err))
	}

	return output.PrintJSON(cmd, props)
}
