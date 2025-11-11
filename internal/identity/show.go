package identity

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, name, resourceGroup, subscriptionOverride string) error {
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

  identity, err := client.Get(ctx, resourceGroup, name, nil)
  if err != nil {
    return fmt.Errorf("failed to get managed identity: %w", err)
  }

  data, err := json.MarshalIndent(identity, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format managed identity: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
