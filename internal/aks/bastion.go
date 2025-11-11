package aks

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
  "github.com/cdobbyn/azure-go-cli/internal/network/bastion"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

// Bastion is a convenience wrapper around network bastion tunnel
// It fetches the AKS cluster details and calls the bastion tunnel with appropriate parameters
func Bastion(ctx context.Context, clusterName, resourceGroup, bastionResourceID, subscriptionOverride string, admin bool, port int) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetSubscription(subscriptionOverride)
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
  // Parse Azure resource IDs
  // Supported formats:
  // - /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/bastionHosts/{name}
  // - /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/virtualNetworks/{name}

  if resourceID == "" {
    return "", "", fmt.Errorf("resource ID cannot be empty")
  }

  // Split by '/' and remove empty strings
  parts := make([]string, 0)
  for _, part := range splitResourceID(resourceID) {
    if part != "" {
      parts = append(parts, part)
    }
  }

  // Minimum valid resource ID should have at least:
  // subscriptions, {id}, resourceGroups, {name}, providers, {namespace}, {type}, {name}
  if len(parts) < 8 {
    return "", "", fmt.Errorf("invalid resource ID format: too few segments")
  }

  // Find resource group
  rgIndex := -1
  for i, part := range parts {
    if part == "resourceGroups" && i+1 < len(parts) {
      rgIndex = i + 1
      break
    }
  }
  if rgIndex == -1 {
    return "", "", fmt.Errorf("resource group not found in resource ID")
  }
  resourceGroup = parts[rgIndex]

  // Find provider and resource type
  providerIndex := -1
  for i, part := range parts {
    if part == "providers" && i+1 < len(parts) {
      providerIndex = i + 1
      break
    }
  }
  if providerIndex == -1 {
    return "", "", fmt.Errorf("provider not found in resource ID")
  }

  // Validate it's a Network resource
  if parts[providerIndex] != "Microsoft.Network" {
    return "", "", fmt.Errorf("expected Microsoft.Network provider, got: %s", parts[providerIndex])
  }

  // Check resource type and get name
  if providerIndex+2 >= len(parts) {
    return "", "", fmt.Errorf("invalid resource ID: missing resource type or name")
  }

  resourceType := parts[providerIndex+1]
  name = parts[providerIndex+2]

  // Support both bastionHosts and virtualNetworks (where the vnet name might be the bastion name)
  switch resourceType {
  case "bastionHosts":
    // Direct bastion host reference
    return name, resourceGroup, nil
  case "virtualNetworks":
    // VNet reference - assume the vnet name is the bastion name
    // This is a common pattern where bastions are named after their containing vnet
    return name, resourceGroup, nil
  default:
    return "", "", fmt.Errorf("unsupported resource type: %s (expected bastionHosts or virtualNetworks)", resourceType)
  }
}

func splitResourceID(resourceID string) []string {
  result := make([]string, 0)
  current := ""
  for _, char := range resourceID {
    if char == '/' {
      result = append(result, current)
      current = ""
    } else {
      current += string(char)
    }
  }
  if current != "" {
    result = append(result, current)
  }
  return result
}
