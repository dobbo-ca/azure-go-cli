package blob

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
)

func Delete(ctx context.Context, accountName, containerName, blobName string) error {
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

  fmt.Printf("Deleting blob '%s'...\n", blobName)

  // Delete blob
  blobClient := client.ServiceClient().NewContainerClient(containerName).NewBlobClient(blobName)
  _, err = blobClient.Delete(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to delete blob: %w", err)
  }

  fmt.Printf("Successfully deleted blob '%s'\n", blobName)
  return nil
}
