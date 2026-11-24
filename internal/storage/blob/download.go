package blob

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func Download(ctx context.Context, cmd *cobra.Command, containerName, blobName, filePath string) error {
	client, err := newClient(ctx, cmd)
	if err != nil {
		return err
	}

	fmt.Printf("Downloading blob '%s' to '%s'...\n", blobName, filePath)

	// Download blob
	blobClient := client.ServiceClient().NewContainerClient(containerName).NewBlobClient(blobName)
	downloadResp, err := blobClient.DownloadStream(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to download blob: %w", storageError(err))
	}
	defer downloadResp.Body.Close()

	// Create output file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy blob data to file
	written, err := io.Copy(file, downloadResp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Successfully downloaded blob '%s' (%d bytes)\n", blobName, written)
	return nil
}
