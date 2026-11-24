# az login: tenant-first selection and inline MFA tenant sign-in

## Purpose

After sign-in, `az login` must choose one default subscription out of every tenant the user can reach. Two problems come up in large estates:

1. A flat list of dozens of subscriptions across many tenants is hard to scan.
2. Some tenants refuse the silent token the initial sign-in produced (MFA / Conditional Access). Their subscriptions cannot be listed until the user signs in to that tenant.

## Difference from Python azure-cli

| | Python azure-cli | This CLI |
|---|---|---|
| Picker | One flat numbered table of subscriptions and tenants, shown in an interactive terminal and skipped when `--subscription` matches exactly one subscription (`profile/_subscription_selector.py`, `profile/custom.py`) | Flat picker for 10 or fewer subscriptions; tenant picker, then subscription picker, above that or with `--tenant-selection` |
| `--tenant-selection` | Does not exist | Forces the two-step picker |
| Tenants needing interaction | Skipped with a warning to run `az login --tenant TENANT_ID` (`azure-cli-core/_profile.py`); every tenant where authentication failed is listed under that hint | MFA (`AADSTS50076`), sign-in-frequency expiry (`AADSTS70043`) and broker interaction-required statuses are listed in the picker; choosing one starts an interactive sign-in inline. Any other failure (e.g. `AADSTS50079`, `AADSTS50158`, `AADSTS65001`, `AADSTS53003`) is dropped with a warning. With `--use-azure-cli`, choosing such a tenant only retries the borrowed Azure CLI token; run the Python `az login --tenant` first |
| `--tenant`, `--subscription` | Present; `--subscription`/`-s` is case-insensitive, and the selector still shows when several subscriptions match; `--tenant` accepts any verified domain (resolved via the OpenID configuration endpoint) | Present (`--tenant` narrows discovery to the tenant ID or its default domain only; `--subscription` skips the picker, is case-sensitive, has no `-s` short form, and the first match wins) |

Not implemented here, though Python has them: `--use-device-code`, service principal, managed identity, `--allow-no-subscriptions`, `--scope`. The browser-flow banner in `login.go` still mentions `--use-device-code`; that flag does not exist. `az login --help` (`commands.go`) likewise says "device code flow", which is not implemented.

## Flow

```
Login (internal/auth/login.go)
  |
  +- pick auth mode (azure-cli | broker | browser), azure.BaseCredential()
  +- azure.DiscoverAllSubscriptionsWithAuth  -> []TenantInfo
  |     per tenant: list subscriptions silently
  |       needsInteractiveAuth(err)  -> TenantInfo{NeedsMFA: true}
  |       subs found                 -> TenantInfo{Subscriptions}
  |       other error / none         -> logged, tenant dropped
  +- --tenant?  filterTenants (ID or default domain, case-insensitive)
  +- split into activeTenants / mfaTenants
  |
  +- --subscription?  resolveSubscription (exact ID or name) ---------+
  +- forceTenantSelection || subs > subscriptionThreshold (10)?       |
  |     yes: promptForTenant -> [NeedsMFA: authenticateMFATenant]     |
  |          -> promptForSubscriptionInTenant                          |
  |     no:  promptForSubscriptionFlatWithMFA                         |
  |          (MFA tenants are placeholder rows at the bottom;          |
  |           selecting one authenticates, then re-renders the list)   |
  +- mark IsDefault, config.Save(Profile) <----------------------------+
```

## Key code

- `internal/auth/commands.go` `NewLoginCommand`: flags `--tenant-selection` (bool), `--tenant`/`-t`, `--use-azure-cli`. `--subscription` is the root persistent flag (`cmd/az/main.go`).
- `internal/auth/login.go`
  - `subscriptionThreshold = 10`; `useTenantSelection := forceTenantSelection || len(allSubscriptions) > subscriptionThreshold`. The count covers active tenants only, not MFA tenants.
  - `filterTenants`, `resolveSubscription`: pure helpers.
  - `promptForTenant`: promptui select; MFA tenants shown with a lock icon and "(requires sign-in)", others with subscription count.
  - `promptForSubscriptionInTenant`: second step.
  - `promptForSubscriptionFlatWithMFA`: flat list with a searcher over name and tenant name. Recurses after an MFA sign-in with the resolved tenant moved from `mfaTenants` to `activeTenants`.
  - `authenticateMFATenant`: calls `azure.AuthenticateForTenant`, then `azure.DiscoverTenantSubscriptions`, and returns a `TenantInfo` with `NeedsMFA: false`.
- `pkg/azure/discovery.go`
  - `TenantInfo.NeedsMFA`, set during discovery when `needsInteractiveAuth(err)` is true: broker interaction-required status, `AADSTS50076` (MFA) or `AADSTS70043` (sign-in frequency refresh token expiry).
  - `AuthenticateForTenant`: by mode. azure-cli mode retries a tenant-scoped Azure CLI credential; broker mode uses `NewBrokerCredential(tenantID, true)`; browser mode runs an MSAL `AcquireTokenInteractive` against `login.microsoftonline.com/<tenant>` with the shared cache.

## Decisions

- Threshold of 10 keeps small estates one step; the tenant step only appears when the flat list stops being scannable. `--tenant-selection` exists for users who want it regardless.
- MFA tenants are surfaced rather than skipped (Python's behaviour) so the user does not need a second `az login --tenant` round trip. The cost is that the picker offers entries whose subscription count is unknown.
- `--subscription` bypasses all prompts, including tenant selection. If it does not match and MFA tenants exist, the error says to run `az login` without `--subscription` to sign in to them. MFA tenants are never signed in automatically.
- `--tenant` filters after discovery, so the initial sign-in still happens once against the default authority. It also filters before the threshold check, so a filtered tenant with more than 10 subscriptions, or any use with `--tenant-selection`, shows a one-entry tenant picker before the subscription picker.

## Edge cases and limitations

- Error when no active tenants and no MFA tenants exist: "no tenants with subscriptions found". Tenants where listing failed for other reasons are dropped, with a warning.
- Two-step path: subscriptions from an MFA tenant signed in during the picker are appended to the saved profile.
- Flat path: the recursive call does not update the caller's `activeTenants`, so only the selected subscription from a newly signed-in tenant is added to the saved profile (`Login` appends it if missing); that tenant's other subscriptions are not saved.
- In azure-cli mode there is no browser flow; if the tenant still needs interaction the error surfaces from `AuthenticateForTenant`/discovery.
- Cancelling a picker returns an error (`failed to select tenant` / `failed to select subscription`).
- Pickers need a TTY (promptui); use `--subscription` for non-interactive use. Browser auth may still open.

## Tests

- `internal/auth/login_test.go`: `TestResolveSubscription`, `TestFilterTenants`.
- `pkg/azure/discovery_test.go`: `TestNeedsInteractiveAuth`.
- The prompts and `AuthenticateForTenant` have no automated tests; verify by hand against a tenant with MFA Conditional Access.
