# `az resource` Command Group — Design

**Date:** 2026-05-05
**Status:** Approved (pending implementation plan)
**Scope:** Add `az resource` command group with all 9 generic subcommands present in the real Azure CLI: `list`, `show`, `delete`, `tag`, `move`, `wait`, `create`, `update`, `invoke-action`.

> The brainstorming conversation referred to "10" subcommands; on review there are 9. `az resource link` is a separate command group in the real CLI (and deprecated in modern versions) — out of scope here, tracked as a follow-up.

## Motivation

The user encountered the following command, which our Go CLI does not yet implement:

```
az resource list \
  --resource-group proscia-prod-base-network \
  --resource-type Microsoft.Network/privateDnsZones \
  --query "[].{name:name, id:id}" -o table
```

`az resource` is the generic resource access surface for ARM. Adding it lets us inspect, tag, move, and manipulate any ARM resource without per-type plumbing — closing a large gap with the Python CLI.

## Parity Principle

**Every flag, alias, default, and JSON output shape must match Python `az resource` exactly.** Implementation uses the Go SDK (`azure-sdk-for-go/sdk/resourcemanager/resources/armresources`), but observable behavior (flag names, semantics, output JSON keys) mirrors Python. Where the SDK is missing capability that Python has (latest API version resolution, generic action invocation, generic update path expressions), we add a thin adapter rather than diverging from Python's surface.

## Subcommands

| Subcommand | Purpose |
|---|---|
| `list` | List resources, optionally filtered |
| `show` | Get a single resource |
| `delete` | Delete one or more resources |
| `tag` | Add/replace tags on a resource |
| `move` | Move resources to another resource group or subscription |
| `wait` | Block until a resource reaches a desired condition |
| `create` | Generic create with caller-supplied JSON `--properties` |
| `update` | Generic update via `--set`/`--add`/`--remove` path expressions |
| `invoke-action` | POST to a resource action endpoint |

## Flag Reference (Python parity)

### Resource selector (used by `show`, `delete`, `tag`, `wait`, `create`, `update`, `invoke-action`)

Two mutually-exclusive modes:

- **By ID:** `--ids ID [ID ...]` (variadic, repeatable). When multiple IDs are passed, the command iterates and emits a JSON array of results.
- **By name:** `-g/--resource-group` + `--resource-type` (qualified `Namespace/type` or unqualified with `--namespace`) + `-n/--name`. Optional `--parent` for child resources (e.g., `subnets/foo` under a vnet).

### Common flags

- `--api-version` — explicit API version. When omitted, resolved automatically (see "API version resolution" below).
- `--latest-include-preview` — include preview API versions during automatic resolution.
- `--include-response-body` (where applicable on `show`/`update`) — include full body in output.

### Per-subcommand flags

| Subcommand | Required | Notable optional |
|---|---|---|
| `list` | (none) | `-n/--name`, `-g/--resource-group`, `--resource-type`, `--namespace`, `-l/--location`, `--tag` |
| `show` | selector | `--api-version`, `--latest-include-preview`, `--include-response-body` |
| `delete` | selector | `--api-version`, `--latest-include-preview` |
| `tag` | `--tags KEY=VALUE [...]` + selector | `--is-incremental` (merge instead of replace), `--api-version`, `--latest-include-preview` |
| `move` | `--ids`, `--destination-group` | `--destination-subscription-id` |
| `wait` | selector + condition | `--created`, `--deleted`, `--updated`, `--exists`, `--custom JMES`, `--interval` (default 30s), `--timeout` (default 3600s), `--api-version` |
| `create` | selector + `--properties JSON` | `--is-full-object`, `-l/--location`, `--api-version`, `--latest-include-preview` |
| `update` | selector | `--set path=val`, `--add path val`, `--remove path [index]`, `--api-version`, `--latest-include-preview`, `--include-response-body` |
| `invoke-action` | selector + `--action` | `--request-body STR_OR_@FILE`, `--api-version`, `--latest-include-preview` |

## Architecture

### Package layout

