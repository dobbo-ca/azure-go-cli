package container

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, accountName, containerName, resourceGroup string) error {
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

  data, err := json.MarshalIndent(container, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format blob container: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
