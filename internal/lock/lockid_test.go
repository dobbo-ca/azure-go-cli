package lock

import "testing"

func TestParseLockID(t *testing.T) {
  tests := []struct {
    name    string
    id      string
    want    lockIDParts
    wantErr bool
  }{
    {
      name: "subscription scoped",
      id:   "/subscriptions/s1/providers/Microsoft.Authorization/locks/l1",
      want: lockIDParts{LockName: "l1"},
    },
    {
      name: "resource group scoped",
      id:   "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Authorization/locks/l1",
      want: lockIDParts{ResourceGroup: "rg1", LockName: "l1"},
    },
    {
      name: "lowercase resourcegroups",
      id:   "/subscriptions/s1/resourcegroups/rg1/providers/Microsoft.Authorization/locks/l1",
      want: lockIDParts{ResourceGroup: "rg1", LockName: "l1"},
    },
    {
      name: "resource scoped",
      id:   "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/v1/providers/Microsoft.Authorization/locks/l1",
      want: lockIDParts{ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1", LockName: "l1"},
    },
    {
      name: "child resource with parent",
      id:   "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/v1/subnets/sub1/providers/Microsoft.Authorization/locks/l1",
      want: lockIDParts{ResourceGroup: "rg1", Namespace: "Microsoft.Network", Parent: "virtualNetworks/v1", ResourceType: "subnets", ResourceName: "sub1", LockName: "l1"},
    },
    {
      name: "trailing garbage tolerated, matching python .match()",
      id:   "/subscriptions/s1/providers/Microsoft.Authorization/locks/l1/extra/stuff",
      want: lockIDParts{LockName: "l1"},
    },
    {name: "empty", id: "", wantErr: true},
    {name: "not a lock id", id: "/subscriptions/s1/resourceGroups/rg1", wantErr: true},
    {name: "garbage", id: "hello", wantErr: true},
  }
  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      got, err := parseLockID(tt.id)
      if (err != nil) != tt.wantErr {
        t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
      }
      if tt.wantErr {
        return
      }
      if got != tt.want {
        t.Errorf("got  %+v\nwant %+v", got, tt.want)
      }
    })
  }
}

// Python crashes with an unhandled AttributeError on an ID that is valid but
// carries no resource name. We return a real error instead. Spec divergence 5.
func TestParseLockIDEmptyResourceNameErrors(t *testing.T) {
  id := "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Network//providers/Microsoft.Authorization/locks/l1"
  if _, err := parseLockID(id); err == nil {
    t.Error("expected an error for an ID with an empty resource name")
  }
}
