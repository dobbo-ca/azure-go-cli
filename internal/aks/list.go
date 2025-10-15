package aks

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
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

  client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create AKS client: %w", err)
  }

  var clusters []map[string]interface{}

  if resourceGroup != "" {
    // List clusters in specific resource group
    pager := client.NewListByResourceGroupPager(resourceGroup, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list AKS clusters: %w", err)
      }

      for _, cluster := range page.Value {
        clusters = append(clusters, formatCluster(cluster))
      }
    }
  } else {
    // List all clusters in subscription
    pager := client.NewListPager(nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list AKS clusters: %w", err)
      }

      for _, cluster := range page.Value {
        clusters = append(clusters, formatCluster(cluster))
      }
    }
  }

  data, err := json.MarshalIndent(clusters, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format clusters: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

func formatCluster(cluster *armcontainerservice.ManagedCluster) map[string]interface{} {
  result := map[string]interface{}{
    "name":          azure.GetStringValue(cluster.Name),
    "location":      azure.GetStringValue(cluster.Location),
    "resourceGroup": getResourceGroupFromID(azure.GetStringValue(cluster.ID)),
  }

  if cluster.Properties != nil {
    if cluster.Properties.KubernetesVersion != nil {
      result["kubernetesVersion"] = *cluster.Properties.KubernetesVersion
    }
    if cluster.Properties.ProvisioningState != nil {
      result["provisioningState"] = *cluster.Properties.ProvisioningState
    }
    if cluster.Properties.PowerState != nil && cluster.Properties.PowerState.Code != nil {
      result["powerState"] = *cluster.Properties.PowerState.Code
    }
  }

  return result
}