```
internal/resource/
├── commands.go      # cobra wiring for all 9 subcommands
├── client.go        # build armresources.Client + TagsClient; resolve subscription
├── resolve.go       # ParseResourceID, BuildResourceID, --ids vs name-mode validation
├── list.go
├── show.go
├── delete.go
├── tag.go
├── move.go
├── wait.go
├── create.go
├── update.go
└── invoke_action.go

pkg/azure/apiversion.go    # latest-API-version resolver (provider lookup + cache)
pkg/genericupdate/         # --set/--add/--remove path-expression parser & mutator
└── genericupdate.go
```

Register in `cmd/az/main.go`:

```go
import "github.com/cdobbyn/azure-go-cli/internal/resource"
...
rootCmd.AddCommand(resource.NewResourceCommand())
```

### SDK mapping

| Operation | SDK call |
|---|---|
| `list` (no group) | `armresources.Client.NewListPager` |
| `list -g X` | `armresources.Client.NewListByResourceGroupPager` |
| `show` | `Client.GetByID(ctx, id, apiVersion, nil)` |
| `delete` | `Client.BeginDeleteByID(ctx, id, apiVersion, nil)` |
| `create` | `Client.BeginCreateOrUpdateByID(ctx, id, apiVersion, GenericResource{...}, nil)` |
| `update` | `Client.BeginUpdateByID(ctx, id, apiVersion, GenericResource{...}, nil)` |
| `tag` | `armresources.TagsClient.BeginUpdateAtScope(ctx, scope, TagsPatchResource{Operation: Replace\|Merge}, nil)` |
| `move` | `Client.BeginMoveResources(ctx, sourceGroup, ResourcesMoveInfo{Resources, TargetResourceGroup}, nil)` |
| `wait` | poll `Client.GetByID` / `CheckExistenceByID` on `--interval` until condition met |
| `invoke-action` | raw `azcore/arm.Client` + `runtime.NewRequest` POST to `{id}/{action}?api-version=` |

### Resource ID handling — `internal/resource/resolve.go`

```go
type ResourceRef struct {
    ID string  // canonical ARM resource ID
}

// Parse an ARM resource ID into its components.
func ParseResourceID(id string) (sub, group, namespace string, types, names []string, err error)

// Build canonical ID from name-mode flags.
func BuildResourceID(sub, group, namespace, resourceType, parent, name string) (string, error)

// ResolveSelector reads --ids or name-mode flags from cobra.Command and
// returns one or more ResourceRefs. Errors if neither or both are given.
func ResolveSelector(cmd *cobra.Command) ([]ResourceRef, error)
```

`--resource-type` accepts:
- `Microsoft.Foo/bar` (qualified, namespace inferred)
- `bar` with `--namespace Microsoft.Foo` (unqualified)
- `bar/sub` for parent/child (or use `--parent`)

### API version resolution — `pkg/azure/apiversion.go`

When `--api-version` is omitted, query the `Microsoft.Resources` provider client for the namespace and pick the latest API version of the matching resource type. With `--latest-include-preview`, include preview versions; otherwise filter to stable.

```go
// ResolveAPIVersion returns the API version to use for the given resource type.
// Returns the explicit value if provided; otherwise queries the provider.
func ResolveAPIVersion(ctx context.Context, cred azcore.TokenCredential, subID, namespace, resourceType, explicit string, includePreview bool) (string, error)
```

In-process cache keyed by `namespace/resourceType` to avoid repeated provider calls in a single command invocation (e.g., when iterating over many `--ids`).

### Generic update — `pkg/genericupdate/`

Implements Python's `--set`/`--add`/`--remove` path syntax against a `map[string]interface{}` representing the resource body.

Supported syntax (matches Python `az` generic update):
- `--set tags.env=prod` — set string
- `--set properties.networkAcls={"defaultAction":"Deny"}` — set JSON value
- `--set properties.subnets[0].name=subnet1` — index access
- `--set properties.subnets[?name=='subnet1'].addressPrefix=10.0.0.0/24` — JMESPath-style filter
- `--add properties.subnets {"name":"subnet2"}` — append to list
- `--remove properties.subnets 1` — remove by index
- `--remove tags.env` — remove key

