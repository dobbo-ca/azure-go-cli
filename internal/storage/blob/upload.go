package blob

import (
  "context"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
)

func Upload(ctx context.Context, accountName, containerName, blobName, filePath string, overwrite bool) error {
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
    return fmt.Errorf("failed to upload blob: %w", err)
  }

  fmt.Printf("Successfully uploaded blob '%s'\n", blobName)
  return nil
}
