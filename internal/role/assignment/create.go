package assignment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
	var assigneeObjectID string
	var assigneePrincipalType string
	var role string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a role assignment",
		Long:  "Create a new Azure RBAC role assignment for a user, group, or service principal",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// --assignee and --assignee-object-id both set the principal ID. The
			// object-id variant skips the Microsoft Graph lookup that --assignee
			// implies in azure-cli, so it works for principals the caller can't
			// resolve in Graph (e.g. service principals / cross-tenant).
			principalID := assignee
			if assigneeObjectID != "" {
				principalID = assigneeObjectID
			}

			// --assignee-principal-type is only valid alongside --assignee-object-id.
			if assigneePrincipalType != "" && assigneeObjectID == "" {
				return fmt.Errorf("--assignee-principal-type can only be used with --assignee-object-id")
			}

			principalType, err := parsePrincipalType(assigneePrincipalType)
			if err != nil {
				return err
			}

			return createRoleAssignment(ctx, output, scope, principalID, principalType, role)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format: json")
	cmd.Flags().StringVar(&scope, "scope", "", "Scope for the assignment (required)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Object ID of the user, group, or service principal")
	cmd.Flags().StringVar(&assigneeObjectID, "assignee-object-id", "", "Object ID of the principal, used directly without a Microsoft Graph lookup")
	cmd.Flags().StringVar(&assigneePrincipalType, "assignee-principal-type", "", "Principal type of the assignee object ID: User, Group, ServicePrincipal, ForeignGroup, or Device (only valid with --assignee-object-id)")
	cmd.Flags().StringVar(&role, "role", "", "Role name or ID to assign (required)")

	cmd.MarkFlagsOneRequired("assignee", "assignee-object-id")
	cmd.MarkFlagsMutuallyExclusive("assignee", "assignee-object-id")
	cmd.MarkFlagRequired("scope")
	cmd.MarkFlagRequired("role")

	return cmd
}

// parsePrincipalType validates s against the Azure RBAC principal types and
// returns nil when no principal type was supplied.
func parsePrincipalType(s string) (*armauthorization.PrincipalType, error) {
	if s == "" {
		return nil, nil
	}
	for _, pt := range armauthorization.PossiblePrincipalTypeValues() {
		if string(pt) == s {
			v := pt
			return &v, nil
		}
	}

	var valid []string
	for _, pt := range armauthorization.PossiblePrincipalTypeValues() {
		valid = append(valid, string(pt))
	}
	return nil, fmt.Errorf("invalid --assignee-principal-type %q: must be one of %s", s, strings.Join(valid, ", "))
}

func createRoleAssignment(ctx context.Context, output, scope, principalID string, principalType *armauthorization.PrincipalType, roleNameOrID string) error {
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
			PrincipalID:      to.Ptr(principalID),
			RoleDefinitionID: to.Ptr(roleDefinitionID),
			PrincipalType:    principalType,
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
