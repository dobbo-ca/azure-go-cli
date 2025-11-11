package privateendpoint

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, endpointName, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armnetwork.NewPrivateEndpointsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create private endpoints client: %w", err)
  }

  endpoint, err := client.Get(ctx, resourceGroup, endpointName, nil)
  if err != nil {
    return fmt.Errorf("failed to get private endpoint: %w", err)
  }

  data, err := json.MarshalIndent(endpoint, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format private endpoint: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
