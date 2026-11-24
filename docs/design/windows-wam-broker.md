# Windows WAM sign-in

Status: implemented. Windows only.

## Purpose and difference from Python azure-cli

On Windows, `az login` signs in through the Web Account Manager (WAM) broker
via Microsoft's `msalruntime.dll`. The broker satisfies device-based
Conditional Access without a browser.

WAM itself is not unique to this CLI: Python azure-cli also uses it by default
on Windows (`core.enable_broker_on_windows`, via `msal[broker]`/`pymsalruntime`).
The differences are behavioural:

| | Python azure-cli | this CLI |
|---|---|---|
| Broker unavailable | falls back to the browser with a logged warning | falls back to the browser silently (debug log only) |
| Broker sign-in fails | error returned, no browser retry | one retry in the browser flow; both errors reported if that also fails |
| Turning WAM off | `core.enable_broker_on_windows=false` (config or `AZURE_CORE_ENABLE_BROKER_ON_WINDOWS`) | `az login --broker never`, or the same env var |
| Forbidding the browser fallback | not possible | `az login --broker always` |
| Choosing the broker DLL | none | `AZ_MSALRUNTIME_DLL`, or `msalruntime.dll` next to `az.exe` |
| DLL source | pip package | embedded in the Windows binary, extracted on first use |

## Broker mode (`--broker`)

`internal/auth/broker_mode.go`:

- `resolveBrokerMode(flag, env)`: the `--broker` flag (`auto|always|never`)
  wins; otherwise `AZURE_CORE_ENABLE_BROKER_ON_WINDOWS` (Python's env form of
  `core.enable_broker_on_windows`, same boolean spellings as configparser:
  false/0/no/off -> `never`, true/1/yes/on -> `auto`); otherwise `auto`.
  Invalid values are errors.
- `chooseAuthMode(useAzureCLI, mode, startBroker)`: azure-cli mode never starts
  the broker and rejects `always`; `never` returns the default mode without
  calling `startBroker` (the DLL is not loaded or extracted); `auto` falls back
  silently when `StartBroker` fails; `always` returns an error (Windows-only
  message on other platforms, otherwise wrapping the startup error).
- `Login` calls `chooseAuthMode` before deleting the profile, so a failed
  `--broker always` leaves the existing login intact. The browser retry below
  is skipped when the mode is `always`.
- The mode itself is not persisted; the resulting `AuthMode` is, as before.

## Data flow

```
az login (internal/auth/login.go)
  chooseAuthMode(--broker / env)
      |-- never: browser flow, broker not loaded
      `-- auto/always: azure.StartBroker() -> msalruntime.Startup()
            |-- fail, auto: browser flow (silent, debug log only)
            |-- fail, always: error, profile untouched
            `-- ok: authMode = broker
  azure.BaseCredential()   -> BrokerCredential (interactive, tenant "organizations")
  DiscoverAllSubscriptionsWithAuth
      |-- ok: save profile (AuthMode, BrokerAccountID)
      `-- error, authMode==broker, mode != always, ctx not cancelled:
            SetAuthMode(default); BaseCredential() -> MSAL interactive; rediscover
            both fail -> "<browser err> (Windows sign-in also failed: <broker err>)"
