# `az resource` Command Group Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the `az resource` command group with all 9 generic subcommands (`list`, `show`, `delete`, `tag`, `move`, `wait`, `create`, `update`, `invoke-action`), matching Python `az resource` flag and JSON-output parity, implemented via the Go SDK.

**Architecture:** New package `internal/resource/` wires cobra subcommands; each subcommand file calls `armresources.Client` / `armresources.TagsClient` from `azure-sdk-for-go`. Two new shared packages: `pkg/azure/apiversion.go` (resolves latest API version from `Microsoft.Resources` provider) and `pkg/genericupdate/` (parses `--set`/`--add`/`--remove` path expressions). `invoke-action` uses raw `azcore/arm` pipeline since no SDK helper exists.

**Tech Stack:** Go 1.25, `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources` v1.2.0, `github.com/Azure/azure-sdk-for-go/sdk/azcore` v1.19, `github.com/spf13/cobra` v1.10, `github.com/jmespath/go-jmespath` (already a dep).

**Reference design:** `docs/superpowers/specs/2026-05-05-az-resource-design.md`

**Conventions reminder (from CLAUDE.md):**
- Build via `make build` (outputs to `bin/az/az`); never `go build` directly.
- Tests via `make test` (`go test -v ./...`).
- Conventional commits: `feat:` for new subcommand functionality, `chore:`/`refactor:` for scaffolding/internals that aren't user-facing features. The whole feature lands as multiple `feat:` commits — that's fine, semantic-release groups them.
- 2-space indentation, LF line endings, trailing newline.

---

## File Structure

```
internal/resource/
├── commands.go         # cobra wiring for all 9 subcommands (Task 1, grows each task)
├── client.go           # newGenericClient, newTagsClient helpers (Task 1)
├── resolve.go          # ParseResourceID, BuildResourceID, ResolveSelector (Tasks 2-4)
├── resolve_test.go     # tests for resolve.go (Tasks 2-4)
├── list.go             # Task 8
├── show.go             # Task 9
├── delete.go           # Task 10
├── tag.go              # Task 11
├── move.go             # Task 12
├── wait.go             # Task 13
├── create.go           # Task 14
├── update.go           # Task 15
└── invoke_action.go    # Task 16

pkg/azure/
├── apiversion.go       # ResolveAPIVersion + selectLatest (Task 5)
└── apiversion_test.go  # tests for selectLatest (Task 5)

pkg/genericupdate/
├── genericupdate.go    # Op, Apply (Tasks 6-7)
└── genericupdate_test.go (Tasks 6-7)

cmd/az/main.go          # add one import + one rootCmd.AddCommand entry (Task 1)
```

---

## Task 1: Package skeleton, cobra wiring, registration

**Goal:** `make build` produces a binary that responds to `./bin/az/az resource --help` listing all 9 subcommands as stubs that print "not yet implemented".

**Files:**
- Create: `internal/resource/commands.go`
- Create: `internal/resource/client.go`
- Modify: `cmd/az/main.go` (add import + register)

- [ ] **Step 1: Create `internal/resource/commands.go`**

```go
package resource

import (
  "fmt"

  "github.com/spf13/cobra"
)

// NewResourceCommand returns the root `az resource` cobra command.
func NewResourceCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "resource",
    Short: "Manage Azure resources generically",
    Long:  "Generic ARM resource access (list, show, delete, tag, move, wait, create, update, invoke-action)",
  }

  cmd.AddCommand(
    newListCmd(),
    newShowCmd(),
    newDeleteCmd(),
    newTagCmd(),
    newMoveCmd(),
    newWaitCmd(),
    newCreateCmd(),
    newUpdateCmd(),
    newInvokeActionCmd(),
  )
  return cmd
}

// stub helper used by the per-subcommand files until each is implemented.
func notImplemented(name string) func(cmd *cobra.Command, args []string) error {
  return func(cmd *cobra.Command, args []string) error {
    return fmt.Errorf("az resource %s: not yet implemented", name)
  }
}
```

- [ ] **Step 2: Create `internal/resource/client.go`**

```go
package resource

import (
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
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

func newGenericClient(cmd *cobra.Command) (*armresources.Client, azcore.TokenCredential, string, error) {
  cred, err := azure.GetCredential()
  if err != nil {
    return nil, nil, "", err
  }
  sub, err := resolveSubscription(cmd)
  if err != nil {
    return nil, nil, "", err
  }
  c, err := armresources.NewClient(sub, cred, nil)
  if err != nil {
    return nil, nil, "", fmt.Errorf("failed to create resources client: %w", err)
  }
  return c, cred, sub, nil
}

func newTagsClient(cmd *cobra.Command) (*armresources.TagsClient, error) {
  cred, err := azure.GetCredential()
  if err != nil {
    return nil, err
  }
  sub, err := resolveSubscription(cmd)
  if err != nil {
    return nil, err
  }
  c, err := armresources.NewTagsClient(sub, cred, nil)
  if err != nil {
    return nil, fmt.Errorf("failed to create tags client: %w", err)
  }
  return c, nil
}
```

- [ ] **Step 3: Create stub files for every subcommand**

Create each of the following files with a single `newXCmd()` constructor returning a cobra command whose `RunE` is `notImplemented("X")`. This lets `commands.go` compile in step 1 even before subcommands have real bodies. For each file, the stub looks like:

`internal/resource/list.go`:
```go
package resource

import "github.com/spf13/cobra"

func newListCmd() *cobra.Command {
  return &cobra.Command{Use: "list", Short: "List resources", RunE: notImplemented("list")}
}
```

Repeat verbatim for `show.go`, `delete.go`, `tag.go`, `move.go`, `wait.go`, `create.go`, `update.go`, `invoke_action.go` — each with its own `Use`, `Short`, and `notImplemented` name. The constructor names match what `commands.go` calls: `newShowCmd`, `newDeleteCmd`, `newTagCmd`, `newMoveCmd`, `newWaitCmd`, `newCreateCmd`, `newUpdateCmd`, `newInvokeActionCmd`.

For `newInvokeActionCmd`, `Use` is `"invoke-action"`.

- [ ] **Step 4: Register in `cmd/az/main.go`**

Add the import (alphabetically with other internal imports — between `quota` and `role`):

```go
"github.com/cdobbyn/azure-go-cli/internal/resource"
```

Add to the `rootCmd.AddCommand(...)` call (after `quota.NewQuotaCommand()`, before `role.NewRoleCmd()`):

```go
resource.NewResourceCommand(),
```

- [ ] **Step 5: Build and verify**

Run: `make build`
Expected: `Binary created: bin/az/az` with no errors.

Run: `./bin/az/az resource --help`
Expected output includes lines for each subcommand:
```
Available Commands:
  create
  delete
  invoke-action
  list
  move
  show
  tag
  update
  wait
```

Run: `./bin/az/az resource list`
Expected: `Error: az resource list: not yet implemented`

- [ ] **Step 6: Commit**

```bash
git add internal/resource cmd/az/main.go
git commit -m "feat(resource): add command group skeleton with stub subcommands"
```

---

## Task 2: `ParseResourceID` (TDD)

**Goal:** Pure function that splits an ARM resource ID into its components. Used by `ResolveSelector` (Task 4) and `move` validation (Task 12).

**Files:**
- Create: `internal/resource/resolve.go`
- Create: `internal/resource/resolve_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/resource/resolve_test.go`:
```go
package resource

import (
  "reflect"
  "testing"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/resource/...`
Expected: build error — `ParseResourceID` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/resource/resolve.go`:
```go
package resource

import (
  "fmt"
  "strings"
)

