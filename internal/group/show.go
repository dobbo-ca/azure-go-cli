package group

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, resourceGroupName string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create resource groups client: %w", err)
  }

  rg, err := client.Get(ctx, resourceGroupName, nil)
  if err != nil {
    return fmt.Errorf("failed to get resource group: %w", err)
  }

  data, err := json.MarshalIndent(rg, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format resource group: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
