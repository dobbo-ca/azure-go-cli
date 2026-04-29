package subnet

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, vnetName, resourceGroup string) error {
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

  pager := client.NewListPager(resourceGroup, vnetName, nil)
  var subnets []map[string]interface{}

  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to list subnets: %w", err)
    }

    for _, subnet := range page.Value {
      subnets = append(subnets, formatSubnet(subnet))
    }
  }

  return output.PrintJSON(cmd, subnets)
}

func formatSubnet(subnet *armnetwork.Subnet) map[string]interface{} {
  result := map[string]interface{}{
    "name": azure.GetStringValue(subnet.Name),
  }

  if subnet.ID != nil {
    result["id"] = *subnet.ID
  }

  if subnet.Properties != nil {
    if subnet.Properties.AddressPrefix != nil {
      result["addressPrefix"] = *subnet.Properties.AddressPrefix
    }
    if len(subnet.Properties.AddressPrefixes) > 0 {
      prefixes := make([]string, 0, len(subnet.Properties.AddressPrefixes))
      for _, p := range subnet.Properties.AddressPrefixes {
        if p != nil {
          prefixes = append(prefixes, *p)
        }
      }
      result["addressPrefixes"] = prefixes
    }
    if subnet.Properties.ProvisioningState != nil {
      result["provisioningState"] = string(*subnet.Properties.ProvisioningState)
    }
    if subnet.Properties.NetworkSecurityGroup != nil {
      nsg := map[string]interface{}{}
      if subnet.Properties.NetworkSecurityGroup.ID != nil {
        nsg["id"] = *subnet.Properties.NetworkSecurityGroup.ID
      }
      result["networkSecurityGroup"] = nsg
    }
    if subnet.Properties.RouteTable != nil {
      rt := map[string]interface{}{}
      if subnet.Properties.RouteTable.ID != nil {
        rt["id"] = *subnet.Properties.RouteTable.ID
      }
      result["routeTable"] = rt
    }
    if subnet.Properties.NatGateway != nil {
      ng := map[string]interface{}{}
      if subnet.Properties.NatGateway.ID != nil {
        ng["id"] = *subnet.Properties.NatGateway.ID
      }
      result["natGateway"] = ng
    }
  }

  return result
}
