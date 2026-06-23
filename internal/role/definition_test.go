package role

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
)

// TestToDefinitionRecords_TopLevelID confirms the flattened shape exposes a
// top-level "id" and "roleName", which "[0].id"-style queries depend on.
func TestToDefinitionRecords_TopLevelID(t *testing.T) {
	roles := []*armauthorization.RoleDefinition{
		{
			ID:   to.Ptr("/subscriptions/s/providers/Microsoft.Authorization/roleDefinitions/role-guid"),
			Name: to.Ptr("role-guid"),
			Type: to.Ptr("Microsoft.Authorization/roleDefinitions"),
			Properties: &armauthorization.RoleDefinitionProperties{
				RoleName: to.Ptr("My Custom Role"),
				RoleType: to.Ptr("CustomRole"),
				Permissions: []*armauthorization.Permission{
					{Actions: []*string{to.Ptr("Microsoft.Compute/*/read")}},
				},
			},
		},
	}

	b, _ := json.Marshal(toDefinitionRecords(roles))
	var generic []map[string]interface{}
	if err := json.Unmarshal(b, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if generic[0]["id"] != "/subscriptions/s/providers/Microsoft.Authorization/roleDefinitions/role-guid" {
		t.Errorf("top-level id = %v", generic[0]["id"])
	}
	if generic[0]["roleName"] != "My Custom Role" {
		t.Errorf("roleName = %v", generic[0]["roleName"])
	}
}

func TestScopeFromRoleID(t *testing.T) {
	id := "/subscriptions/abc/providers/Microsoft.Authorization/roleDefinitions/guid-123"
	if got := scopeFromRoleID(id); got != "/subscriptions/abc" {
		t.Errorf("scopeFromRoleID = %q", got)
	}
	if got := lastSegment(id); got != "guid-123" {
		t.Errorf("lastSegment = %q", got)
	}
}

func TestParseRoleDefinitionInput_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "role.json")
	body := `{"name":"My Role","assignableScopes":["/subscriptions/s"],"actions":["a/read"],"notActions":[]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	input, err := parseRoleDefinitionInput("@" + path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if input.Name != "My Role" || len(input.Actions) != 1 || input.Actions[0] != "a/read" {
		t.Errorf("parsed = %+v", input)
	}
}

// TestBuildUpdatedProperties_PreservesPermissions guards the critical fix:
// a partial update (e.g. description-only) must NOT wipe the role's existing
// permissions, because CreateOrUpdate is a full PUT.
func TestBuildUpdatedProperties_PreservesPermissions(t *testing.T) {
	existing := &armauthorization.RoleDefinition{
		Properties: &armauthorization.RoleDefinitionProperties{
			RoleName:         to.Ptr("My Role"),
			AssignableScopes: []*string{to.Ptr("/subscriptions/s")},
			Permissions: []*armauthorization.Permission{
				{Actions: []*string{to.Ptr("Microsoft.Compute/virtualMachines/read")}},
			},
		},
	}

	// Input changes only the description — omits permissions entirely.
	props := buildUpdatedProperties(&roleDefinitionInput{Description: "new desc"}, existing)

	if len(props.Permissions) != 1 || len(props.Permissions[0].Actions) != 1 ||
		*props.Permissions[0].Actions[0] != "Microsoft.Compute/virtualMachines/read" {
		t.Fatalf("partial update wiped permissions: %+v", props.Permissions)
	}
	// Assignable scopes also preserved.
	if len(props.AssignableScopes) != 1 || *props.AssignableScopes[0] != "/subscriptions/s" {
		t.Errorf("assignable scopes not preserved: %+v", props.AssignableScopes)
	}
}

// TestBuildUpdatedProperties_OverwritesWhenSupplied confirms supplied
// permissions still replace the existing ones.
func TestBuildUpdatedProperties_OverwritesWhenSupplied(t *testing.T) {
	existing := &armauthorization.RoleDefinition{
		Properties: &armauthorization.RoleDefinitionProperties{
			Permissions: []*armauthorization.Permission{
				{Actions: []*string{to.Ptr("old/read")}},
			},
		},
	}
	props := buildUpdatedProperties(&roleDefinitionInput{Actions: []string{"new/write"}}, existing)
	if len(props.Permissions) != 1 || *props.Permissions[0].Actions[0] != "new/write" {
		t.Errorf("supplied permissions not applied: %+v", props.Permissions)
	}
}

func TestBuildPermissions_TopLevelAndNested(t *testing.T) {
	// Top-level form produces a single permission.
	top := buildPermissions(&roleDefinitionInput{Actions: []string{"a/read"}, NotActions: []string{"a/delete"}})
	if len(top) != 1 || *top[0].Actions[0] != "a/read" || *top[0].NotActions[0] != "a/delete" {
		t.Errorf("top-level permissions = %+v", top)
	}

	// Nested form is used verbatim and takes precedence.
	nested := &roleDefinitionInput{Actions: []string{"ignored"}}
	nested.Permissions = append(nested.Permissions, struct {
		Actions        []string `json:"actions"`
		NotActions     []string `json:"notActions"`
		DataActions    []string `json:"dataActions"`
		NotDataActions []string `json:"notDataActions"`
	}{Actions: []string{"b/write"}})
	got := buildPermissions(nested)
	if len(got) != 1 || *got[0].Actions[0] != "b/write" {
		t.Errorf("nested permissions = %+v", got)
	}
}
