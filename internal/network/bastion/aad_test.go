package bastion

import (
	"testing"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

func TestSelectAccount(t *testing.T) {
	alice := public.Account{HomeAccountID: "a.t1", PreferredUsername: "alice@contoso.com", Realm: "t1"}
	bobHome := public.Account{HomeAccountID: "b.t1", PreferredUsername: "bob@contoso.com", Realm: "t1"}
	bobGuest := public.Account{HomeAccountID: "b.t1", PreferredUsername: "Bob@contoso.com", Realm: "t2"}
	all := []public.Account{alice, bobHome, bobGuest}

	tests := []struct {
		name               string
		home, user, tenant string
		accounts           []public.Account
		want               public.Account
		wantErr            bool
	}{
		{name: "home account id wins", home: "a.t1", user: "bob@contoso.com", accounts: all, want: alice},
		{name: "user in tenant", user: "bob@contoso.com", tenant: "t2", accounts: all, want: bobGuest},
		{name: "user any tenant", user: "bob@contoso.com", tenant: "t9", accounts: all, want: bobHome},
		{name: "single unknown", accounts: []public.Account{alice}, want: alice},
		{name: "no match", user: "carol@contoso.com", accounts: all, wantErr: true},
		{name: "ambiguous", accounts: all, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectAccount(tt.accounts, tt.home, tt.user, tt.tenant)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (got.HomeAccountID != tt.want.HomeAccountID || got.Realm != tt.want.Realm || got.PreferredUsername != tt.want.PreferredUsername) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNeedsInteraction(t *testing.T) {
	tests := map[string]bool{
		`{"error":"interaction_required","error_codes":[50076]}`: true,
		`{"error":"invalid_grant","error_codes":[53003]}`:        true,
		`{"error":"invalid_grant","suberror":"basic_action"}`:    true,
		`{"error":"invalid_grant","error_codes":[70000]}`:        false,
		`not json`: false,
	}
	for body, want := range tests {
		if got := needsInteraction([]byte(body)); got != want {
			t.Errorf("needsInteraction(%s) = %v, want %v", body, got, want)
		}
	}
}
