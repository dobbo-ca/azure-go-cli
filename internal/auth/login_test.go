package auth

import (
	"testing"

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
