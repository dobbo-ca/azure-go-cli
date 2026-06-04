package pim

import (
	"strings"
	"testing"
)

// ResolveScope inputs reference a synthetic eligible-assignments slice (one
// scope per entry: subscription ID + tenant ID + display names).
func sampleScopeIndex() []ScopeIndexEntry {
	return []ScopeIndexEntry{
		{
			ArmPath:           "/subscriptions/aaaa1111-0000-0000-0000-000000000000",
			SubscriptionID:    "aaaa1111-0000-0000-0000-000000000000",
			SubscriptionName:  "Acme Production",
			TenantID:          "tenant-acme-uuid",
			TenantDisplayName: "Acme Corp",
		},
		{
			ArmPath:           "/subscriptions/bbbb2222-0000-0000-0000-000000000000",
			SubscriptionID:    "bbbb2222-0000-0000-0000-000000000000",
			SubscriptionName:  "Acme Dev",
			TenantID:          "tenant-acme-uuid",
			TenantDisplayName: "Acme Corp",
		},
		{
			ArmPath:           "/subscriptions/cccc3333-0000-0000-0000-000000000000",
			SubscriptionID:    "cccc3333-0000-0000-0000-000000000000",
			SubscriptionName:  "Acme Production", // same name in a different tenant
			TenantID:          "tenant-beta-uuid",
			TenantDisplayName: "Beta LLC",
		},
	}
}

func TestResolveScope_FullArmPath(t *testing.T) {
	got, err := ResolveScope("/subscriptions/aaaa1111-0000-0000-0000-000000000000/resourceGroups/rg",
		sampleScopeIndex())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != "/subscriptions/aaaa1111-0000-0000-0000-000000000000/resourceGroups/rg" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveScope_SubscriptionUUID(t *testing.T) {
	got, err := ResolveScope("aaaa1111-0000-0000-0000-000000000000", sampleScopeIndex())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != "/subscriptions/aaaa1111-0000-0000-0000-000000000000" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveScope_TenantSlashSubscription(t *testing.T) {
	got, err := ResolveScope("Acme Corp/Acme Production", sampleScopeIndex())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != "/subscriptions/aaaa1111-0000-0000-0000-000000000000" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveScope_BareSubscriptionUnambiguous(t *testing.T) {
	got, err := ResolveScope("Acme Dev", sampleScopeIndex())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != "/subscriptions/bbbb2222-0000-0000-0000-000000000000" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveScope_BareSubscriptionAmbiguous(t *testing.T) {
	_, err := ResolveScope("Acme Production", sampleScopeIndex())
	if err == nil {
		t.Fatal("want error for ambiguous match")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error should mention ambiguity: %v", err)
	}
}

func TestResolveScope_NoMatch(t *testing.T) {
	_, err := ResolveScope("Nonexistent", sampleScopeIndex())
	if err == nil {
		t.Fatal("want error for no match")
	}
}

func TestResolveScope_TenantSlashRG(t *testing.T) {
	got, err := ResolveScope("Acme Corp/Acme Production/my-rg", sampleScopeIndex())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := "/subscriptions/aaaa1111-0000-0000-0000-000000000000/resourceGroups/my-rg"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
