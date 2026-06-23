package role

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	output_ "github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var output string
	var custom bool
	var scope string
	var name string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List role definitions",
		Long:  "List Azure RBAC role definitions in the subscription",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			return listRoleDefinitions(ctx, cmd, output, custom, scope, name)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: json, table, or tsv")
	cmd.Flags().BoolVar(&custom, "custom", false, "Show only custom roles")
	cmd.Flags().StringVar(&scope, "scope", "", "Scope to list roles for (defaults to subscription scope)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Filter by role definition's name (GUID) or roleName")

	return cmd
}

func listRoleDefinitions(ctx context.Context, cmd *cobra.Command, output string, customOnly bool, scope string, nameFilter string) error {
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

	var filter *string
	if customOnly {
		filter = to.Ptr("type eq 'CustomRole'")
	}

	pager := client.NewListPager(scope, &armauthorization.RoleDefinitionsClientListOptions{
		Filter: filter,
	})

	var roles []*armauthorization.RoleDefinition
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get page: %w", err)
		}
		roles = append(roles, page.Value...)
	}

	// Client-side filter by name if specified
	if nameFilter != "" {
		var filtered []*armauthorization.RoleDefinition
		for _, r := range roles {
			if r.Properties != nil {
				// Match against role name (display name) or role ID (GUID in the Name field)
				matchesName := r.Properties.RoleName != nil && *r.Properties.RoleName == nameFilter
				matchesID := r.Name != nil && *r.Name == nameFilter
				if matchesName || matchesID {
					filtered = append(filtered, r)
				}
			}
		}
		roles = filtered
	}

	// json/tsv: emit azure-cli-shaped records (flattened) so JMESPath --query
	// expressions written for azure-cli (e.g. "[0].id") work unchanged. A
	// --query also forces this path so the filter is never silently dropped in
	// the default (table) mode.
	queryStr, _ := cmd.Flags().GetString("query")
	if output != "table" || queryStr != "" {
		format := output
		if format == "table" {
			format = "json"
		}
		return output_.PrintFormatted(cmd, toDefinitionRecords(roles), format)
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tROLE TYPE\tDESCRIPTION")

	for _, role := range roles {
		name := ""
		if role.Properties != nil && role.Properties.RoleName != nil {
			name = *role.Properties.RoleName
		}

		roleType := ""
		if role.Properties != nil && role.Properties.RoleType != nil {
			roleType = *role.Properties.RoleType
		}

		description := ""
		if role.Properties != nil && role.Properties.Description != nil {
			description = *role.Properties.Description
			if len(description) > 60 {
				description = description[:57] + "..."
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", name, roleType, description)
	}

	return w.Flush()
}