// ParseResourceID splits an ARM resource ID into its components.
// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/{ns}/{type}/{name}[/{type}/{name}...]
func ParseResourceID(id string) (sub, group, namespace string, types, names []string, err error) {
  if id == "" {
    return "", "", "", nil, nil, fmt.Errorf("resource ID is empty")
  }
  parts := strings.Split(strings.TrimPrefix(id, "/"), "/")
  if len(parts) < 8 || parts[0] != "subscriptions" || parts[2] != "resourceGroups" || parts[4] != "providers" {
    return "", "", "", nil, nil, fmt.Errorf("invalid resource ID: %s", id)
  }
  sub = parts[1]
  group = parts[3]
  namespace = parts[5]
  remainder := parts[6:]
  if len(remainder)%2 != 0 {
    return "", "", "", nil, nil, fmt.Errorf("invalid resource ID type/name pairing: %s", id)
  }
  for i := 0; i < len(remainder); i += 2 {
    types = append(types, remainder[i])
    names = append(names, remainder[i+1])
  }
  return sub, group, namespace, types, names, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/resource/... -run TestParseResourceID -v`
Expected: PASS for all four cases.

- [ ] **Step 5: Commit**

```bash
git add internal/resource/resolve.go internal/resource/resolve_test.go
git commit -m "feat(resource): add ParseResourceID helper"
```

---

## Task 3: `BuildResourceID` (TDD)

**Goal:** Inverse of `ParseResourceID`. Build canonical ID from name-mode flags.

**Files:**
- Modify: `internal/resource/resolve.go`
- Modify: `internal/resource/resolve_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/resource/resolve_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/resource/... -run TestBuildResourceID -v`
Expected: build error — `BuildResourceID` undefined.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/resource/resolve.go`:
```go
// BuildResourceID assembles an ARM resource ID from name-mode flag inputs.
// resourceType may be qualified ("Microsoft.X/y") or unqualified ("y") if namespace is given.
// parent is an optional "type/name[/type/name...]" prefix for child resources.
func BuildResourceID(sub, group, namespace, resourceType, parent, name string) (string, error) {
  if sub == "" || group == "" || resourceType == "" || name == "" {
    return "", fmt.Errorf("subscription, resource group, resource type, and name are all required")
  }

  ns := namespace
  rt := resourceType
  if strings.Contains(resourceType, "/") {
    // qualified: split first segment as namespace
    idx := strings.Index(resourceType, "/")
    ns = resourceType[:idx]
    rt = resourceType[idx+1:]
  }
  if ns == "" {
    return "", fmt.Errorf("namespace required when --resource-type is unqualified")
  }

  parentPart := ""
  if parent != "" {
    parentPart = "/" + strings.Trim(parent, "/")
  }

  return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/%s%s/%s/%s",
    sub, group, ns, parentPart, rt, name), nil
}
```

- [ ] **Step 4: Run tests to verify it passes**

Run: `go test ./internal/resource/... -v`
Expected: PASS for both `TestParseResourceID` and `TestBuildResourceID`.

- [ ] **Step 5: Commit**

```bash
git add internal/resource/resolve.go internal/resource/resolve_test.go
git commit -m "feat(resource): add BuildResourceID helper"
```

---

## Task 4: `ResolveSelector` (TDD)

**Goal:** Read `--ids` or name-mode flags from a cobra.Command and produce a slice of resource IDs. Validates mutual exclusion.

**Files:**
- Modify: `internal/resource/resolve.go`
- Modify: `internal/resource/resolve_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/resource/resolve_test.go`:
```go
import "github.com/spf13/cobra"  // add to existing imports

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/resource/... -run TestResolveSelector -v`
Expected: build error — `AddSelectorFlags`, `ResolveSelector` undefined.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/resource/resolve.go`:
```go
import (
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)
// ^ merge into the existing import block at the top of resolve.go

// AddSelectorFlags registers the resource-selector flag set on cmd.
// Used by every subcommand that operates on a specific resource.
func AddSelectorFlags(cmd *cobra.Command) {
  cmd.Flags().StringSlice("ids", nil, "One or more resource IDs (space- or comma-separated). If supplied, no other resource arguments should be specified.")
  cmd.Flags().StringP("name", "n", "", "Resource name. Required when --ids is not given.")
  cmd.Flags().StringP("resource-group", "g", "", "Resource group. Required when --ids is not given.")
  cmd.Flags().String("resource-type", "", "Resource type, qualified (Microsoft.Foo/bar) or unqualified with --namespace.")
  cmd.Flags().String("namespace", "", "Provider namespace, e.g. Microsoft.Network.")
  cmd.Flags().String("parent", "", "Parent path for child resources (e.g. virtualNetworks/myvnet).")
}

// ResolveSelector returns the resource IDs implied by the flags on cmd.
// Returns multiple IDs only when --ids was used.
func ResolveSelector(cmd *cobra.Command) ([]string, error) {
  ids, _ := cmd.Flags().GetStringSlice("ids")
  name, _ := cmd.Flags().GetString("name")
  group, _ := cmd.Flags().GetString("resource-group")
  rtype, _ := cmd.Flags().GetString("resource-type")
  namespace, _ := cmd.Flags().GetString("namespace")
  parent, _ := cmd.Flags().GetString("parent")

  hasIDs := len(ids) > 0
  hasName := name != "" || group != "" || rtype != ""

  if hasIDs && hasName {
    return nil, fmt.Errorf("cannot mix --ids with -g/--resource-type/-n")
  }
  if !hasIDs && !hasName {
    return nil, fmt.Errorf("please specify either --ids or both -g and resource info")
  }

  if hasIDs {
    return ids, nil
  }

  if name == "" || group == "" || rtype == "" {
    return nil, fmt.Errorf("--resource-group, --resource-type, and --name are all required when --ids is not given")
  }

  sub, _ := cmd.Flags().GetString("subscription")
  if sub == "" {
    var err error
    sub, err = config.GetDefaultSubscription()
    if err != nil {
      return nil, err
    }
  }

  id, err := BuildResourceID(sub, group, namespace, rtype, parent, name)
  if err != nil {
    return nil, err
  }
  return []string{id}, nil
}
```

- [ ] **Step 4: Run tests to verify all pass**

Run: `go test ./internal/resource/... -v`
Expected: PASS for all three test functions.

- [ ] **Step 5: Commit**

```bash
git add internal/resource/resolve.go internal/resource/resolve_test.go
git commit -m "feat(resource): add ResolveSelector and selector flags"
```

---

## Task 5: API version resolver (TDD pure helper)

**Goal:** Pick the latest API version from a provider response. The pure picker is unit-tested; the live caller is exercised via the CLI smoke test.

**Files:**
- Create: `pkg/azure/apiversion.go`
- Create: `pkg/azure/apiversion_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/azure/apiversion_test.go`:
```go
package azure

import "testing"

func TestSelectLatestAPIVersion(t *testing.T) {
  versions := []string{
    "2021-01-01",
    "2022-06-01-preview",
    "2023-04-01",
    "2024-01-01-preview",
  }

  t.Run("stable only", func(t *testing.T) {
    got, err := selectLatestAPIVersion(versions, false)
    if err != nil {
      t.Fatal(err)
    }
    if got != "2023-04-01" {
      t.Errorf("got %s want 2023-04-01", got)
    }
  })

  t.Run("include preview", func(t *testing.T) {
    got, err := selectLatestAPIVersion(versions, true)
    if err != nil {
      t.Fatal(err)
    }
    if got != "2024-01-01-preview" {
      t.Errorf("got %s want 2024-01-01-preview", got)
    }
  })

  t.Run("empty", func(t *testing.T) {
    if _, err := selectLatestAPIVersion(nil, false); err == nil {
      t.Error("expected error")
    }
  })

  t.Run("only preview, stable requested", func(t *testing.T) {
    if _, err := selectLatestAPIVersion([]string{"2024-01-01-preview"}, false); err == nil {
      t.Error("expected error when no stable version")
    }
  })
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/azure/... -run TestSelectLatestAPIVersion -v`
Expected: build error — `selectLatestAPIVersion` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `pkg/azure/apiversion.go`:
```go
package azure

import (
  "context"
  "fmt"
  "sort"
  "strings"
  "sync"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

var apiVersionCache sync.Map // key: subID|namespace|type|preview, value: string

// ResolveAPIVersion returns the API version to use for the given fully qualified
// resource type (e.g. "Microsoft.Network/virtualNetworks").
// If explicit is non-empty, returns it unchanged. Otherwise queries the provider
// and selects the latest stable version (or includes preview if includePreview).
func ResolveAPIVersion(ctx context.Context, cred azcore.TokenCredential, subID, namespace, resourceType, explicit string, includePreview bool) (string, error) {
  if explicit != "" {
    return explicit, nil
  }
  cacheKey := fmt.Sprintf("%s|%s|%s|%v", subID, namespace, resourceType, includePreview)
  if v, ok := apiVersionCache.Load(cacheKey); ok {
    return v.(string), nil
  }

  client, err := armresources.NewProvidersClient(subID, cred, nil)
  if err != nil {
    return "", fmt.Errorf("failed to create providers client: %w", err)
  }
  resp, err := client.Get(ctx, namespace, nil)
  if err != nil {
    return "", fmt.Errorf("failed to get provider %s: %w", namespace, err)
  }
  for _, rt := range resp.ResourceTypes {
    if rt.ResourceType != nil && strings.EqualFold(*rt.ResourceType, resourceType) {
      versions := make([]string, 0, len(rt.APIVersions))
      for _, v := range rt.APIVersions {
        if v != nil {
          versions = append(versions, *v)
        }
      }
      picked, err := selectLatestAPIVersion(versions, includePreview)
      if err != nil {
        return "", err
      }
      apiVersionCache.Store(cacheKey, picked)
      return picked, nil
    }
  }
  return "", fmt.Errorf("resource type %s not found under provider %s", resourceType, namespace)
}

// selectLatestAPIVersion picks the highest-sorting API version from versions.
// Preview versions are excluded unless includePreview is true. ARM API versions
// sort lexically by date, so reverse string sort gives the newest first.
func selectLatestAPIVersion(versions []string, includePreview bool) (string, error) {
  if len(versions) == 0 {
    return "", fmt.Errorf("no API versions available")
  }
  filtered := make([]string, 0, len(versions))
  for _, v := range versions {
    isPreview := strings.Contains(v, "-preview") || strings.Contains(v, "-beta")
    if isPreview && !includePreview {
      continue
    }
    filtered = append(filtered, v)
  }
  if len(filtered) == 0 {
    return "", fmt.Errorf("no stable API versions available")
  }
  sort.Sort(sort.Reverse(sort.StringSlice(filtered)))
  return filtered[0], nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/azure/... -v`
Expected: PASS for all four `TestSelectLatestAPIVersion` cases.

- [ ] **Step 5: Commit**

```bash
git add pkg/azure/apiversion.go pkg/azure/apiversion_test.go
git commit -m "feat(azure): add API version resolver"
```

---

## Task 6: `pkg/genericupdate` — `--set` (TDD)

**Goal:** Apply `--set path=value` mutations to a `map[string]interface{}` resource body.

**Files:**
- Create: `pkg/genericupdate/genericupdate.go`
- Create: `pkg/genericupdate/genericupdate_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/genericupdate/genericupdate_test.go`:
```go
package genericupdate

import (
  "encoding/json"
  "reflect"
  "testing"
)

func mustJSON(s string) map[string]interface{} {
  var m map[string]interface{}
  if err := json.Unmarshal([]byte(s), &m); err != nil {
    panic(err)
  }
  return m
}

func TestApplySet(t *testing.T) {
  t.Run("set top-level string", func(t *testing.T) {
    obj := mustJSON(`{"location":"eastus","tags":{}}`)
    err := Apply(obj, []Op{{Kind: Set, Path: "location", Value: "westus"}})
    if err != nil {
      t.Fatal(err)
    }
    if obj["location"] != "westus" {
      t.Errorf("got %v", obj["location"])
    }
  })

  t.Run("set nested path creates intermediate maps", func(t *testing.T) {
    obj := mustJSON(`{}`)
    err := Apply(obj, []Op{{Kind: Set, Path: "tags.env", Value: "prod"}})
    if err != nil {
      t.Fatal(err)
    }
    want := mustJSON(`{"tags":{"env":"prod"}}`)
    if !reflect.DeepEqual(obj, want) {
      t.Errorf("got %v want %v", obj, want)
    }
  })

  t.Run("set with JSON value", func(t *testing.T) {
    obj := mustJSON(`{}`)
    err := Apply(obj, []Op{{Kind: Set, Path: "properties.networkAcls", Value: `{"defaultAction":"Deny"}`}})
    if err != nil {
      t.Fatal(err)
    }
    nacl := obj["properties"].(map[string]interface{})["networkAcls"].(map[string]interface{})
    if nacl["defaultAction"] != "Deny" {
      t.Errorf("got %v", nacl)
    }
  })

  t.Run("set list element by index", func(t *testing.T) {
    obj := mustJSON(`{"properties":{"subnets":[{"name":"a"},{"name":"b"}]}}`)
    err := Apply(obj, []Op{{Kind: Set, Path: "properties.subnets[1].name", Value: "renamed"}})
    if err != nil {
      t.Fatal(err)
    }
    got := obj["properties"].(map[string]interface{})["subnets"].([]interface{})[1].(map[string]interface{})["name"]
    if got != "renamed" {
      t.Errorf("got %v", got)
    }
  })
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/genericupdate/... -v`
Expected: build error — package missing.

- [ ] **Step 3: Write minimal implementation**

Create `pkg/genericupdate/genericupdate.go`:
```go
// Package genericupdate implements Python az's generic update path syntax
// (--set, --add, --remove) against a map[string]interface{} resource body.
package genericupdate

import (
  "encoding/json"
  "fmt"
  "regexp"
  "strconv"
  "strings"
)

type OpKind int

const (
  Set OpKind = iota
  Add
  Remove
)

type Op struct {
  Kind  OpKind
  Path  string
  Value string // raw value as supplied on CLI; parsed per Kind
}

// Apply mutates obj per the slice of operations, in order.
func Apply(obj map[string]interface{}, ops []Op) error {
  for _, op := range ops {
    switch op.Kind {
    case Set:
      if err := applySet(obj, op.Path, op.Value); err != nil {
        return fmt.Errorf("--set %s: %w", op.Path, err)
      }
    case Add:
      if err := applyAdd(obj, op.Path, op.Value); err != nil {
        return fmt.Errorf("--add %s: %w", op.Path, err)
      }
    case Remove:
      if err := applyRemove(obj, op.Path, op.Value); err != nil {
        return fmt.Errorf("--remove %s: %w", op.Path, err)
      }
    }
  }
  return nil
}

// path = key("." key | "[" index "]")*
var indexRE = regexp.MustCompile(`^\[(\d+)\]`)

type segment struct {
  key   string // map key, empty if isIndex
  index int    // list index, -1 if not isIndex
  isIndex bool
}

func parsePath(path string) ([]segment, error) {
  if path == "" {
    return nil, fmt.Errorf("empty path")
  }
  segs := []segment{}
  for path != "" {
    if m := indexRE.FindStringSubmatch(path); m != nil {
      n, _ := strconv.Atoi(m[1])
      segs = append(segs, segment{index: n, isIndex: true})
      path = path[len(m[0]):]
      if strings.HasPrefix(path, ".") {
        path = path[1:]
      }
      continue
    }
    dot := strings.IndexAny(path, ".[")
    if dot == -1 {
      segs = append(segs, segment{key: path})
      path = ""
    } else {
      segs = append(segs, segment{key: path[:dot]})
      if path[dot] == '.' {
        path = path[dot+1:]
      } else {
        path = path[dot:]
      }
    }
  }
  return segs, nil
}

// parseValue tries to JSON-unmarshal value; falls back to a plain string.
func parseValue(value string) interface{} {
  var v interface{}
  if err := json.Unmarshal([]byte(value), &v); err == nil {
    return v
  }
  return value
}

func applySet(obj map[string]interface{}, path, value string) error {
  segs, err := parsePath(path)
  if err != nil {
    return err
  }
  parsed := parseValue(value)
  return setAtPath(obj, segs, parsed)
}

func setAtPath(root map[string]interface{}, segs []segment, value interface{}) error {
  if len(segs) == 0 {
    return fmt.Errorf("empty path")
  }
  // Navigate to parent of final segment, creating maps along the way.
  var cursor interface{} = root
  for i := 0; i < len(segs)-1; i++ {
    seg := segs[i]
    next := segs[i+1]
    if seg.isIndex {
      list, ok := cursor.([]interface{})
      if !ok {
        return fmt.Errorf("expected list at index %d", seg.index)
      }
      if seg.index < 0 || seg.index >= len(list) {
        return fmt.Errorf("index %d out of range", seg.index)
      }
      cursor = list[seg.index]
      continue
    }
    m, ok := cursor.(map[string]interface{})
    if !ok {
      return fmt.Errorf("expected map at key %q", seg.key)
    }
    if _, exists := m[seg.key]; !exists {
      // Auto-create intermediate map; if next is index we'd need a list,
      // but auto-creating lists is unsupported (matches Python behavior).
      if next.isIndex {
        return fmt.Errorf("cannot auto-create list at %q", seg.key)
      }
      m[seg.key] = map[string]interface{}{}
    }
    cursor = m[seg.key]
  }
  last := segs[len(segs)-1]
  if last.isIndex {
    list, ok := cursor.([]interface{})
    if !ok {
      return fmt.Errorf("expected list for index %d", last.index)
    }
    if last.index < 0 || last.index >= len(list) {
      return fmt.Errorf("index %d out of range", last.index)
    }
    list[last.index] = value
    return nil
  }
  m, ok := cursor.(map[string]interface{})
  if !ok {
    return fmt.Errorf("expected map for key %q", last.key)
  }
  m[last.key] = value
  return nil
}

// stubs filled in Task 7
func applyAdd(obj map[string]interface{}, path, value string) error    { return fmt.Errorf("not implemented") }
func applyRemove(obj map[string]interface{}, path, value string) error { return fmt.Errorf("not implemented") }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/genericupdate/... -v`
Expected: PASS for all four `TestApplySet` cases.

- [ ] **Step 5: Commit**

```bash
git add pkg/genericupdate/
git commit -m "feat(genericupdate): add Set operation with path parsing"
```

---

## Task 7: `pkg/genericupdate` — `--add` and `--remove` (TDD)

**Files:**
- Modify: `pkg/genericupdate/genericupdate.go`
- Modify: `pkg/genericupdate/genericupdate_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `pkg/genericupdate/genericupdate_test.go`:
```go
func TestApplyAdd(t *testing.T) {
  t.Run("append JSON object to list", func(t *testing.T) {
    obj := mustJSON(`{"properties":{"subnets":[{"name":"a"}]}}`)
    err := Apply(obj, []Op{{Kind: Add, Path: "properties.subnets", Value: `{"name":"b"}`}})
    if err != nil {
      t.Fatal(err)
    }
    list := obj["properties"].(map[string]interface{})["subnets"].([]interface{})
    if len(list) != 2 {
      t.Fatalf("len %d", len(list))
    }
    if list[1].(map[string]interface{})["name"] != "b" {
      t.Errorf("got %v", list[1])
    }
  })

  t.Run("append to non-list errors", func(t *testing.T) {
    obj := mustJSON(`{"properties":{"name":"foo"}}`)
    err := Apply(obj, []Op{{Kind: Add, Path: "properties.name", Value: `"x"`}})
    if err == nil {
      t.Error("expected error")
    }
  })
}

func TestApplyRemove(t *testing.T) {
  t.Run("remove map key", func(t *testing.T) {
    obj := mustJSON(`{"tags":{"a":"1","b":"2"}}`)
    err := Apply(obj, []Op{{Kind: Remove, Path: "tags.a"}})
    if err != nil {
      t.Fatal(err)
    }
    if _, ok := obj["tags"].(map[string]interface{})["a"]; ok {
      t.Error("a should be removed")
    }
  })

  t.Run("remove list index", func(t *testing.T) {
    obj := mustJSON(`{"items":[1,2,3]}`)
    err := Apply(obj, []Op{{Kind: Remove, Path: "items", Value: "1"}})
    if err != nil {
      t.Fatal(err)
    }
    list := obj["items"].([]interface{})
    if len(list) != 2 || list[0].(float64) != 1 || list[1].(float64) != 3 {
      t.Errorf("got %v", list)
    }
  })
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/genericupdate/... -run "TestApplyAdd|TestApplyRemove" -v`
Expected: FAIL with "not implemented" errors.

- [ ] **Step 3: Replace the stubs**

In `pkg/genericupdate/genericupdate.go`, replace the two stub functions:
```go
func applyAdd(obj map[string]interface{}, path, value string) error {
  segs, err := parsePath(path)
  if err != nil {
    return err
  }
  parent, last, err := navigateToParent(obj, segs)
  if err != nil {
    return err
  }
  // last must point at a list inside parent.
  if last.isIndex {
    return fmt.Errorf("--add path must end at a map key, not an index")
  }
  m, ok := parent.(map[string]interface{})
  if !ok {
    return fmt.Errorf("expected map at parent of %q", last.key)
  }
  cur, ok := m[last.key].([]interface{})
  if !ok {
    return fmt.Errorf("path %q does not refer to a list", last.key)
  }
  m[last.key] = append(cur, parseValue(value))
  return nil
}

func applyRemove(obj map[string]interface{}, path, value string) error {
  segs, err := parsePath(path)
  if err != nil {
    return err
  }
  parent, last, err := navigateToParent(obj, segs)
  if err != nil {
    return err
  }
  if last.isIndex {
    return fmt.Errorf("--remove path must end at a key (use 'list_path INDEX' to remove an element)")
  }
  m, ok := parent.(map[string]interface{})
  if !ok {
    return fmt.Errorf("expected map at parent of %q", last.key)
  }
  // If value is a numeric index, treat path as a list and remove that index.
  if value != "" {
    idx, err := strconv.Atoi(value)
    if err != nil {
      return fmt.Errorf("--remove value must be a list index, got %q", value)
    }
    list, ok := m[last.key].([]interface{})
    if !ok {
      return fmt.Errorf("path %q does not refer to a list", last.key)
    }
    if idx < 0 || idx >= len(list) {
      return fmt.Errorf("index %d out of range", idx)
    }
    m[last.key] = append(list[:idx], list[idx+1:]...)
    return nil
  }
  delete(m, last.key)
  return nil
}

// navigateToParent walks segs[:-1] and returns the parent value plus the last segment.
func navigateToParent(root map[string]interface{}, segs []segment) (interface{}, segment, error) {
  if len(segs) == 0 {
    return nil, segment{}, fmt.Errorf("empty path")
  }
  var cursor interface{} = root
  for i := 0; i < len(segs)-1; i++ {
    seg := segs[i]
    if seg.isIndex {
      list, ok := cursor.([]interface{})
      if !ok {
        return nil, segment{}, fmt.Errorf("expected list at index %d", seg.index)
      }
      if seg.index < 0 || seg.index >= len(list) {
        return nil, segment{}, fmt.Errorf("index %d out of range", seg.index)
      }
      cursor = list[seg.index]
      continue
    }
    m, ok := cursor.(map[string]interface{})
    if !ok {
      return nil, segment{}, fmt.Errorf("expected map at key %q", seg.key)
    }
    cursor = m[seg.key]
  }
  return cursor, segs[len(segs)-1], nil
}
```

- [ ] **Step 4: Run all genericupdate tests to verify they pass**

Run: `go test ./pkg/genericupdate/... -v`
Expected: PASS for `TestApplySet`, `TestApplyAdd`, `TestApplyRemove`.

- [ ] **Step 5: Commit**

```bash
git add pkg/genericupdate/
git commit -m "feat(genericupdate): add Add and Remove operations"
```

---

## Task 8: `az resource list`

**Goal:** Implement `list` matching the user's example command:
`az resource list -g <rg> --resource-type Microsoft.Network/privateDnsZones`

**Files:**
- Modify: `internal/resource/list.go`

- [ ] **Step 1: Replace the stub**

Replace contents of `internal/resource/list.go`:
```go
package resource

import (
  "context"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List resources",
    Long:  "List resources, optionally filtered by group, type, name, location, or tag.",
    RunE:  runList,
  }
  cmd.Flags().StringP("name", "n", "", "Filter by resource name")
  cmd.Flags().StringP("resource-group", "g", "", "Limit to a single resource group")
  cmd.Flags().String("resource-type", "", "Filter by resource type (e.g. Microsoft.Network/virtualNetworks)")
  cmd.Flags().String("namespace", "", "Provider namespace (combined with --resource-type if unqualified)")
  cmd.Flags().StringP("location", "l", "", "Filter by location")
  cmd.Flags().String("tag", "", "Filter by tag (key or key=value)")
  return cmd
}

func runList(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  client, _, _, err := newGenericClient(cmd)
  if err != nil {
    return err
  }

  filter := buildListFilter(cmd)
  group, _ := cmd.Flags().GetString("resource-group")

  var results []map[string]interface{}
  if group != "" {
    opts := &armresources.ClientListByResourceGroupOptions{}
    if filter != "" {
      opts.Filter = &filter
    }
    pager := client.NewListByResourceGroupPager(group, opts)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list failed: %w", err)
      }
      for _, r := range page.Value {
        results = append(results, genericResourceToMap(r))
      }
    }
  } else {
    opts := &armresources.ClientListOptions{}
    if filter != "" {
      opts.Filter = &filter
    }
    pager := client.NewListPager(opts)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list failed: %w", err)
      }
      for _, r := range page.Value {
        results = append(results, genericResourceToMap(r))
      }
    }
  }

  if results == nil {
    results = []map[string]interface{}{}
  }
  return output.PrintJSON(cmd, results)
}

func buildListFilter(cmd *cobra.Command) string {
  name, _ := cmd.Flags().GetString("name")
  rtype, _ := cmd.Flags().GetString("resource-type")
  namespace, _ := cmd.Flags().GetString("namespace")
  location, _ := cmd.Flags().GetString("location")
  tag, _ := cmd.Flags().GetString("tag")

  // Combine namespace+type if --resource-type is unqualified.
  if rtype != "" && !strings.Contains(rtype, "/") && namespace != "" {
    rtype = namespace + "/" + rtype
  }

  parts := []string{}
  if name != "" {
    parts = append(parts, fmt.Sprintf("name eq '%s'", name))
  }
  if rtype != "" {
    parts = append(parts, fmt.Sprintf("resourceType eq '%s'", rtype))
  }
  if location != "" {
    parts = append(parts, fmt.Sprintf("location eq '%s'", location))
  }
  if tag != "" {
    if eq := strings.Index(tag, "="); eq != -1 {
      parts = append(parts, fmt.Sprintf("tagName eq '%s' and tagValue eq '%s'", tag[:eq], tag[eq+1:]))
    } else {
      parts = append(parts, fmt.Sprintf("tagName eq '%s'", tag))
    }
  }
  return strings.Join(parts, " and ")
}

// genericResourceToMap marshals an armresources.GenericResourceExpanded to the
// shape Python az resource emits (id, name, type, location, tags, etc.).
func genericResourceToMap(r *armresources.GenericResourceExpanded) map[string]interface{} {
  if r == nil {
    return nil
  }
  m := map[string]interface{}{}
  if r.ID != nil { m["id"] = *r.ID }
  if r.Name != nil { m["name"] = *r.Name }
  if r.Type != nil { m["type"] = *r.Type }
  if r.Location != nil { m["location"] = *r.Location }
  if r.Kind != nil { m["kind"] = *r.Kind }
  if r.ManagedBy != nil { m["managedBy"] = *r.ManagedBy }
  if r.Tags != nil {
    tags := map[string]string{}
    for k, v := range r.Tags {
      if v != nil {
        tags[k] = *v
      }
    }
    m["tags"] = tags
  }
  if r.SKU != nil { m["sku"] = r.SKU }
  if r.Identity != nil { m["identity"] = r.Identity }
  if r.Plan != nil { m["plan"] = r.Plan }
  return m
}
```

- [ ] **Step 2: Build**

Run: `make build`
Expected: build succeeds.

- [ ] **Step 3: Smoke test help**

Run: `./bin/az/az resource list --help`
Expected: help text includes `-g`, `--resource-type`, `--name`, `--namespace`, `-l`, `--tag`.

- [ ] **Step 4: Smoke test (live, requires login)**

If logged in, run the user's original example:
```
./bin/az/az resource list -g proscia-prod-base-network --resource-type Microsoft.Network/privateDnsZones --query "[].{name:name, id:id}"
```
Expected: JSON array of `{name, id}` objects, one per private DNS zone.

If not logged in or no such RG: skip and verify in Task 17 smoke checklist instead.

- [ ] **Step 5: Commit**

```bash
git add internal/resource/list.go
git commit -m "feat(resource): implement list subcommand"
```

---

## Task 9: `az resource show`

**Files:**
- Modify: `internal/resource/show.go`

- [ ] **Step 1: Replace the stub**

Replace contents of `internal/resource/show.go`:
```go
package resource

import (
  "context"
  "fmt"

  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Get details of a resource",
    RunE:  runShow,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  return cmd
}

func runShow(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  client, cred, sub, err := newGenericClient(cmd)
  if err != nil {
    return err
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")

  results := make([]interface{}, 0, len(ids))
  for _, id := range ids {
    _, _, namespace, types, _, perr := ParseResourceID(id)
    if perr != nil {
      return perr
    }
    rt := joinTypes(types)
    apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, rt, explicit, preview)
    if err != nil {
      return err
    }
    resp, err := client.GetByID(ctx, id, apiVer, nil)
    if err != nil {
      return fmt.Errorf("get %s: %w", id, err)
    }
    results = append(results, resp.GenericResource)
  }
  if len(results) == 1 {
    return output.PrintJSON(cmd, results[0])
  }
  return output.PrintJSON(cmd, results)
}

// joinTypes turns ["virtualNetworks","subnets"] into "virtualNetworks/subnets".
func joinTypes(types []string) string {
  s := ""
  for i, t := range types {
    if i > 0 {
      s += "/"
    }
    s += t
  }
  return s
}
```

- [ ] **Step 2: Build**

Run: `make build`
Expected: build succeeds.

- [ ] **Step 3: Smoke test help**

Run: `./bin/az/az resource show --help`
Expected: help text includes `--ids`, `-g`, `--resource-type`, `-n`, `--api-version`, `--latest-include-preview`.

- [ ] **Step 4: Commit**

```bash
git add internal/resource/show.go
git commit -m "feat(resource): implement show subcommand"
```

---

## Task 10: `az resource delete`

**Files:**
- Modify: `internal/resource/delete.go`

- [ ] **Step 1: Replace the stub**

Replace contents of `internal/resource/delete.go`:
```go
package resource

import (
  "context"
  "fmt"

  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a resource",
    RunE:  runDelete,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  return cmd
}

func runDelete(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  client, cred, sub, err := newGenericClient(cmd)
  if err != nil {
    return err
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")

  for _, id := range ids {
    _, _, namespace, types, _, perr := ParseResourceID(id)
    if perr != nil {
      return perr
    }
    apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, joinTypes(types), explicit, preview)
    if err != nil {
      return err
    }
    poller, err := client.BeginDeleteByID(ctx, id, apiVer, nil)
    if err != nil {
      return fmt.Errorf("delete %s: %w", id, err)
    }
    if _, err := poller.PollUntilDone(ctx, nil); err != nil {
      return fmt.Errorf("delete %s: %w", id, err)
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", id)
  }
  return nil
}
```

- [ ] **Step 2: Build**

Run: `make build`
Expected: build succeeds.

- [ ] **Step 3: Smoke test help**

Run: `./bin/az/az resource delete --help`
Expected: help text includes `--ids`, `-g`, etc.

- [ ] **Step 4: Commit**

```bash
git add internal/resource/delete.go
git commit -m "feat(resource): implement delete subcommand"
```

---

## Task 11: `az resource tag`

**Files:**
- Modify: `internal/resource/tag.go`

- [ ] **Step 1: Replace the stub**

Replace contents of `internal/resource/tag.go`:
```go
package resource

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newTagCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "tag",
    Short: "Add or replace tags on a resource",
    RunE:  runTag,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().StringToString("tags", nil, "Tags as key=value pairs (space-separated)")
  cmd.Flags().Bool("is-incremental", false, "Merge tags with existing ones instead of replacing")
  cmd.MarkFlagRequired("tags")
  return cmd
}

func runTag(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  tagsClient, err := newTagsClient(cmd)
  if err != nil {
    return err
  }
  raw, _ := cmd.Flags().GetStringToString("tags")
  incremental, _ := cmd.Flags().GetBool("is-incremental")
  op := armresources.TagsPatchOperationReplace
  if incremental {
    op = armresources.TagsPatchOperationMerge
  }

  tags := map[string]*string{}
  for k, v := range raw {
    tags[k] = to.Ptr(v)
  }

  results := make([]interface{}, 0, len(ids))
  for _, id := range ids {
    poller, err := tagsClient.BeginUpdateAtScope(ctx, id, armresources.TagsPatchResource{
      Operation:  &op,
      Properties: &armresources.Tags{Tags: tags},
    }, nil)
    if err != nil {
      return fmt.Errorf("tag %s: %w", id, err)
    }
    resp, err := poller.PollUntilDone(ctx, nil)
    if err != nil {
      return fmt.Errorf("tag %s: %w", id, err)
    }
    results = append(results, resp.TagsResource)
  }
  if len(results) == 1 {
    return output.PrintJSON(cmd, results[0])
  }
  return output.PrintJSON(cmd, results)
}
```

- [ ] **Step 2: Build**

Run: `make build`
Expected: build succeeds. (If `armresources.TagsPatchOperationReplace` constant name differs in the SDK version, replace with the correct identifier — check `armresources/constants.go` in the module cache.)

- [ ] **Step 3: Smoke test help**

Run: `./bin/az/az resource tag --help`
Expected: help text includes `--tags`, `--is-incremental`, selector flags.

- [ ] **Step 4: Commit**

```bash
git add internal/resource/tag.go
git commit -m "feat(resource): implement tag subcommand"
```

---

## Task 12: `az resource move`

**Files:**
- Modify: `internal/resource/move.go`

- [ ] **Step 1: Replace the stub**

Replace contents of `internal/resource/move.go`:
```go
package resource

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/spf13/cobra"
)

func newMoveCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "move",
    Short: "Move resources to another resource group or subscription",
    RunE:  runMove,
  }
  cmd.Flags().StringSlice("ids", nil, "One or more resource IDs to move (must share a resource group)")
  cmd.Flags().String("destination-group", "", "Target resource group name")
  cmd.Flags().String("destination-subscription-id", "", "Target subscription ID (defaults to current)")
  cmd.MarkFlagRequired("ids")
  cmd.MarkFlagRequired("destination-group")
  return cmd
}

func runMove(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, _ := cmd.Flags().GetStringSlice("ids")
  destGroup, _ := cmd.Flags().GetString("destination-group")
  destSub, _ := cmd.Flags().GetString("destination-subscription-id")

  if len(ids) == 0 {
    return fmt.Errorf("--ids is required")
  }

  // All IDs must share a source subscription and resource group.
  sourceSub, sourceGroup := "", ""
  for i, id := range ids {
    sub, group, _, _, _, err := ParseResourceID(id)
    if err != nil {
      return err
    }
    if i == 0 {
      sourceSub, sourceGroup = sub, group
      continue
    }
    if sub != sourceSub || group != sourceGroup {
      return fmt.Errorf("all --ids must share the same source subscription and resource group")
    }
  }

  client, _, _, err := newGenericClient(cmd)
  if err != nil {
    return err
  }

  // Build target resource group ID.
  targetSub := sourceSub
  if destSub != "" {
    targetSub = destSub
  }
  targetGroupID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", targetSub, destGroup)

  resources := make([]*string, 0, len(ids))
  for _, id := range ids {
    resources = append(resources, to.Ptr(id))
  }

  poller, err := client.BeginMoveResources(ctx, sourceGroup, armresources.ResourcesMoveInfo{
    Resources:           resources,
    TargetResourceGroup: to.Ptr(targetGroupID),
  }, nil)
  if err != nil {
    return fmt.Errorf("move: %w", err)
  }
  if _, err := poller.PollUntilDone(ctx, nil); err != nil {
    return fmt.Errorf("move: %w", err)
  }
  fmt.Fprintf(cmd.OutOrStdout(), "Moved %d resource(s) to %s\n", len(ids), targetGroupID)
  return nil
}
```

- [ ] **Step 2: Build**

Run: `make build`
Expected: build succeeds.

- [ ] **Step 3: Smoke test help**

Run: `./bin/az/az resource move --help`
Expected: help text includes `--ids`, `--destination-group`, `--destination-subscription-id`.

- [ ] **Step 4: Commit**

```bash
git add internal/resource/move.go
git commit -m "feat(resource): implement move subcommand"
```

---

## Task 13: `az resource wait`

**Files:**
- Modify: `internal/resource/wait.go`

- [ ] **Step 1: Replace the stub**

Replace contents of `internal/resource/wait.go`:
```go
package resource

import (
  "context"
  "encoding/json"
  "fmt"
  "time"

  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/jmespath/go-jmespath"
  "github.com/spf13/cobra"
)

func newWaitCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "wait",
    Short: "Wait until a resource reaches a desired condition",
    RunE:  runWait,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().Bool("created", false, "Wait until the resource exists")
  cmd.Flags().Bool("deleted", false, "Wait until the resource no longer exists")
  cmd.Flags().Bool("updated", false, "Wait until provisioningState reaches a terminal state")
  cmd.Flags().Bool("exists", false, "Alias for --created")
  cmd.Flags().String("custom", "", "Custom JMESPath query that must evaluate truthy on the resource body")
  cmd.Flags().Int("interval", 30, "Polling interval in seconds")
  cmd.Flags().Int("timeout", 3600, "Timeout in seconds")
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  return cmd
}

func runWait(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  if len(ids) != 1 {
    return fmt.Errorf("wait operates on a single resource; pass exactly one --ids or use name-mode flags")
  }
  id := ids[0]

  created, _ := cmd.Flags().GetBool("created")
  deleted, _ := cmd.Flags().GetBool("deleted")
  updated, _ := cmd.Flags().GetBool("updated")
  exists, _ := cmd.Flags().GetBool("exists")
  custom, _ := cmd.Flags().GetString("custom")
  interval, _ := cmd.Flags().GetInt("interval")
  timeout, _ := cmd.Flags().GetInt("timeout")

  conditions := 0
  for _, c := range []bool{created, deleted, updated, exists, custom != ""} {
    if c {
      conditions++
    }
  }
  if conditions != 1 {
    return fmt.Errorf("specify exactly one of --created, --deleted, --updated, --exists, --custom")
  }

  client, cred, sub, err := newGenericClient(cmd)
  if err != nil {
    return err
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")

  _, _, namespace, types, _, perr := ParseResourceID(id)
  if perr != nil {
    return perr
  }
  apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, joinTypes(types), explicit, preview)
  if err != nil {
    return err
  }

  deadline := time.Now().Add(time.Duration(timeout) * time.Second)
  for {
    if time.Now().After(deadline) {
      return fmt.Errorf("timed out waiting for %s", id)
    }

    resp, getErr := client.GetByID(ctx, id, apiVer, nil)
    notFound := getErr != nil && isNotFound(getErr)

    switch {
    case deleted:
      if notFound {
        return nil
      }
    case created || exists:
      if getErr == nil {
        return nil
      }
    case updated:
      if getErr == nil {
        if state := provisioningState(resp.GenericResource); isTerminal(state) {
          return nil
        }
      }
    case custom != "":
      if getErr == nil {
        body, _ := json.Marshal(resp.GenericResource)
        var parsed interface{}
        json.Unmarshal(body, &parsed)
        result, jerr := jmespath.Search(custom, parsed)
        if jerr != nil {
          return fmt.Errorf("--custom JMESPath: %w", jerr)
        }
        if isTruthy(result) {
          return nil
        }
      }
    }

    if getErr != nil && !notFound {
      return getErr
    }

    time.Sleep(time.Duration(interval) * time.Second)
  }
}

func provisioningState(r armresources.GenericResource) string {
  if r.Properties == nil {
    return ""
  }
  m, ok := r.Properties.(map[string]interface{})
  if !ok {
    return ""
  }
  if s, ok := m["provisioningState"].(string); ok {
    return s
  }
  return ""
}

func isTerminal(state string) bool {
  switch state {
  case "Succeeded", "Failed", "Canceled":
    return true
  }
  return false
}

func isNotFound(err error) bool {
  return err != nil && (containsAny(err.Error(), []string{"ResourceNotFound", "404"}))
}

func containsAny(s string, subs []string) bool {
  for _, sub := range subs {
    if sub != "" && (len(s) >= len(sub)) {
      if indexOf(s, sub) != -1 {
        return true
      }
    }
  }
  return false
}

func indexOf(s, sub string) int {
  for i := 0; i+len(sub) <= len(s); i++ {
    if s[i:i+len(sub)] == sub {
      return i
    }
  }
  return -1
}

func isTruthy(v interface{}) bool {
  switch t := v.(type) {
  case nil:
    return false
  case bool:
    return t
  case string:
    return t != ""
  case float64:
    return t != 0
  case []interface{}:
    return len(t) > 0
  case map[string]interface{}:
    return len(t) > 0
  }
  return true
}
```

Add the `armresources` import to the import block:
```go
"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
```

- [ ] **Step 2: Build**

Run: `make build`
Expected: build succeeds.

- [ ] **Step 3: Smoke test help**

Run: `./bin/az/az resource wait --help`
Expected: help text includes `--created`, `--deleted`, `--updated`, `--exists`, `--custom`, `--interval`, `--timeout`.

- [ ] **Step 4: Commit**

```bash
git add internal/resource/wait.go
git commit -m "feat(resource): implement wait subcommand"
```

---

## Task 14: `az resource create`

**Files:**
- Modify: `internal/resource/create.go`

- [ ] **Step 1: Replace the stub**

Replace contents of `internal/resource/create.go`:
```go
package resource

import (
  "context"
  "encoding/json"
  "fmt"
  "os"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "create",
    Short: "Create a resource generically from JSON properties",
    RunE:  runCreate,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().String("properties", "", "Resource properties as JSON (or @file.json)")
  cmd.Flags().Bool("is-full-object", false, "Treat --properties as the full request body, not just .properties")
  cmd.Flags().StringP("location", "l", "", "Resource location")
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  cmd.MarkFlagRequired("properties")
  return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  if len(ids) != 1 {
    return fmt.Errorf("create operates on a single resource")
  }
  id := ids[0]

  rawProps, _ := cmd.Flags().GetString("properties")
  isFull, _ := cmd.Flags().GetBool("is-full-object")
  location, _ := cmd.Flags().GetString("location")

  body, err := readJSONInput(rawProps)
  if err != nil {
    return fmt.Errorf("--properties: %w", err)
  }

  var resource armresources.GenericResource
  if isFull {
    raw, _ := json.Marshal(body)
    if err := json.Unmarshal(raw, &resource); err != nil {
      return fmt.Errorf("--properties as full object: %w", err)
    }
  } else {
    resource.Properties = body
    if location != "" {
      resource.Location = &location
    }
  }

  client, cred, sub, err := newGenericClient(cmd)
  if err != nil {
    return err
  }
  _, _, namespace, types, _, perr := ParseResourceID(id)
  if perr != nil {
    return perr
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")
  apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, joinTypes(types), explicit, preview)
  if err != nil {
    return err
  }

  poller, err := client.BeginCreateOrUpdateByID(ctx, id, apiVer, resource, nil)
  if err != nil {
    return fmt.Errorf("create %s: %w", id, err)
  }
  resp, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("create %s: %w", id, err)
  }
  return output.PrintJSON(cmd, resp.GenericResource)
}

// readJSONInput parses raw as JSON; if raw begins with '@', reads from the
// referenced file path first.
func readJSONInput(raw string) (interface{}, error) {
  if strings.HasPrefix(raw, "@") {
    data, err := os.ReadFile(raw[1:])
    if err != nil {
      return nil, err
    }
    var v interface{}
    if err := json.Unmarshal(data, &v); err != nil {
      return nil, err
    }
    return v, nil
  }
  var v interface{}
  if err := json.Unmarshal([]byte(raw), &v); err != nil {
    return nil, err
  }
  return v, nil
}
```

- [ ] **Step 2: Build**

Run: `make build`
Expected: build succeeds.

- [ ] **Step 3: Smoke test help**

Run: `./bin/az/az resource create --help`
Expected: help text includes `--properties`, `--is-full-object`, `-l`.

- [ ] **Step 4: Commit**

```bash
git add internal/resource/create.go
git commit -m "feat(resource): implement create subcommand"
```

---

## Task 15: `az resource update`

**Files:**
- Modify: `internal/resource/update.go`

- [ ] **Step 1: Replace the stub**

Replace contents of `internal/resource/update.go`:
```go
package resource

import (
  "context"
  "encoding/json"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/genericupdate"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "update",
    Short: "Update a resource generically via --set/--add/--remove",
    RunE:  runUpdate,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().StringArray("set", nil, "Set a property: path=value (repeatable)")
  cmd.Flags().StringArray("add", nil, "Append to a list property: path JSON_VALUE (repeatable)")
  cmd.Flags().StringArray("remove", nil, "Remove a key or list element: path [INDEX] (repeatable)")
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  return cmd
}

func runUpdate(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  if len(ids) != 1 {
    return fmt.Errorf("update operates on a single resource")
  }
  id := ids[0]

  setOps, _ := cmd.Flags().GetStringArray("set")
  addOps, _ := cmd.Flags().GetStringArray("add")
  removeOps, _ := cmd.Flags().GetStringArray("remove")

  ops, err := parseUpdateOps(setOps, addOps, removeOps)
  if err != nil {
    return err
  }
  if len(ops) == 0 {
    return fmt.Errorf("at least one --set/--add/--remove is required")
  }

  client, cred, sub, err := newGenericClient(cmd)
  if err != nil {
    return err
  }
  _, _, namespace, types, _, perr := ParseResourceID(id)
  if perr != nil {
    return perr
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")
  apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, joinTypes(types), explicit, preview)
  if err != nil {
    return err
  }

  // GET the current state, mutate, PUT it back via UpdateByID (PATCH semantics).
  resp, err := client.GetByID(ctx, id, apiVer, nil)
  if err != nil {
    return fmt.Errorf("get %s: %w", id, err)
  }

  body, err := json.Marshal(resp.GenericResource)
  if err != nil {
    return err
  }
  var obj map[string]interface{}
  if err := json.Unmarshal(body, &obj); err != nil {
    return err
  }
  if err := genericupdate.Apply(obj, ops); err != nil {
    return err
  }

  raw, _ := json.Marshal(obj)
  var updated armresources.GenericResource
  if err := json.Unmarshal(raw, &updated); err != nil {
    return err
  }

  poller, err := client.BeginUpdateByID(ctx, id, apiVer, updated, nil)
  if err != nil {
    return fmt.Errorf("update %s: %w", id, err)
  }
  out, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("update %s: %w", id, err)
  }
  return output.PrintJSON(cmd, out.GenericResource)
}

func parseUpdateOps(setOps, addOps, removeOps []string) ([]genericupdate.Op, error) {
  out := []genericupdate.Op{}
  for _, s := range setOps {
    eq := strings.Index(s, "=")
    if eq == -1 {
      return nil, fmt.Errorf("--set %q: expected path=value", s)
    }
    out = append(out, genericupdate.Op{Kind: genericupdate.Set, Path: s[:eq], Value: s[eq+1:]})
  }
  for _, a := range addOps {
    sp := strings.IndexAny(a, " \t")
    if sp == -1 {
      return nil, fmt.Errorf("--add %q: expected 'path JSON_VALUE'", a)
    }
    out = append(out, genericupdate.Op{Kind: genericupdate.Add, Path: a[:sp], Value: strings.TrimSpace(a[sp+1:])})
  }
  for _, r := range removeOps {
    sp := strings.IndexAny(r, " \t")
    if sp == -1 {
      out = append(out, genericupdate.Op{Kind: genericupdate.Remove, Path: r})
      continue
    }
    out = append(out, genericupdate.Op{Kind: genericupdate.Remove, Path: r[:sp], Value: strings.TrimSpace(r[sp+1:])})
  }
  return out, nil
}
```

- [ ] **Step 2: Build**

Run: `make build`
Expected: build succeeds.

- [ ] **Step 3: Smoke test help**

Run: `./bin/az/az resource update --help`
Expected: help text includes `--set`, `--add`, `--remove`.

- [ ] **Step 4: Commit**

```bash
git add internal/resource/update.go
git commit -m "feat(resource): implement update subcommand"
```

---

## Task 16: `az resource invoke-action`

**Files:**
- Modify: `internal/resource/invoke_action.go`

- [ ] **Step 1: Replace the stub**

Replace contents of `internal/resource/invoke_action.go`:
```go
package resource

import (
  "context"
  "encoding/json"
  "fmt"
  "io"
  "net/http"
  "os"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
  "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newInvokeActionCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "invoke-action",
    Short: "POST to a resource action endpoint",
    RunE:  runInvokeAction,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().String("action", "", "Action name (e.g. 'restart', 'powerOff')")
  cmd.Flags().String("request-body", "", "JSON body or @file.json")
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  cmd.MarkFlagRequired("action")
  return cmd
}

func runInvokeAction(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  if len(ids) != 1 {
    return fmt.Errorf("invoke-action operates on a single resource")
  }
  id := ids[0]

  action, _ := cmd.Flags().GetString("action")
  rawBody, _ := cmd.Flags().GetString("request-body")

  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }
  sub, err := resolveSubscription(cmd)
  if err != nil {
    return err
  }
  _, _, namespace, types, _, perr := ParseResourceID(id)
  if perr != nil {
    return perr
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")
  apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, joinTypes(types), explicit, preview)
  if err != nil {
    return err
  }

  client, err := arm.NewClient("github.com/cdobbyn/azure-go-cli/internal/resource", "", cred, nil)
  if err != nil {
    return err
  }
  endpoint := "https://management.azure.com"
  url := fmt.Sprintf("%s%s/%s", endpoint, id, action)

  req, err := runtime.NewRequest(ctx, http.MethodPost, url)
  if err != nil {
    return err
  }
  q := req.Raw().URL.Query()
  q.Set("api-version", apiVer)
  req.Raw().URL.RawQuery = q.Encode()

  if rawBody != "" {
    body, err := readBodyInput(rawBody)
    if err != nil {
      return err
    }
    if err := req.SetBody(body, "application/json"); err != nil {
      return err
    }
  }

  resp, err := client.Pipeline().Do(req)
  if err != nil {
    return fmt.Errorf("invoke-action %s: %w", action, err)
  }
  defer resp.Body.Close()
  data, _ := io.ReadAll(resp.Body)

  if resp.StatusCode >= 400 {
    return fmt.Errorf("invoke-action %s: %s: %s", action, resp.Status, string(data))
  }

  if len(data) == 0 {
    fmt.Fprintf(cmd.OutOrStdout(), "Action %s completed (status %s)\n", action, resp.Status)
    return nil
  }
  var parsed interface{}
  if err := json.Unmarshal(data, &parsed); err != nil {
    fmt.Fprintln(cmd.OutOrStdout(), string(data))
    return nil
  }
  return output.PrintJSON(cmd, parsed)
}

// readBodyInput returns an io.ReadSeekCloser usable by req.SetBody, accepting
// either a literal JSON string or @path/to/file.json.
func readBodyInput(raw string) (io.ReadSeekCloser, error) {
  if strings.HasPrefix(raw, "@") {
    f, err := os.Open(raw[1:])
    if err != nil {
      return nil, err
    }
    return f, nil
  }
  return nopReadSeekCloser{strings.NewReader(raw)}, nil
}

type nopReadSeekCloser struct{ *strings.Reader }
func (nopReadSeekCloser) Close() error { return nil }
```

- [ ] **Step 2: Build**

Run: `make build`
Expected: build succeeds.

- [ ] **Step 3: Smoke test help**

Run: `./bin/az/az resource invoke-action --help`
Expected: help text includes `--action`, `--request-body`, selector flags.

- [ ] **Step 4: Commit**

```bash
git add internal/resource/invoke_action.go
git commit -m "feat(resource): implement invoke-action subcommand"
```

---

## Task 17: Full smoke test against real Azure

**Goal:** Verify the user's original example plus one round-trip per subcommand against a live subscription.

**Files:**
- (no code changes; this task only validates)

- [ ] **Step 1: Run the user's original command**

Run:
```
./bin/az/az resource list -g proscia-prod-base-network --resource-type Microsoft.Network/privateDnsZones --query "[].{name:name, id:id}"
```
Expected: JSON array of `{name, id}` pairs for each private DNS zone in the group.

- [ ] **Step 2: Show one of the listed resources**

Pick an ID from Step 1's output. Run:
```
./bin/az/az resource show --ids <id>
```
Expected: a single JSON object with `id`, `name`, `type`, `location`, `properties`, etc.

- [ ] **Step 3: Tag it (incremental)**

```
./bin/az/az resource tag --ids <id> --tags smoke=test --is-incremental
```
Expected: JSON output of the tags resource.

- [ ] **Step 4: Update it via --set**

```
./bin/az/az resource update --ids <id> --set tags.smoke=updated
```
Expected: JSON output of the updated resource with `tags.smoke == "updated"`.

- [ ] **Step 5: Wait condition**

```
./bin/az/az resource wait --ids <id> --updated --interval 5 --timeout 60
```
Expected: returns within seconds (resource is already in `Succeeded` state).

- [ ] **Step 6: Clean up the smoke tag**

```
./bin/az/az resource update --ids <id> --remove tags.smoke
```
Expected: JSON output without `tags.smoke`.

- [ ] **Step 7: Move and invoke-action — defer if no safe target**

`move` and `invoke-action` are destructive on production resources. If a non-production target is available, run an end-to-end test there. Otherwise, document them as untested in this commit's PR description (smoke list above is the minimum acceptance bar).

- [ ] **Step 8: Final commit (only if any fixes were needed)**

If smoke testing surfaced issues, fix them and commit per the conventional-commits format. If everything passed, no commit needed for this task.

---

## Self-Review Notes

The plan covers every spec section: 9 subcommands (Tasks 8–16), API version resolution (Task 5), selector flags (Tasks 2–4), generic update path syntax (Tasks 6–7), `invoke-action` raw-pipeline approach (Task 16), error handling for selector validation (Task 4), `move` cross-RG validation (Task 12), and `wait` polling with all five condition flags (Task 13). `-o table` is explicitly out of scope per the spec.

Type/method names that flow across tasks:
- `ParseResourceID`, `BuildResourceID`, `ResolveSelector`, `AddSelectorFlags` — Tasks 2-4, used in 9-16
- `azure.ResolveAPIVersion` — Task 5, used in 9, 10, 13, 14, 15, 16
- `genericupdate.Op{Kind, Path, Value}`, `genericupdate.Set/Add/Remove`, `genericupdate.Apply` — Tasks 6-7, used in 15
- `newGenericClient`, `newTagsClient`, `resolveSubscription`, `joinTypes` — Task 1/15, used throughout

One gap to flag at implementation time, not now: the exact identifier of the Replace/Merge tag patch operation enum may be `armresources.TagsPatchOperationReplace` or `armresources.TagsPatchOperationKindReplace` depending on SDK v1.2 specifics — Task 11 notes this. If the implementer hits a build error there, check the SDK constants file in the module cache and substitute the correct name.
