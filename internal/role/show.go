package role

import (
  "context"
  "encoding/json"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
  var output string
  var scope string

  cmd := &cobra.Command{
    Use:   "show <role-name-or-id>",
    Short: "Show details of a role definition",
    Long:  "Show detailed information about a specific Azure RBAC role definition",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
      ctx := context.Background()
      return showRoleDefinition(ctx, args[0], output, scope)
    },
  }

  cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format: json")
  cmd.Flags().StringVar(&scope, "scope", "", "Scope to query role from (defaults to subscription scope)")

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

  // Default to subscription scope if not specified
  if scope == "" {
    scope = fmt.Sprintf("/subscriptions/%s", subscriptionID)
  }

  client, err := armauthorization.NewRoleDefinitionsClient(cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create role definitions client: %w", err)
  }

  // Try to get by ID first, if it looks like a full resource ID
  var role *armauthorization.RoleDefinition
  if len(roleNameOrID) > 30 && (roleNameOrID[0] == '/' || len(roleNameOrID) == 36) {
    // Looks like a resource ID or GUID
    resp, err := client.Get(ctx, scope, roleNameOrID, nil)
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
