package blob

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Upload(ctx context.Context, cmd *cobra.Command, containerName, blobName, filePath string, overwrite bool) error {
	client, err := newClient(ctx, cmd)
	if err != nil {
		return err
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	fmt.Printf("Uploading '%s' to blob '%s' (%d bytes)...\n", filePath, blobName, fileInfo.Size())

	// Upload blob
	blobClient := client.ServiceClient().NewContainerClient(containerName).NewBlockBlobClient(blobName)
	_, err = blobClient.UploadFile(ctx, file, nil)
	if err != nil {
		return fmt.Errorf("failed to upload blob: %w", storageError(err))
	}

	fmt.Printf("Successfully uploaded blob '%s'\n", blobName)
	return nil
}
