package container

import (
	"context"
	"testing"
)

// TestUserDelegationTidFlag locks the --user-delegation-tid flag surface
// against azure-cli's dev-branch _params.py:1754 (storage container
// generate-sas): c.argument('user_delegation_tid', ..., help='The delegated
// user tenant id in Azure AD. This parameter can only be specified when
// using OAuth.').
func TestUserDelegationTidFlag(t *testing.T) {
	cmd := NewGenerateSASCommand()
	f := cmd.Flags().Lookup("user-delegation-tid")
	if f == nil {
		t.Fatal("--user-delegation-tid flag not registered")
	}
	if f.Value.Type() != "string" {
		t.Errorf("flag type = %q, want %q", f.Value.Type(), "string")
	}
	if f.DefValue != "" {
		t.Errorf("flag default = %q, want empty", f.DefValue)
	}
	want := "The delegated user tenant id in Azure AD. This parameter can only be specified when using OAuth."
	if f.Usage != want {
		t.Errorf("flag usage = %q, want %q", f.Usage, want)
	}
}

// TestUserDelegationTidRequiresOid locks the validation order: --user-delegation-tid
// requires --user-delegation-oid, which in turn requires --as-user. Validation
// runs before any credential lookup, so this hits no network.
func TestUserDelegationTidRequiresOid(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "tid only",
			args: []string{"--name=c", "--permissions=r", "--expiry=2026-01-02T00:00Z", "--user-delegation-tid=11111111-2222-3333-4444-555555555555"},
			want: "incorrect usage: need to specify '--user-delegation-oid' when '--user-delegation-tid' is provided",
		},
		{
			name: "tid and oid, no as-user",
			args: []string{"--name=c", "--permissions=r", "--expiry=2026-01-02T00:00Z", "--user-delegation-oid=22222222-3333-4444-5555-666666666666", "--user-delegation-tid=11111111-2222-3333-4444-555555555555"},
			want: "incorrect usage: need to specify '--as-user' when '--user-delegation-oid' is provided",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := NewGenerateSASCommand()
			if err := cmd.ParseFlags(c.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			err := runGenerateSAS(context.Background(), cmd)
			if err == nil {
				t.Fatal("runGenerateSAS: got nil error, want validation error")
			}
			if err.Error() != c.want {
				t.Errorf("error = %q, want %q", err.Error(), c.want)
			}
		})
	}
}
