package secret

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
)

func Delete(ctx context.Context, vaultName, name string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  // Key Vault URL format: https://{vault-name}.vault.azure.net/
  vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

  client, err := azsecrets.NewClient(vaultURL, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create secrets client: %w", err)
  }

  _, err = client.DeleteSecret(ctx, name, nil)
  if err != nil {
    return fmt.Errorf("failed to delete secret: %w", err)
  }

  fmt.Printf("Deleted secret '%s' from vault '%s' (soft delete enabled, can be recovered)\n", name, vaultName)
  return nil
}
