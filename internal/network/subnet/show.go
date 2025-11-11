package subnet

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, vnetName, subnetName, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create subnets client: %w", err)
  }

  subnet, err := client.Get(ctx, resourceGroup, vnetName, subnetName, nil)
  if err != nil {
    return fmt.Errorf("failed to get subnet: %w", err)
  }

  data, err := json.MarshalIndent(subnet, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format subnet: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
