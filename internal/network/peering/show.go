package peering

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, vnetName, peeringName, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armnetwork.NewVirtualNetworkPeeringsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create virtual network peerings client: %w", err)
  }

  peering, err := client.Get(ctx, resourceGroup, vnetName, peeringName, nil)
  if err != nil {
    return fmt.Errorf("failed to get virtual network peering: %w", err)
  }

  data, err := json.MarshalIndent(peering, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format virtual network peering: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
