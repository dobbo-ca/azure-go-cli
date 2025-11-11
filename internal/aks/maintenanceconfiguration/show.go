package maintenanceconfiguration

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, clusterName, configName, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armcontainerservice.NewMaintenanceConfigurationsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create maintenance configurations client: %w", err)
  }

  config, err := client.Get(ctx, resourceGroup, clusterName, configName, nil)
  if err != nil {
    return fmt.Errorf("failed to get maintenance configuration: %w", err)
  }

  data, err := json.MarshalIndent(config, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format maintenance configuration: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
