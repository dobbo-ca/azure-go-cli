package resource

import (
  "reflect"
  "testing"

  "github.com/spf13/cobra"
)

func TestParseResourceID(t *testing.T) {
  tests := []struct {
    name      string
    id        string
    wantSub   string
    wantGroup string
    wantNS    string
    wantTypes []string
    wantNames []string
    wantErr   bool
  }{
    {
      name:      "top-level resource",
      id:        "/subscriptions/abc/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1",
      wantSub:   "abc",
      wantGroup: "rg1",
      wantNS:    "Microsoft.Network",
      wantTypes: []string{"virtualNetworks"},
      wantNames: []string{"vnet1"},
    },
    {
      name:      "child resource",
      id:        "/subscriptions/abc/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/sub1",
      wantSub:   "abc",
      wantGroup: "rg1",
      wantNS:    "Microsoft.Network",
      wantTypes: []string{"virtualNetworks", "subnets"},
      wantNames: []string{"vnet1", "sub1"},
    },
    {
      name:    "missing providers segment",
      id:      "/subscriptions/abc/resourceGroups/rg1/Microsoft.Network/virtualNetworks/vnet1",
      wantErr: true,
    },
    {
      name:    "empty",
      id:      "",
      wantErr: true,
    },
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      sub, group, ns, types, names, err := ParseResourceID(tt.id)
      if (err != nil) != tt.wantErr {
        t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
      }
      if tt.wantErr {
        return
      }
      if sub != tt.wantSub || group != tt.wantGroup || ns != tt.wantNS {
        t.Errorf("sub/group/ns: got %s/%s/%s want %s/%s/%s", sub, group, ns, tt.wantSub, tt.wantGroup, tt.wantNS)
      }
      if !reflect.DeepEqual(types, tt.wantTypes) {
        t.Errorf("types: got %v want %v", types, tt.wantTypes)
      }
      if !reflect.DeepEqual(names, tt.wantNames) {
        t.Errorf("names: got %v want %v", names, tt.wantNames)
      }
    })
  }
}

func TestBuildResourceID(t *testing.T) {
  tests := []struct {
    name         string
    sub, group   string
    namespace    string
    resourceType string
    parent       string
    rname        string
    want         string
    wantErr      bool
  }{
    {
      name:         "qualified type, no parent",
      sub:          "abc", group: "rg1", namespace: "", resourceType: "Microsoft.Network/virtualNetworks", rname: "vnet1",
      want: "/subscriptions/abc/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1",
    },
    {
      name:         "unqualified type with namespace",
      sub:          "abc", group: "rg1", namespace: "Microsoft.Network", resourceType: "virtualNetworks", rname: "vnet1",
      want: "/subscriptions/abc/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1",
    },
    {
      name:         "with parent",
      sub:          "abc", group: "rg1", namespace: "Microsoft.Network", resourceType: "subnets", parent: "virtualNetworks/vnet1", rname: "sub1",
      want: "/subscriptions/abc/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/sub1",
    },
    {
      name:         "missing namespace and unqualified type",
      sub:          "abc", group: "rg1", namespace: "", resourceType: "virtualNetworks", rname: "vnet1",
      wantErr: true,
    },
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      got, err := BuildResourceID(tt.sub, tt.group, tt.namespace, tt.resourceType, tt.parent, tt.rname)
      if (err != nil) != tt.wantErr {
        t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
      }
      if !tt.wantErr && got != tt.want {
        t.Errorf("got %s want %s", got, tt.want)
      }
    })
  }
}

func newSelectorCmd() *cobra.Command {
  c := &cobra.Command{Use: "x"}
  AddSelectorFlags(c)
  c.PersistentFlags().String("subscription", "test-sub", "")
  return c
}

func TestResolveSelector(t *testing.T) {
  t.Run("ids mode multiple", func(t *testing.T) {
    c := newSelectorCmd()
    c.ParseFlags([]string{"--ids", "/subscriptions/a/resourceGroups/r/providers/Microsoft.Foo/bar/n1", "--ids", "/subscriptions/a/resourceGroups/r/providers/Microsoft.Foo/bar/n2"})
    ids, err := ResolveSelector(c)
    if err != nil {
      t.Fatal(err)
    }
    if len(ids) != 2 {
      t.Errorf("want 2 ids, got %d", len(ids))
    }
  })

  t.Run("name mode qualified", func(t *testing.T) {
    c := newSelectorCmd()
    c.ParseFlags([]string{"-g", "rg1", "--resource-type", "Microsoft.Foo/bar", "-n", "name1"})
    ids, err := ResolveSelector(c)
    if err != nil {
      t.Fatal(err)
    }
    if len(ids) != 1 || ids[0] != "/subscriptions/test-sub/resourceGroups/rg1/providers/Microsoft.Foo/bar/name1" {
      t.Errorf("got %v", ids)
    }
  })

  t.Run("neither mode", func(t *testing.T) {
    c := newSelectorCmd()
    c.ParseFlags([]string{})
    if _, err := ResolveSelector(c); err == nil {
      t.Error("expected error")
    }
  })

  t.Run("both modes", func(t *testing.T) {
    c := newSelectorCmd()
    c.ParseFlags([]string{"--ids", "/subscriptions/a/resourceGroups/r/providers/Microsoft.Foo/bar/n1", "-g", "rg1"})
    if _, err := ResolveSelector(c); err == nil {
      t.Error("expected error")
    }
  })
}
