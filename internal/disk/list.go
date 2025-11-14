package disk

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armcompute.NewDisksClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create disk client: %w", err)
  }

  fmt.Printf("%-40s %-20s %-15s %-15s\n", "NAME", "LOCATION", "SIZE (GB)", "SKU")
  fmt.Println("--------------------------------------------------------------------------------------------")

  if resourceGroup != "" {
    pager := client.NewListByResourceGroupPager(resourceGroup, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to get next page: %w", err)
      }

      for _, disk := range page.Value {
        printDisk(disk)
      }
    }
  } else {
    pager := client.NewListPager(nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to get next page: %w", err)
      }

      for _, disk := range page.Value {
        printDisk(disk)
      }
    }
  }

  return nil
}

func printDisk(disk *armcompute.Disk) {
  name := ""
  if disk.Name != nil {
    name = *disk.Name
  }

  location := ""
  if disk.Location != nil {
    location = *disk.Location
  }

  size := ""
  if disk.Properties != nil && disk.Properties.DiskSizeGB != nil {
    size = fmt.Sprintf("%d", *disk.Properties.DiskSizeGB)
  }

  sku := ""
  if disk.SKU != nil && disk.SKU.Name != nil {
    sku = string(*disk.SKU.Name)
  }

  fmt.Printf("%-40s %-20s %-15s %-15s\n", name, location, size, sku)
}
