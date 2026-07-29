# `az lock` Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the Azure CLI `lock` surface — `az lock`, `az account lock`, `az group lock`, `az resource lock`, five verbs each — into azure-go-cli.

**Architecture:** One shared implementation in `internal/lock/`. Pure helpers (record flattening, lock-ID parsing, scope resolution, scope precheck) are built and unit-tested first; the five verbs then wire those helpers to `armlocks`. The four command groups differ only in which scope flags they register. A generic azure-cli-faithful `renderTable` lands in `pkg/output`.

**Tech Stack:** Go, cobra, `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks` v1.2.0.

**Spec:** `docs/superpowers/specs/2026-07-16-az-lock-port-design.md` — read it before starting. It records every deliberate divergence from azure-cli.

## Global Constraints

- **Indentation: 2 spaces** in all new files (per CLAUDE.md). Exception: `pkg/output/output.go` is an existing tab-indented file — match its tabs when editing it. Never mix both styles inside one file. Do not run `gofmt -w` on anything.
- All files end with a newline (LF).
- Build with `make build` (never plain `go build`). Binary lands at `bin/az/az`.
- Test with `make test`.
- Conventional commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`).
- Error strings: repo house style — lowercase, terse, no trailing punctuation, name the offending flag. Model: `internal/resource/resolve.go:89-101`. Do **not** copy azure-cli's wording.
- `armlocks` import path has **no version suffix**: `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks`.
- Lock type values are exactly `CanNotDelete` and `ReadOnly`. Accept input case-insensitively (`strings.EqualFold`), emit canonical. Reject `NotSpecified` even though the SDK defines it.
- Baseline before starting: `make test` exits 0.

---

### Task 1: Lock record — flatten `properties`

ARM marks `ManagementLockProperties` with `x-ms-client-flatten`, so azure-cli prints `level`/`notes`/`owners` at the top level. The Go SDK does not flatten. Without this task every azure-cli `--query "[].level"` breaks.

`resourceGroup` is not an ARM field — azure-cli injects it via a global result transform, so it appears in JSON too, only for RG- and resource-scoped locks.

Struct fields are declared **alphabetically on purpose**: knack emits JSON with `sort_keys=True` (`knack/output.py:37`), and Go marshals in declaration order, so alphabetical declaration gives byte-identical key order.

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/lock/record.go`
- Test: `internal/lock/record_test.go`

**Interfaces:**
- Produces: `lockRecord` struct; `toLockRecord(*armlocks.ManagementLockObject) lockRecord`; `toLockRecords([]*armlocks.ManagementLockObject) []lockRecord`; `resourceGroupFromID(string) string`

- [ ] **Step 1: Add the armlocks dependency**

```bash
go get github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks@v1.2.0
```

Expected: `go: added github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks v1.2.0`. It will be marked `// indirect` until Step 3 imports it.

- [ ] **Step 2: Write the failing test**

Create `internal/lock/record_test.go`:

```go
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
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/lock/ -run TestToLockRecord -v`
Expected: FAIL — the package does not compile (`undefined: toLockRecord`).

- [ ] **Step 4: Write the implementation**

Create `internal/lock/record.go`:

```go
package lock

import (
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
)

// lockRecord is the azure-cli-shaped, flattened view of a management lock.
//
// ARM marks ManagementLockProperties with x-ms-client-flatten, so azure-cli
// prints level/notes/owners at the top level. The Go SDK keeps them nested
// under Properties, so we flatten here; without it every --query expression
// written for azure-cli (e.g. "[].level") breaks.
//
// Fields are declared in alphabetical order deliberately: knack emits JSON
// with sort_keys=True, and encoding/json emits declaration order, so this is
// what makes our key order match azure-cli's byte for byte.
//
// SystemData is intentionally absent. azure-cli pins api-version 2016-09-01,
// which has no such field; armlocks is generated against 2020-05-01, which
// does. Emitting it would diverge from azure-cli.
type lockRecord struct {
  ID            string      `json:"id"`
  Level         string      `json:"level"`
  Name          string      `json:"name"`
  Notes         *string     `json:"notes"`
  Owners        []lockOwner `json:"owners"`
  ResourceGroup string      `json:"resourceGroup,omitempty"`
  Type          string      `json:"type"`
}

type lockOwner struct {
  ApplicationID string `json:"applicationId"`
}

// resourceGroupFromID mirrors azure-cli's global _resource_group_transform,
// which injects resourceGroup into every result whose ID carries a resource
// group. It fires before output formatting, so the field shows up in JSON as
// well as table output, and is absent for subscription-scoped locks.
func resourceGroupFromID(id string) string {
  parts := strings.Split(id, "/")
  if len(parts) < 5 || !strings.EqualFold(parts[3], "resourcegroups") {
    return ""
  }
  return parts[4]
}

func toLockRecord(o *armlocks.ManagementLockObject) lockRecord {
  rec := lockRecord{}
  if o == nil {
    return rec
  }
  if o.ID != nil {
    rec.ID = *o.ID
    rec.ResourceGroup = resourceGroupFromID(*o.ID)
  }
  if o.Name != nil {
    rec.Name = *o.Name
  }
  if o.Type != nil {
    rec.Type = *o.Type
  }
  if p := o.Properties; p != nil {
    if p.Level != nil {
      rec.Level = string(*p.Level)
    }
    rec.Notes = p.Notes
    for _, ow := range p.Owners {
      if ow == nil {
        continue
      }
      owner := lockOwner{}
      if ow.ApplicationID != nil {
        owner.ApplicationID = *ow.ApplicationID
      }
      rec.Owners = append(rec.Owners, owner)
    }
  }
  return rec
}

func toLockRecords(objs []*armlocks.ManagementLockObject) []lockRecord {
  recs := make([]lockRecord, 0, len(objs))
  for _, o := range objs {
    recs = append(recs, toLockRecord(o))
  }
  return recs
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/lock/ -v`
Expected: PASS for all four tests.

- [ ] **Step 6: Tidy and commit**

```bash
go mod tidy
git add go.mod go.sum internal/lock/record.go internal/lock/record_test.go
git commit -m "feat(lock): add flattened lock record

ARM flattens ManagementLockProperties via x-ms-client-flatten but the Go SDK
does not, so hand-flatten level/notes/owners to the top level. Without this,
--query expressions written for azure-cli do not match.

Fields are declared alphabetically because knack emits JSON with sort_keys=True
while encoding/json emits declaration order; alphabetical declaration is what
makes the key order match. systemData is deliberately never emitted: azure-cli
pins api-version 2016-09-01, which has no such field."
```

---

### Task 2: Lock ID parser

Lock IDs at resource scope contain **two** `/providers/` segments, so `resource.ParseResourceID` cannot parse them. Python uses `.match()` (not `.fullmatch()`), tolerating trailing garbage; Go's `FindStringSubmatch` has the same unanchored-at-end semantics, so **do not append `$`**.

**Files:**
- Create: `internal/lock/lockid.go`
- Test: `internal/lock/lockid_test.go`

**Interfaces:**
- Produces: `lockIDParts` struct; `parseLockID(string) (lockIDParts, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/lock/lockid_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/lock/ -run TestParseLockID -v`
Expected: FAIL — `undefined: parseLockID`.

- [ ] **Step 3: Write the implementation**

Create `internal/lock/lockid.go`:

