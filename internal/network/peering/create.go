package peering

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

func Create(ctx context.Context, cmd *cobra.Command, name, vnetName, resourceGroup, remoteVNetID string, allowVNetAccess, allowForwardedTraffic, allowGatewayTransit, useRemoteGateways bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armnetwork.NewVirtualNetworkPeeringsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create virtual network peerings client: %w", err)
  }

  parameters := armnetwork.VirtualNetworkPeering{
    Properties: &armnetwork.VirtualNetworkPeeringPropertiesFormat{
      RemoteVirtualNetwork: &armnetwork.SubResource{
        ID: to.Ptr(remoteVNetID),
      },
      AllowVirtualNetworkAccess: to.Ptr(allowVNetAccess),
      AllowForwardedTraffic:     to.Ptr(allowForwardedTraffic),
      AllowGatewayTransit:       to.Ptr(allowGatewayTransit),
      UseRemoteGateways:         to.Ptr(useRemoteGateways),
    },
  }

  fmt.Printf("Creating virtual network peering '%s'...\n", name)
  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vnetName, name, parameters, nil)
  if err != nil {
    return fmt.Errorf("failed to begin create virtual network peering: %w", err)
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to create virtual network peering: %w", err)
  }

  return output.PrintJSON(cmd, result.VirtualNetworkPeering)
}
