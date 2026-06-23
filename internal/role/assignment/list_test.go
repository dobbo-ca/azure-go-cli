package assignment

import (
	"encoding/json"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
)

// TestToAssignmentRecords_AzureCLIShape locks the flattened JSON field names
// that azure-cli --query expressions (and the role-assignment idempotency
// check) rely on:
// top-level principalId, roleDefinitionName, scope, and id.
func TestToAssignmentRecords_AzureCLIShape(t *testing.T) {
	roleDefID := "/subscriptions/s/providers/Microsoft.Authorization/roleDefinitions/11111111-1111-1111-1111-111111111111"
	assignments := []*armauthorization.RoleAssignment{
		{
			ID:   to.Ptr("/subscriptions/s/providers/Microsoft.Authorization/roleAssignments/ra-guid"),
			Name: to.Ptr("ra-guid"),
			Type: to.Ptr("Microsoft.Authorization/roleAssignments"),
			Properties: &armauthorization.RoleAssignmentProperties{
				PrincipalID:      to.Ptr("principal-1"),
				PrincipalType:    to.Ptr(armauthorization.PrincipalTypeServicePrincipal),
				RoleDefinitionID: to.Ptr(roleDefID),
				Scope:            to.Ptr("/subscriptions/s"),
			},
		},
	}
	names := map[string]string{"11111111-1111-1111-1111-111111111111": "Contributor"}

	records := toAssignmentRecords(assignments, names)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	b, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var generic []map[string]interface{}
	if err := json.Unmarshal(b, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	rec := generic[0]
	checks := map[string]string{
		"principalId":        "principal-1",
		"principalType":      "ServicePrincipal",
		"roleDefinitionId":   roleDefID,
		"roleDefinitionName": "Contributor",
		"scope":              "/subscriptions/s",
	}
	for key, want := range checks {
		if got, ok := rec[key].(string); !ok || got != want {
			t.Errorf("record[%q] = %v, want %q", key, rec[key], want)
		}
	}
}

func TestToAssignmentRecords_UnknownRoleNameEmpty(t *testing.T) {
	assignments := []*armauthorization.RoleAssignment{
		{Properties: &armauthorization.RoleAssignmentProperties{
			RoleDefinitionID: to.Ptr("/x/roleDefinitions/unknown-guid"),
		}},
	}
	records := toAssignmentRecords(assignments, map[string]string{})
	if records[0].RoleDefinitionName != "" {
		t.Errorf("unknown role name = %q, want empty", records[0].RoleDefinitionName)
	}
}
