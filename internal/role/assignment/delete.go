package assignment

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var scope string
	var assignee string
	var role string

	cmd := &cobra.Command{
		Use:   "delete [assignment-id]",
		Short: "Delete a role assignment",
		Long:  "Delete an Azure RBAC role assignment by ID or by scope/assignee/role combination",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// If assignment ID is provided as argument, use it
			if len(args) > 0 {
				return deleteRoleAssignmentByID(ctx, args[0])
			}

			// Otherwise, require scope, assignee, and role flags
			if scope == "" || assignee == "" || role == "" {
				return fmt.Errorf("either provide assignment-id as argument, or use --scope, --assignee, and --role flags")
			}

			return deleteRoleAssignmentByFilter(ctx, scope, assignee, role)
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Scope of the assignment")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Object ID of the user, group, or service principal")
	cmd.Flags().StringVar(&role, "role", "", "Role name or ID")

	return cmd
}

func deleteRoleAssignmentByID(ctx context.Context, assignmentID string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armauthorization.NewRoleAssignmentsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create role assignments client: %w", err)
	}

	fmt.Printf("Deleting role assignment '%s'...\n", assignmentID)

	_, err = client.DeleteByID(ctx, assignmentID, nil)
	if err != nil {
		return fmt.Errorf("failed to delete role assignment: %w", err)
	}

	fmt.Printf("Successfully deleted role assignment '%s'\n", assignmentID)
	return nil
}

func deleteRoleAssignmentByFilter(ctx context.Context, scope, assignee, roleNameOrID string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armauthorization.NewRoleAssignmentsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create role assignments client: %w", err)
	}

	// Resolve role name to ID if needed
	roleDefinitionID, err := resolveRoleDefinitionID(ctx, cred, subscriptionID, scope, roleNameOrID)
	if err != nil {
		return fmt.Errorf("failed to resolve role: %w", err)
	}

	// Find matching assignment
	filter := fmt.Sprintf("principalId eq '%s'", assignee)
	pager := client.NewListForScopePager(scope, &armauthorization.RoleAssignmentsClientListForScopeOptions{
		Filter: &filter,
	})

	var matchingAssignment *armauthorization.RoleAssignment
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list assignments: %w", err)
		}

		for _, a := range page.Value {
			if a.Properties != nil && a.Properties.RoleDefinitionID != nil {
				if *a.Properties.RoleDefinitionID == roleDefinitionID {
					matchingAssignment = a
					break
				}
			}
		}

		if matchingAssignment != nil {
			break
		}
	}

	if matchingAssignment == nil {
		return fmt.Errorf("no matching role assignment found for assignee '%s' with role '%s' at scope '%s'",
			assignee, roleNameOrID, scope)
	}

	if matchingAssignment.ID == nil {
		return fmt.Errorf("assignment ID is nil")
	}

	fmt.Printf("Deleting role assignment '%s'...\n", *matchingAssignment.ID)

	_, err = client.DeleteByID(ctx, *matchingAssignment.ID, nil)
	if err != nil {
		return fmt.Errorf("failed to delete role assignment: %w", err)
	}

	fmt.Printf("Successfully deleted role assignment\n")
	return nil
}
