package lock

import (
  "strings"
  "testing"
)

func TestValidateScopeMatchesLock(t *testing.T) {
  base := lockIDParts{ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1", Parent: "", LockName: "l1"}

  tests := []struct {
    name    string
    want    lockScope
    got     lockIDParts
    wantErr string
  }{
    {
      name: "exact match",
      want: lockScope{ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1"},
      got:  base,
    },
    {
      name: "resource group compares case insensitively",
      want: lockScope{ResourceGroup: "RG1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1"},
      got:  base,
    },
    {
      name: "namespace compares case insensitively",
      want: lockScope{ResourceGroup: "rg1", Namespace: "microsoft.network", ResourceType: "virtualNetworks", ResourceName: "v1"},
      got:  base,
    },
    {
      name:    "resource group mismatch",
      want:    lockScope{ResourceGroup: "other", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1"},
      got:     base,
      wantErr: "--resource-group",
    },
    {
      name:    "resource type compares case sensitively",
      want:    lockScope{ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualnetworks", ResourceName: "v1"},
      got:     base,
      wantErr: "--resource-type",
    },
    {
      name:    "resource name compares case sensitively",
      want:    lockScope{ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "V1"},
      got:     base,
      wantErr: "--resource",
    },
    {
      name:    "parent mismatch",
      want:    lockScope{ResourceGroup: "rg1", Namespace: "Microsoft.Network", ResourceType: "virtualNetworks", ResourceName: "v1", Parent: "x/y"},
      got:     base,
      wantErr: "--parent",
    },
    {
      name: "empty user flags never conflict",
      want: lockScope{},
      got:  base,
    },
  }
  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      err := validateScopeMatchesLock(tt.want, tt.got, "l1")
      if tt.wantErr == "" {
        if err != nil {
          t.Fatalf("unexpected error: %v", err)
        }
        return
      }
      if err == nil {
        t.Fatalf("expected error containing %q, got nil", tt.wantErr)
      }
      if !strings.Contains(err.Error(), tt.wantErr) {
        t.Errorf("got %q want substring %q", err.Error(), tt.wantErr)
      }
    })
  }
}
