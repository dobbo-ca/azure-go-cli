package vm

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Stop(ctx context.Context, name, resourceGroup string, noWait bool) error {
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

  fmt.Printf("Stopping and deallocating virtual machine '%s'...\n", name)
  poller, err := client.BeginDeallocate(ctx, resourceGroup, name, nil)
  if err != nil {
    return fmt.Errorf("failed to begin deallocate VM: %w", err)
  }

  if noWait {
    fmt.Printf("Started operation to stop virtual machine '%s'\n", name)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to deallocate VM: %w", err)
  }

  fmt.Printf("Stopped and deallocated virtual machine '%s'\n", name)
  return nil
}
