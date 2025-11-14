package lb

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

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, skuName string, tags map[string]string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armnetwork.NewLoadBalancersClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create load balancers client: %w", err)
  }

  // Convert tags to Azure format
  azureTags := make(map[string]*string)
  for k, v := range tags {
    azureTags[k] = to.Ptr(v)
  }

  // Parse SKU name
  var lbSKU armnetwork.LoadBalancerSKUName
  switch skuName {
  case "Basic":
    lbSKU = armnetwork.LoadBalancerSKUNameBasic
  case "Standard":
    lbSKU = armnetwork.LoadBalancerSKUNameStandard
  case "Gateway":
    lbSKU = armnetwork.LoadBalancerSKUNameGateway
  default:
    return fmt.Errorf("invalid SKU name: %s (must be Basic, Standard, or Gateway)", skuName)
  }

  parameters := armnetwork.LoadBalancer{
    Location: to.Ptr(location),
    Tags:     azureTags,
    SKU: &armnetwork.LoadBalancerSKU{
      Name: to.Ptr(lbSKU),
    },
    Properties: &armnetwork.LoadBalancerPropertiesFormat{
      FrontendIPConfigurations: []*armnetwork.FrontendIPConfiguration{},
      BackendAddressPools:      []*armnetwork.BackendAddressPool{},
      LoadBalancingRules:       []*armnetwork.LoadBalancingRule{},
      Probes:                   []*armnetwork.Probe{},
    },
  }

  fmt.Printf("Creating load balancer '%s'...\n", name)
  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
  if err != nil {
    return fmt.Errorf("failed to begin create load balancer: %w", err)
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to create load balancer: %w", err)
  }

  return output.PrintJSON(cmd, result.LoadBalancer)
}
