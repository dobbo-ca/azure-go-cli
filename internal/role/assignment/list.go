package assignment

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	output_ "github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// roleAssignmentRecord is the azure-cli-shaped, flattened view of a role
// assignment emitted for json/tsv output (so --query expressions match).
type roleAssignmentRecord struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Type               string  `json:"type"`
	PrincipalID        string  `json:"principalId"`
	PrincipalType      string  `json:"principalType,omitempty"`
	RoleDefinitionID   string  `json:"roleDefinitionId"`
	RoleDefinitionName string  `json:"roleDefinitionName"`
	Scope              string  `json:"scope"`
	Condition          *string `json:"condition"`
	ConditionVersion   *string `json:"conditionVersion"`
	Description        *string `json:"description"`
}

func toAssignmentRecords(assignments []*armauthorization.RoleAssignment, names map[string]string) []roleAssignmentRecord {
	records := make([]roleAssignmentRecord, 0, len(assignments))
	for _, a := range assignments {
		rec := roleAssignmentRecord{}
		if a.ID != nil {
			rec.ID = *a.ID
		}
		if a.Name != nil {
			rec.Name = *a.Name
		}
		if a.Type != nil {
			rec.Type = *a.Type
		}
		if p := a.Properties; p != nil {
			if p.PrincipalID != nil {
				rec.PrincipalID = *p.PrincipalID
			}
			if p.PrincipalType != nil {
				rec.PrincipalType = string(*p.PrincipalType)
			}
			if p.RoleDefinitionID != nil {
				rec.RoleDefinitionID = *p.RoleDefinitionID
				rec.RoleDefinitionName = names[getRoleNameFromID(*p.RoleDefinitionID)]
			}
			if p.Scope != nil {
				rec.Scope = *p.Scope
			}
			rec.Condition = p.Condition
			rec.ConditionVersion = p.ConditionVersion
			rec.Description = p.Description
		}
		records = append(records, rec)
	}
	return records
}

func newListCmd() *cobra.Command {
	var output string
	var scope string
	var assignee string
	var role string
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List role assignments",
		Long:  "List Azure RBAC role assignments at a given scope",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			return listRoleAssignments(ctx, cmd, output, scope, assignee, role, all)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: json, table, or tsv")
	cmd.Flags().StringVar(&scope, "scope", "", "Scope to list assignments for (defaults to subscription scope)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee (user, group, or service principal object ID)")
	cmd.Flags().StringVar(&role, "role", "", "Filter by role name or ID")
	cmd.Flags().BoolVar(&all, "all", false, "Show all assignments under the current subscription")

	return cmd
}

func listRoleAssignments(ctx context.Context, cmd *cobra.Command, output, scope, assignee, role string, all bool) error {
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
	//
	// When --all is specified, we don't use atScope() to get assignments at all levels.
	// Otherwise, we use atScope() to only get assignments at the specified scope level.
	var filter *string
	if !all {
		filterStr := "atScope()"
		filter = &filterStr
	}

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

	// json/tsv: emit azure-cli-shaped records (flattened, with
	// roleDefinitionName resolved) so JMESPath --query expressions written for
	// azure-cli work unchanged. A --query also forces this path so the filter is
	// never silently dropped in the default (table) mode.
	queryStr, _ := cmd.Flags().GetString("query")
	if output != "table" || queryStr != "" {
		// roleDefinitionName enrichment is best-effort: a caller with
		// roleAssignments/read but not roleDefinitions/read still gets output
		// (the name falls back to empty rather than failing the command). Names
		// are resolved only at `scope`, so --all may leave custom roles defined
		// at a child scope unnamed.
		names, err := resolveRoleDefinitionNames(ctx, cred, scope)
		if err != nil {
			names = map[string]string{}
		}
		records := toAssignmentRecords(assignments, names)
		format := output
		if format == "table" {
			format = "json"
		}
		return output_.PrintFormatted(cmd, records, format)
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
