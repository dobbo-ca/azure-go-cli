# Privileged Identity Management (`az pim`)

`az pim` lists your eligible and currently-active Azure resource role assignments (e.g. eligible Contributor on a customer subscription) and activates them on demand.

Entra group PIM (`az pim activate group`, `az pim list --type group`) was removed: Microsoft retired the legacy PIM API it used (`api.azrbac.mspim.azure.com`) for CLI tokens around 2026-05-16, and the Microsoft Graph replacement needs a custom app registration because the Azure CLI client cannot get the `PrivilegedAccess` scopes ([netr0m/az-pim-cli#121](https://github.com/netr0m/az-pim-cli/issues/121)). Activate group memberships in the Azure portal. Entra ID directory roles (e.g. Global Reader) are out of scope.

## Quick reference

```bash
# List eligible + currently-active assignments
az pim list
az pim list --type resource
az pim list --output json

# Activate a resource role (ticket + justification + duration required)
az pim activate resource \
  --role Contributor \
  --scope "Acme Corp/Acme Production" \
  --ticket Jira:TEC-1234 \
  --justification "Investigating incident INC-9999" \
  --duration 60
```

Bare command lines prompt for any missing required flags on a TTY. Use `--no-input` (or pipe stdin) to require explicit flags — missing values then produce `missing required flag: --role, --ticket, ...` rather than blocking on a prompt.

## Activating customer-environment access

The primary workflow is requesting elevated access to a specific customer's Azure environment, doing the work, and letting the elevation expire: your account holds an eligible `Contributor` (or similar) directly on the customer's subscription, and you activate it with `az pim activate resource`.

When juggling several customers in parallel terminals, combine PIM with `AZ_SESSION` to keep token caches isolated:

```bash
# Terminal 1 — Acme work
export AZ_SESSION=acme
az login
az pim activate resource --role Contributor --scope "Acme Corp/Acme Production" \
  --ticket Jira:TEC-1234 --justification "ticket investigation" --duration 60 \
  --set-subscription
az aks bastion -n acme-cluster -g acme-rg --bastion /subscriptions/.../bastionHosts/acme-bastion

# Terminal 2 — Beta work (separate MSAL cache, separate active subscription)
export AZ_SESSION=beta
az login
az pim activate resource --role Contributor --scope "Beta Inc/Beta Production" \
  --ticket Jira:TEC-1250 --justification "scheduled maintenance" --duration 90
```

`az pim` inherits `AZ_SESSION` automatically through the shared MSAL credential — there is no PIM-specific flag for session isolation. See the [`AZ_SESSION` section](user-guide.md#parallel-sign-ins-with-az_session) of the user guide for how the per-session profile and token cache files are named.

`--set-subscription` is opt-in on `az pim activate resource`. When set, the activated subscription becomes the session's default by rewriting `~/.azure/azureProfile[-<session>].json` — so the next command in the same shell targets the customer environment without an extra `az account set`. Off by default to avoid surprising side-effects.

## Scope resolution

`--scope` accepts four forms, resolved in order. The first matching form wins.

1. **Full ARM path** (`/subscriptions/.../resourceGroups/...`) — used verbatim. Best for scripts and CI where the path is already known.

   ```bash
   az pim activate resource --role Contributor \
     --scope /subscriptions/aaaa1111-0000-0000-0000-000000000000 \
     --ticket Jira:TEC-1234 --justification "..." --duration 60
   ```

2. **Subscription UUID** — expanded to `/subscriptions/<UUID>`. Convenient when pasting from `az account list` output.

   ```bash
   az pim activate resource --role Contributor \
     --scope aaaa1111-0000-0000-0000-000000000000 \
     --ticket Jira:TEC-1234 --justification "..." --duration 60
   ```

3. **`tenant-name/subscription-name[/resource-group]`** — looked up against your eligible-assignments list and local subscription cache. Use this when a subscription name is ambiguous across tenants (e.g. two customers both have an "Acme Production" sub):

   ```bash
   az pim activate resource --role Contributor \
     --scope "Acme Corp/Acme Production" \
     --ticket Jira:TEC-1234 --justification "..." --duration 60
   ```

   For a resource-group scope, append a third segment:

   ```bash
   az pim activate resource --role Owner \
     --scope "Acme Corp/Acme Production/network-rg" \
     --ticket Jira:TEC-1234 --justification "..." --duration 60
   ```

4. **Bare subscription name** — accepted only if it matches exactly one entry in your eligible list. Most ergonomic when subscription names are already globally unique to you.

   ```bash
   az pim activate resource --role Contributor \
     --scope "Acme Production" \
     --ticket Jira:TEC-1234 --justification "..." --duration 60
   ```

If a bare name matches in two tenants, you get an error listing the candidates and a hint to disambiguate with form 3.

You can also omit `--scope` entirely when `--role` uniquely identifies a single eligible assignment across all your tenants.

## Validation pre-flight

`az pim activate resource` calls Azure's validation endpoint before submitting the activation. If validation fails, the CLI exits non-zero with:

```
Error: Azure rejected the activation request during validation; check role, scope, ticket, and duration
```

Common reasons:

- **Duration exceeds the policy maximum.** Each PIM-enabled role has a max activation duration configured by the tenant admin (commonly 1, 4, or 8 hours). Activating for longer than allowed is rejected.
- **Ticket required but missing or malformed.** Some PIM policies require both a ticket system and number. `--ticket Jira:TEC-1234` parses to system=`Jira`, number=`TEC-1234`; a value without `:` parses as a number with empty system, which fails policy checks that require a system.
- **Role not actually eligible.** If your role assignment was removed since the last `az pim list`, validation surfaces the staleness.
- **Justification too short.** Some policies require a minimum justification length.

If validation passes but Azure requires approval before activation, the request is submitted and the response status comes back as `Pending*` (e.g. `PendingApproval`). The CLI exits 0 with `Pending approval; request <id>` so scripts can detect the deferred state without treating it as a failure.

## Vendored client

The PIM HTTP client is vendored from [`netr0m/az-pim-cli`](https://github.com/netr0m/az-pim-cli) (MIT-licensed) into `internal/pim/vendor/`. The original commit is pinned (`63d8f2ce47be44d61d15e92d964a1b35558e29f5`, release 1.14.0) and re-syncing instructions live in `internal/pim/vendor/README-VENDORED.md`.

Local modifications applied on top of the upstream:

1. All `os.Exit(1)` error paths converted to returned errors so cobra handles them through its standard error printer rather than killing the process.
2. `log/slog` calls replaced with the project's `pkg/logger`. Failed HTTP requests log at debug level only, because the returned error is printed by the command; the upstream `slog.Error` line printed every failure twice.
3. The upstream `AzureClient.GetAccessToken` (which built its own `azidentity.AzureCLICredential`) was removed. Tokens are supplied by the `TokenSource` adapter in `internal/pim/tokencred.go`, which wraps our existing `pkg/azure.GetCredential()` and inherits its MSAL cache (and therefore `AZ_SESSION` isolation).
4. The upstream `pkg/common.Error` type was absorbed into `internal/pim/vendor/common.go` so we don't import the upstream Go module.
5. Additional defensive fixes: the HTTP error branch no longer panics on a nil `*http.Response`; `errors.Unwrap` reaches the underlying error from `ScheduleInfo` wrappers; the `IsValidationOnly = true` mutation that the validate path applies to the activation request is now isolated by a value copy so the subsequent real submission is not a dry-run; the `TestParseDateTime` upstream test was patched to compute its expected timezone offset from the parsed date rather than `time.Now()`.

## Tokens and tenants

`az pim` requests every token for the tenant of the active subscription (`az account show`), so it lists and activates assignments in that tenant only. To work in another tenant, run `az account set` or use a separate `AZ_SESSION`. All calls go to Azure Resource Manager with a `https://management.azure.com/.default` token.

If the cached sign-in was revoked or expired (for example after a password reset, AADSTS50173), every command — not only `az pim` — fails with the AAD reason and asks you to run `az login` (in `--use-azure-cli` mode the Python Azure CLI reports it instead). It does not open a browser mid-command, which matches the Python Azure CLI.

## Limitations

- **Tenant display names depend on prior `az login` against the tenant.** If you have PIM eligibility in a tenant you have never logged into within the current session, `az pim list` shows the tenant UUID instead of a friendly name, and the `tenant-name/...` form of `--scope` will not resolve for that tenant. Run `az login` once with that tenant active to populate the cache.
- **Entra group memberships and Entra ID directory roles** are not supported. See the top of this page.
- **Interactive picker is not yet wired.** On a TTY, missing values are filled via free-text prompts. The numbered picker over the eligible-assignments list is implemented in `internal/pim/prompt.go` but not yet invoked by the activate commands — a follow-up will wire it.
- **Post-submission `Failed` status is reported but does not change the exit code.** The pre-flight validation catches most failures before submission, but if Azure returns a 200 response with a `Failed` body status, the CLI prints the failure and exits 0. HTTP-level errors (4xx/5xx) propagate correctly and exit non-zero.
