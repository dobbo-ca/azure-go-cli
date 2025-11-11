package vpngateway

import (
  "context"
  "encoding/json"
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
    return err
  }

  client, err := armnetwork.NewVirtualNetworkGatewaysClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create virtual network gateways client: %w", err)
  }

  var gateways []map[string]interface{}

  if resourceGroup != "" {
    // List VPN gateways in specific resource group
    pager := client.NewListPager(resourceGroup, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list virtual network gateways: %w", err)
      }

      for _, gw := range page.Value {
        gateways = append(gateways, formatVpnGateway(gw))
      }
    }
  } else {
    return fmt.Errorf("resource group is required for listing virtual network gateways")
  }

  data, err := json.MarshalIndent(gateways, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format virtual network gateways: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

func formatVpnGateway(gw *armnetwork.VirtualNetworkGateway) map[string]interface{} {
  result := map[string]interface{}{
    "name":          azure.GetStringValue(gw.Name),
    "location":      azure.GetStringValue(gw.Location),
    "resourceGroup": getResourceGroupFromID(azure.GetStringValue(gw.ID)),
  }

  if gw.Properties != nil {
    if gw.Properties.GatewayType != nil {
      result["gatewayType"] = string(*gw.Properties.GatewayType)
    }
    if gw.Properties.VPNType != nil {
      result["vpnType"] = string(*gw.Properties.VPNType)
    }
    if gw.Properties.ProvisioningState != nil {
      result["provisioningState"] = string(*gw.Properties.ProvisioningState)
    }
    if gw.Properties.EnableBgp != nil {
      result["enableBgp"] = *gw.Properties.EnableBgp
    }
  }

  return result
}

func getResourceGroupFromID(id string) string {
  parts := make([]string, 0)
  for _, part := range []rune(id) {
    if part == '/' {
      parts = append(parts, "")
    } else if len(parts) > 0 {
      parts[len(parts)-1] += string(part)
    }
  }

  for i, part := range parts {
    if part == "resourceGroups" && i+1 < len(parts) {
      return parts[i+1]
    }
  }
  return ""
}
