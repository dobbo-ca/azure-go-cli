package auth

import (
	"testing"

	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func TestResolveSubscription(t *testing.T) {
	subs := []config.Subscription{
		{ID: "11111111-1111-1111-1111-111111111111", Name: "Prod"},
		{ID: "22222222-2222-2222-2222-222222222222", Name: "Dev"},
	}

	cases := []struct {
		name    string
		query   string
		wantID  string
		wantNil bool
	}{
		{"match by ID", "22222222-2222-2222-2222-222222222222", "22222222-2222-2222-2222-222222222222", false},
		{"match by name", "Prod", "11111111-1111-1111-1111-111111111111", false},
		{"no match", "Staging", "", true},
		{"empty query", "", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveSubscription(subs, tc.query)
			if tc.wantNil {
				if got != nil {
					t.Errorf("resolveSubscription(%q) = %+v, want nil", tc.query, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("resolveSubscription(%q) = nil, want ID %s", tc.query, tc.wantID)
			}
			if got.ID != tc.wantID {
				t.Errorf("resolveSubscription(%q).ID = %s, want %s", tc.query, got.ID, tc.wantID)
			}
		})
	}
}

func TestFilterTenants(t *testing.T) {
	tenants := []azure.TenantInfo{
		{TenantID: "aaaaaaaa-0000-0000-0000-000000000000", DefaultDomain: "contoso.onmicrosoft.com"},
		{TenantID: "bbbbbbbb-0000-0000-0000-000000000000", DefaultDomain: "fabrikam.com"},
	}

	cases := []struct {
		name    string
		want    string
		wantLen int
		wantID  string
	}{
		{"by tenant ID", "aaaaaaaa-0000-0000-0000-000000000000", 1, "aaaaaaaa-0000-0000-0000-000000000000"},
		{"by domain", "fabrikam.com", 1, "bbbbbbbb-0000-0000-0000-000000000000"},
		{"case-insensitive domain", "Contoso.OnMicrosoft.com", 1, "aaaaaaaa-0000-0000-0000-000000000000"},
		{"no match", "nope.com", 0, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterTenants(tenants, tc.want)
			if len(got) != tc.wantLen {
				t.Fatalf("filterTenants(%q) len = %d, want %d", tc.want, len(got), tc.wantLen)
			}
			if tc.wantLen == 1 && got[0].TenantID != tc.wantID {
				t.Errorf("filterTenants(%q) = %s, want %s", tc.want, got[0].TenantID, tc.wantID)
			}
		})
	}
}
