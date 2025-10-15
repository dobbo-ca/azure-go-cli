package aks

import (
  "context"
  "encoding/json"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, clusterName, resourceGroup string) error {
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

  cluster, err := client.Get(ctx, resourceGroup, clusterName, nil)
  if err != nil {
    return fmt.Errorf("failed to get AKS cluster: %w", err)
  }

  // Convert to JSON for display
  data, err := json.MarshalIndent(cluster, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format cluster: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

// Helper function to extract resource group from Azure resource ID
func getResourceGroupFromID(id string) string {
  parts := strings.Split(id, "/")
  for i, part := range parts {
    if strings.EqualFold(part, "resourceGroups") && i+1 < len(parts) {
      return parts[i+1]
    }
  }
  return ""
}
