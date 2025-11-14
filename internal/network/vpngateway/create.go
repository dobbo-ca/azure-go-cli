package vpngateway

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, publicIPID, subnetID, gatewayType, vpnType, skuName string, tags map[string]string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armnetwork.NewVirtualNetworkGatewaysClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create virtual network gateways client: %w", err)
  }

  // Convert tags to Azure format
  azureTags := make(map[string]*string)
  for k, v := range tags {
    azureTags[k] = to.Ptr(v)
  }

  // Parse gateway type
  var gwType armnetwork.VirtualNetworkGatewayType
  switch gatewayType {
  case "Vpn":
    gwType = armnetwork.VirtualNetworkGatewayTypeVPN
  case "ExpressRoute":
    gwType = armnetwork.VirtualNetworkGatewayTypeExpressRoute
  default:
    return fmt.Errorf("invalid gateway type: %s (must be Vpn or ExpressRoute)", gatewayType)
  }

  // Parse VPN type
  var vt armnetwork.VPNType
  switch vpnType {
  case "PolicyBased":
    vt = armnetwork.VPNTypePolicyBased
  case "RouteBased":
    vt = armnetwork.VPNTypeRouteBased
  default:
    return fmt.Errorf("invalid VPN type: %s (must be PolicyBased or RouteBased)", vpnType)
  }

  // Parse SKU
  var sku armnetwork.VirtualNetworkGatewaySKUName
  switch skuName {
  case "Basic":
    sku = armnetwork.VirtualNetworkGatewaySKUNameBasic
  case "VpnGw1":
    sku = armnetwork.VirtualNetworkGatewaySKUNameVPNGw1
  case "VpnGw2":
    sku = armnetwork.VirtualNetworkGatewaySKUNameVPNGw2
  case "VpnGw3":
    sku = armnetwork.VirtualNetworkGatewaySKUNameVPNGw3
  case "VpnGw1AZ":
    sku = armnetwork.VirtualNetworkGatewaySKUNameVPNGw1AZ
  case "VpnGw2AZ":
    sku = armnetwork.VirtualNetworkGatewaySKUNameVPNGw2AZ
  case "VpnGw3AZ":
    sku = armnetwork.VirtualNetworkGatewaySKUNameVPNGw3AZ
  default:
    return fmt.Errorf("invalid SKU: %s", skuName)
  }

  parameters := armnetwork.VirtualNetworkGateway{
    Location: to.Ptr(location),
    Tags:     azureTags,
    Properties: &armnetwork.VirtualNetworkGatewayPropertiesFormat{
      GatewayType: to.Ptr(gwType),
      VPNType:     to.Ptr(vt),
      IPConfigurations: []*armnetwork.VirtualNetworkGatewayIPConfiguration{
        {
          Name: to.Ptr(name + "-ipconfig"),
          Properties: &armnetwork.VirtualNetworkGatewayIPConfigurationPropertiesFormat{
            PublicIPAddress: &armnetwork.SubResource{
              ID: to.Ptr(publicIPID),
            },
            Subnet: &armnetwork.SubResource{
              ID: to.Ptr(subnetID),
            },
          },
        },
      },
      SKU: &armnetwork.VirtualNetworkGatewaySKU{
        Name: to.Ptr(sku),
      },
    },
  }

  fmt.Printf("Creating virtual network gateway '%s' (this may take 30-45 minutes)...\n", name)
  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
  if err != nil {
    return fmt.Errorf("failed to begin create virtual network gateway: %w", err)
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to create virtual network gateway: %w", err)
  }

  return output.PrintJSON(cmd, result.VirtualNetworkGateway)
}
