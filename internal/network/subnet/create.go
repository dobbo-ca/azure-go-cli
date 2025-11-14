package subnet

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

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, vnetName, addressPrefix string) error {
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

  parameters := armnetwork.Subnet{
    Properties: &armnetwork.SubnetPropertiesFormat{
      AddressPrefix: to.Ptr(addressPrefix),
    },
  }

  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vnetName, name, parameters, nil)
  if err != nil {
    return fmt.Errorf("failed to begin create subnet: %w", err)
  }

  fmt.Printf("Creating subnet '%s' in VNet '%s'...\n", name, vnetName)
  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to create subnet: %w", err)
  }

  return output.PrintJSON(cmd, result.Subnet)
}
