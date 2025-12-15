package assignment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	var output string
	var scope string
	var assignee string
	var role string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a role assignment",
		Long:  "Create a new Azure RBAC role assignment for a user, group, or service principal",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			return createRoleAssignment(ctx, output, scope, assignee, role)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format: json")
	cmd.Flags().StringVar(&scope, "scope", "", "Scope for the assignment (required)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Object ID of the user, group, or service principal (required)")
	cmd.Flags().StringVar(&role, "role", "", "Role name or ID to assign (required)")

	cmd.MarkFlagRequired("scope")
	cmd.MarkFlagRequired("assignee")
	cmd.MarkFlagRequired("role")

	return cmd
}

func createRoleAssignment(ctx context.Context, output, scope, assignee, roleNameOrID string) error {
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

	// Resolve role name to role definition ID if needed
	roleDefinitionID, err := resolveRoleDefinitionID(ctx, cred, subscriptionID, scope, roleNameOrID)
	if err != nil {
		return fmt.Errorf("failed to resolve role: %w", err)
	}

	// Generate a unique name for the role assignment
	assignmentName := uuid.New().String()

	// Create the role assignment
	params := armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID:      to.Ptr(assignee),
			RoleDefinitionID: to.Ptr(roleDefinitionID),
		},
	}

	resp, err := client.Create(ctx, scope, assignmentName, params, nil)
	if err != nil {
		return fmt.Errorf("failed to create role assignment: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp.RoleAssignment)
}
