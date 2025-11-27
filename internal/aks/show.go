package aks

import (
  "context"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, clusterName, resourceGroup string) error {
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

  // Return properties directly for easier querying (no need for 'properties.' prefix)
  return output.PrintJSON(cmd, cluster.Properties)
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
