package nic

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
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

  client, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create NIC client: %w", err)
  }

  fmt.Printf("%-40s %-20s %-30s\n", "NAME", "LOCATION", "PRIVATE IP")
  fmt.Println("------------------------------------------------------------------------------------------------")

  if resourceGroup != "" {
    pager := client.NewListPager(resourceGroup, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to get next page: %w", err)
      }

      for _, nic := range page.Value {
        printNIC(nic)
      }
    }
  } else {
    pager := client.NewListAllPager(nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to get next page: %w", err)
      }

      for _, nic := range page.Value {
        printNIC(nic)
      }
    }
  }

  return nil
}

func printNIC(nic *armnetwork.Interface) {
  name := ""
  if nic.Name != nil {
    name = *nic.Name
  }

  location := ""
  if nic.Location != nil {
    location = *nic.Location
  }

  privateIP := ""
  if nic.Properties != nil && len(nic.Properties.IPConfigurations) > 0 {
    ipConfig := nic.Properties.IPConfigurations[0]
    if ipConfig.Properties != nil && ipConfig.Properties.PrivateIPAddress != nil {
      privateIP = *ipConfig.Properties.PrivateIPAddress
    }
  }

  fmt.Printf("%-40s %-20s %-30s\n", name, location, privateIP)
}
