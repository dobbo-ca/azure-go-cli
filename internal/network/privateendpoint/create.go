package privateendpoint

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

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, subnetID, privateLinkResourceID, groupID string, tags map[string]string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armnetwork.NewPrivateEndpointsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create private endpoints client: %w", err)
  }

  // Convert tags to Azure format
  azureTags := make(map[string]*string)
  for k, v := range tags {
    azureTags[k] = to.Ptr(v)
  }

  parameters := armnetwork.PrivateEndpoint{
    Location: to.Ptr(location),
    Tags:     azureTags,
    Properties: &armnetwork.PrivateEndpointProperties{
      Subnet: &armnetwork.Subnet{
        ID: to.Ptr(subnetID),
      },
      PrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{
        {
          Name: to.Ptr(name + "-connection"),
          Properties: &armnetwork.PrivateLinkServiceConnectionProperties{
            PrivateLinkServiceID: to.Ptr(privateLinkResourceID),
            GroupIDs: []*string{
              to.Ptr(groupID),
            },
          },
        },
      },
    },
  }

  fmt.Printf("Creating private endpoint '%s'...\n", name)
  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
  if err != nil {
    return fmt.Errorf("failed to begin create private endpoint: %w", err)
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to create private endpoint: %w", err)
  }

  return output.PrintJSON(cmd, result.PrivateEndpoint)
}
