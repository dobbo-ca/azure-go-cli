package assignment

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
)

func resolveRoleDefinitionID(ctx context.Context, cred azcore.TokenCredential, subscriptionID, scope, roleNameOrID string) (string, error) {
  // If it looks like a full resource ID or GUID, use it directly
  if len(roleNameOrID) == 36 || (len(roleNameOrID) > 36 && roleNameOrID[0] == '/') {
    return roleNameOrID, nil
  }

  // Search for role by name
  client, err := armauthorization.NewRoleDefinitionsClient(cred, nil)
  if err != nil {
    return "", fmt.Errorf("failed to create role definitions client: %w", err)
  }

  pager := client.NewListPager(scope, nil)
  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return "", fmt.Errorf("failed to list roles: %w", err)
    }

    for _, r := range page.Value {
      if r.Properties != nil && r.Properties.RoleName != nil && *r.Properties.RoleName == roleNameOrID {
        if r.ID != nil {
          return *r.ID, nil
        }
      }
    }
  }

  return "", fmt.Errorf("role '%s' not found", roleNameOrID)
}
