package vm

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
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

  client, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create VM client: %w", err)
  }

  fmt.Printf("Deleting virtual machine '%s'...\n", name)
  poller, err := client.BeginDelete(ctx, resourceGroup, name, nil)
  if err != nil {
    return fmt.Errorf("failed to begin delete VM: %w", err)
  }

  if noWait {
    fmt.Printf("Started deletion of virtual machine '%s'\n", name)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to delete VM: %w", err)
  }

  fmt.Printf("Deleted virtual machine '%s'\n", name)
  return nil
}
