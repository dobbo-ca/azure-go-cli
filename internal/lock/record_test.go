package lock

import (
  "encoding/json"
  "testing"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
)

func strPtr(s string) *string { return &s }

func TestResourceGroupFromID(t *testing.T) {
  tests := []struct {
    name string
    id   string
    want string
  }{
    {"subscription scoped", "/subscriptions/s1/providers/Microsoft.Authorization/locks/l1", ""},
    {"resource group scoped", "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Authorization/locks/l1", "rg1"},
    {"lowercase resourcegroups", "/subscriptions/s1/resourcegroups/rg1/providers/Microsoft.Authorization/locks/l1", "rg1"},
    {"resource scoped", "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/v1/providers/Microsoft.Authorization/locks/l1", "rg1"},
    {"empty", "", ""},
    {"too short", "/subscriptions/s1", ""},
  }
  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      if got := resourceGroupFromID(tt.id); got != tt.want {
        t.Errorf("got %q want %q", got, tt.want)
      }
    })
  }
}

func TestToLockRecordFlattens(t *testing.T) {
  level := armlocks.LockLevelCanNotDelete
  obj := &armlocks.ManagementLockObject{
    ID:   strPtr("/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Authorization/locks/l1"),
    Name: strPtr("l1"),
    Type: strPtr("Microsoft.Authorization/locks"),
    Properties: &armlocks.ManagementLockProperties{
      Level: &level,
      Notes: strPtr("do not delete"),
    },
  }
  rec := toLockRecord(obj)
  if rec.Level != "CanNotDelete" {
    t.Errorf("level: got %q want CanNotDelete", rec.Level)
  }
  if rec.Notes == nil || *rec.Notes != "do not delete" {
    t.Errorf("notes: got %v", rec.Notes)
  }
  if rec.ResourceGroup != "rg1" {
    t.Errorf("resourceGroup: got %q want rg1", rec.ResourceGroup)
  }
}

// azure-cli emits `"owners": null`, never `[]`.
func TestToLockRecordOwnersNull(t *testing.T) {
  level := armlocks.LockLevelReadOnly
  obj := &armlocks.ManagementLockObject{
    ID:         strPtr("/subscriptions/s1/providers/Microsoft.Authorization/locks/l1"),
    Name:       strPtr("l1"),
    Type:       strPtr("Microsoft.Authorization/locks"),
    Properties: &armlocks.ManagementLockProperties{Level: &level},
  }
  b, err := json.Marshal(toLockRecord(obj))
  if err != nil {
    t.Fatal(err)
  }
  var m map[string]json.RawMessage
  if err := json.Unmarshal(b, &m); err != nil {
    t.Fatal(err)
  }
  if string(m["owners"]) != "null" {
    t.Errorf("owners: got %s want null", m["owners"])
  }
  if _, ok := m["resourceGroup"]; ok {
    t.Error("resourceGroup must be omitted for a subscription-scoped lock")
  }
  if _, ok := m["systemData"]; ok {
    t.Error("systemData must never be emitted; azure-cli pins api-version 2016-09-01, which has no such field")
  }
}

// Go marshals in declaration order; knack emits sort_keys=True. Declaring the
// struct alphabetically is what makes the two agree.
func TestLockRecordKeyOrderIsAlphabetical(t *testing.T) {
  level := armlocks.LockLevelCanNotDelete
  obj := &armlocks.ManagementLockObject{
    ID:   strPtr("/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Authorization/locks/l1"),
    Name: strPtr("l1"),
    Type: strPtr("Microsoft.Authorization/locks"),
    Properties: &armlocks.ManagementLockProperties{
      Level: &level,
      Notes: strPtr("n"),
    },
  }
  b, err := json.Marshal(toLockRecord(obj))
  if err != nil {
    t.Fatal(err)
  }
  want := `{"id":"/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Authorization/locks/l1","level":"CanNotDelete","name":"l1","notes":"n","owners":null,"resourceGroup":"rg1","type":"Microsoft.Authorization/locks"}`
  if string(b) != want {
    t.Errorf("got  %s\nwant %s", b, want)
  }
}
