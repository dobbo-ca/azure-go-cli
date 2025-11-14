package group

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create resource groups client: %w", err)
  }

  poller, err := client.BeginDelete(ctx, name, nil)
  if err != nil {
    return fmt.Errorf("failed to begin delete resource group: %w", err)
  }

  if noWait {
    fmt.Printf("Started deletion of resource group '%s'\n", name)
    return nil
  }

  fmt.Printf("Deleting resource group '%s'...\n", name)
  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to delete resource group: %w", err)
  }

  fmt.Printf("Deleted resource group '%s'\n", name)
  return nil
}
