package vnet

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, vnetName, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create virtual networks client: %w", err)
  }

  vnet, err := client.Get(ctx, resourceGroup, vnetName, nil)
  if err != nil {
    return fmt.Errorf("failed to get virtual network: %w", err)
  }

  data, err := json.MarshalIndent(vnet, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format virtual network: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