```

Later commands: `TenantCredential` builds a silent `BrokerCredential` keyed by
the saved `BrokerAccountID`. If the broker is gone by then (`ErrNotAvailable`),
`brokerCredential` in `pkg/azure/authmode.go` returns nil and the MSAL silent
credential is used instead. The MSAL cache holds no tokens from a broker
login, so this fallback turns a DLL load failure into a normal "please run
`az login`" error rather than keeping the session alive.

## Key pieces

- `internal/auth/login.go`: broker selection and the single browser retry.
  The retry is guarded on `ctx.Err()`, but `az login` passes
  `context.Background()`, so the guard never fires today. Ctrl-C ends the
  process via default SIGINT handling. A WAM sign-in that hits the 10 minute
  `defaultOperationTimeout` does fall through to the browser retry.
- `pkg/azure/broker.go`: `StartBroker`, `BrokerCredential` (silent first,
  interactive only for the initial sign-in credential), `ErrBrokerUnavailable`
  (alias of `msalruntime.ErrNotAvailable`), `BrokerAccountID`.
- `pkg/azure/msalruntime/dll_windows.go`: DLL resolution, `Startup`, proc binding.
- `pkg/azure/msalruntime/extract_windows.go`: embedded DLL extraction.
- `pkg/azure/msalruntime/embed_windows_{amd64,arm64}.go`, `dll/`: the embedded
  copies. Attribution in `THIRD-PARTY-NOTICES.md`.
- `pkg/azure/msalruntime/msalruntime_other.go`: non-Windows stubs, all return
  `ErrNotAvailable`, so macOS and Linux always use the browser flow.
- `pkg/azure/msalruntime/msalruntime_windows.go` (`resolveParentWindow`),
  `window_windows.go` (`pumpWindow`): parent window for the WAM dialog.
- `internal/auth/console_windows.go`: console handling for the login prompts
  (enables VT processing on `CONOUT$`); not part of broker parenting.

## DLL resolution (`dllPath`)

1. `AZ_MSALRUNTIME_DLL` if set (used as given, not validated here).
2. `msalruntime.dll` in the directory of `os.Executable()`, if it exists.
3. Otherwise `Startup` extracts the embedded copy.

`dllPath` runs at package init on every command, so it only stats; it never
writes. Extraction happens only in `Startup`, which only broker paths call.

Extraction (`extractEmbeddedDLL`) writes to
`<os.UserCacheDir()>/az-go/msalruntime/<first 8 bytes of sha256, hex>/msalruntime.dll`
(`%LOCALAPPDATA%` on Windows) via temp file and rename. An existing file of the
right size is reused.

`Startup` runs once per process (`sync.Once`): load the DLL, resolve every
bound export, call `MSALRUNTIME_Startup`. Any failure wraps `ErrNotAvailable`.

## Decisions

- **Never load by bare name.** The default `LoadLibrary` search includes the
  current directory, so availability would depend on cwd and open a DLL
  preloading hole. Copies shipped by Office, Teams or Python azure-cli live in
  private dirs off the search path anyway.
- **Content-hashed cache dir.** Different builds extract to different paths, so
  upgrades never overwrite a DLL another running `az` has loaded.
- **Cache under `%LOCALAPPDATA%`, not beside the exe.** Program Files (MSI
  install) may be read-only.
- **Temp file + rename.** A concurrent `az` never loads a partial DLL. If rename
  loses a race, the winner's file is used.
- **Resolve all exports in `Startup`.** `LazyProc.Call` panics on a missing
  export; an older DLL must disable the broker, not crash the process.
- **Any broker failure retries in the browser, once** (mode `auto`). Covers WAM
  dialogs that die on broken VDI images. Not gated on error type. Mode
  `always` exists for tenants where only a WAM sign-in satisfies Conditional
  Access, so a browser retry would only produce a confusing second failure. The retry runs the browser
  flow with the default auth mode, so the saved profile is a normal MSAL one.
- **Only the initial credential is interactive.** Tenant-scoped credentials are
  silent so discovering N tenants cannot open N windows.
- **Parent window (`resolveParentWindow`).** The broker needs a window whose
  message queue this process owns. Under conhost the console window (or its
  root owner, if ours) is used. Under ConPTY hosts (Windows Terminal, VS Code,
  AVD RemoteApp) the console window is hidden and owned by the terminal; the
  dialog shows but completion never arrives and sign-in blocks until the
  timeout. There a hidden top-level window is created on a locked OS thread
  with a message pump (`newPumpWindow`). If that fails, the desktop window is
  used. See commits c7aacd0 and 3281425.

## Edge cases and limitations

- Non-Windows builds embed nothing; broker is never used.
- Each WAM operation has a 10 minute timeout (`defaultOperationTimeout`); on expiry login falls back to the browser.
- A broken `AZ_MSALRUNTIME_DLL` path means broker unavailable (`dll.Load` fails),
  which on login silently selects the browser flow; the cause appears only in
  debug logs.
- An env/adjacent DLL missing any bound export is rejected, so a pinned broker
  must be recent enough.
- The profile pins broker mode at login; if the DLL later disappears, commands
  fall back to the MSAL silent credential (`TenantCredential`), which has
  no tokens, so they fail with a re-login error.
- `--use-azure-cli` is a separate login mode and bypasses this path.

## Tests

- `pkg/azure/msalruntime/extract_windows_test.go`: extraction.
- `pkg/azure/msalruntime/window_windows_test.go`: pump window ownership/close.
- `pkg/azure/msalruntime/types_test.go`: status values, error unwrapping.
- `internal/auth/broker_mode_test.go`: flag/env resolution and the mode
  decision (`chooseAuthMode`) with a fake broker start.
- No test covers the login fallback in `internal/auth/login.go`
  (`login_test.go` tests subscription and tenant filtering only).
