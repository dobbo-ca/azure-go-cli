package role

import (
  "context"
  "encoding/json"
  "fmt"
  "os"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
  var output string
  var scope string
  var name string
  var id string

  cmd := &cobra.Command{
    Use:   "show [role-name-or-id]",
    Short: "Show details of a role definition",
    Long:  "Show detailed information about a specific Azure RBAC role definition",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
      ctx := context.Background()

      // Determine which identifier to use (priority: --id, --name, positional arg)
      var roleIdentifier string
      if id != "" {
        roleIdentifier = id
      } else if name != "" {
        roleIdentifier = name
      } else if len(args) > 0 {
        roleIdentifier = args[0]
      } else {
        return fmt.Errorf("must specify role via --id, --name, or positional argument")
      }

      return showRoleDefinition(ctx, roleIdentifier, output, scope)
    },
  }

  cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format: json")
  cmd.Flags().StringVar(&scope, "scope", "", "Scope to query role from (defaults to subscription scope)")
  cmd.Flags().StringVarP(&name, "name", "n", "", "The role definition's name (GUID)")
  cmd.Flags().StringVar(&id, "id", "", "The fully qualified role definition ID")

  return cmd
}

func showRoleDefinition(ctx context.Context, roleNameOrID, output, scope string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return fmt.Errorf("failed to get credentials: %w", err)
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  // Parse role ID if it's a full resource ID
  var roleID string
  if len(roleNameOrID) > 36 && roleNameOrID[0] == '/' {
    // Full resource ID format: /subscriptions/{sub}/providers/Microsoft.Authorization/roleDefinitions/{guid}
    // Extract the subscription and role ID from the path
    parts := strings.Split(roleNameOrID, "/")
    for i, part := range parts {
      if part == "subscriptions" && i+1 < len(parts) {
        subscriptionID = parts[i+1]
      }
      if part == "roleDefinitions" && i+1 < len(parts) {
        roleID = parts[i+1]
        break
      }
    }
    if roleID == "" {
      return fmt.Errorf("invalid role definition ID format: %s", roleNameOrID)
    }
  } else {
    // It's just a GUID or role name
    roleID = roleNameOrID
  }

  // Default to subscription scope if not specified
  if scope == "" {
    scope = fmt.Sprintf("/subscriptions/%s", subscriptionID)
  }

  client, err := armauthorization.NewRoleDefinitionsClient(cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create role definitions client: %w", err)
  }

  // Try to get by ID first if it looks like a GUID
  var role *armauthorization.RoleDefinition
  if len(roleID) == 36 {
    resp, err := client.Get(ctx, scope, roleID, nil)
    if err == nil {
      role = &resp.RoleDefinition
    }
  }

  // If not found by ID, search by name
  if role == nil {
    pager := client.NewListPager(scope, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list roles: %w", err)
      }

      for _, r := range page.Value {
        if r.Properties != nil && r.Properties.RoleName != nil && *r.Properties.RoleName == roleNameOrID {
          role = r
          break
        }
      }

      if role != nil {
        break
      }
    }
  }

  if role == nil {
    return fmt.Errorf("role definition '%s' not found", roleNameOrID)
  }

  enc := json.NewEncoder(os.Stdout)
  enc.SetIndent("", "  ")
  return enc.Encode(role)
}
