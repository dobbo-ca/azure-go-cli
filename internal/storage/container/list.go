package container

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, accountName, resourceGroup string) error {
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

  pager := client.NewListPager(resourceGroup, accountName, nil)
  var containers []map[string]interface{}

  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to list blob containers: %w", err)
    }

    for _, container := range page.Value {
      containers = append(containers, formatContainer(container))
    }
  }

  data, err := json.MarshalIndent(containers, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format blob containers: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

func formatContainer(container *armstorage.ListContainerItem) map[string]interface{} {
  result := map[string]interface{}{
    "name": azure.GetStringValue(container.Name),
  }

  if container.Properties != nil {
    if container.Properties.PublicAccess != nil {
      result["publicAccess"] = string(*container.Properties.PublicAccess)
    }
    if container.Properties.LastModifiedTime != nil {
      result["lastModifiedTime"] = container.Properties.LastModifiedTime.String()
    }
    if container.Properties.HasImmutabilityPolicy != nil {
      result["hasImmutabilityPolicy"] = *container.Properties.HasImmutabilityPolicy
    }
    if container.Properties.HasLegalHold != nil {
      result["hasLegalHold"] = *container.Properties.HasLegalHold
    }
  }

  return result
}
