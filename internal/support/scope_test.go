package support

import "testing"

func TestScopeLabel(t *testing.T) {
	got := Scope{TenantID: "t1", SubscriptionID: "s1", SubscriptionName: "Prod"}.Label()
	if got != "t1 · Prod (s1)" {
		t.Fatalf("got %q", got)
	}
	got = Scope{TenantID: "t1", TenantName: "Contoso"}.Label()
	if got != "Contoso · tenant-level (no subscription)" {
		t.Fatalf("got %q", got)
	}
}
