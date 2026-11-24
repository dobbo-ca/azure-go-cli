# AZ_SESSION: parallel sign-ins

## Purpose

`AZ_SESSION=<name>` gives each terminal its own profile and MSAL token cache, so
several terminals can stay signed in to different tenants or accounts, each with
its own default subscription, without overwriting each other.

## Difference from Python azure-cli

Python has no `AZ_SESSION`. Its nearest equivalent is `AZURE_CONFIG_DIR`, which
relocates the whole config directory (azure-cli-core `_environment.py:14`).
`AZ_SESSION` is narrower: it renames only the profile and token cache inside the
shared `~/.azure`, and it is carried into the kubeconfig written by
`az aks bastion` (see below); `az aks get-credentials` and
`az aks convert-kubeconfig` do not pin it. Python has nothing that pins its
config dir into a kubeconfig.

Apart from the devops commands, this CLI does not honor `AZURE_CONFIG_DIR`. Its
only reader is `configDir()` in `internal/devops/ado/auth.go:97`.

## How it works

There is no session abstraction. Five places read the env var directly: four
derive a file name from it (`GetConfigPath`, `GetSharedMSALCache`,
`deleteProfile`, and the bastion AAD cache lookup in `aad.go`), and
`WriteKubeconfig` pins it into the kubeconfig.

```
AZ_SESSION=foo
   |
   +-- pkg/config/profile.go  GetConfigPath()   -> ~/.azure/azureProfile-foo.json
   +-- pkg/azure/msalcache.go GetSharedMSALCache() -> ~/.azure/msal_token_cache-foo.json
   +-- pkg/config/profile.go  deleteProfile()   -> removes the two files above (logout)
   +-- internal/network/bastion/aad.go          -> msal_token_cache-foo.json if it exists

az aks bastion -> internal/aks/kubeconfig.go WriteKubeconfig()
   exec env: PATH=<exe dir>:$PATH, AZ_SESSION="foo"
   kubectl -> `az aks get-token` -> reads the same session files
```

| Piece | Location | Behaviour |
|-------|----------|-----------|
| Profile path | `pkg/config/profile.go:48-62` (`GetConfigPath`) | `azureProfile-<session>.json`, else `azureProfile.json` |
| Token cache | `pkg/azure/msalcache.go:27-29` (`GetSharedMSALCache`) | `msal_token_cache-<session>.json`, else `msal_token_cache.json` |
| Logout | `pkg/config/profile.go:226-231` (`deleteProfile`) | removes only the active session's cache unless `keepMSALCache` |
| Kubeconfig | `internal/aks/kubeconfig.go:73-80` (`WriteKubeconfig`) | adds `AZ_SESSION` to the exec env block when set |
| AAD bastion ssh | `internal/network/bastion/aad.go:81-90` | uses the session cache if the file exists, else the shared cache |

In the default MSAL sign-in mode, every credential-based command (`az pim`,
`az aks get-token`, `az aks bastion`, and so on) goes through the shared MSAL
cache or profile loader, so none needs session-specific code. The token-cache
part does not hold in azure-cli or Windows broker (WAM) mode; see the edge cases
below.

## Decisions

- **Env var, not a flag.** It must reach subprocesses (kubectl exec plugins,
  child shells) and every command without per-command plumbing.
- **Files in the shared `~/.azure`, not a new directory.** Keeps one config root
  and the Python-compatible file layout when no session is set.
- **Pin into kubeconfig.** kubectl mints tokens by running `az` as a subprocess
  from whatever shell started it; without the pin it would read the default
  session's cache. The value is written with `%q`.
- **AAD bastion falls back to the shared cache.** A session with no cache file
  of its own (for example, never logged in through this binary) uses the shared
  `msal_token_cache.json`, which is the default session's or the Python CLI's
  tokens. The session file is only preferred when it exists; in azure-cli mode it
  is never consulted.

## Edge cases and limitations

- Token-cache isolation applies to the default sign-in. With
  `az login --use-azure-cli`, tokens come from the Python CLI's single shared
  login, so only the profile and default subscription are per-session. With the
  Windows WAM broker, tokens are held by WAM; the per-session profile records
  which broker account to use, and no `msal_token_cache-<session>.json` is
  written.
- Only `az aks bastion` pins `AZ_SESSION` into a kubeconfig. `get-credentials`
  and `convert-kubeconfig` leave the exec env empty, so kubectl uses whatever
  `AZ_SESSION` the calling shell has. The bastion kubeconfig is a temp file
  deleted on exit unless `--kubeconfig <path>` is given.
- A session with no cache file of its own falls back to the shared
  `msal_token_cache.json` in `az network bastion ssh --auth-type aad`, silently
  using the default session's tokens.
- Session name is used verbatim in the file name. There is no validation or
  sanitising, so path separators or odd characters in the name are not rejected.
- Logout deletes `msal_http_cache.bin` (`pkg/config/profile.go`, after the token
  cache) regardless of session. That file is shared, so a session logout can
  clear it for other sessions (it is a cache; it is recreated).
- Unset or empty `AZ_SESSION` means the default files. A session never falls back
  to the default profile; only the AAD bastion token lookup falls back, and only
  for the token cache.
- Only profile and token cache are isolated. Other state under `~/.azure` is
  shared.
- `AZURE_CONFIG_DIR` is ignored outside devops (see above).
- The kubeconfig pin freezes the session at write time. Changing `AZ_SESSION`
  later does not change an already-written kubeconfig.
- Local end-to-end testing of kubeconfig commands needs `--absolute-path`; see
  `.claude/rules/aks-kubeconfig-testing.md`.

## Tests

- `internal/aks/kubeconfig_test.go`: `AZ_SESSION` is pinned when set and absent
  when unset.
- `internal/devops/*_test.go` (`admin_test.go`, `repos/policy_test.go`,
  `pipelines/release_test.go`, `boards/query_test.go`): set `AZ_SESSION` only to
  point `profile.Load()` at a hermetic, nonexistent profile.
- `pkg/config/profile_test.go` clears `AZ_SESSION` but does not test session
  paths.
- No tests cover the `GetConfigPath` session file name, `GetSharedMSALCache`,
  session-aware logout, or the `aad.go` fallback.

## See also

- User docs: [user guide](../user-guide.md#parallel-sign-ins-with-az_session),
  [pim.md](../pim.md) (multi-customer workflow).
