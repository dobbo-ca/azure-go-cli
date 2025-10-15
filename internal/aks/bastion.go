package aks

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
  "github.com/cdobbyn/azure-go-cli/internal/network/bastion"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

// Bastion is a convenience wrapper around network bastion tunnel
// It fetches the AKS cluster details and calls the bastion tunnel with appropriate parameters
func Bastion(ctx context.Context, clusterName, resourceGroup, bastionResourceID string, admin bool, port int) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  // Get cluster info
  client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create AKS client: %w", err)
  }

  cluster, err := client.Get(ctx, resourceGroup, clusterName, nil)
  if err != nil {
    return fmt.Errorf("failed to get cluster: %w", err)
  }

  if cluster.ID == nil {
    return fmt.Errorf("cluster ID not found")
  }

  fmt.Printf("Opening tunnel to AKS cluster %s through Bastion...\n", clusterName)

  // Extract bastion details from resource ID
  // Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/bastionHosts/{name}
  bastionName, bastionRG, err := parseBastionResourceID(bastionResourceID)
  if err != nil {
    return fmt.Errorf("failed to parse bastion resource ID: %w", err)
  }

  // Delegate to network bastion tunnel with AKS-specific parameters
  return bastion.Tunnel(ctx, bastionName, bastionRG, *cluster.ID, 443, port)
}

func parseBastionResourceID(resourceID string) (name string, resourceGroup string, err error) {
  // Simple parser for Azure resource IDs
  // Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/bastionHosts/{name}

  // This is a simplified implementation
  // In production, you'd want to use azure.ParseResourceID or similar
  fmt.Println("Note: Bastion resource ID parsing not fully implemented")

  return "", "", fmt.Errorf("bastion resource ID parsing not yet implemented")
}
