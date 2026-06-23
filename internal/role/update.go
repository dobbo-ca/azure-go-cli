package role

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/spf13/cobra"
)

// roleDefinitionInput is the azure-cli `--role-definition` JSON shape. It
// accepts permissions either as a top-level action set or as a permissions
// array, and identifies the target role by id, roleName, or name.
type roleDefinitionInput struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	RoleName         string   `json:"roleName"`
	Description      string   `json:"description"`
	AssignableScopes []string `json:"assignableScopes"`

	// Top-level permission form.
	Actions        []string `json:"actions"`
	NotActions     []string `json:"notActions"`
	DataActions    []string `json:"dataActions"`
	NotDataActions []string `json:"notDataActions"`

	// Nested permission form.
	Permissions []struct {
		Actions        []string `json:"actions"`
		NotActions     []string `json:"notActions"`
		DataActions    []string `json:"dataActions"`
		NotDataActions []string `json:"notDataActions"`
	} `json:"permissions"`
}

func newUpdateCmd() *cobra.Command {
	var roleDefinition string
	var scope string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a custom role definition",
		Long:  "Update an existing Azure RBAC custom role definition from inline JSON or a JSON file (@file)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			return updateRoleDefinition(ctx, roleDefinition, scope)
		},
	}

	cmd.Flags().StringVar(&roleDefinition, "role-definition", "", "Role definition as inline JSON or @path-to-file (required)")
	cmd.Flags().StringVar(&scope, "scope", "", "Scope to search for the role (defaults to subscription scope)")
	cmd.MarkFlagRequired("role-definition")

	return cmd
}

func updateRoleDefinition(ctx context.Context, roleDefinition, scope string) error {
	input, err := parseRoleDefinitionInput(roleDefinition)
	if err != nil {
		return err
	}

	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Resolve the existing role from the subscription scope (or an explicit
	// --scope), not from the input's assignableScopes: when the update *changes*
	// assignableScopes, the new scope may be somewhere the role isn't visible
	// yet, which would falsely report "not found". A full input.id short-circuits
	// this in findRoleDefinition.
	if scope == "" {
		scope = fmt.Sprintf("/subscriptions/%s", subscriptionID)
	}

	identifier := firstNonEmpty(input.ID, input.RoleName, input.Name)
	if identifier == "" {
		return fmt.Errorf("role definition must specify one of: id, roleName, name")
	}

	existing, err := findRoleDefinition(ctx, cred, scope, identifier)
	if err != nil {
		return err
	}
	if existing == nil || existing.ID == nil || existing.Name == nil {
		return fmt.Errorf("role definition '%s' not found", identifier)
	}

	updateScope := scopeFromRoleID(*existing.ID)
	guid := *existing.Name

	props := buildUpdatedProperties(input, existing)

	client, err := armauthorization.NewRoleDefinitionsClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create role definitions client: %w", err)
	}

	resp, err := client.CreateOrUpdate(ctx, updateScope, guid, armauthorization.RoleDefinition{Properties: props}, nil)
	if err != nil {
		return fmt.Errorf("failed to update role definition: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp.RoleDefinition)
}

// parseRoleDefinitionInput reads the --role-definition value, which is either
// inline JSON or @path-to-file (azure-cli convention).
func parseRoleDefinitionInput(value string) (*roleDefinitionInput, error) {
	data := []byte(value)
	if strings.HasPrefix(value, "@") {
		path := value[1:]
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read role definition file %q: %w", path, err)
		}
		data = b
	}

	var input roleDefinitionInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("failed to parse role definition JSON: %w", err)
	}
	return &input, nil
}

