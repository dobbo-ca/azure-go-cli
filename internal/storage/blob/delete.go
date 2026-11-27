package blob

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func Delete(ctx context.Context, cmd *cobra.Command, containerName, blobName string) error {
	client, err := newClient(ctx, cmd)
	if err != nil {
		return err
	}

	fmt.Printf("Deleting blob '%s'...\n", blobName)

	// Delete blob
	blobClient := client.ServiceClient().NewContainerClient(containerName).NewBlobClient(blobName)
	_, err = blobClient.Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete blob: %w", storageError(err))
	}

	fmt.Printf("Successfully deleted blob '%s'\n", blobName)
	return nil
}
