package privateendpoint

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

  client, err := armnetwork.NewPrivateEndpointsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create private endpoints client: %w", err)
  }

  var endpoints []map[string]interface{}

  if resourceGroup != "" {
    // List private endpoints in specific resource group
    pager := client.NewListPager(resourceGroup, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list private endpoints: %w", err)
      }

      for _, ep := range page.Value {
        endpoints = append(endpoints, formatPrivateEndpoint(ep))
      }
    }
  } else {
    // List all private endpoints in subscription
    pager := client.NewListBySubscriptionPager(nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list private endpoints: %w", err)
      }

      for _, ep := range page.Value {
        endpoints = append(endpoints, formatPrivateEndpoint(ep))
      }
    }
  }

  data, err := json.MarshalIndent(endpoints, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format private endpoints: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

func formatPrivateEndpoint(ep *armnetwork.PrivateEndpoint) map[string]interface{} {
  result := map[string]interface{}{
    "name":          azure.GetStringValue(ep.Name),
    "location":      azure.GetStringValue(ep.Location),
    "resourceGroup": getResourceGroupFromID(azure.GetStringValue(ep.ID)),
  }

  if ep.Properties != nil {
    if ep.Properties.ProvisioningState != nil {
      result["provisioningState"] = string(*ep.Properties.ProvisioningState)
    }
    if ep.Properties.Subnet != nil && ep.Properties.Subnet.ID != nil {
      result["subnet"] = *ep.Properties.Subnet.ID
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
