package lb

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

  client, err := armnetwork.NewLoadBalancersClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create load balancers client: %w", err)
  }

  var loadBalancers []map[string]interface{}

  if resourceGroup != "" {
    // List load balancers in specific resource group
    pager := client.NewListPager(resourceGroup, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list load balancers: %w", err)
      }

      for _, lb := range page.Value {
        loadBalancers = append(loadBalancers, formatLoadBalancer(lb))
      }
    }
  } else {
    // List all load balancers in subscription
    pager := client.NewListAllPager(nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list load balancers: %w", err)
      }

      for _, lb := range page.Value {
        loadBalancers = append(loadBalancers, formatLoadBalancer(lb))
      }
    }
  }

  data, err := json.MarshalIndent(loadBalancers, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format load balancers: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

func formatLoadBalancer(lb *armnetwork.LoadBalancer) map[string]interface{} {
  result := map[string]interface{}{
    "name":          azure.GetStringValue(lb.Name),
    "location":      azure.GetStringValue(lb.Location),
    "resourceGroup": getResourceGroupFromID(azure.GetStringValue(lb.ID)),
  }

  if lb.SKU != nil && lb.SKU.Name != nil {
    result["sku"] = string(*lb.SKU.Name)
  }

  if lb.Properties != nil {
    if lb.Properties.ProvisioningState != nil {
      result["provisioningState"] = string(*lb.Properties.ProvisioningState)
    }
    if lb.Properties.FrontendIPConfigurations != nil {
      result["frontendIPConfigurations"] = len(lb.Properties.FrontendIPConfigurations)
    }
    if lb.Properties.BackendAddressPools != nil {
      result["backendAddressPools"] = len(lb.Properties.BackendAddressPools)
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
