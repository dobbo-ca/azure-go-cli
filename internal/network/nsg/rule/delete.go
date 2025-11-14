package rule

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name, nsgName, resourceGroup string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armnetwork.NewSecurityRulesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create security rules client: %w", err)
  }

  fmt.Printf("Deleting security rule '%s'...\n", name)
  poller, err := client.BeginDelete(ctx, resourceGroup, nsgName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to delete security rule: %w", err)
  }

  if noWait {
    fmt.Printf("Delete operation started for security rule '%s'\n", name)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to complete security rule deletion: %w", err)
  }

  fmt.Printf("Deleted security rule '%s'\n", name)
  return nil
}
