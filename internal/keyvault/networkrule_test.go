package keyvault

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
)

func TestPrefixOf(t *testing.T) {
	// A bare address counts as a single-address prefix, as Python's
	// ip_network reads it.
	single, err := prefixOf("10.0.0.4")
	if err != nil || single.Bits() != 32 {
		t.Fatalf("got %v, %v", single, err)
	}
	block, err := prefixOf("10.0.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if !block.Overlaps(single) {
		t.Error("10.0.0.0/24 should overlap 10.0.0.4")
	}
	if single == block {
		t.Error("an address and a range should not compare equal")
	}
	if _, err := prefixOf("not an address"); err == nil {
		t.Error("expected an error for a bad address")
	}
}

func TestSubnetID(t *testing.T) {
	id := "/subscriptions/x/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/v/subnets/s"
	got, err := subnetID(id, "", "rg")
	if err != nil || got != id {
		t.Errorf("an id should pass through unchanged; got %q, %v", got, err)
	}
	if got, err := subnetID("", "", "rg"); err != nil || got != "" {
		t.Errorf("an empty subnet should stay empty; got %q, %v", got, err)
	}
	if _, err := subnetID("mysubnet", "", "rg"); err == nil {
		t.Error("a subnet name without --vnet-name should fail")
	}
}

func TestPolicyMatches(t *testing.T) {
	props := &armkeyvault.VaultProperties{TenantID: to.Ptr("TENANT")}
	policy := &armkeyvault.AccessPolicyEntry{
		TenantID: to.Ptr("tenant"),
		ObjectID: to.Ptr("OBJECT"),
	}
	// Every comparison is case-insensitive (custom.py:817).
	if !policyMatches(props, policy, "object", "") {
		t.Error("expected a case-insensitive match")
	}
	if policyMatches(props, policy, "other", "") {
		t.Error("a different object id should not match")
	}
	if policyMatches(props, policy, "object", "app") {
		t.Error("an application id should have to match too")
	}
	policy.ApplicationID = to.Ptr("APP")
	if !policyMatches(props, policy, "object", "app") {
		t.Error("expected the application ids to match case-insensitively")
	}
}

func TestDistinct(t *testing.T) {
	got := distinct([]string{"get", "list", "get"})
	if len(got) != 2 || got[0] != "get" || got[1] != "list" {
		t.Errorf("got %v", got)
	}
	if distinct(nil) != nil {
		t.Error("distinct(nil) should be nil")
	}
}

func TestPermissionList(t *testing.T) {
	// An unset flag must stay nil, because set_policy keeps the previous
	// value in that case (custom.py:834).
	if permissionList[armkeyvault.KeyPermissions](nil) != nil {
		t.Error("a nil list should stay nil")
	}
	got := permissionList[armkeyvault.KeyPermissions]([]string{"get", "get", "list"})
	if len(got) != 2 || string(*got[0]) != "get" {
		t.Errorf("got %v", got)
	}
	if empty := permissionList[armkeyvault.KeyPermissions]([]string{}); empty == nil || len(empty) != 0 {
		t.Error("an empty list should stay an empty list, not nil")
	}
}
