# `az lock` Port — Design

**Date:** 2026-07-16
**Status:** Approved (pending spec review)
**Branch:** `worktree-az-lock-port-4c1e`

## Goal

Port the Azure CLI `lock` command surface into azure-go-cli: four command groups
(`az lock`, `az account lock`, `az group lock`, `az resource lock`), five verbs each,
over one shared implementation in `internal/lock/`.

All 24 lock entries currently sit in `docs/missing-commands.txt` (lines 2-7, 1485-1490,
1699-1704, 2425-2430). This port moves them to `docs/implemented-commands.txt` via
`./scripts/audit-commands.sh`.

## Source of Truth

The flag surface below was captured from the **real** Azure CLI
(`mcr.microsoft.com/azure-cli:latest`, via `scripts/az-official`), not from documentation.
Raw captures are in the session scratchpad. Behavioral detail (validator internals, the
`--ids` code path, knack's table algorithm) comes from reading the `Azure/azure-cli` and
`microsoft/knack` sources.

Two flags that early research attributed to `create`/`delete`/`update` —
`--acquire-policy-token` and `--change-reference` — **do not exist** on any lock command in
the shipping CLI. They are not ported and represent no parity gap.

## Command Surface

### `az lock` — scope inferred from flags

| flag | create | delete | list | show | update |
|---|---|---|---|---|---|
| `-t, --lock-type` | **required** | — | — | — | optional |
| `-n, --name` | **required** | optional | — | optional | optional |
| `--notes` | optional | — | — | — | optional |
| `--ids` | — | yes | — | yes | yes |
| `-g, --resource-group` | optional | optional | optional | optional | optional |
| `--resource` / `--resource-name` | optional | optional | optional | optional | optional |
| `--resource-type` | optional | optional | optional | optional | optional |
| `--namespace` | optional | optional | optional | optional | optional |
| `--parent` | optional | optional | optional | optional | optional |
| `--filter-string` | — | — | yes | — | — |

`create` has no `--ids`; `list` has neither `--ids` nor `--name`; `show` has no `--notes`.

### Sibling groups — same verbs, narrower flags

Each sibling registers a fixed subset of the scope flags. Verified against the shipping CLI:

| group | scope | scope flags registered |
|---|---|---|
| `az account lock` | always subscription | *none* |
| `az group lock` | always resource group | `-g` |
| `az resource lock` | always resource | `-g`, `--namespace`, `--parent`, `--resource`/`--resource-name`, `--resource-type` |

Per-verb, on top of those scope flags:

| verb | adds | `--ids`? |
|---|---|---|
| `create` | `-t` **(req)**, `-n` **(req)**, `--notes` | no |
| `list` | `--filter-string` | no |
| `delete` | `-n` | yes |
| `show` | `-n` | yes |
| `update` | `-t`, `-n`, `--notes` | yes |

**The required-flag rule is mechanical: a flag is `[Required]` exactly on the verbs that
lack `--ids`.** azure-cli's generic `--ids` mechanism demotes every "Resource Id" argument
to optional wherever it is registered — which is `delete`/`show`/`update`, never
`create`/`list`. Hence `az group lock create` and `az group lock list` require `-g`, while
`az group lock show` does not. Same for `-n` and `-t` across all four groups.

The siblings are not separate logic — they register a subset of the scope flags and skip the
validator branches those flags would reach. `az resource lock`'s `-g` is never `[Required]`
because `--resource` may carry a full resource ID instead.

### Flag details

- **`--resource` / `--resource-name` are co-equal long aliases**, not a flag plus shorthand
  (`_params.py:499`, confirmed in real `--help` output: `--resource --resource-name`). There
  is no single-dash form. Implement with `Flags().SetNormalizeFunc` folding `--resource-name`
  → `--resource`, so help shows one line.
- **`--lock-type` values are `CanNotDelete` and `ReadOnly`** — exact casing, capital N.
  The SDK's `LockLevel` also has `NotSpecified`; the CLI narrows it out (`_params.py:526`).
  Python accepts input case-insensitively and emits canonical casing. Match with
  `strings.EqualFold`, then map to `armlocks.LockLevelCanNotDelete` / `LockLevelReadOnly`,
  rejecting `NotSpecified` explicitly.
- Globals come free from `cmd/az/main.go:51-54`: `--subscription`, `-o/--output`, `--query`,
  `--debug`.

## Scope Resolution

Ported from `internal_validate_lock_parameters`. Error strings are **rewritten in repo house
style** (lowercase, terse, names the offending flag) rather than reproduced verbatim — the
Python originals say "is ignored" while actually raising, and one contains a missing-space
typo (`present.Expected`).

```
resolve(rg, resource, rtype, ns, parent) -> scope

A. rg == "":
     if resource != "":
         if !isValidResourceID(resource):
             err: --resource must be a full resource ID when --resource-group is omitted
         resource, rtype, rg, ns, parent = parseResourceID(resource)
     if rtype  != "": err: --resource-type requires --resource-group
     if ns     != "": err: --namespace requires --resource-group
     if parent != "": err: --parent requires --resource-group
B. rg != "" && resource == "":
     if rtype  != "": err: --resource-type requires --resource
     if ns     != "": err: --namespace requires --resource
     if parent != "": err: --parent requires --resource
C. rg != "" && resource != "":
     if rtype == "": err: --resource-type is required when --resource is given
     parts = split(rtype, "/")
     if ns == "" && len(parts) == 1:
         err: --resource-type must be namespace/type, or pass --namespace
     if ns != "" && len(parts) != 1:
         err: --namespace given in both --resource-type and --namespace

# split namespace out of a qualified type
if ns == "" && len(split(rtype, "/", 2)) == 2: ns, rtype = parts[0], parts[1]

# pick scope
rg == ""                   -> SUBSCRIPTION
rg != "" && resource == "" -> RESOURCE GROUP
else                       -> RESOURCE  (parentResourcePath = parent, "" when unset)
```

`parentResourcePath` is a required positional in the SDK — pass `""`, never omit (matches
Python's `parent_resource_path or ''`).

**Divergence — 3-segment `--resource-type`:** Python's `split('/', 2)` only splits when
exactly 2 parts result, so `Microsoft.Network/virtualNetworks/subnets` passes through
unsplit and silently malforms the ARM path. We **error** on ≥3 segments instead, directing
users to `--parent` (the documented form:
`--resource-type Microsoft.Network/subnets --parent virtualNetworks/myVnet`). Silent
corruption is not worth reproducing.

## Lock ID Parsing

Lock IDs at resource scope contain **two** `/providers/` segments, so the repo's existing
`resource.ParseResourceID` cannot parse them. `lockid.go` gets a bespoke regex:

```
/subscriptions/[^/]*(/resource[gG]roups/(?P<resource_group>[^/]*)
(/providers/(?P<resource_provider_namespace>[^/]*)
(/(?P<parent_resource_path>.*))?/(?P<resource_type>[^/]*)/(?P<resource_name>[^/]*))?)?
/providers/Microsoft.Authorization/locks/(?P<lock_name>[^/]*)
```

Both `resourcegroups` and `resourceGroups` casings. Everything after the subscription ID is
optional, so subscription-scoped IDs parse with only `lock_name`. Python uses `.match()`
(not `.fullmatch()`), tolerating trailing garbage; Go's `regexp.FindStringSubmatch` has the
same unanchored-at-end semantics, so **do not append `$`**.

**Divergence:** an ID that parses with an empty resource name returns a real error. In
Python this is an unhandled `AttributeError` crash
(`id_dict.get('resource_parent').strip('/')` on an ID with no name).

## `--ids`

Ported with the dynamic return shape (1 id → bare object, ≥2 → array), but an unparseable ID
returns an error from `RunE` rather than Python's `logger.error(...)` + exit 0, which
silently swallows typos. `--ids` applies the same `--lock-type`/`--notes` to every id on
`update`; there is no per-id override.

## Scope Precheck

`_validate_lock_params_match_lock` is ported onto `show`/`delete`/`update` (not
`create`/`list`): list locks subscription-wide, count by name, and if exactly one matches,
compare its parsed scope against the user's flags. If the count is not exactly 1, no
validation occurs. Comparison is case-insensitive on `--resource-group`/`--namespace` and
case-sensitive on `--resource-type`/`--resource-name`/`--parent`.

Messages are emitted in repo house style, not azure-cli's wording. **This means the
precheck's validation behavior is ported but its stderr text is not** — deliberate, per the
error-string decision above.

Cost, accepted knowingly: a subscription-wide lock list on every show/delete/update, which
requires subscription-wide lock-read permission even to touch one RG-scoped lock.

## Output

### JSON

ARM marks `properties` with `x-ms-client-flatten`, so azure-cli prints a **flattened**
object. The Go SDK does not flatten (`ManagementLockObject.Properties *ManagementLockProperties`).
`record.go` hand-flattens into:

```json
{ "id": "...", "level": "CanNotDelete", "name": "mylock",
  "notes": "do not delete", "owners": null,
  "resourceGroup": "myrg", "type": "Microsoft.Authorization/locks" }
```

Precedent: `roleAssignmentRecord` (`internal/role/assignment/list.go:18`). Without this,
every azure-cli `--query "[].level"` breaks.

**`resourceGroup` is not an ARM field.** azure-cli injects it via a *global* transform
registered on `EVENT_INVOKER_TRANSFORM_RESULT` (`core/commands/transform.py`), which fires
before any output formatting — so it appears in JSON too, not just table. It is present only
when the ID's fourth segment is `resourcegroups`, i.e. for RG- and resource-scoped locks, and
absent for subscription-scoped ones. Model as `omitempty`.

`owners` emits `null`, not `[]`. `list` returns a JSON array. `delete` returns no body.

### Table

`-o table` is supported on `list` and `show`, rendered **fully faithful to azure-cli**:
capitalize-first-char headers plus tabulate's rule row.

```
Level         Name    Notes          ResourceGroup
------------  ------  -------------  ---------------
CanNotDelete  mylock  do not delete  myrg
```

knack's algorithm (`knack/output.py`, `_TableOutput`):
- `SKIP_KEYS = ['id', 'type', 'etag']` — dropped unconditionally.
- Values that are `list`/`dict`/`set`, or `None`, are dropped.
- Keys sorted alphabetically (`should_sort_keys` is true here: no `table_transformer`, no
  `--query`).
- Header = first character uppercased only (`resourceGroup` → `ResourceGroup`).
- A non-list result is wrapped in a single-element list, so `show` and `list` render
  identically.

Consequence: **the column set is not fixed.** Subscription-scoped locks render
`Level Name Notes`; RG- and resource-scoped locks add `ResourceGroup`. A null in one row of
a multi-row list yields a blank cell; a column null across *every* row disappears entirely.

Verified empirically against real knack 0.14.0 + tabulate 0.10.0, and confirmed against the
shipping CLI: no lock command registers a `table_transformer`
(`resource/commands.py:306,323,343,578`), so all four groups fall through to this generic
formatter.

### Renderer placement

`renderTable` goes in **`pkg/output`** as a generic formatter beside `renderTSV`, wired into
`PrintFormatted`'s switch as `case "table":`.

This fixes a live inconsistency: `pkg/output/output.go:73` already advertises
`(use json, table, or tsv)` while rejecting `table`.

**Blast radius, accepted:** commands that today error on `-o table` will start rendering.
`internal/role`, `internal/quota`, `internal/pim`, and `internal/disk/encryptionset` are
unaffected — each hand-renders with `text/tabwriter` and maps `table`→`json` before reaching
`PrintFormatted` (`internal/role/list.go:101-106`). Their tables use UPPERCASE headers with
no rule row and are left untouched; `lock` is deliberately the new azure-cli-faithful
precedent, not a migration of the others.

## `--scope` (divergence, additive)

The Go SDK exposes a `*ByScope` client family (`CreateOrUpdateByScope`, `DeleteByScope`,
`GetByScope`, `NewListByScopePager`) that collapses all three scope levels into one call and
additionally reaches **management groups** — impossible in azure-cli. Python has no
equivalent.

`--scope` short-circuits the entire validator tree and routes to `*ByScope`. Purely additive:
every azure-cli invocation keeps working, so drop-in compatibility is preserved. Precedent:
commit `8c9eae3` added `--scope` to `account get-access-token`; `internal/role/assignment`
uses `--scope` long-only.

## File Layout

```
internal/lock/commands.go   NewLockCommand(), NewAccountLockCommand(),
                            NewGroupLockCommand(), NewResourceLockCommand()
internal/lock/client.go     resolveSubscription + newLocksClient, shaped after
                            internal/resource/client.go:15-36; honors --subscription
internal/lock/scope.go      flag registration per group + the validator tree
internal/lock/lockid.go     parseLockID — the two-/providers/ regex
internal/lock/record.go     lockRecord + toLockRecord — flatten + resourceGroup injection
internal/lock/create.go
internal/lock/delete.go
internal/lock/list.go
internal/lock/show.go
internal/lock/update.go
internal/lock/scope_test.go
internal/lock/lockid_test.go
internal/lock/record_test.go
pkg/output/output.go        + renderTable, + case "table"
pkg/output/output_test.go   + renderTable tests
```

Registration in `cmd/az/main.go`: import `internal/lock`, add `lock.NewLockCommand()` to the
`rootCmd.AddCommand(...)` block. The three siblings mount under the existing `account`,
`group`, and `resource` groups.

`go.mod`: add `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks`
(**no version suffix in the import path**, unlike `armcompute/v6`). armlocks pins older
azcore/azidentity than this repo; MVS keeps the newer ones, no conflict. Then `go mod tidy`
and `make build` (never plain `go build`).

## SDK Mapping

Client: `armlocks.NewManagementLocksClient(subscriptionID, cred, nil)` — subscription ID is
bound at construction, never a method argument.

**API version divergence.** azure-cli pins locks to `2016-09-01`
(`azure/cli/core/profiles/_shared.py:175`); armlocks v1.2.0 is generated against
`2020-05-01`. We therefore talk to ARM on a different wire version than azure-cli does.

The one visible consequence: `2020-05-01`'s `ManagementLockObject` carries a `SystemData`
field that `2016-09-01` has no concept of. Because `record.go` flattens into an explicit
struct, we simply never emit `systemData`, and the JSON stays azure-cli-shaped. Do not
"helpfully" add it — that would diverge from az output.

Verified against the module itself (`go doc`), not documentation:
`LockLevelCanNotDelete = "CanNotDelete"`, `LockLevelReadOnly = "ReadOnly"`,
`LockLevelNotSpecified = "NotSpecified"`; `ManagementLockProperties{Level *LockLevel,
Notes *string, Owners []*ManagementLockOwner}`; `ManagementLockOwner{ApplicationID *string}`;
and every `List*Options` carries exactly one field, `Filter *string`.

| verb | subscription | resource group | resource |
|---|---|---|---|
| create | `CreateOrUpdateAtSubscriptionLevel(ctx, name, params, nil)` | `CreateOrUpdateAtResourceGroupLevel(ctx, rg, name, params, nil)` | `CreateOrUpdateAtResourceLevel(ctx, rg, ns, parent, rtype, rname, name, params, nil)` |
| delete | `DeleteAtSubscriptionLevel(ctx, name, nil)` | `DeleteAtResourceGroupLevel(ctx, rg, name, nil)` | `DeleteAtResourceLevel(ctx, rg, ns, parent, rtype, rname, name, nil)` |
| show | `GetAtSubscriptionLevel(ctx, name, nil)` | `GetAtResourceGroupLevel(ctx, rg, name, nil)` | `GetAtResourceLevel(ctx, rg, ns, parent, rtype, rname, name, nil)` |
| list | `NewListAtSubscriptionLevelPager(&…{Filter})` | `NewListAtResourceGroupLevelPager(rg, &…{Filter})` | `NewListAtResourceLevelPager(rg, ns, parent, rtype, rname, &…{Filter})` |
| update | Get → mutate → CreateOrUpdate at the same scope | ″ | ″ |

`params = armlocks.ManagementLockObject{Properties: &armlocks.ManagementLockProperties{Level: to.Ptr(lvl), Notes: notesPtr}}`.
`Level` is required. Only the four `List*Options` structs carry `Filter *string`; create/
delete/get option structs are empty — pass `nil`.

## Update Semantics

`update` is a true partial merge via read-modify-write. `_update_lock_parameters` mutates
only non-nil fields, so omitted fields are preserved.

**This means `--notes ""` (clear the notes) and omitting `--notes` (preserve them) are
different operations.** In Go that requires `cmd.Flags().Changed("notes")`, **not**
`notes != ""`. Same for `--lock-type`. Getting this wrong makes notes unclearable.

## Testing

Table-driven unit tests on pure helpers, matching the repo's existing ceiling (there are no
network, httptest, or mock tests anywhere in this codebase). Verbs stay untested.

- `scope_test.go` — all three scope branches, every validation error, the namespace split,
  the ≥3-segment rejection, and per-group flag subsets.
- `lockid_test.go` — subscription/RG/resource/child IDs, both `resourcegroups` casings,
  trailing garbage, unparseable input, empty-resource-name rejection.
- `record_test.go` — flatten, `owners: null`, `resourceGroup` present only when RG-scoped.
- `pkg/output/output_test.go` — knack column selection, the null-drop, alphabetical order,
  header casing, the rule row, and the shifting column set.

Use the `newSelectorCmd()` pattern from `internal/resource/resolve_test.go:119-149`
(`--subscription` defaulted to `"test-sub"` keeps the resolver off disk).

Baseline before changes: `make test` exit 0.

## Divergences from azure-cli — Summary

Each is deliberate:

1. Error strings rewritten in repo house style (incl. the precheck's messages).
2. `--scope` added — additive, reaches management groups.
3. `--ids` hard-errors on an unparseable ID instead of exiting 0 silently.
4. `--resource-type` with ≥3 segments errors instead of silently malforming the ARM path.
5. A lock ID with an empty resource name errors instead of crashing.
6. `az account lock` accepts the inherited global `--subscription`. azure-cli is the *only*
   lock group to omit it there, meaning `az account lock` can only target the active
   subscription. Un-inheriting a cobra persistent flag fights the framework, and no
   azure-cli script passes `--subscription` to `account lock` (it errors), so accepting it
   cannot break parity.
7. Legacy `2015-01-01` dead branches in Python's `get_lock`/`update_lock` are not ported —
   they crash or silently skip the update if reached.

## Risks

- **`az lock list` is cumulative.** ARM returns locks at the scope *and all ancestor
  scopes* — `az lock list -g mygroup` includes subscription locks. There is no CLI-side
  filtering; the only escape is `--filter-string "atScope()"`, passed verbatim to the SDK
  with no validation. Document in the flag help; note shells need the quotes (bare
  `atScope()` is a zsh/bash syntax error).
- **`create` is create-or-update** — the same name at the same scope silently overwrites,
  and `create` does not run the precheck.
- **Forgetting `record.go`** breaks every azure-cli `--query`. Highest-probability porting
  mistake.
- **`SilenceUsage` is unset repo-wide** (except aks), so each of ~8 validation errors will
  dump full cobra usage alongside `Error: ...`. Set `SilenceUsage: true` on the lock
  commands.
- `notes` max 512 chars is ARM-enforced only; Python adds no client-side check. Match it.
