package keyvault

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armkeyvault.NewVaultsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create key vaults client: %w", err)
  }

  _, err = client.Delete(ctx, resourceGroup, name, nil)
  if err != nil {
    return fmt.Errorf("failed to delete key vault: %w", err)
  }

  fmt.Printf("Deleted key vault '%s' (soft delete enabled, can be recovered)\n", name)
  return nil
}