// buildUpdatedProperties merges the input over the existing role definition,
// keeping existing assignable scopes when the input omits them.
func buildUpdatedProperties(input *roleDefinitionInput, existing *armauthorization.RoleDefinition) *armauthorization.RoleDefinitionProperties {
	props := &armauthorization.RoleDefinitionProperties{
		RoleType: to.Ptr("CustomRole"),
	}

	if name := firstNonEmpty(input.RoleName, input.Name); name != "" {
		props.RoleName = to.Ptr(name)
	} else if existing.Properties != nil {
		props.RoleName = existing.Properties.RoleName
	}

	if input.Description != "" {
		props.Description = to.Ptr(input.Description)
	} else if existing.Properties != nil {
		props.Description = existing.Properties.Description
	}

	if len(input.AssignableScopes) > 0 {
		props.AssignableScopes = toPtrSlice(input.AssignableScopes)
	} else if existing.Properties != nil {
		props.AssignableScopes = existing.Properties.AssignableScopes
	}

	// CreateOrUpdate is a full PUT, so an omitted permission set would otherwise
	// wipe the role's actions. Only overwrite permissions when the input
	// actually supplies them; otherwise keep the existing ones.
	if inputHasPermissions(input) {
		props.Permissions = buildPermissions(input)
	} else if existing.Properties != nil {
		props.Permissions = existing.Properties.Permissions
	}
	return props
}

// inputHasPermissions reports whether the input JSON supplied any permission
// data, in either the nested or top-level form.
func inputHasPermissions(input *roleDefinitionInput) bool {
	return len(input.Permissions) > 0 ||
		len(input.Actions) > 0 ||
		len(input.NotActions) > 0 ||
		len(input.DataActions) > 0 ||
		len(input.NotDataActions) > 0
}

func buildPermissions(input *roleDefinitionInput) []*armauthorization.Permission {
	if len(input.Permissions) > 0 {
		perms := make([]*armauthorization.Permission, 0, len(input.Permissions))
		for _, p := range input.Permissions {
			perms = append(perms, &armauthorization.Permission{
				Actions:        toPtrSlice(p.Actions),
				NotActions:     toPtrSlice(p.NotActions),
				DataActions:    toPtrSlice(p.DataActions),
				NotDataActions: toPtrSlice(p.NotDataActions),
			})
		}
		return perms
	}

	return []*armauthorization.Permission{{
		Actions:        toPtrSlice(input.Actions),
		NotActions:     toPtrSlice(input.NotActions),
		DataActions:    toPtrSlice(input.DataActions),
		NotDataActions: toPtrSlice(input.NotDataActions),
	}}
}

// findRoleDefinition resolves a role by full ID, GUID, or display name.
func findRoleDefinition(ctx context.Context, cred azcore.TokenCredential, scope, roleNameOrID string) (*armauthorization.RoleDefinition, error) {
	client, err := armauthorization.NewRoleDefinitionsClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create role definitions client: %w", err)
	}

	// Full resource ID.
	if strings.HasPrefix(roleNameOrID, "/") {
		resp, err := client.Get(ctx, scopeFromRoleID(roleNameOrID), lastSegment(roleNameOrID), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get role definition: %w", err)
		}
		return &resp.RoleDefinition, nil
	}

	// Bare GUID.
	if len(roleNameOrID) == 36 {
		if resp, err := client.Get(ctx, scope, roleNameOrID, nil); err == nil {
			return &resp.RoleDefinition, nil
		}
	}

	// Search by display name.
	pager := client.NewListPager(scope, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list roles: %w", err)
		}
		for _, r := range page.Value {
			if r.Properties != nil && r.Properties.RoleName != nil && *r.Properties.RoleName == roleNameOrID {
				return r, nil
			}
		}
	}
	return nil, nil
}

// scopeFromRoleID returns the scope portion of a role definition resource ID
// (everything before /providers/Microsoft.Authorization/roleDefinitions/...).
func scopeFromRoleID(roleID string) string {
	const marker = "/providers/Microsoft.Authorization/roleDefinitions/"
	if i := strings.Index(roleID, marker); i >= 0 {
		return roleID[:i]
	}
	return roleID
}

func lastSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func toPtrSlice(in []string) []*string {
	out := make([]*string, 0, len(in))
	for i := range in {
		out = append(out, to.Ptr(in[i]))
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
