package keyvault

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, vaultName, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armkeyvault.NewVaultsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create key vaults client: %w", err)
  }

  vault, err := client.Get(ctx, resourceGroup, vaultName, nil)
  if err != nil {
    return fmt.Errorf("failed to get key vault: %w", err)
  }

  data, err := json.MarshalIndent(vault, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format key vault: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
