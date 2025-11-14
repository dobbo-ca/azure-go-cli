package vnet

import (
  "context"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location string, addressPrefixes []string, tags map[string]string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create virtual networks client: %w", err)
  }

  // Convert tags to Azure format
  azureTags := make(map[string]*string)
  for k, v := range tags {
    azureTags[k] = to.Ptr(v)
  }

  // Convert address prefixes to pointers
  azureAddressPrefixes := make([]*string, 0, len(addressPrefixes))
  for _, prefix := range addressPrefixes {
    azureAddressPrefixes = append(azureAddressPrefixes, to.Ptr(prefix))
  }

  parameters := armnetwork.VirtualNetwork{
    Location: to.Ptr(location),
    Tags:     azureTags,
    Properties: &armnetwork.VirtualNetworkPropertiesFormat{
      AddressSpace: &armnetwork.AddressSpace{
        AddressPrefixes: azureAddressPrefixes,
      },
    },
  }

  fmt.Printf("Creating virtual network '%s'...\n", name)
  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
  if err != nil {
    return fmt.Errorf("failed to begin create virtual network: %w", err)
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to create virtual network: %w", err)
  }

  return output.PrintJSON(cmd, result.VirtualNetwork)
}

// ParseAddressPrefixes parses a comma-separated string of address prefixes
func ParseAddressPrefixes(prefixes string) []string {
  if prefixes == "" {
    return []string{}
  }

  parts := strings.Split(prefixes, ",")
  result := make([]string, 0, len(parts))
  for _, part := range parts {
    trimmed := strings.TrimSpace(part)
    if trimmed != "" {
      result = append(result, trimmed)
    }
  }

  return result
}
