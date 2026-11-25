package support

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func twoTenantProfile() *config.Profile {
	return &config.Profile{Subscriptions: []config.Subscription{
		{ID: "s1", Name: "Sub One", TenantID: "t1"},
		{ID: "s2", Name: "Sub Two", TenantID: "t2", IsDefault: true},
	}}
}

func newTestApp(t *testing.T, fc *fakeClient) *app {
	t.Helper()
	fc.profile = twoTenantProfile()
	a, err := newApp(context.Background(), fakeDeps(fc), Scope{TenantID: "t2", SubscriptionID: "s2", SubscriptionName: "Sub Two"})
	if err != nil {
		t.Fatal(err)
	}
	a.width, a.height = 100, 40
	return a
}

func view(a *app) string { return a.View().Content }

func TestStartScope(t *testing.T) {
	newCmd := func(args ...string) *cobra.Command {
		c := &cobra.Command{Use: "x", Run: func(*cobra.Command, []string) {}}
		c.Flags().String("tenant", "", "")
		c.Flags().String("subscription", "", "")
		c.SetArgs(args)
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
		return c
	}
	p := twoTenantProfile()
	for _, tc := range []struct {
		args []string
		want Scope
	}{
		{nil, Scope{TenantID: "t2", SubscriptionID: "s2", SubscriptionName: "Sub Two"}},
		{[]string{"--tenant", "t9"}, Scope{TenantID: "t9"}},
		{[]string{"--subscription", "s1"}, Scope{TenantID: "t1", SubscriptionID: "s1", SubscriptionName: "Sub One"}},
	} {
		got, err := startScope(newCmd(tc.args...), p)
		if err != nil || got != tc.want {
			t.Fatalf("%v: got %+v, %v; want %+v", tc.args, got, err, tc.want)
		}
	}
}

func TestScopeScreenRows(t *testing.T) {
	a := newTestApp(t, newFake())
	drive(t, a, keys("t")...)
	v := view(a)
	for _, want := range []string{"Tenant t1", "Tenant-level tickets (no subscription) · t1", "Sub One (s1)", "Other tenant…"} {
		if !strings.Contains(v, want) {
			t.Fatalf("view missing %q:\n%s", want, v)
		}
	}
	before := a.scope
	drive(t, a, keys("enter")...) // cursor is on the "Tenant t1" heading
	if a.scope != before {
		t.Fatalf("heading changed scope to %+v", a.scope)
	}
}

func TestScopeSwitchToTenantLevel(t *testing.T) {
	fc := newFake()
	a := newTestApp(t, fc)
	gen := a.gen
	drive(t, a, keys("t", "t2", "down", "enter")...) // heading, then the tenant-level row
	if a.scope != (Scope{TenantID: "t2"}) {
		t.Fatalf("scope = %+v", a.scope)
	}
	if a.gen != gen+1 {
		t.Fatalf("gen = %d", a.gen)
	}
	if last := fc.scopes[len(fc.scopes)-1]; last != (Scope{TenantID: "t2"}) {
		t.Fatalf("newClient got %+v", last)
	}
}

func TestScopeOtherTenantDiscovery(t *testing.T) {
	fc := newFake()
	fc.tenants = []azure.TenantInfo{
		{TenantID: "t3", DisplayName: "Fabrikam", Subscriptions: []config.Subscription{{ID: "s3", Name: "Sub3", TenantID: "t3"}}},
		{TenantID: "t4", DisplayName: "Contoso", NeedsMFA: true},
	}
	a := newTestApp(t, fc)
	drive(t, a, keys("t", "other", "enter")...)
	if fc.discovered != 1 {
		t.Fatalf("discover called %d times", fc.discovered)
	}
	v := view(a)
	for _, want := range []string{"Fabrikam (t3)", "Contoso (t4) — sign in"} {
		if !strings.Contains(v, want) {
			t.Fatalf("view missing %q:\n%s", want, v)
		}
	}
}

func TestScopeSignedInRows(t *testing.T) {
	a := newTestApp(t, newFake())
	drive(t, a, keys("t")...)
	drive(t, a, tenantSignedInMsg{tenantID: "t4", subs: []config.Subscription{{ID: "s9", Name: "Nine", TenantID: "t4"}}})
	v := view(a)
	for _, want := range []string{"Tenant-level tickets (no subscription) · t4", "s9"} {
		if !strings.Contains(v, want) {
			t.Fatalf("view missing %q:\n%s", want, v)
		}
	}
}

func TestScopeDoesNotWriteProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	if err := config.Save(twoTenantProfile()); err != nil {
		t.Fatal(err)
	}
	path, _ := config.GetConfigPath()
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	a := newTestApp(t, newFake())
	drive(t, a, keys("t", "t1", "down", "enter")...)
	after, err := os.Stat(path)
	if err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("profile touched: %v %v", err, after)
	}
}

func TestCredentialErrorOffersSignIn(t *testing.T) {
	fc := newFake()
	a := newTestApp(t, fc)
	fc.newClientErr = errors.New("AADSTS50076: MFA required")
	driveCmd(t, a, a.setScope(Scope{TenantID: "t1"}))
	if !strings.Contains(view(a), "press ctrl+l to sign in to this tenant") {
		t.Fatalf("view:\n%s", view(a))
	}
	n := len(fc.scopes)
	fc.newClientErr = nil
	drive(t, a, reauthDoneMsg{})
	if len(fc.scopes) != n+1 || fc.scopes[n] != (Scope{TenantID: "t1"}) {
		t.Fatalf("scopes = %+v", fc.scopes)
	}
}
