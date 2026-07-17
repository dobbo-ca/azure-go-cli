package lock

import (
  "strings"
  "testing"

  "github.com/spf13/cobra"
)

func newScopeCmd(kind scopeKind) *cobra.Command {
  c := &cobra.Command{Use: "x"}
  addScopeFlags(c, kind)
  c.PersistentFlags().String("subscription", "test-sub", "")
  return c
}

func TestResolveScopeLevels(t *testing.T) {
  tests := []struct {
    name string
    args []string
    want lockScope
  }{
    {
      name: "no flags is subscription scope",
      args: []string{},
      want: lockScope{Level: scopeSubscription},
    },
    {
      name: "resource group only",
      args: []string{"-g", "rg1"},
      want: lockScope{Level: scopeResourceGroup, ResourceGroup: "rg1"},
    },
    {
      name: "qualified resource type splits namespace",
      args: []string{"-g", "rg1", "--resource", "v1", "--resource-type", "Microsoft.Network/virtualNetworks"},
      want: lockScope{Level: scopeResource, ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1"},
    },
    {
      name: "explicit namespace with bare type",
      args: []string{"-g", "rg1", "--resource", "v1", "--resource-type", "virtualNetworks", "--namespace", "Microsoft.Network"},
      want: lockScope{Level: scopeResource, ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1"},
    },
    {
      name: "child resource via parent",
      args: []string{"-g", "rg1", "--resource", "sub1", "--resource-type", "Microsoft.Network/subnets", "--parent", "virtualNetworks/v1"},
      want: lockScope{Level: scopeResource, ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "subnets", ResourceName: "sub1", Parent: "virtualNetworks/v1"},
    },
    {
      name: "resource-name is an alias for resource",
      args: []string{"-g", "rg1", "--resource-name", "v1", "--resource-type", "Microsoft.Network/virtualNetworks"},
      want: lockScope{Level: scopeResource, ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1"},
    },
  }
  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      c := newScopeCmd(kindGeneric)
      if err := c.ParseFlags(tt.args); err != nil {
        t.Fatal(err)
      }
      got, err := resolveScope(c)
      if err != nil {
        t.Fatal(err)
      }
      if got != tt.want {
        t.Errorf("got  %+v\nwant %+v", got, tt.want)
      }
    })
  }
}

func TestResolveScopeBackPopulatesFromResourceID(t *testing.T) {
  c := newScopeCmd(kindGeneric)
  if err := c.ParseFlags([]string{"--resource", "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/v1"}); err != nil {
    t.Fatal(err)
  }
  got, err := resolveScope(c)
  if err != nil {
    t.Fatal(err)
  }
  want := lockScope{Level: scopeResource, ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1"}
  if got != want {
    t.Errorf("got  %+v\nwant %+v", got, want)
  }
}

func TestResolveScopeErrors(t *testing.T) {
  tests := []struct {
    name    string
    args    []string
    wantErr string
  }{
    {"resource-type without resource-group", []string{"--resource-type", "Microsoft.Network/virtualNetworks"}, "--resource-type requires --resource-group"},
    {"namespace without resource-group", []string{"--namespace", "Microsoft.Network"}, "--namespace requires --resource-group"},
    {"parent without resource-group", []string{"--parent", "virtualNetworks/v1"}, "--parent requires --resource-group"},
    {"resource name without resource-group", []string{"--resource", "notanid"}, "--resource must be a full resource ID when --resource-group is omitted"},
    {"resource-type without resource", []string{"-g", "rg1", "--resource-type", "x"}, "--resource-type requires --resource"},
    {"namespace without resource", []string{"-g", "rg1", "--namespace", "Microsoft.Network"}, "--namespace requires --resource"},
    {"parent without resource", []string{"-g", "rg1", "--parent", "virtualNetworks/v1"}, "--parent requires --resource"},
    {"resource without resource-type", []string{"-g", "rg1", "--resource", "v1"}, "--resource-type is required when --resource is given"},
    {"bare type without namespace", []string{"-g", "rg1", "--resource", "v1", "--resource-type", "virtualNetworks"}, "--resource-type must be namespace/type, or pass --namespace"},
    {"namespace given twice", []string{"-g", "rg1", "--resource", "v1", "--resource-type", "Microsoft.Network/virtualNetworks", "--namespace", "Microsoft.Network"}, "--namespace given in both --resource-type and --namespace"},
    {"three segment type", []string{"-g", "rg1", "--resource", "s1", "--resource-type", "Microsoft.Network/virtualNetworks/subnets"}, "--resource-type must be namespace/type; use --parent for child resources"},
  }
  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      c := newScopeCmd(kindGeneric)
      if err := c.ParseFlags(tt.args); err != nil {
        t.Fatal(err)
      }
      _, err := resolveScope(c)
      if err == nil {
        t.Fatalf("expected error %q, got nil", tt.wantErr)
      }
      if !strings.Contains(err.Error(), tt.wantErr) {
        t.Errorf("got  %q\nwant %q", err.Error(), tt.wantErr)
      }
    })
  }
}

// Each group registers only its own scope flags.
func TestScopeFlagsPerKind(t *testing.T) {
  tests := []struct {
    kind    scopeKind
    present []string
    absent  []string
  }{
    {kindAccount, []string{}, []string{"resource-group", "resource", "resource-type", "namespace", "parent"}},
    {kindGroup, []string{"resource-group"}, []string{"resource", "resource-type", "namespace", "parent"}},
    {kindResource, []string{"resource-group", "resource", "resource-type", "namespace", "parent"}, []string{}},
    {kindGeneric, []string{"resource-group", "resource", "resource-type", "namespace", "parent"}, []string{}},
  }
  for _, tt := range tests {
    c := newScopeCmd(tt.kind)
    for _, f := range tt.present {
      if c.Flags().Lookup(f) == nil {
        t.Errorf("kind %v: flag --%s should be registered", tt.kind, f)
      }
    }
    for _, f := range tt.absent {
      if c.Flags().Lookup(f) != nil {
        t.Errorf("kind %v: flag --%s should NOT be registered", tt.kind, f)
      }
    }
  }
}

// account lock has no scope flags at all, so it is always subscription scope.
func TestResolveScopeAccountKindIsAlwaysSubscription(t *testing.T) {
  c := newScopeCmd(kindAccount)
  if err := c.ParseFlags([]string{}); err != nil {
    t.Fatal(err)
  }
  got, err := resolveScope(c)
  if err != nil {
    t.Fatal(err)
  }
  if got.Level != scopeSubscription {
    t.Errorf("got %v want scopeSubscription", got.Level)
  }
}