```go
package lock

import (
  "fmt"
  "regexp"
  "strings"
)

// lockIDParts is the scope carried by a lock resource ID.
type lockIDParts struct {
  ResourceGroup string
  Namespace     string
  Parent        string
  ResourceType  string
  ResourceName  string
  LockName      string
}

// lockIDRe matches a management lock ID at any of the three scopes.
//
// A resource-scoped lock ID contains TWO /providers/ segments — the locked
// resource's, then Microsoft.Authorization's — which is why the repo's
// resource.ParseResourceID cannot be reused here.
//
// Everything after the subscription ID is optional, so subscription-scoped IDs
// match with only LockName populated. There is deliberately no trailing `$`:
// azure-cli uses re.match(), which tolerates trailing garbage, and
// FindStringSubmatch matches that behavior.
var lockIDRe = regexp.MustCompile(
  `^/subscriptions/[^/]*` +
    `(?:/resource[gG]roups/([^/]*)` +
    `(?:/providers/([^/]*)` +
    `(?:/(.*))?/([^/]*)/([^/]*))?)?` +
    `/providers/Microsoft\.Authorization/locks/([^/]*)`,
)

func parseLockID(id string) (lockIDParts, error) {
  m := lockIDRe.FindStringSubmatch(id)
  if m == nil {
    return lockIDParts{}, fmt.Errorf("invalid lock ID: %s", id)
  }
  p := lockIDParts{
    ResourceGroup: m[1],
    Namespace:     m[2],
    Parent:        strings.Trim(m[3], "/"),
    ResourceType:  m[4],
    ResourceName:  m[5],
    LockName:      m[6],
  }
  if p.LockName == "" {
    return lockIDParts{}, fmt.Errorf("invalid lock ID, no lock name: %s", id)
  }
  // A namespace with no resource name is a malformed resource scope. azure-cli
  // crashes here with an unhandled AttributeError; return a real error.
  if p.Namespace != "" && p.ResourceName == "" {
    return lockIDParts{}, fmt.Errorf("invalid lock ID, no resource name: %s", id)
  }
  return p, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/lock/ -v`
Expected: PASS. If the `child resource with parent` case fails, the greedy `(.*)` is the likely cause — verify it captures `virtualNetworks/v1` and not more.

- [ ] **Step 5: Commit**

```bash
git add internal/lock/lockid.go internal/lock/lockid_test.go
git commit -m "feat(lock): add lock ID parser

Resource-scoped lock IDs carry two /providers/ segments, so the existing
resource.ParseResourceID cannot parse them. Deliberately unanchored at the end
to match azure-cli's re.match() semantics, which tolerate trailing garbage.

Diverges from azure-cli on one case: an ID that is otherwise valid but carries
an empty resource name returns an error rather than crashing with an unhandled
AttributeError."
```

---

### Task 3: Scope resolution

Ports `internal_validate_lock_parameters`. Error strings are rewritten in repo house style — azure-cli's originals say "is ignored" while actually raising, and one has a missing-space typo.

**Files:**
- Create: `internal/lock/scope.go`
- Test: `internal/lock/scope_test.go`

**Interfaces:**
- Consumes: `parseLockID` (Task 2)
- Produces: `scopeLevel` (`scopeSubscription`, `scopeResourceGroup`, `scopeResource`); `lockScope` struct; `addScopeFlags(*cobra.Command, scopeKind)`; `resolveScope(*cobra.Command) (lockScope, error)`; `scopeKind` (`kindGeneric`, `kindAccount`, `kindGroup`, `kindResource`)

- [ ] **Step 1: Write the failing test**

Create `internal/lock/scope_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/lock/ -run TestResolveScope -v`
Expected: FAIL — `undefined: resolveScope`.

- [ ] **Step 3: Write the implementation**

Create `internal/lock/scope.go`:

```go
package lock

import (
  "fmt"
  "strings"

  "github.com/spf13/cobra"
)

type scopeLevel int

const (
  scopeSubscription scopeLevel = iota
  scopeResourceGroup
  scopeResource
)

// scopeKind selects which scope flags a command group registers. The four
// azure-cli lock groups share one implementation and differ only here.
type scopeKind int

const (
  kindGeneric scopeKind = iota // az lock: scope inferred from flags
  kindAccount                  // az account lock: always subscription
  kindGroup                    // az group lock: always resource group
  kindResource                 // az resource lock: always resource
)

// lockScope is the resolved target of a lock operation.
type lockScope struct {
  Level         scopeLevel
  ResourceGroup string
  Namespace     string
  Parent        string
  ResourceType  string
  ResourceName  string
}

// addScopeFlags registers the scope flags appropriate to kind.
//
// --resource and --resource-name are co-equal long aliases in azure-cli, not a
// flag plus a shorthand. A normalize func folds the alias onto one flag so help
// shows a single line.
func addScopeFlags(cmd *cobra.Command, kind scopeKind) {
  switch kind {
  case kindAccount:
    return
  case kindGroup:
    cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
    return
  }

  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("resource", "", "Name or ID of the resource being locked. If an ID is given, other resource arguments should not be given")
  cmd.Flags().String("resource-type", "", "Resource type, qualified (Microsoft.Provider/resC) or bare with --namespace")
  cmd.Flags().String("namespace", "", "Provider namespace, e.g. Microsoft.Provider")
  cmd.Flags().String("parent", "", "Parent path for child resources, e.g. resA/myA/resB/myB")
  cmd.Flags().SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
    if name == "resource-name" {
      name = "resource"
    }
    return pflag.NormalizedName(name)
  })
}

// flagOrEmpty reads a string flag that may not be registered for this kind.
func flagOrEmpty(cmd *cobra.Command, name string) string {
  if cmd.Flags().Lookup(name) == nil {
    return ""
  }
  v, _ := cmd.Flags().GetString(name)
  return v
}

// resolveScope ports azure-cli's internal_validate_lock_parameters.
//
// azure-cli's messages say a flag "is ignored" while actually raising, and one
// carries a missing-space typo. These are rewritten in repo house style.
func resolveScope(cmd *cobra.Command) (lockScope, error) {
  rg := flagOrEmpty(cmd, "resource-group")
  resource := flagOrEmpty(cmd, "resource")
  rtype := flagOrEmpty(cmd, "resource-type")
  ns := flagOrEmpty(cmd, "namespace")
  parent := flagOrEmpty(cmd, "parent")

  if rg == "" {
    if resource != "" {
      if !strings.HasPrefix(resource, "/subscriptions/") {
        return lockScope{}, fmt.Errorf("--resource must be a full resource ID when --resource-group is omitted")
      }
      parts, err := parseResourceScopeID(resource)
      if err != nil {
        return lockScope{}, err
      }
      if rtype != "" {
        return lockScope{}, fmt.Errorf("--resource-type not allowed when --resource is a full resource ID")
      }
      if ns != "" {
        return lockScope{}, fmt.Errorf("--namespace not allowed when --resource is a full resource ID")
      }
      if parent != "" {
        return lockScope{}, fmt.Errorf("--parent not allowed when --resource is a full resource ID")
      }
      parts.Level = scopeResource
      return parts, nil
    }
    if rtype != "" {
      return lockScope{}, fmt.Errorf("--resource-type requires --resource-group")
    }
    if ns != "" {
      return lockScope{}, fmt.Errorf("--namespace requires --resource-group")
    }
    if parent != "" {
      return lockScope{}, fmt.Errorf("--parent requires --resource-group")
    }
    return lockScope{Level: scopeSubscription}, nil
  }

  if resource == "" {
    if rtype != "" {
      return lockScope{}, fmt.Errorf("--resource-type requires --resource")
    }
    if ns != "" {
      return lockScope{}, fmt.Errorf("--namespace requires --resource")
    }
    if parent != "" {
      return lockScope{}, fmt.Errorf("--parent requires --resource")
    }
    return lockScope{Level: scopeResourceGroup, ResourceGroup: rg}, nil
  }

  if rtype == "" {
    return lockScope{}, fmt.Errorf("--resource-type is required when --resource is given")
  }
  segments := strings.Split(rtype, "/")
  switch {
  case len(segments) > 2:
    // azure-cli's split('/', 2) silently leaves a 3-segment type unsplit and
    // malforms the ARM path. Error instead. Spec divergence 4.
    return lockScope{}, fmt.Errorf("--resource-type must be namespace/type; use --parent for child resources")
  case len(segments) == 2:
    if ns != "" {
      return lockScope{}, fmt.Errorf("--namespace given in both --resource-type and --namespace")
    }
    ns, rtype = segments[0], segments[1]
  default:
    if ns == "" {
      return lockScope{}, fmt.Errorf("--resource-type must be namespace/type, or pass --namespace")
    }
  }

  return lockScope{
    Level:         scopeResource,
    ResourceGroup: rg,
    Namespace:     ns,
    Parent:        strings.Trim(parent, "/"),
    ResourceType:  rtype,
    ResourceName:  resource,
  }, nil
}

// parseResourceScopeID back-populates a scope from a full resource ID passed
// to --resource, mirroring azure-cli's use of parse_resource_id there.
func parseResourceScopeID(id string) (lockScope, error) {
  trimmed := strings.Trim(id, "/")
  parts := strings.Split(trimmed, "/")
  if len(parts) < 8 || !strings.EqualFold(parts[2], "resourcegroups") || !strings.EqualFold(parts[4], "providers") {
    return lockScope{}, fmt.Errorf("--resource is not a valid resource ID: %s", id)
  }
  rest := parts[6:]
  if len(rest)%2 != 0 {
    return lockScope{}, fmt.Errorf("--resource is not a valid resource ID: %s", id)
  }
  s := lockScope{ResourceGroup: parts[3], Namespace: parts[5]}
  s.ResourceType = rest[len(rest)-2]
  s.ResourceName = rest[len(rest)-1]
  if len(rest) > 2 {
    s.Parent = strings.Join(rest[:len(rest)-2], "/")
  }
  return s, nil
}
```

