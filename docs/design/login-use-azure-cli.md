# az login --use-azure-cli

Status: implemented. Python azure-cli equivalent: none.

## Purpose

`az login --use-azure-cli` makes this CLI borrow tokens from the Python
`az` instead of signing in itself. The motivation is device-based Conditional
Access: some tenants only accept the sign-in that the Python CLI performs
through the WAM broker on Windows. If the Python CLI is already signed in, this
binary can reuse that session.

Python azure-cli has no counterpart. The mode exists to delegate to Python's own
sign-in, and Python has no flag to borrow tokens from another CLI (see the login
arguments in its `profile/__init__.py`).

## How it works

```
az login --use-azure-cli
  internal/auth/commands.go   --use-azure-cli (bool, default false) -> Login(..., useAzureCLI)
  internal/auth/login.go      config.DeleteKeepAzureCLICache()   (keeps ~/.azure/msal_token_cache.json)
                              azure.SetAuthMode("azure-cli")      (override, no profile on disk yet)
                              BaseCredential() -> AzureCLICredential
                              DiscoverAllSubscriptionsWithAuth, pick subscription
                              config.Save(Profile{AuthMode:"azure-cli", AuthenticationRecord:nil})

later commands
  azure.TenantCredential(tenant, rec)
    authMode() reads Profile.AuthMode == "azure-cli"
    -> checkAzureCLIOnPath()
    -> azidentity.NewAzureCLICredential({TenantID})   -> shells out to Python `az account get-access-token`
```

Key pieces:

- `pkg/azure/authmode.go`
  - `AuthModeAzureCLI = "azure-cli"`, `AuthModeDefault = "default"`.
  - `SetAuthMode` / `authMode()` / exported `AuthMode()`: a process-level override wins; otherwise the mode comes from `config.Load().AuthMode`.
  - `BaseCredential()` (tenant discovery and first sign-in) and `TenantCredential()` (per-tenant) return `azidentity.AzureCLICredential` in this mode.
  - `checkAzureCLIOnPath()`: pre-flight check.
- `internal/auth/login.go`: `Login` chooses the mode, prints "Borrowing tokens from the Azure CLI (az). Make sure you are already signed in with 'az login'.", and saves the profile.
- `pkg/config/profile.go`: `Profile.AuthMode` (`json:"authMode,omitempty"`), `DeleteKeepAzureCLICache()` / `deleteProfile(keepMSALCache)`.
- `internal/network/bastion/aad.go`: `requestSSHCertWithToken` reads the Python CLI's cache directly.

## Decisions

- **Mode is persisted in the profile.** Every later command resolves it through `config.Load()`, so no flag is needed after login.
- **Explicit `AuthModeDefault`.** Login sets `default` rather than `""` for a normal login so a stale profile left behind by a failed delete cannot leak `azure-cli` into a plain `az login`. `""` means "no override".
- **Keep `msal_token_cache.json` at login.** On macOS/Linux, where Python's cache is plaintext, that file is the Python CLI's own cache. The normal `config.Delete()` removes it, which would sign Python out. On Windows, Python keeps an encrypted `msal_token_cache.bin` (DPAPI) that this CLI neither reads nor deletes. `DeleteKeepAzureCLICache` still removes the profile and `msal_http_cache.bin`.
- **No authentication record.** `profile.AuthenticationRecord` is left nil in this mode, since tokens are fetched from Python on every request instead of being silently refreshed from our own MSAL cache.
- **Fail fast on PATH.** `checkAzureCLIOnPath` errors if `az` is not on PATH, or if `az` resolves (after `filepath.EvalSymlinks`) to this same binary. This project also installs as `az` (Homebrew `az-go`), and otherwise `AzureCLICredential` would shell out to ourselves and fail deep inside token acquisition.
- **Bastion SSH reads Python's cache.** `az network bastion ssh --auth-type aad` needs a refresh-token redemption with cert-request claims, which `AzureCLICredential` cannot do. When `azure.AuthMode() == azure-cli`, `requestSSHCertWithToken` always uses `~/.azure/msal_token_cache.json` (hardcoded `~/.azure`, so `AZURE_CONFIG_DIR` is not honoured; ignoring `AZ_SESSION`'s per-session file), and `activeLogin` ignores `AuthenticationRecord` for account choice, selecting by default-subscription user name and tenant via `selectAccount`. This works only when Python's cache is plaintext (macOS/Linux default); on Windows, with the default encrypted cache or the WAM broker, it does not work.

## Edge cases and limitations

- The Python `az` must be installed, ahead of this binary on PATH, and already signed in. Login does not run Python's login for you.
- Each token request spawns the Python CLI, so commands are slower than in the default mode.
- No broker fallback: the broker retry in `Login` applies only when `authMode == broker`.
- Bastion AAD SSH: missing cache file or no cached accounts gives an error telling the user to sign in with the Python CLI (not this binary). On Windows with Python's default encrypted `msal_token_cache.bin` or WAM, this error appears even when Python is signed in.
- Signing in again with plain `az login`, or running `az logout`, here deletes `~/.azure/msal_token_cache.json` (unless `AZ_SESSION` is set, in which case only `msal_token_cache-<session>.json` is removed). On macOS/Linux that file is shared with the Python CLI, so this also signs it out. On Windows, Python's encrypted `msal_token_cache.bin` is not touched. A plain `az login` replaces the profile with the default mode (`authMode` unset), or `broker` when the WAM broker is available and sign-in through it succeeds.

## Tests

No test targets azure-cli mode directly. `internal/auth/login_test.go` covers `resolveSubscription` and `filterTenants` (shared with this flow). `internal/network/bastion/aad_test.go` covers the account-selection helpers. `pkg/config/profile_test.go` does not cover `DeleteKeepAzureCLICache`. A gap worth filling.
