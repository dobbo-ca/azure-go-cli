package identity

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name, resourceGroup string, subscriptionOverride string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetSubscription(subscriptionOverride)
  if err != nil {
    return err
  }

  client, err := armmsi.NewUserAssignedIdentitiesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create managed identities client: %w", err)
  }

  _, err = client.Delete(ctx, resourceGroup, name, nil)
  if err != nil {
    return fmt.Errorf("failed to delete managed identity: %w", err)
  }

  fmt.Printf("Deleted managed identity '%s' in resource group '%s'\n", name, resourceGroup)
  return nil
}
