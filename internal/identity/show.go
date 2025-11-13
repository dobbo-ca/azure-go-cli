package identity

import (
  "context"
  "encoding/json"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, name, resourceGroup, subscriptionOverride string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetSubscription(subscriptionOverride)
  if err != nil {
    return err
  }

  client, err := armmsi.NewUserAssignedIdentitiesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create managed identities client: %w", err)
  }

  identity, err := client.Get(ctx, resourceGroup, name, nil)
  if err != nil {
    return fmt.Errorf("failed to get managed identity: %w", err)
  }

  data, err := json.MarshalIndent(identity, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format managed identity: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

// ShowByIDs shows one or more managed identities by their resource IDs
func ShowByIDs(ctx context.Context, ids []string, subscriptionOverride string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  var results []interface{}

  for _, id := range ids {
    // Parse resource ID
    // Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/{name}
    parts := strings.Split(id, "/")
    if len(parts) < 9 {
      return fmt.Errorf("invalid resource ID format: %s", id)
    }

    subscriptionID := parts[2]
    resourceGroup := parts[4]
    name := parts[8]

    // Override subscription if specified
    if subscriptionOverride != "" {
      subscriptionID, err = config.GetSubscription(subscriptionOverride)
      if err != nil {
        return err
      }
    }

    client, err := armmsi.NewUserAssignedIdentitiesClient(subscriptionID, cred, nil)
    if err != nil {
      return fmt.Errorf("failed to create managed identities client: %w", err)
    }

    identity, err := client.Get(ctx, resourceGroup, name, nil)
    if err != nil {
      return fmt.Errorf("failed to get managed identity %s: %w", id, err)
    }

    results = append(results, identity)
  }

  // Output results
  var data []byte
  if len(results) == 1 {
    data, err = json.MarshalIndent(results[0], "", "  ")
  } else {
    data, err = json.MarshalIndent(results, "", "  ")
  }

  if err != nil {
    return fmt.Errorf("failed to format managed identities: %w", err)
  }

  fmt.Println(string(data))
  return nil
}
