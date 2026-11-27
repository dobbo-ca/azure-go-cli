# az pim: list and activate PIM resource role assignments

Package: `internal/pim`. User docs: [docs/pim.md](../pim.md). Original design and plan: [spec](../superpowers/specs/2026-05-14-pim-design.md), [plan](../superpowers/plans/2026-05-14-pim.md).

## Purpose

`az pim list` shows eligible Azure resource role PIM assignments (current activations are not listed); `az pim activate resource` activates one. Entra group PIM was removed: the legacy governance API at `api.azrbac.mspim.azure.com` stopped accepting Azure CLI tokens around 2026-05-16, and its Microsoft Graph replacement needs a custom app registration (netr0m/az-pim-cli#121).

The Python azure-cli has no PIM command group: none in core `command_modules`, none among the azure-cli-extensions `src/` packages. Python users must call the PIM REST API through `az rest`. Here the commands run in the same binary and credential as every other command, so `AZ_SESSION` isolation applies without a separate PIM token cache.

## Data flow

```
cobra cmd (commands.go)
  list.go / activate_resource.go
     |  flags -> args struct; on TTY, Prompter fills gaps (prompt.go)
     v
  azure.GetCredential()  --> TokenSource (tokencred.go)  --> token per scope
     |                         ARM:   https://management.azure.com/.default
     v
  vendor.AzureClient (internal/pim/vendor/client.go)
     |  GetEligible*  ->  resolve scope  ->  Validate*  ->  Request*
     v
  renderActivationResult / RenderListTable / output.PrintJSON
```

## Key pieces

| Piece | File | Role |
|---|---|---|
| `NewPIMCommand` | `commands.go` | Registers `list` and `activate resource`; wired in `cmd/az/main.go`. |
| `runList`, `collectRows`, `ListRow` | `list.go` | Builds rows (`type, tenant, subscription, name, status`) from the ARM eligibility API. |
| `runActivateResource` | `activate_resource.go` | Flag validation, scope resolution, validate, submit, optional `--set-subscription`. |
| `ResolveScope`, `ScopeIndexEntry` | `scope.go` | Four-form `--scope` resolver. |
| `ParseTicket` | `ticket.go` | Splits `SYSTEM:NUMBER` on the first `:`; no colon means number only, empty system. |
| `SetDefaultSubscription` | `setsub.go` | Marks one subscription default in the profile, clears the rest. |
| `TokenSource` | `tokencred.go` | Adapts `azcore.TokenCredential` to the vendored client's token-string calls. |
| `Prompter` | `prompt.go` | TTY-gated prompts; `ErrNonInteractive` when not a TTY or `--no-input`. |
| vendored client | `vendor/` | PIM HTTP client from netr0m/az-pim-cli (MIT, pinned commit; see `vendor/README-VENDORED.md`). |

Resource roles go through ARM `roleEligibilityScheduleInstances` (filter `asTarget()`). The vendored governance-API functions (`GetEligibleGovernanceRoleAssignments` and friends) are kept to ease upstream re-syncs but are no longer called.

## Decisions

- **Vendor, don't depend.** The upstream is a single-maintainer CLI, and its own token acquisition (`azidentity.AzureCLICredential`) cannot join our session model. We removed `GetAccessToken` from it and feed tokens from `TokenSource`, which wraps `pkg/azure.GetCredential()`. Other local changes (returned errors instead of `os.Exit`, `pkg/logger` instead of `slog`, nil-response and error-unwrap fixes) are listed in docs/pim.md.
- **Validate before submit.** `activate resource` calls Azure's validation endpoint first and fail with a fixed message if it returns false. The resource path passes a value copy of the request to validation, because the vendor validate mutates `IsValidationOnly` through the shared pointer, which would turn the real submission into a dry run.
- **Four-form scope.** ARM path (verbatim), subscription UUID, `tenant/sub[/rg]`, bare subscription name (must match exactly one entry). The index is built from the eligible list plus the local profile, so only scopes the user is eligible for resolve. `--scope` may be omitted when `--role` matches exactly one eligible assignment.
- **Prompts only on a TTY.** Missing `--role/--ticket/--justification/--duration` are prompted for. With `--no-input` the command errors with `missing required flag: ...`. With a non-TTY stdin and no `--no-input`, the prompter fails first with `prompt required but running non-interactively`.
- **`--set-subscription` is opt-in** and rewrites the session profile via `config.Save`, so it follows `AZ_SESSION`. Off by default to avoid surprising side effects.
- **`list` defaults to table**, unlike the global JSON default; an explicit `--output` is honored. JSON uses `output.PrintJSON` (struct field order kept), and empty results print `[]`.
- **Pending approval is not a failure.** A `Pending*` status exits 0. With `-o table` it prints `Pending approval; request <id>`; the default JSON output shows `status: Pending...` and `requestId`.

## Edge cases and limitations

- Out of scope: Entra group memberships (removed, see Purpose), Entra directory roles, approvals (approver side), scheduled/future-dated activations, polling after submit.
- Tenant column: the tenant ID found in the local profile for that subscription, and empty if the subscription is not in the profile (`lookupTenantAndSub`). No display names are shown. The `tenant/sub` scope form matches `TenantDisplayName`, which `buildScopeIndex` also sets to the tenant ID, so in practice that form is `<tenant-id>/<subscription-name>`.
- Resource rows carry whatever `Status` the schedule instance reports.
- Role names are matched by exact display name; ambiguous role scopes error with candidate scopes.
- `SetDefaultSubscription` errors if the subscription is not in the local profile; when `--set-subscription` is given and the profile is nil or the scope has no subscription id, it silently does nothing.
- A post-submit `Failed` body status exits 0. JSON output shows `status: Failed`; table output wrongly prints `Activated <role> on <scope>`; HTTP errors exit non-zero.
- `Prompter.PickAssignment` exists but no activate command calls it yet.
- Only the global ARM cloud (`ARM_GLOBAL_BASE_URL`) is used.

## Tests

Unit tests sit next to the code in `internal/pim/*_test.go` (`scope`, `ticket`, `setsub`, `prompt`, `tokencred`, `list`, `activate_resource`) and the vendored client's tests in `internal/pim/vendor/*_test.go`. Run with `make test`.
