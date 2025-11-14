package assignment

import (
  "context"
  "encoding/json"
  "fmt"
  "os"
  "text/tabwriter"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
  var output string
  var scope string
  var assignee string
  var role string

  cmd := &cobra.Command{
    Use:   "list",
    Short: "List role assignments",
    Long:  "List Azure RBAC role assignments at a given scope",
    RunE: func(cmd *cobra.Command, args []string) error {
      ctx := context.Background()
      return listRoleAssignments(ctx, output, scope, assignee, role)
    },
  }

  cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: json or table")
  cmd.Flags().StringVar(&scope, "scope", "", "Scope to list assignments for (defaults to subscription scope)")
  cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee (user, group, or service principal object ID)")
  cmd.Flags().StringVar(&role, "role", "", "Filter by role name or ID")

  return cmd
}

func listRoleAssignments(ctx context.Context, output, scope, assignee, role string) error {
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

  client, err := armauthorization.NewRoleAssignmentsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create role assignments client: %w", err)
  }

  // Build filter
  // Note: Azure API has different filter support depending on scope:
  // - Subscription scope: supports 'principalId eq {value}'
  // - Resource scope: only supports 'atScope()' or no filter
  // We use atScope() and filter client-side for all scopes to keep it simple.
  var filter *string
  filterStr := "atScope()"
  filter = &filterStr

  pager := client.NewListForScopePager(scope, &armauthorization.RoleAssignmentsClientListForScopeOptions{
    Filter: filter,
  })

  var assignments []*armauthorization.RoleAssignment
  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to get page: %w", err)
    }
    assignments = append(assignments, page.Value...)
  }

  // Client-side filter by assignee if specified
  if assignee != "" {
    var filtered []*armauthorization.RoleAssignment
    for _, a := range assignments {
      if a.Properties != nil && a.Properties.PrincipalID != nil && *a.Properties.PrincipalID == assignee {
        filtered = append(filtered, a)
      }
    }
    assignments = filtered
  }

  // Filter by role if specified
  if role != "" {
    var filtered []*armauthorization.RoleAssignment
    for _, a := range assignments {
      if a.Properties != nil && a.Properties.RoleDefinitionID != nil {
        roleDefID := *a.Properties.RoleDefinitionID
        if roleDefID == role || getRoleNameFromID(roleDefID) == role {
          filtered = append(filtered, a)
        }
      }
    }
    assignments = filtered
  }

  if output == "json" {
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    return enc.Encode(assignments)
  }

  // Table output
  w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
  fmt.Fprintln(w, "PRINCIPAL ID\tROLE\tSCOPE")

  for _, a := range assignments {
    principalID := ""
    if a.Properties != nil && a.Properties.PrincipalID != nil {
      principalID = *a.Properties.PrincipalID
    }

    roleID := ""
    if a.Properties != nil && a.Properties.RoleDefinitionID != nil {
      roleID = getRoleNameFromID(*a.Properties.RoleDefinitionID)
    }

    assignmentScope := ""
    if a.Properties != nil && a.Properties.Scope != nil {
      assignmentScope = *a.Properties.Scope
      // Shorten long scopes for table display
      if len(assignmentScope) > 60 {
        assignmentScope = assignmentScope[:57] + "..."
      }
    }

    fmt.Fprintf(w, "%s\t%s\t%s\n", principalID, roleID, assignmentScope)
  }

  return w.Flush()
}

// getRoleNameFromID extracts the role definition ID from a full resource ID
// Example: /subscriptions/.../providers/Microsoft.Authorization/roleDefinitions/{guid}
// Returns: {guid}
func getRoleNameFromID(roleDefID string) string {
  // Extract just the GUID at the end
  parts := []rune(roleDefID)
  for i := len(parts) - 1; i >= 0; i-- {
    if parts[i] == '/' {
      return string(parts[i+1:])
    }
  }
  return roleDefID
}
