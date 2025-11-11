package flexibleserver

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, serverName, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create postgresql flexible servers client: %w", err)
  }

  server, err := client.Get(ctx, resourceGroup, serverName, nil)
  if err != nil {
    return fmt.Errorf("failed to get postgresql flexible server: %w", err)
  }

  data, err := json.MarshalIndent(server, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format postgresql flexible server: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
