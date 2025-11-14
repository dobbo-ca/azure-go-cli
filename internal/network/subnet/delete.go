package subnet

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name, resourceGroup, vnetName string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create subnets client: %w", err)
  }

  poller, err := client.BeginDelete(ctx, resourceGroup, vnetName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to begin delete subnet: %w", err)
  }

  if noWait {
    fmt.Printf("Started deletion of subnet '%s' in VNet '%s'\n", name, vnetName)
    return nil
  }

  fmt.Printf("Deleting subnet '%s' in VNet '%s'...\n", name, vnetName)
  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to delete subnet: %w", err)
  }

  fmt.Printf("Deleted subnet '%s'\n", name)
  return nil
}
