package account

import "testing"

func TestCheckTenantFlags(t *testing.T) {
	if checkTenantFlags("sub", "ten") == nil {
		t.Fatal("expected conflict error")
	}
	if checkTenantFlags("sub", "") != nil || checkTenantFlags("", "ten") != nil {
		t.Fatal("unexpected error")
	}
}