Add `"github.com/spf13/pflag"` to the import block.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/lock/ -v`
Expected: PASS for every subtest.

- [ ] **Step 5: Commit**

```bash
git add internal/lock/scope.go internal/lock/scope_test.go
git commit -m "feat(lock): add scope resolution and per-group flags

Ports azure-cli's internal_validate_lock_parameters. The four lock groups share
this implementation and differ only in which scope flags they register, which is
what scopeKind selects.

Two deliberate divergences: validation messages are rewritten in repo house
style rather than reproducing azure-cli's wording (its messages claim a flag 'is
ignored' while actually raising, and one has a missing-space typo), and a
3-segment --resource-type now errors instead of silently malforming the ARM
path the way azure-cli's split('/', 2) does."
```

---

### Task 4: Scope precheck

Ports `_validate_lock_params_match_lock`. It lists locks subscription-wide, counts by name, and validates the user's scope flags only when exactly one lock matches. Comparison is case-insensitive on resource group and namespace, case-sensitive on type, name, and parent — that asymmetry is azure-cli's, and is reproduced.

This task builds only the pure comparison; Task 7 wires it to the live list call.

**Files:**
- Create: `internal/lock/precheck.go`
- Test: `internal/lock/precheck_test.go`

**Interfaces:**
- Consumes: `lockScope`, `lockIDParts` (Tasks 2-3)
- Produces: `validateScopeMatchesLock(want lockScope, got lockIDParts, lockName string) error`

- [ ] **Step 1: Write the failing test**

Create `internal/lock/precheck_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/lock/ -run TestValidateScopeMatchesLock -v`
Expected: FAIL — `undefined: validateScopeMatchesLock`.

- [ ] **Step 3: Write the implementation**

Create `internal/lock/precheck.go`:

```go
package lock

import (
  "fmt"
  "strings"
)

