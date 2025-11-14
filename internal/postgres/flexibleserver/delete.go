package flexibleserver

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name, resourceGroup string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create PostgreSQL client: %w", err)
  }

  fmt.Printf("Deleting PostgreSQL flexible server '%s'...\n", name)
  poller, err := client.BeginDelete(ctx, resourceGroup, name, nil)
  if err != nil {
    return fmt.Errorf("failed to begin delete PostgreSQL server: %w", err)
  }

  if noWait {
    fmt.Printf("Started deletion of PostgreSQL flexible server '%s'\n", name)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to delete PostgreSQL server: %w", err)
  }

  fmt.Printf("Deleted PostgreSQL flexible server '%s'\n", name)
  return nil
}