```go
// Apply mutates obj per the slice of path expressions.
type Op struct { Kind OpKind; Path string; Value string } // Kind: Set | Add | Remove
func Apply(obj map[string]interface{}, ops []Op) error
```

Lives in `pkg/genericupdate/` (shared) so other commands that need ARM-style generic updates can reuse it.

### `invoke-action`

No first-class SDK helper exists. Implementation:

```go
client, err := arm.NewClient("github.com/cdobbyn/azure-go-cli/internal/resource", "", cred, nil)
req, err := runtime.NewRequest(ctx, http.MethodPost, fmt.Sprintf("%s/%s", endpoint+id, action))
q := req.Raw().URL.Query()
q.Set("api-version", apiVersion)
req.Raw().URL.RawQuery = q.Encode()
if requestBody != "" {
    body := readRequestBody(requestBody) // handles @file.json
    req.SetBody(streaming.NopCloser(strings.NewReader(body)), "application/json")
}
resp, err := client.Pipeline().Do(req)
// parse and emit response body as JSON
```

`--request-body` accepts a literal JSON string or `@path/to/file.json` (matches Python).

### Subscription handling

Each command reads the global `--subscription` flag; falls back to `config.GetDefaultSubscription()`. Same pattern already used in `internal/aks/`.

### Output

- All commands emit JSON via `pkg/output.PrintJSON` (handles `--query`).
- Output JSON shape comes from `armresources.GenericResource` and ARM response bodies. The Go SDK's `json` tags already match Python's field names (`id`, `name`, `type`, `location`, `tags`, `properties`, `sku`, `identity`, `kind`, `managedBy`, `plan`); to be verified during implementation.
- **`-o table` is out of scope for this pass.** When set, fall back to JSON with a one-line stderr note: `Note: -o table not yet supported for resource commands; falling back to JSON.` A generic table renderer is tracked as a separate follow-up task.

### Error handling

- Selector validation: "Please specify either `--ids` or both `-g` and resource info" when neither given (matches Python).
- ARM errors: parse the `{code, message}` from the response body and surface as `{code}: {message}` rather than raw HTTP text.
- `wait --timeout`: exits with code 1 on timeout.
- `move`: validates that all `--ids` belong to the same source resource group before issuing the call (matches ARM requirement; Python pre-validates client-side).

## Testing

### Unit

- `internal/resource/resolve_test.go` — `ParseResourceID`/`BuildResourceID` round-trip, qualified vs unqualified `--resource-type`, parent/child IDs, validation errors when both/neither selector mode given.
- `pkg/genericupdate/genericupdate_test.go` — each operation against representative bodies; index access; JMES-style filter; missing-path errors.
- `pkg/azure/apiversion_test.go` — selection from a fixture provider response; preview vs stable filtering; cache hit on repeated calls.

### Integration (manual)

A smoke checklist run against a real subscription:

1. The user's original command:
   `az resource list -g proscia-prod-base-network --resource-type Microsoft.Network/privateDnsZones`
2. `az resource show --ids <id>` for one of the listed zones
3. `az resource tag --ids <id> --tags owner=test`
4. `az resource update --ids <id> --set tags.env=staging`
5. `az resource wait --ids <id> --updated`
6. `az resource invoke-action --ids <vm-id> --action restart`
7. `az resource move --ids <id> --destination-group <other-rg>`
8. `az resource delete --ids <id>`

No live-call automation in CI.

## Out of Scope (explicit follow-ups)

- `-o table` generic renderer (separate task)
- `az resource link` (separate command group; deprecated in modern Azure CLI anyway)
- Persisted API-version cache across invocations (in-process only for this pass)

## Open Questions

None at design time. Verification points deferred to implementation:

- Confirm `armresources.GenericResource` JSON tags match Python output verbatim.
- Confirm `BeginMoveResources` accepts cross-subscription via `TargetResourceGroup` only, or whether a different SDK method is needed for `--destination-subscription-id`.
