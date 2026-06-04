# `az pim` — Privileged Identity Management

## Summary

Add a new command group `az pim` that lists eligible/active Azure PIM assignments and activates them on demand. v1 covers two assignment types:

- **Azure resource role assignments** (e.g. eligible Contributor on a customer subscription)
- **Entra ID group memberships** (e.g. eligible member of `customer-acme-admins`)

Entra ID directory roles are intentionally out of scope for v1.

The PIM client code is vendored from [`netr0m/az-pim-cli`](https://github.com/netr0m/az-pim-cli) (MIT) into `internal/pim/vendor/`. We own and modify it locally. Authentication uses our existing `pkg/azure` credential layer, so `AZ_SESSION` isolation applies to PIM the same way it applies to every other command — no new env vars, no new caches.

## Motivation

We support multiple customer Azure tenants. Day-to-day access to a customer environment requires activating a PIM role (resource role or group membership), supplying a justification, and — for resource roles — a ticket reference. The official `az` CLI does not expose PIM activation. `netr0m/az-pim-cli` does, but it is a separate binary with a separate token cache, so it cannot participate in our `AZ_SESSION`-based session isolation. Integrating PIM into this CLI gives us:

- One binary, one auth flow, one session model.
- Activations scoped to whichever `AZ_SESSION` is set — no cross-customer token bleed.
- Our existing cobra UX conventions (JSON-default output, `--output table` opt-in, TTY-aware prompting).

## Non-goals

- Entra ID directory roles (e.g. Global Reader). May come later; not in v1.
- Approval workflows for approvers (`pim approve` etc.).
- Scheduled / future-dated activations. v1 activates immediately.
- Polling for activation status after submission. The command submits and reports the response; if Azure reports `Pending`, the user is told it is pending and the request ID is printed.
- A local profile / config file. PIM activation is stateless on the client side; no local persistence.

## User-facing surface

### Command tree

```
az pim list [--type resource|group] [--output json|table]

az pim activate resource
    --role NAME
    [--scope SCOPE]
    --ticket SYSTEM:NUMBER
    --justification "..."
    --duration MINUTES
    [--set-subscription]
    [--no-input]
    [--output json|table]

az pim activate group
    --name NAME
    --justification "..."
    --duration MINUTES
    [--no-input]
    [--output json|table]
```

### Behaviour

**`az pim list`** — combined eligible + currently-active assignments, one row per assignment.

Default table:

```
TYPE      TENANT       SUBSCRIPTION       NAME                    STATUS
resource  Acme Corp    Acme Production    Contributor             Eligible
resource  Acme Corp    Acme Dev           Owner                   Active (expires 15:42 UTC)
resource  Beta LLC     Beta Production    Reader                  Eligible
group     Acme Corp    —                  customer-acme-admins    Eligible
```

- `TENANT` and `SUBSCRIPTION` are resolved against the local `~/.azure/azureProfile[-<session>].json` cache and tenant discovery data. If a tenant has never been logged into in this session, its tenant UUID is shown in `TENANT` and `tenant-name/...` lookups will not resolve for it (limitation, documented in README).
- `SUBSCRIPTION` is intentionally empty for group rows — PIM does not couple group activations to a subscription, and resolving a group's RBAC assignments requires N additional ARM calls. Not done in v1.
- `--type` filters the view to one type.
- `--output json` emits the combined slice; each item carries a `status` field.

**`az pim activate resource`** — required flags: `--role`, `--ticket`, `--justification`, `--duration`. `--scope` is required *unless* `--role` matches exactly one eligible resource assignment, in which case the unambiguous scope is used.

- `--ticket` accepts `SYSTEM:NUMBER` (e.g. `Jira:TEC-1234`). Split on the first `:` into the API's `ticketSystem` and `ticketNumber` fields. If no `:` is present, the whole value goes to `ticketNumber` and `ticketSystem` is empty.
- `--scope` accepts (resolved in this order, first match wins):
  1. Full ARM path (`/subscriptions/.../resourceGroups/...`) — used verbatim.
  2. Subscription UUID — expanded to `/subscriptions/<UUID>`.
  3. `tenant-name/subscription-name` — looked up in eligible list ∪ local cache.
  4. `subscription-name` alone — accepted if unambiguous across all eligible assignments. Ambiguous matches return an error listing the candidates.
- `--set-subscription` rewrites `~/.azure/azureProfile[-<session>].json` to set the activated subscription as the default (off by default).

**`az pim activate group`** — required flags: `--name`, `--justification`, `--duration`. `--name` is the group display name; resolved against your eligible group assignments before the API call so we error early on typos. Ambiguous matches (two eligible groups with the same display name) return an error listing the candidates' group IDs.

**Interactive prompting** — on a TTY, any missing required flag triggers a prompt. The prompt for "which assignment" is a numbered picker over your eligible list (filtered to the right type). `--no-input` forces non-interactive; non-TTY contexts (piped stdin, CI) are treated the same. Missing required values under `--no-input` → error, exit non-zero.

**Defaults** — `--duration` has no default. The user must specify it (per the PIM policy of every customer we currently support). netr0m's `DEFAULT_DURATION_MINUTES=480` is ignored.

**Output** — JSON by default for `activate` (project convention). `--output table` collapses to one line per activation: `Activated <role|group> on <tenant>/<subscription|—>; expires <RFC3339>` or `Pending approval; request <id>`.

## AZ_SESSION integration

No new code. `pkg/azure.GetCredential()` already returns a `FileCachedCredential` whose cache path keys off `AZ_SESSION`. PIM tokens flow through that credential, so:

- `AZ_SESSION=acme az pim list` uses the acme MSAL cache.
- `AZ_SESSION=beta az pim activate resource ...` uses the beta MSAL cache.
- Unset `AZ_SESSION` uses the default cache, exactly as today.

There is no implicit coupling between `AZ_SESSION` value and PIM behaviour; the user chooses whether to use session isolation. The README will document this explicitly.

## Architecture

### Package layout

```
internal/pim/
├── commands.go          # cobra: pim, pim list, pim activate, pim activate {resource,group}
├── list.go              # `az pim list` — combined eligible+active table/JSON
├── activate_resource.go # `az pim activate resource` — flags, validation, call client
├── activate_group.go    # `az pim activate group`   — flags, validation, call client
├── scope.go             # --scope resolution (forms 1–4)
├── prompt.go            # interactive prompts for missing values (TTY check)
├── tokencred.go         # adapter: wraps pkg/azure credential → vendor's Client interface
└── vendor/              # netr0m vendored code
    ├── LICENSE          # MIT, attribution preserved
    ├── README-VENDORED.md  # source repo + commit hash + list of local modifications
    ├── client.go        # GetEligible*, Validate*, Request*
    ├── const.go         # endpoint URLs, role-type constants (DEFAULT_DURATION removed)
    ├── models.go        # request/response structs
    ├── utils.go         # CreateRequest helpers, Is{Failed,Pending,OK}
    └── common.go        # absorbed pkg/common types we need
```

Total vendor footprint ≈ 900 LOC non-test.

### Vendoring modifications

The vendored code is altered in two mechanical, file-bounded ways during the vendor step:

1. **`os.Exit(1)` → returned errors.** Their client exits on every error; we convert each error path to return `(*T, error)` or `error`. Surface change: every exported `Get*`, `Validate*`, `Request*` function gains an `error` return.
2. **`slog` → `pkg/logger`.** Same call sites, our logger interface.

`README-VENDORED.md` records the upstream commit hash and lists these modifications, so future syncs from upstream are auditable.

### Token wiring

`internal/pim/tokencred.go` defines:

```go
type credClient struct {
    cred azcore.TokenCredential
}

func (c *credClient) GetAccessToken(scope string) (string, error) { ... }
```

This satisfies the vendored `Client` interface (one method). The credential is obtained from `pkg/azure.GetCredential()`, which already respects `AZ_SESSION` and reuses the MSAL cache.

Two scopes are used:
- `https://management.azure.com/.default` — resource assignments (PIM/RBAC endpoints under `api.azrbac.mspim.azure.com`).
- `https://graph.microsoft.com/.default` — Entra group governance assignments.

### Scope resolution

`internal/pim/scope.go` exposes:

```go
func ResolveScope(input string, eligible []vendored.ResourceAssignment, profile *config.AzureProfile) (string, error)
```

Implements the four-form lookup described in the user-facing surface section. Pure function; no I/O; covered by table-driven tests.

### Prompting

`internal/pim/prompt.go` exposes:

```go
func PromptIfTTY(label string, validate func(string) error) (string, error)
func PickAssignment(label string, items []DisplayItem) (int, error)  // numbered picker
```

`PromptIfTTY` returns an error if stdin/stdout is not a TTY or `--no-input` is set, so the caller can decide whether the missing value is fatal.

### Activation flow

1. Parse flags. Apply `--no-input` and TTY detection.
2. Resolve `--scope` (resource) or `--name` (group) against the eligible list — surfacing an error if the user is asking for something they cannot activate.
3. Call the appropriate validate endpoint first (`ValidateResourceAssignmentRequest` for resource, `ValidateGovernanceRoleAssignmentRequest` for group); abort on validation failure with Azure's error message.
4. Submit the activation request (`RequestResourceAssignment` or `RequestGovernanceRoleAssignment`).
5. Inspect the response:
   - `OK` (active immediately) → success output.
   - `Pending` (approval required) → exit 0, print request ID and a "pending approval" message.
   - `Failed` → exit non-zero, print Azure's error.
6. If `--set-subscription` and the activation is a subscription-scoped resource role, rewrite the local profile to set that subscription as default.

## Errors

- Validation/policy rejections: surface Azure's error verbatim, exit non-zero. Do not retry.
- Ambiguous scope or group name: list candidates, exit non-zero.
- Missing required flag on non-TTY or with `--no-input`: print which flag, exit non-zero.
- Token acquisition failure: same surfacing as the rest of the CLI uses today (existing `pkg/azure` error paths).
- All vendored `os.Exit` removed; everything reaches cobra's `RunE` as a returned error, which the existing top-level error printer handles.

## Testing

- Vendor `client_test.go` and `test_data.go` from netr0m; update assertions where the `os.Exit → return error` refactor changes function signatures. They cover the API client against fixtures (no live calls).
- New tests in `internal/pim/`:
  - `scope_test.go` — table-driven for all four `--scope` forms, including the ambiguous case.
  - `ticket_test.go` — `SYSTEM:NUMBER` splitting, including the no-`:` case.
  - `list_test.go` — table formatting from a synthetic combined slice; JSON output shape.
  - `prompt_test.go` — TTY-vs-non-TTY branching using an injected `io.Reader`/`io.Writer` and a TTY-check seam.
- No live Azure calls in any test. All HTTP goes through the vendored `Client` interface and is faked.

## README

A new `### Privileged Identity Management (PIM)` section between `### Identity & Access` and `### Key Vault` in the existing features list, and a longer usage block after the AKS section. Contents:

- One-paragraph what-it-does plus attribution: "PIM client code is vendored from [netr0m/az-pim-cli](https://github.com/netr0m/az-pim-cli) (MIT). See `internal/pim/vendor/LICENSE`."
- The two command shapes with realistic examples — resource activation with ticket; group activation; `pim list`.
- `AZ_SESSION` note: PIM uses the same per-session token cache as every other `az` command, so `AZ_SESSION=acme az pim activate ...` activates against the acme session in isolation from other open shells. Cross-link to the existing `AZ_SESSION` section.
- Limitations:
  - Tenant friendly names rely on a prior `az login` in this session. Tenants we have not discovered show their UUID. Option A from design discussion; the `tenant-name/...` scope form will not resolve for un-discovered tenants.
  - Group rows in `pim list` show `—` for `SUBSCRIPTION`. PIM does not couple a group activation to a subscription; resolving a group's RBAC assignments would require N extra ARM calls per group and is not done in v1.

## Open questions for the implementation plan

None. Every interactive decision from brainstorming has been resolved. The next step (writing-plans skill) decomposes this spec into discrete implementation tasks.
