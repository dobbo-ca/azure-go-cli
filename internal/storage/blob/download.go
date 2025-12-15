package blob

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

func Download(ctx context.Context, accountName, containerName, blobName, filePath string) error {
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

	fmt.Printf("Downloading blob '%s' to '%s'...\n", blobName, filePath)

	// Download blob
	blobClient := client.ServiceClient().NewContainerClient(containerName).NewBlobClient(blobName)
	downloadResp, err := blobClient.DownloadStream(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to download blob: %w", err)
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