// validateScopeMatchesLock ports azure-cli's _validate_lock_params_match_lock.
//
// It compares the scope the user asked for against the scope of the single lock
// that matched by name, turning what would otherwise be a bare 404 into a
// message naming the offending flag. An empty user flag never conflicts.
//
// The case-sensitivity asymmetry is azure-cli's, reproduced deliberately:
// resource group and namespace compare case-insensitively, everything else
// case-sensitively.
func validateScopeMatchesLock(want lockScope, got lockIDParts, lockName string) error {
  if want.ResourceGroup != "" && !strings.EqualFold(want.ResourceGroup, got.ResourceGroup) {
    return fmt.Errorf("unexpected --resource-group for lock %s, expected %s", lockName, got.ResourceGroup)
  }
  if want.Namespace != "" && !strings.EqualFold(want.Namespace, got.Namespace) {
    return fmt.Errorf("unexpected --namespace for lock %s, expected %s", lockName, got.Namespace)
  }
  if want.ResourceType != "" && want.ResourceType != got.ResourceType {
    return fmt.Errorf("unexpected --resource-type for lock %s, expected %s", lockName, got.ResourceType)
  }
  if want.ResourceName != "" && want.ResourceName != got.ResourceName {
    return fmt.Errorf("unexpected --resource for lock %s, expected %s", lockName, got.ResourceName)
  }
  if want.Parent != "" && want.Parent != got.Parent {
    return fmt.Errorf("unexpected --parent for lock %s, expected %s", lockName, got.Parent)
  }
  return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/lock/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/lock/precheck.go internal/lock/precheck_test.go
git commit -m "feat(lock): add scope precheck comparison

Ports the comparison half of azure-cli's _validate_lock_params_match_lock,
which turns a bare 404 into a message naming the offending flag. The
case-sensitivity asymmetry is azure-cli's and is reproduced deliberately:
resource group and namespace compare case-insensitively, the rest do not.

Messages use repo house style rather than azure-cli's wording."
```

---

### Task 5: Generic `renderTable` in `pkg/output`

Reimplements knack's generic table formatter. `pkg/output/output.go:73` already advertises `table` while rejecting it; this fixes that.

**`pkg/output/output.go` is tab-indented and gofmt-clean — match its tabs, not 2 spaces.**

Rules, from `knack/output.py` `_TableOutput`:
- `SKIP_KEYS = ['id', 'type', 'etag']` dropped unconditionally.
- Values that are objects, arrays, or null are dropped.
- Keys sorted alphabetically; header is first-char-uppercased only.
- A non-list result is wrapped in a single-element list.
- Column set is the union across rows; a key null in *every* row disappears, but null in *some* rows yields a blank cell.

**Files:**
- Modify: `pkg/output/output.go:61-74`
- Test: `pkg/output/output_test.go`

**Interfaces:**
- Produces: `renderTable(interface{}) string`; `PrintFormatted` gains `case "table":`

- [ ] **Step 1: Write the failing test**

Append to `pkg/output/output_test.go` (tabs):

```go
func TestRenderTable(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{
			name: "drops id/type, sorts keys, capitalizes header",
			in: []interface{}{map[string]interface{}{
				"id":    "/subscriptions/s1/providers/Microsoft.Authorization/locks/l1",
				"type":  "Microsoft.Authorization/locks",
				"level": "CanNotDelete",
				"name":  "mylock",
				"notes": "do not delete",
			}},
			want: "Level         Name    Notes\n" +
				"------------  ------  -------------\n" +
				"CanNotDelete  mylock  do not delete\n",
		},
		{
			name: "null in every row drops the column entirely",
			in: []interface{}{map[string]interface{}{
				"level":  "ReadOnly",
				"name":   "l1",
				"owners": nil,
			}},
			want: "Level     Name\n" +
				"--------  ------\n" +
				"ReadOnly  l1\n",
		},
		{
			name: "null in only some rows leaves a blank cell",
			in: []interface{}{
				map[string]interface{}{"level": "ReadOnly", "name": "a", "notes": "keep"},
				map[string]interface{}{"level": "ReadOnly", "name": "b", "notes": nil},
			},
			want: "Level     Name    Notes\n" +
				"--------  ------  -------\n" +
				"ReadOnly  a       keep\n" +
				"ReadOnly  b\n",
		},
		{
			name: "single object renders like a one-row list",
			in:   map[string]interface{}{"level": "ReadOnly", "name": "l1"},
			want: "Level     Name\n" +
				"--------  ------\n" +
				"ReadOnly  l1\n",
		},
		{
			name: "nested values are dropped",
			in: []interface{}{map[string]interface{}{
				"name":       "l1",
				"systemData": map[string]interface{}{"createdBy": "x"},
			}},
			want: "Name\n" +
				"------\n" +
				"l1\n",
		},
		{name: "empty list", in: []interface{}{}, want: "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderTable(tt.in); got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/output/ -run TestRenderTable -v`
Expected: FAIL — `undefined: renderTable`.

- [ ] **Step 3: Write the implementation**

In `pkg/output/output.go`, add `case "table":` to the switch at line 61 (tabs):

```go
	case "table":
		fmt.Print(renderTable(result))
		return nil
```

Then append (tabs):

```go
// tableSkipKeys mirrors knack's _TableOutput.SKIP_KEYS.
var tableSkipKeys = map[string]bool{"id": true, "type": true, "etag": true}

// renderTable renders a result the way azure-cli's generic table formatter
// does. azure-cli commands only get a bespoke table layout when they register a
// table_transformer; without one, knack derives columns from the data itself:
// skip id/type/etag, drop values that are null or non-scalar, sort the
// remaining keys alphabetically, and uppercase the first character of each for
// the header. A non-list result renders as a single row.
//
// Consequence worth knowing: the column set is data-dependent, so it shifts
// with the shape of what came back.
func renderTable(v interface{}) string {
	var rows []map[string]interface{}
	switch val := v.(type) {
	case nil:
		return "\n"
	case []interface{}:
		for _, el := range val {
			if m, ok := el.(map[string]interface{}); ok {
				rows = append(rows, m)
			}
		}
	case map[string]interface{}:
		rows = append(rows, val)
	default:
		return fmt.Sprintf("%v\n", val)
	}
	if len(rows) == 0 {
		return "\n"
	}

	keySet := map[string]bool{}
	for _, row := range rows {
		for k, val := range row {
			if tableSkipKeys[k] || !isTableScalar(val) {
				continue
			}
			keySet[k] = true
		}
	}
	if len(keySet) == 0 {
		return "\n"
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	headers := make([]string, len(keys))
	for i, k := range keys {
		headers[i] = capitalizeFirst(k)
	}
	cells := make([][]string, len(rows))
	for i, row := range rows {
		cells[i] = make([]string, len(keys))
		for j, k := range keys {
			if val, ok := row[k]; ok && isTableScalar(val) {
				cells[i][j] = tsvScalar(val)
			}
		}
	}

	widths := make([]int, len(keys))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range cells {
		for j, c := range row {
			if len(c) > widths[j] {
				widths[j] = len(c)
			}
		}
	}

	var b strings.Builder
	writeTableRow(&b, headers, widths)
	rules := make([]string, len(keys))
	for i, w := range widths {
		rules[i] = strings.Repeat("-", w)
	}
	writeTableRow(&b, rules, widths)
	for _, row := range cells {
		writeTableRow(&b, row, widths)
	}
	return b.String()
}

// writeTableRow joins cells with two spaces, padding all but the last. Trailing
// whitespace is trimmed, matching tabulate.
func writeTableRow(b *strings.Builder, cells []string, widths []int) {
	line := make([]string, len(cells))
	for i, c := range cells {
		if i == len(cells)-1 {
			line[i] = c
			continue
		}
		line[i] = c + strings.Repeat(" ", widths[i]-len(c))
	}
	b.WriteString(strings.TrimRight(strings.Join(line, "  "), " "))
	b.WriteByte('\n')
}

// isTableScalar reports whether knack would keep this value as a column. It
// drops nulls and anything non-scalar.
func isTableScalar(v interface{}) bool {
	switch v.(type) {
	case nil:
		return false
	case map[string]interface{}, []interface{}:
		return false
	}
	return true
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/output/ -v`
Expected: PASS, including the pre-existing `TestPrintFormatted_UnsupportedFormat`. If that test asserted `table` was rejected, update it — `table` is now supported, and that is the point of this task.

- [ ] **Step 5: Verify the file stayed gofmt-clean**

Run: `gofmt -l pkg/output/`
Expected: no output. If it lists a file, the new code used spaces instead of tabs.

- [ ] **Step 6: Commit**

```bash
git add pkg/output/output.go pkg/output/output_test.go
git commit -m "feat(output): support -o table with azure-cli's generic formatter

output.go already advertised 'table' in its unsupported-format error while
rejecting it. Implement it the way azure-cli actually does: no lock command
registers a table_transformer, so knack derives columns from the data — skip
id/type/etag, drop null and non-scalar values, sort keys alphabetically, and
uppercase the first character for the header.

Commands that hand-render tables (role, quota, pim, disk) map table to json
before reaching PrintFormatted, so they are unaffected."
```

---

### Task 6: Client, command groups, registration, and `list`

This task ends with `az lock list` working end to end across all four groups.

`commands.go` wires **only `list`** for now; Tasks 7-10 each append their verb to `newVerbCmds` as they create it. That keeps every task's build green — wiring all five constructors up front would break the build until Task 10.

`--filter-string` is passed verbatim to the SDK with no validation.

**ARM's list is cumulative** — it returns locks at the requested scope *and every ancestor scope*, so `az lock list -g mygroup` includes subscription locks. There is no client-side filtering; `--filter-string "atScope()"` is the only escape. Say so in the flag help.

**Files:**
- Create: `internal/lock/client.go`, `internal/lock/commands.go`, `internal/lock/list.go`
- Modify: `cmd/az/main.go`, `internal/account/commands.go:64`, `internal/group/commands.go:65`, `internal/resource/commands.go:15-25`

**Interfaces:**
- Consumes: `resolveScope` (Task 3), `toLockRecords` (Task 1)
- Produces: `newLocksClient(*cobra.Command) (*armlocks.ManagementLocksClient, error)`; `NewLockCommand()`, `NewAccountLockCommand()`, `NewGroupLockCommand()`, `NewResourceLockCommand()`; `newVerbCmds(scopeKind) []*cobra.Command`; `newListCmd(scopeKind) *cobra.Command`

- [ ] **Step 1: Write client.go**

```go
package lock

import (
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

// resolveSubscription returns the subscription ID from --subscription if set,
// otherwise from the default profile.
func resolveSubscription(cmd *cobra.Command) (string, error) {
  if sub, _ := cmd.Flags().GetString("subscription"); sub != "" {
    return sub, nil
  }
  return config.GetDefaultSubscription()
}

func newLocksClient(cmd *cobra.Command) (*armlocks.ManagementLocksClient, error) {
  cred, err := azure.GetCredential()
  if err != nil {
    return nil, err
  }
  sub, err := resolveSubscription(cmd)
  if err != nil {
    return nil, err
  }
  c, err := armlocks.NewManagementLocksClient(sub, cred, nil)
  if err != nil {
    return nil, fmt.Errorf("failed to create locks client: %w", err)
  }
  return c, nil
}
```

- [ ] **Step 2: Write commands.go**

The verbs are added in later tasks; this scaffolds the four groups. `SilenceUsage` is set because scope resolution has roughly a dozen validation errors and the repo otherwise dumps full usage on each.

```go
package lock

import (
  "github.com/spf13/cobra"
)

// newVerbCmds builds the lock verbs for a given group kind. The four azure-cli
// lock groups share one implementation and differ only in which scope flags
// each verb registers.
//
// Tasks 7-10 append newShowCmd, newDeleteCmd, newCreateCmd, and newUpdateCmd
// here as each is created.
func newVerbCmds(kind scopeKind) []*cobra.Command {
  return []*cobra.Command{
    newListCmd(kind),
  }
}

func newGroupCmd(use, short string, kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:          use,
    Short:        short,
    SilenceUsage: true,
  }
  cmd.AddCommand(newVerbCmds(kind)...)
  return cmd
}

// NewLockCommand returns the root `az lock` cobra command.
func NewLockCommand() *cobra.Command {
  return newGroupCmd("lock", "Manage Azure locks", kindGeneric)
}

// NewAccountLockCommand returns `az account lock`.
func NewAccountLockCommand() *cobra.Command {
  return newGroupCmd("lock", "Manage Azure subscription level locks", kindAccount)
}

// NewGroupLockCommand returns `az group lock`.
func NewGroupLockCommand() *cobra.Command {
  return newGroupCmd("lock", "Manage Azure resource group locks", kindGroup)
}

// NewResourceLockCommand returns `az resource lock`.
func NewResourceLockCommand() *cobra.Command {
  return newGroupCmd("lock", "Manage Azure resource level locks", kindResource)
}
```

- [ ] **Step 3: Register all four groups**

In `cmd/az/main.go`, add the import `"github.com/cdobbyn/azure-go-cli/internal/lock"` and insert into the `rootCmd.AddCommand(...)` block after `identity.NewIdentityCommand(),`:

```go
		lock.NewLockCommand(),
```

In `internal/account/commands.go:64`, add `lock.NewAccountLockCommand()`:

```go
	cmd.AddCommand(listCmd, showCmd, setCmd, clearCmd, getAccessTokenCmd, lock.NewAccountLockCommand())
```

In `internal/group/commands.go:65`:

```go
	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, lock.NewGroupLockCommand())
```

In `internal/resource/commands.go`, add `lock.NewResourceLockCommand(),` to the `cmd.AddCommand(...)` list.

Each of those three files needs the `internal/lock` import. There is no import cycle: `internal/lock` imports none of them.

- [ ] **Step 4: Write list.go**

`--scope` is added in Task 10; this version deliberately has no `--scope` branch.

```go
package lock

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newListCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List lock information",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runList(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  cmd.Flags().String("filter-string", "", `A query filter to restrict the results. ARM returns locks at the given scope AND all ancestor scopes; pass "atScope()" to list only locks at this scope exactly`)
  if kind == kindGroup {
    _ = cmd.MarkFlagRequired("resource-group")
  }
  return cmd
}

func runList(cmd *cobra.Command) error {
  ctx := context.Background()
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }
  var filter *string
  if f, _ := cmd.Flags().GetString("filter-string"); f != "" {
    filter = &f
  }

  s, err := resolveScope(cmd)
  if err != nil {
    return err
  }

  var objs []*armlocks.ManagementLockObject
  switch s.Level {
  case scopeResourceGroup:
    p := client.NewListAtResourceGroupLevelPager(s.ResourceGroup, &armlocks.ManagementLocksClientListAtResourceGroupLevelOptions{Filter: filter})
    for p.More() {
      page, err := p.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list locks: %w", err)
      }
      objs = append(objs, page.Value...)
    }
  case scopeResource:
    p := client.NewListAtResourceLevelPager(s.ResourceGroup, s.Namespace, s.Parent, s.ResourceType, s.ResourceName, &armlocks.ManagementLocksClientListAtResourceLevelOptions{Filter: filter})
    for p.More() {
      page, err := p.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list locks: %w", err)
      }
      objs = append(objs, page.Value...)
    }
  default:
    p := client.NewListAtSubscriptionLevelPager(&armlocks.ManagementLocksClientListAtSubscriptionLevelOptions{Filter: filter})
    for p.More() {
      page, err := p.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list locks: %w", err)
      }
      objs = append(objs, page.Value...)
    }
  }

  format, _ := cmd.Flags().GetString("output")
  return output.PrintFormatted(cmd, toLockRecords(objs), format)
}
```

- [ ] **Step 5: Build and check the help**

```bash
make build
./bin/az/az lock list --help
./bin/az/az group lock list --help
./bin/az/az account lock list --help
```

Expected: build succeeds. `az lock list` shows `-g`, `--resource`, `--resource-type`, `--namespace`, `--parent`, `--filter-string`. `az group lock list` shows `-g` (required) and `--filter-string` only. `az account lock list` shows `--filter-string` only.

- [ ] **Step 6: Commit**

```bash
git add internal/lock/client.go internal/lock/commands.go internal/lock/list.go cmd/az/main.go internal/account/commands.go internal/group/commands.go internal/resource/commands.go
git commit -m "feat(lock): add locks client, command groups, and list

az lock, az account lock, az group lock, and az resource lock share one
implementation and differ only in which scope flags they register.

--filter-string is passed verbatim to ARM. Note that ARM's list is cumulative:
it returns locks at the requested scope and every ancestor scope, so listing a
resource group also returns subscription locks. atScope() is the only escape,
which the flag help says.

SilenceUsage is set on the lock groups: scope resolution has roughly a dozen
validation errors, and the repo otherwise prints full cobra usage alongside
each one."
```

---

### Task 7: `show` and `delete`

These share the `--ids` selector and the scope precheck. azure-cli's dynamic return shape (1 id → bare object, ≥2 → array) matches `internal/resource/show.go:54-57`.

**Files:**
- Create: `internal/lock/show.go`, `internal/lock/delete.go`, `internal/lock/ids.go`

**Interfaces:**
- Consumes: `parseLockID`, `validateScopeMatchesLock`, `resolveScope`, `toLockRecord`
- Produces: `newShowCmd(scopeKind)`, `newDeleteCmd(scopeKind)`, `addIDsFlag(*cobra.Command)`, `resolveTargets(*cobra.Command) ([]lockTarget, error)`, `lockTarget`, `runPrecheck(context.Context, *cobra.Command, lockScope, string) error`, `getLock(context.Context, *armlocks.ManagementLocksClient, lockTarget) (*armlocks.ManagementLockObject, error)`

- [ ] **Step 1: Write ids.go**

```go
package lock

import (
  "context"
  "fmt"

  "github.com/spf13/cobra"
)

// lockTarget is a single resolved lock operation target.
type lockTarget struct {
  Scope    lockScope
  LockName string
}

func addIDsFlag(cmd *cobra.Command) {
  cmd.Flags().StringSlice("ids", nil, "One or more lock IDs (space- or comma-separated). If supplied, no other resource arguments should be specified")
}

// resolveTargets turns --ids or the scope flags into one target per lock.
//
// azure-cli logs an error and exits 0 on an unparseable ID, silently swallowing
// typos. We return a real error instead. Spec divergence 3.
func resolveTargets(cmd *cobra.Command) ([]lockTarget, error) {
  ids, _ := cmd.Flags().GetStringSlice("ids")
  name, _ := cmd.Flags().GetString("name")

  if len(ids) > 0 {
    if name != "" {
      return nil, fmt.Errorf("cannot mix --ids with --name")
    }
    targets := make([]lockTarget, 0, len(ids))
    for _, id := range ids {
      parts, err := parseLockID(id)
      if err != nil {
        return nil, err
      }
      targets = append(targets, lockTarget{Scope: scopeFromIDParts(parts), LockName: parts.LockName})
    }
    return targets, nil
  }

  if name == "" {
    return nil, fmt.Errorf("--name is required when --ids is not given")
  }
  s, err := resolveScope(cmd)
  if err != nil {
    return nil, err
  }
  return []lockTarget{{Scope: s, LockName: name}}, nil
}

func scopeFromIDParts(p lockIDParts) lockScope {
  s := lockScope{
    ResourceGroup: p.ResourceGroup,
    Namespace:     p.Namespace,
    Parent:        p.Parent,
    ResourceType:  p.ResourceType,
    ResourceName:  p.ResourceName,
  }
  switch {
  case p.ResourceName != "":
    s.Level = scopeResource
  case p.ResourceGroup != "":
    s.Level = scopeResourceGroup
  default:
    s.Level = scopeSubscription
  }
  return s
}

// runPrecheck ports the listing half of _validate_lock_params_match_lock: list
// locks subscription-wide, and if exactly one matches by name, verify the
// user's scope flags against it. If the count is not exactly one, azure-cli
// performs no validation at all — match that.
//
// This costs a subscription-wide list on every show/delete/update and needs
// subscription-wide lock-read permission. That trade was made deliberately; see
// the spec.
func runPrecheck(ctx context.Context, cmd *cobra.Command, want lockScope, lockName string) error {
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }
  var matches []lockIDParts
  p := client.NewListAtSubscriptionLevelPager(nil)
  for p.More() {
    page, err := p.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("list locks: %w", err)
    }
    for _, o := range page.Value {
      if o == nil || o.Name == nil || *o.Name != lockName || o.ID == nil {
        continue
      }
      parts, err := parseLockID(*o.ID)
      if err != nil {
        continue
      }
      matches = append(matches, parts)
    }
  }
  if len(matches) != 1 {
    return nil
  }
  return validateScopeMatchesLock(want, matches[0], lockName)
}
```

- [ ] **Step 2: Write show.go**

```go
package lock

import (
  "context"
  "fmt"

  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newShowCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Show the properties of a lock",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runShow(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  addIDsFlag(cmd)
  cmd.Flags().StringP("name", "n", "", "Name of the lock")
  return cmd
}

func runShow(cmd *cobra.Command) error {
  ctx := context.Background()
  targets, err := resolveTargets(cmd)
  if err != nil {
    return err
  }
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }

  results := make([]lockRecord, 0, len(targets))
  for _, t := range targets {
    if err := runPrecheck(ctx, cmd, t.Scope, t.LockName); err != nil {
      return err
    }
    obj, err := getLock(ctx, client, t)
    if err != nil {
      return err
    }
    results = append(results, toLockRecord(obj))
  }

  format, _ := cmd.Flags().GetString("output")
  // azure-cli returns a bare object for one id and an array for several.
  if len(results) == 1 {
    return output.PrintFormatted(cmd, results[0], format)
  }
  return output.PrintFormatted(cmd, results, format)
}

func getLock(ctx context.Context, client *armlocks.ManagementLocksClient, t lockTarget) (*armlocks.ManagementLockObject, error) {
  switch t.Scope.Level {
  case scopeResourceGroup:
    resp, err := client.GetAtResourceGroupLevel(ctx, t.Scope.ResourceGroup, t.LockName, nil)
    if err != nil {
      return nil, fmt.Errorf("get lock %s: %w", t.LockName, err)
    }
    return &resp.ManagementLockObject, nil
  case scopeResource:
    resp, err := client.GetAtResourceLevel(ctx, t.Scope.ResourceGroup, t.Scope.Namespace, t.Scope.Parent, t.Scope.ResourceType, t.Scope.ResourceName, t.LockName, nil)
    if err != nil {
      return nil, fmt.Errorf("get lock %s: %w", t.LockName, err)
    }
    return &resp.ManagementLockObject, nil
  default:
    resp, err := client.GetAtSubscriptionLevel(ctx, t.LockName, nil)
    if err != nil {
      return nil, fmt.Errorf("get lock %s: %w", t.LockName, err)
    }
    return &resp.ManagementLockObject, nil
  }
}
```

Add the `armlocks` import.

- [ ] **Step 3: Write delete.go**

```go
package lock

import (
  "context"
  "fmt"

  "github.com/spf13/cobra"
)

func newDeleteCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a lock",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runDelete(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  addIDsFlag(cmd)
  cmd.Flags().StringP("name", "n", "", "Name of the lock")
  return cmd
}

func runDelete(cmd *cobra.Command) error {
  ctx := context.Background()
  targets, err := resolveTargets(cmd)
  if err != nil {
    return err
  }
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }
  for _, t := range targets {
    if err := runPrecheck(ctx, cmd, t.Scope, t.LockName); err != nil {
      return err
    }
    switch t.Scope.Level {
    case scopeResourceGroup:
      _, err = client.DeleteAtResourceGroupLevel(ctx, t.Scope.ResourceGroup, t.LockName, nil)
    case scopeResource:
      _, err = client.DeleteAtResourceLevel(ctx, t.Scope.ResourceGroup, t.Scope.Namespace, t.Scope.Parent, t.Scope.ResourceType, t.Scope.ResourceName, t.LockName, nil)
    default:
      _, err = client.DeleteAtSubscriptionLevel(ctx, t.LockName, nil)
    }
    if err != nil {
      return fmt.Errorf("delete lock %s: %w", t.LockName, err)
    }
  }
  // azure-cli's delete returns no body.
  return nil
}
```

- [ ] **Step 4: Wire both verbs into commands.go**

In `internal/lock/commands.go`, extend `newVerbCmds`:

```go
func newVerbCmds(kind scopeKind) []*cobra.Command {
  return []*cobra.Command{
    newDeleteCmd(kind),
    newListCmd(kind),
    newShowCmd(kind),
  }
}
```

- [ ] **Step 5: Build and check the help**

```bash
make build
./bin/az/az lock show --help
./bin/az/az account lock show --help
```

Expected: build succeeds. `az lock show` shows `--ids`, `-n`, and all five scope flags, and **no** `--notes`. `az account lock show` shows only `--ids` and `-n`.

- [ ] **Step 6: Commit**

```bash
git add internal/lock/ids.go internal/lock/show.go internal/lock/delete.go
git commit -m "feat(lock): add lock show and delete

Both take --ids and run the scope precheck. The dynamic return shape matches
azure-cli: a bare object for one id, an array for several.

Diverges from azure-cli on one point: an unparseable ID returns an error rather
than logging and exiting 0, which silently swallows typos."
```

Stage `internal/lock/commands.go` with this commit too.

---

### Task 8: `create`

`create` is really create-or-update: the same name at the same scope silently overwrites, and `create` deliberately does **not** run the precheck (matching azure-cli).

**Files:**
- Create: `internal/lock/create.go`

**Interfaces:**
- Produces: `newCreateCmd(scopeKind)`, `parseLockLevel(string) (armlocks.LockLevel, error)`, `buildLockParams(level, notes) armlocks.ManagementLockObject`

- [ ] **Step 1: Add a lock-level test**

Append to `internal/lock/scope_test.go`:

```go
func TestParseLockLevel(t *testing.T) {
  tests := []struct {
    in      string
    want    string
    wantErr bool
  }{
    {in: "CanNotDelete", want: "CanNotDelete"},
    {in: "cannotdelete", want: "CanNotDelete"},
    {in: "READONLY", want: "ReadOnly"},
    {in: "ReadOnly", want: "ReadOnly"},
    {in: "NotSpecified", wantErr: true},
    {in: "bogus", wantErr: true},
    {in: "", wantErr: true},
  }
  for _, tt := range tests {
    t.Run(tt.in, func(t *testing.T) {
      got, err := parseLockLevel(tt.in)
      if (err != nil) != tt.wantErr {
        t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
      }
      if !tt.wantErr && string(got) != tt.want {
        t.Errorf("got %q want %q", got, tt.want)
      }
    })
  }
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/lock/ -run TestParseLockLevel -v`
Expected: FAIL — `undefined: parseLockLevel`.

- [ ] **Step 3: Write create.go**

```go
package lock

import (
  "context"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

// parseLockLevel accepts a lock type case-insensitively and returns the
// canonical value, matching azure-cli. NotSpecified exists in the SDK but
// azure-cli narrows it out, so reject it.
func parseLockLevel(s string) (armlocks.LockLevel, error) {
  switch {
  case strings.EqualFold(s, string(armlocks.LockLevelCanNotDelete)):
    return armlocks.LockLevelCanNotDelete, nil
  case strings.EqualFold(s, string(armlocks.LockLevelReadOnly)):
    return armlocks.LockLevelReadOnly, nil
  }
  return "", fmt.Errorf("invalid --lock-type %q (use CanNotDelete or ReadOnly)", s)
}

func newCreateCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "create",
    Short: "Create a lock",
    Long:  "Create a lock. Locks can exist at three different scopes: subscription, resource group and resource.",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runCreate(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  cmd.Flags().StringP("name", "n", "", "Name of the lock")
  cmd.Flags().StringP("lock-type", "t", "", "The type of lock restriction. Allowed values: CanNotDelete, ReadOnly")
  cmd.Flags().String("notes", "", "Notes about this lock")
  _ = cmd.MarkFlagRequired("name")
  _ = cmd.MarkFlagRequired("lock-type")
  if kind == kindGroup {
    _ = cmd.MarkFlagRequired("resource-group")
  }
  return cmd
}

func runCreate(cmd *cobra.Command) error {
  ctx := context.Background()
  name, _ := cmd.Flags().GetString("name")
  lockType, _ := cmd.Flags().GetString("lock-type")
  level, err := parseLockLevel(lockType)
  if err != nil {
    return err
  }
  s, err := resolveScope(cmd)
  if err != nil {
    return err
  }
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }

  params := armlocks.ManagementLockObject{
    Properties: &armlocks.ManagementLockProperties{Level: &level},
  }
  if cmd.Flags().Changed("notes") {
    notes, _ := cmd.Flags().GetString("notes")
    params.Properties.Notes = &notes
  }

  // create is really create-or-update: an existing lock with the same name at
  // the same scope is silently overwritten. azure-cli behaves the same way, and
  // deliberately runs no precheck here.
  var obj *armlocks.ManagementLockObject
  switch s.Level {
  case scopeResourceGroup:
    resp, err := client.CreateOrUpdateAtResourceGroupLevel(ctx, s.ResourceGroup, name, params, nil)
    if err != nil {
      return fmt.Errorf("create lock %s: %w", name, err)
    }
    obj = &resp.ManagementLockObject
  case scopeResource:
    resp, err := client.CreateOrUpdateAtResourceLevel(ctx, s.ResourceGroup, s.Namespace, s.Parent, s.ResourceType, s.ResourceName, name, params, nil)
    if err != nil {
      return fmt.Errorf("create lock %s: %w", name, err)
    }
    obj = &resp.ManagementLockObject
  default:
    resp, err := client.CreateOrUpdateAtSubscriptionLevel(ctx, name, params, nil)
    if err != nil {
      return fmt.Errorf("create lock %s: %w", name, err)
    }
    obj = &resp.ManagementLockObject
  }

  format, _ := cmd.Flags().GetString("output")
  return output.PrintFormatted(cmd, toLockRecord(obj), format)
}
```

- [ ] **Step 4: Wire create into commands.go**

Add `newCreateCmd(kind),` to the slice returned by `newVerbCmds` in `internal/lock/commands.go`, keeping the list alphabetical.

- [ ] **Step 5: Run tests and build**

```bash
go test ./internal/lock/ -v
make build
./bin/az/az lock create --help
./bin/az/az group lock create --help
```

Expected: tests PASS. `az lock create` marks `-n` and `-t` required and has **no** `--ids`. `az group lock create` additionally marks `-g` required.

- [ ] **Step 6: Commit**

```bash
git add internal/lock/create.go internal/lock/scope_test.go internal/lock/commands.go
git commit -m "feat(lock): add lock create

Lock types are accepted case-insensitively and emitted canonically, matching
azure-cli. NotSpecified exists in the SDK but azure-cli narrows it out, so it is
rejected.

create is create-or-update: an existing lock with the same name at the same
scope is silently overwritten, and no precheck runs. Both match azure-cli."
```

---

### Task 9: `update`

`update` is a read-modify-write partial merge. **`--notes ""` clears the notes; omitting `--notes` preserves them.** That requires `Flags().Changed(...)`, not `notes != ""`. Same for `--lock-type`. Getting this wrong makes notes unclearable.

**Files:**
- Create: `internal/lock/update.go`

**Interfaces:**
- Consumes: `resolveTargets`, `runPrecheck`, `getLock`, `parseLockLevel`
- Produces: `newUpdateCmd(scopeKind)`

- [ ] **Step 1: Write update.go**

```go
package lock

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newUpdateCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "update",
    Short: "Update a lock",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runUpdate(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  addIDsFlag(cmd)
  cmd.Flags().StringP("name", "n", "", "Name of the lock")
  cmd.Flags().StringP("lock-type", "t", "", "The type of lock restriction. Allowed values: CanNotDelete, ReadOnly")
  cmd.Flags().String("notes", "", "Notes about this lock")
  return cmd
}

func runUpdate(cmd *cobra.Command) error {
  ctx := context.Background()
  targets, err := resolveTargets(cmd)
  if err != nil {
    return err
  }
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }

  // A read-modify-write partial merge: only flags the user actually passed are
  // applied. Test Changed(), not emptiness — `--notes ""` clears the notes
  // while omitting --notes preserves them, and those must stay distinct.
  var newLevel *armlocks.LockLevel
  if cmd.Flags().Changed("lock-type") {
    lockType, _ := cmd.Flags().GetString("lock-type")
    lvl, err := parseLockLevel(lockType)
    if err != nil {
      return err
    }
    newLevel = &lvl
  }
  var newNotes *string
  if cmd.Flags().Changed("notes") {
    notes, _ := cmd.Flags().GetString("notes")
    newNotes = &notes
  }

  results := make([]lockRecord, 0, len(targets))
  for _, t := range targets {
    if err := runPrecheck(ctx, cmd, t.Scope, t.LockName); err != nil {
      return err
    }
    existing, err := getLock(ctx, client, t)
    if err != nil {
      return err
    }
    params := armlocks.ManagementLockObject{Properties: &armlocks.ManagementLockProperties{}}
    if existing.Properties != nil {
      params.Properties.Level = existing.Properties.Level
      params.Properties.Notes = existing.Properties.Notes
      params.Properties.Owners = existing.Properties.Owners
    }
    if newLevel != nil {
      params.Properties.Level = newLevel
    }
    if newNotes != nil {
      params.Properties.Notes = newNotes
    }
    if params.Properties.Level == nil {
      return fmt.Errorf("lock %s has no level; --lock-type is required", t.LockName)
    }

    var obj *armlocks.ManagementLockObject
    switch t.Scope.Level {
    case scopeResourceGroup:
      resp, err := client.CreateOrUpdateAtResourceGroupLevel(ctx, t.Scope.ResourceGroup, t.LockName, params, nil)
      if err != nil {
        return fmt.Errorf("update lock %s: %w", t.LockName, err)
      }
      obj = &resp.ManagementLockObject
    case scopeResource:
      resp, err := client.CreateOrUpdateAtResourceLevel(ctx, t.Scope.ResourceGroup, t.Scope.Namespace, t.Scope.Parent, t.Scope.ResourceType, t.Scope.ResourceName, t.LockName, params, nil)
      if err != nil {
        return fmt.Errorf("update lock %s: %w", t.LockName, err)
      }
      obj = &resp.ManagementLockObject
    default:
      resp, err := client.CreateOrUpdateAtSubscriptionLevel(ctx, t.LockName, params, nil)
      if err != nil {
        return fmt.Errorf("update lock %s: %w", t.LockName, err)
      }
      obj = &resp.ManagementLockObject
    }
    results = append(results, toLockRecord(obj))
  }

  format, _ := cmd.Flags().GetString("output")
  if len(results) == 1 {
    return output.PrintFormatted(cmd, results[0], format)
  }
  return output.PrintFormatted(cmd, results, format)
}
```

- [ ] **Step 2: Wire update into commands.go**

Add `newUpdateCmd(kind),` to `newVerbCmds` in `internal/lock/commands.go`. All five verbs are now wired:

```go
func newVerbCmds(kind scopeKind) []*cobra.Command {
  return []*cobra.Command{
    newCreateCmd(kind),
    newDeleteCmd(kind),
    newListCmd(kind),
    newShowCmd(kind),
    newUpdateCmd(kind),
  }
}
```

- [ ] **Step 3: Build and check**

```bash
make build
./bin/az/az lock update --help
```

Expected: shows `--ids`, `-t`, `-n`, `--notes`, and the scope flags, none marked required.

- [ ] **Step 4: Commit**

```bash
git add internal/lock/update.go internal/lock/commands.go
git commit -m "feat(lock): add lock update

A read-modify-write partial merge matching azure-cli: only flags the user
actually passed are applied, and omitted fields are preserved.

Uses Flags().Changed rather than testing for an empty string, because
`--notes \"\"` (clear the notes) and omitting --notes (preserve them) are
different operations. Testing emptiness would make notes unclearable."
```

---

### Task 10: `--scope`

An additive divergence: `--scope` bypasses the whole validator tree and routes to the SDK's `*ByScope` family, which also reaches management groups — something azure-cli cannot do. Every azure-cli invocation keeps working.

**Files:**
- Create: `internal/lock/scope_shortcut.go`
- Modify: `internal/lock/list.go`, `internal/lock/show.go`, `internal/lock/create.go`, `internal/lock/delete.go`, `internal/lock/update.go`

**Interfaces:**
- Produces: `addScopeShortcutFlag(*cobra.Command)`; each verb gains a `--scope` branch

- [ ] **Step 1: Write scope_shortcut.go**

```go
package lock

import (
  "github.com/spf13/cobra"
)

// addScopeShortcutFlag registers --scope.
//
// This has no azure-cli equivalent. The SDK's *ByScope family collapses all
// three scope levels into one call and additionally reaches management groups,
// which az lock cannot. It is purely additive: --scope bypasses the scope
// validator entirely, and every azure-cli invocation keeps working untouched.
func addScopeShortcutFlag(cmd *cobra.Command) {
  cmd.Flags().String("scope", "", "Full scope to lock (e.g. /subscriptions/{id}, or /providers/Microsoft.Management/managementGroups/{id}). Bypasses the other scope flags")
}
```

- [ ] **Step 2: Wire --scope into each verb**

In each of the five verbs, call `addScopeShortcutFlag(cmd)` in the constructor, and branch at the top of the runner before `resolveScope`:

```go
  if scope, _ := cmd.Flags().GetString("scope"); scope != "" {
    // ... call the *ByScope method
  }
```

For `create`:

```go
  if scope, _ := cmd.Flags().GetString("scope"); scope != "" {
    resp, err := client.CreateOrUpdateByScope(ctx, scope, name, params, nil)
    if err != nil {
      return fmt.Errorf("create lock %s: %w", name, err)
    }
    format, _ := cmd.Flags().GetString("output")
    return output.PrintFormatted(cmd, toLockRecord(&resp.ManagementLockObject), format)
  }
```

For `show`: `client.GetByScope(ctx, scope, name, nil)`. For `delete`: `client.DeleteByScope(ctx, scope, name, nil)`. For `list`: `client.NewListByScopePager(scope, &armlocks.ManagementLocksClientListByScopeOptions{Filter: filter})`. For `update`: `GetByScope` then `CreateOrUpdateByScope`.

`--scope` and `--ids` are mutually exclusive; return `cannot mix --scope with --ids` when both are set.

- [ ] **Step 3: Build and check**

```bash
make build
./bin/az/az lock create --help | grep -- --scope
```

Expected: the `--scope` line appears.

- [ ] **Step 4: Commit**

```bash
git add internal/lock/scope_shortcut.go internal/lock/list.go internal/lock/show.go internal/lock/create.go internal/lock/delete.go internal/lock/update.go
git commit -m "feat(lock): add --scope

No azure-cli equivalent. The SDK's ByScope client family collapses all three
scope levels into one call and additionally reaches management groups, which az
lock cannot express. Purely additive: --scope bypasses the scope validator and
every azure-cli invocation is untouched."
```

---

### Task 11: Verify end to end and refresh command coverage

**Files:**
- Modify: `docs/implemented-commands.txt`, `docs/missing-commands.txt`, `docs/command-tree.md`, `docs/official-az-commands.txt` (all regenerated, never hand-edited)

- [ ] **Step 1: Full test and build**

```bash
make test
make build
```

Expected: both exit 0.

- [ ] **Step 2: Verify every group's help renders**

```bash
for g in "lock" "account lock" "group lock" "resource lock"; do
  for v in create delete list show update; do
    ./bin/az/az $g $v --help > /dev/null || echo "FAILED: az $g $v"
  done
done
```

Expected: no output.

- [ ] **Step 3: Compare the flag surface against the real CLI**

For each group and verb, diff our flags against the official CLI's. The official image is already pulled.

```bash
docker run --rm mcr.microsoft.com/azure-cli:latest az lock create --help 2>/dev/null \
  | awk '/^Arguments/{f=1;next} /^Global Arguments/{f=0} f' | grep -oE '^ +--[a-z-]+' | tr -d ' ' | sort > /tmp/az-flags.txt
./bin/az/az lock create --help | grep -oE '^ +--[a-z-]+' | tr -d ' ' | sort > /tmp/go-flags.txt
diff /tmp/az-flags.txt /tmp/go-flags.txt
```

Expected: the only difference is `--scope`, present in ours by design. Any other difference is a bug — fix it before continuing.

- [ ] **Step 4: Regenerate the coverage docs**

```bash
./scripts/audit-commands.sh
```

Takes 5-10 minutes and needs Docker. Expected: the 24 lock entries move from `docs/missing-commands.txt` to `docs/implemented-commands.txt`.

Verify:

```bash
grep -c "lock" docs/implemented-commands.txt   # expect 24
grep -c "^az lock" docs/missing-commands.txt   # expect 0
```

- [ ] **Step 5: Commit**

```bash
git add docs/implemented-commands.txt docs/missing-commands.txt docs/command-tree.md docs/official-az-commands.txt
git commit -m "docs: refresh command coverage after az lock port

Regenerated with ./scripts/audit-commands.sh. Moves the 24 lock entries across
all four groups from missing to implemented."
```

- [ ] **Step 6: Verify against a live subscription (optional, recommended)**

Nothing in this plan exercises a real ARM call — the repo has no network tests, so every verb is unverified against the live API. Before merging, drive one round trip against a throwaway resource group:

```bash
./bin/az/az group create -n lock-test-rg -l eastus
./bin/az/az lock create -n testlock -g lock-test-rg -t CanNotDelete --notes "temp"
./bin/az/az lock show -n testlock -g lock-test-rg
./bin/az/az lock list -g lock-test-rg -o table
./bin/az/az lock list -g lock-test-rg --filter-string "atScope()" -o table
./bin/az/az lock update -n testlock -g lock-test-rg --notes ""
./bin/az/az lock show -n testlock -g lock-test-rg --query notes
./bin/az/az lock delete -n testlock -g lock-test-rg
./bin/az/az group delete -n lock-test-rg --yes
```

Confirm: the table shows `Level Name Notes ResourceGroup`; the `atScope()` list excludes subscription-level locks while the unfiltered one includes them; and `--notes ""` actually clears the notes (the `--query notes` returns `""`, not `temp`).

---

## Notes for the implementer

- **The single highest-risk mistake** is forgetting that `record.go` exists and passing raw `armlocks.ManagementLockObject` values to `output.PrintFormatted`. That nests everything under `properties` and breaks every azure-cli `--query`. If a `--query "[].level"` returns null, this is why.
- **Second highest** is writing `if notes != ""` in `update`. That silently makes notes unclearable. It must be `Flags().Changed("notes")`.
- Do not add `systemData` to `lockRecord` even though the SDK model has it — azure-cli pins an API version that has no such field.
- Do not anchor the lock-ID regex with `$`.
- Every deliberate divergence from azure-cli is listed in the spec's "Divergences" section. If you find yourself adding a new one, it belongs there too.
