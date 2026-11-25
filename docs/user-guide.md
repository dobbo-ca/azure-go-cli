# User guide: capabilities beyond the Python Azure CLI

This guide covers what `az` (azure-go-cli) does that the official Python Azure CLI does not, or does differently. Commands that behave the same as the Python CLI are not covered here; use the Python CLI documentation for those.

Each section links to a design document in [`docs/design/`](design/) that explains how the feature is implemented.

## Contents

- Sign-in and sessions
  - [Parallel sign-ins with AZ_SESSION](#parallel-sign-ins-with-az_session)
  - [Tenant-first login](#tenant-first-login)
  - [Reusing the Python CLI sign-in (--use-azure-cli)](#reusing-the-python-cli-sign-in---use-azure-cli)
  - [Windows WAM broker](#windows-wam-broker)
- Kubernetes (AKS)
  - [Built-in kubelogin](#built-in-kubelogin)
  - [Renaming kubeconfig identifiers](#renaming-kubeconfig-identifiers)
  - [AKS through Azure Bastion (az aks bastion)](#aks-through-azure-bastion-az-aks-bastion)
  - [Bastion tunnel tuning](#bastion-tunnel-tuning)
- Access
  - [Privileged Identity Management (az pim)](#privileged-identity-management-az-pim)
- Support
  - [Support tickets (az support)](#support-tickets-az-support)
- Tooling
  - [Terraform-compatible az version](#terraform-compatible-az-version)


## Parallel sign-ins with AZ_SESSION

Set `AZ_SESSION=<name>` to give a terminal its own profile and token cache, so several terminals can stay signed in to different tenants or accounts, each with its own default subscription.

**Difference from the Python CLI:** Python has no `AZ_SESSION`; the nearest equivalent is `AZURE_CONFIG_DIR`, which moves the whole config directory. `AZ_SESSION` isolates only the profile and token cache inside the shared `~/.azure`, and `az aks bastion` carries it into the kubeconfig it writes.

```bash
# Terminal 1
export AZ_SESSION=customer-a
az login                 # (default sign-in mode) ~/.azure/azureProfile-customer-a.json, ~/.azure/msal_token_cache-customer-a.json
az pim activate resource --role Contributor --scope "Acme Corp/Acme Production" \
  --ticket Jira:TEC-1234 --justification "Investigating INC-9999" --duration 60 \
  --set-subscription
az aks bastion -n acme-cluster -g acme-rg --bastion /subscriptions/.../bastionHosts/acme-bastion

# Terminal 2
export AZ_SESSION=customer-b
az login                 # separate profile, token cache and default subscription
az account show

# Sign out of one session only
az logout                # removes this session's profile and token cache (plus the shared msal_http_cache.bin); other sessions' tokens are untouched
```

Notes:

- Unset, the default `~/.azure/azureProfile.json` and `~/.azure/msal_token_cache.json` are used.
- Every credential-based command honors it, including `az pim`, `az aks get-token` and `az aks bastion`.
- Token-cache isolation applies to the default sign-in. With `az login --use-azure-cli`, every session borrows the Python CLI's single shared login, so only the profile and default subscription are per-session. With the Windows WAM broker, tokens are held by WAM; the per-session profile records which broker account to use, and no `msal_token_cache-<session>.json` is written.
- `az aks bastion` pins the active `AZ_SESSION` into the kubeconfig's exec env, so kubectl (including `--cmd` and, with `--kubeconfig <path>`, other shells) mints tokens from the right session. `az aks get-credentials` and `az aks convert-kubeconfig` do not pin it.
- `az network bastion ssh --auth-type aad` uses the session's token cache if it exists, otherwise the shared one (the default session's or the Python CLI's tokens).
- Outside the devops commands, this CLI ignores `AZURE_CONFIG_DIR`.
- The session name is used as-is in file names (no validation).
- When testing kubeconfig-generating `az aks` commands locally, add `--absolute-path`.

See also the PIM workflow in [pim.md](pim.md).

Design: [docs/design/az-session.md](design/az-session.md)

## Tenant-first login

When you have access to many tenants and subscriptions, `az login` can ask for a tenant first and then a subscription in it. Tenants that need MFA (AADSTS50076) or a fresh sign-in after a sign-in-frequency token expiry (AADSTS70043) are listed in the picker, and choosing one signs you in to that tenant on the spot. Tenants that fail silent sign-in for any other reason are dropped with a warning.

**Differences from the Python CLI.** Python shows one flat table of subscriptions and tenants (in an interactive terminal, and skipped when `--subscription` matches exactly one subscription), has no `--tenant-selection` flag, and skips tenants that need interaction with a warning to run `az login --tenant TENANT_ID`. Here, up to 10 subscriptions show a flat picker; more than 10 (or `--tenant-selection`) shows a tenant picker, then a subscription picker. MFA-blocked tenants appear marked "requires sign-in" (at the bottom of the flat list, or in the tenant picker). Also:

- Python accepts any verified domain for `--tenant`; here only the tenant ID or the tenant's default domain matches.
- `--subscription` is case-sensitive and has no `-s` short form (Python's is case-insensitive and accepts `-s`).

```bash
# Default: flat picker for 10 or fewer subscriptions, tenant-then-subscription above that
az login

# Always pick the tenant first
az login --tenant-selection

# Limit login to one tenant (tenant ID or the tenant's default domain), then pick among its subscriptions
# (if that tenant has more than 10 subscriptions, a one-entry tenant picker is shown first)
az login --tenant contoso.onmicrosoft.com
az login -t 00000000-0000-0000-0000-000000000000

# Skip the picker entirely (exact, case-sensitive subscription ID or name; first match wins)
az login --subscription "My Subscription"
```

Notes and limitations:

- `--subscription` never signs in to MFA-blocked tenants. If the subscription is not found and such tenants exist, the error tells you to run `az login` without `--subscription`.
- If you pick an MFA-blocked tenant in the flat list, only the subscription you then select from it is saved to your profile; in the tenant-first flow all of its subscriptions are saved.
- With `--use-azure-cli`, choosing an MFA-blocked tenant does not sign you in; it only retries the borrowed Azure CLI token. Run the Python `az login --tenant` first.
- The pickers need an interactive terminal.
- Python's `--use-device-code`, service principal, managed identity, `--allow-no-subscriptions` and `--scope` login options are not available, even though the browser prompt text and `az login --help` mention device code flow.

Design: [docs/design/login-tenant-selection.md](design/login-tenant-selection.md)

## Reusing the Python CLI sign-in (--use-azure-cli)

Use this when your tenant's device-based Conditional Access only accepts the Python Azure CLI's sign-in (its WAM broker on Windows). Instead of signing in itself, this CLI gets tokens from the Python `az` on every command.

**Difference from the Python CLI:** Python azure-cli has no equivalent. This mode exists to borrow Python's own sign-in, and Python has no flag to borrow tokens from another CLI.

```bash
# 1. Sign in with the Python Azure CLI first
az login

# 2. Then log in here, borrowing that session
az login --use-azure-cli

# Pick a subscription or tenant non-interactively
az login --use-azure-cli --subscription "<name-or-id>"
az login --use-azure-cli --tenant <tenant-id-or-domain>

# SSH through Bastion with an AAD certificate (macOS/Linux; reads the Python CLI's plaintext token cache)
az network bastion ssh --auth-type aad ...
```

Notes and limitations:

- The Python `az` must be installed and already signed in. Login does not sign it in for you.
- It must be found on `PATH` ahead of this binary. If `az` resolves to this binary, login fails with an error telling you to put the Python CLI first (or run this build under another name).
- The mode is saved in the profile (`authMode: azure-cli`), so later commands keep using it without the flag.
- On macOS/Linux, `~/.azure/msal_token_cache.json` is the Python CLI's plaintext cache and is left untouched at login. A later plain `az login`, or `az logout`, here deletes it (unless `AZ_SESSION` is set, in which case only the per-session file is removed), which also signs the Python CLI out. On Windows, Python keeps an encrypted `msal_token_cache.bin` that this CLI neither reads nor deletes.
- Each token request runs the Python CLI, so commands are slower than in the default mode.
- `az network bastion ssh --auth-type aad` reads `~/.azure/msal_token_cache.json`, so it works only when the Python CLI's cache is plaintext (macOS/Linux default). It does not work on Windows with the default encrypted cache or WAM. When no cache is found, it asks you to sign in with the Python CLI.

Design: [docs/design/login-use-azure-cli.md](design/login-use-azure-cli.md)

## Windows WAM broker

On Windows, `az login` signs in through the Windows broker (WAM), which satisfies device-based Conditional Access without a browser. The broker DLL is bundled in the Windows build, and if broker sign-in fails the login retries in your browser.

**How it differs from the Python Azure CLI.** Python also uses WAM by default on Windows, so WAM itself is not new. The differences: if WAM fails, Python reports the failure, whereas this CLI retries once in the browser and reports both errors if that also fails. Python has no setting to choose the broker DLL; this CLI lets you point at one with `AZ_MSALRUNTIME_DLL`. Python lets you turn WAM off with `az config set core.enable_broker_on_windows=false`; this CLI honors the same setting through its environment variable `AZURE_CORE_ENABLE_BROKER_ON_WINDOWS=false`, and adds `az login --broker` to choose per login: `never` (browser only) or `always` (WAM only, no browser fallback). Python has no way to forbid the browser fallback.

```powershell
# Normal sign-in: uses WAM automatically on Windows (same as --broker auto)
az login

# Never use WAM: browser sign-in only, msalruntime.dll is not loaded
az login --broker never
$env:AZURE_CORE_ENABLE_BROKER_ON_WINDOWS = 'false'   # same, for every login

# WAM only: fail instead of falling back to the browser
# (for tenants whose Conditional Access only a WAM sign-in satisfies)
az login --broker always

# Pin a specific broker DLL (full path) for this session
$env:AZ_MSALRUNTIME_DLL = 'C:\tools\msalruntime\msalruntime.dll'
az login

# Same effect without an environment variable: put msalruntime.dll next to az.exe.
# Run from an elevated PowerShell (the MSI installs to Program Files).
# Use the explicit path: `Get-Command az` can resolve to Microsoft's az.cmd.
Copy-Item C:\tools\msalruntime\msalruntime.dll 'C:\Program Files\Azure Go CLI\bin\'

# Debug why the broker was skipped or failed
az login --debug
```

If WAM fails you will see `Windows sign-in failed, opening a browser instead.` and the browser sign-in starts. If that also fails, the error ends with `(Windows sign-in also failed: ...)`.

The mode you sign in with is saved in your profile, so later commands keep it until the next `az login`. `--broker` overrides `AZURE_CORE_ENABLE_BROKER_ON_WINDOWS`. With `--broker always`, a missing or broken broker stops the login before your existing profile is cleared; on macOS and Linux it fails with `the WAM broker is only available on Windows`. `--broker always` cannot be combined with `--use-azure-cli`.

Notes:

- DLL lookup order: `AZ_MSALRUNTIME_DLL`, then `msalruntime.dll` beside the `az` executable, then the copy embedded in the binary (extracted on first use to `%LOCALAPPDATA%\az-go\msalruntime\<hash>\`).
- An invalid `AZ_MSALRUNTIME_DLL` path, or a DLL missing required exports, makes the broker unavailable. Login then uses the browser without a message unless you pass `--debug` (or fails, with `--broker always`).
- macOS and Linux builds have no broker and always use the browser flow; `--broker never` and `auto` have no effect there.
- The WAM window works under Windows Terminal, VS Code and conhost. WAM sign-in times out after 10 minutes, after which the login falls back to the browser (or fails, with `--broker always`).
- `az login --use-azure-cli` is a separate mode that borrows tokens from the Python CLI and skips this path.

Design: [docs/design/windows-wam-broker.md](design/windows-wam-broker.md)



## Built-in kubelogin

kubectl can authenticate to AKS with Entra ID using only this binary; the external `kubelogin` is not needed.

**Difference from the Python CLI:** Python azure-cli has no `get-token` or `convert-kubeconfig`. Its `az aks get-credentials` writes the server's kubeconfig as-is and only calls an external `kubelogin` for devicecode configs. Its `az aks install-cli` also installs kubelogin. Here `get-credentials` rewrites the kubeconfig to call `az aks get-token`, and `install-cli` installs only kubectl.

### Examples

```bash
# Normal use: get-credentials already writes an `az aks get-token` exec entry
az aks get-credentials -g my-rg -n my-cluster
kubectl get nodes

# Rewrite an existing kubeconfig (legacy auth-provider: azure, or kubelogin exec) in place
az aks convert-kubeconfig                      # defaults to ~/.kube/config
az aks convert-kubeconfig -f ./my-kubeconfig

# Pin the exact binary in the kubeconfig instead of the bare name `az`
az aks get-credentials -g my-rg -n my-cluster --absolute-path
az aks convert-kubeconfig --absolute-path
az aks bastion ... --absolute-path

# The credential plugin itself (kubectl calls this; rarely run by hand)
az aks get-token --server-id <aks-server-app-id> [--tenant-id <tenant>] [--client-id <client>]
```

### Notes

- `get-token` reads `KUBERNETES_EXEC_INFO`, requests a token for `<server-id>/.default` through the normal credential chain (so `AZ_SESSION` applies) and prints an `ExecCredential`. `--server-id` is required.
- `get-credentials` converts legacy `auth-provider: azure` and `kubelogin` exec entries automatically.
- `convert-kubeconfig` ignores `KUBECONFIG` (pass `-f` explicitly), leaves the file alone if nothing needs converting, and sets the file mode to 0600 when it rewrites.
- `--absolute-path` stops a different `az` earlier on `PATH` (Homebrew or Python) from shadowing this binary. It is mainly for local testing or when you keep several `az` binaries; avoid it for kubeconfigs shared across machines.
- Only exec entries whose command is exactly `kubelogin` are rewritten. YAML comments are not preserved when a file is rewritten.
- `--tenant-id` and `--client-id` are accepted (and carried over from converted kubelogin entries) for compatibility. `--client-id` has no effect, and `--tenant-id` only matters in `--use-azure-cli` mode; otherwise the token comes from the default subscription's tenant.
- kubelogin's `--login` modes and PoP tokens are not supported. get-token uses the account from `az login` (MSAL cache, WAM broker, or the Python CLI's token in `--use-azure-cli` mode). Service-principal, managed-identity and workload-identity environment variables are not read.

Design: [docs/design/aks-builtin-kubelogin.md](design/aks-builtin-kubelogin.md)

## Renaming kubeconfig identifiers

Rename the cluster in the kubeconfig that `az aks get-credentials` or `az aks bastion` produces, so several clusters with the same name can sit side by side in `~/.kube/config`.

**Differences from the Python azure-cli:** Python has no regex rename. Its `--context` only changes the first context, cluster and `current-context`, leaves the user entry alone, and is ignored with `-f -`. With `--admin`, Python also appends `-admin` to the context name (for example `acme-prod-admin`). This CLI does not, so the admin context is named exactly as renamed. Here every identifier is renamed (`current-context`, `clusters[].name`, `contexts[].name`, `contexts[].context.cluster`, `contexts[].context.user`, `users[].name`), in every output mode including `-f -`. Python's equivalent, `az aks bastion tunnel` (aks-preview), has no context flag.

```bash
# Literal rename (get-credentials only)
az aks get-credentials -n appcluster-prod-usw2-k8s-20251209 -g my-rg \
  --context acme-prod

# Regex rename: $1, $2 capture groups supported
az aks get-credentials -n appcluster-prod-usw2-k8s-20251209 -g my-rg \
  --context-regex '^appcluster-(.+)$' --context-replacement 'acme-$1'
# context becomes acme-prod-usw2-k8s-20251209

# Print the renamed kubeconfig to stdout
az aks get-credentials -n appcluster-prod-usw2-k8s-20251209 -g my-rg -f - \
  --context-regex '^appcluster' --context-replacement 'acme'

# Bastion
az aks bastion -n appcluster-prod-usw2-k8s-20251209 -g my-rg \
  --bastion /subscriptions/.../bastionHosts/my-bastion \
  --context-regex '^appcluster' --context-replacement 'acme'
```

Notes:

- `--context-regex` and `--context-replacement` must be supplied together. An empty replacement is allowed and removes the matched text.
- If the replacement leaves the name empty (for example the regex matches the whole name and the replacement is empty), no error is raised and the kubeconfig is written with empty names. Make sure the result is non-empty.
- `--context-regex` cannot be combined with `--context`. `--context` exists only on `get-credentials`.
- The regex is matched against the cluster name, and the result replaces that name wherever it appears in the identifier fields, so the `clusterUser_` / `clusterAdmin_` prefix is kept.
- If the regex does not match the cluster name, nothing is renamed.
- Uses Go (RE2) regex syntax. Single-quote the replacement so your shell does not expand `$1`.
- Write `${1}` instead of `$1` when the next character is a letter, digit or underscore. For example, `'${1}_eu'` works, but `'$1_eu'` expands to an empty string and blanks the names.
- When testing locally, add `--absolute-path` so the kubeconfig calls the binary you built.

Design: [docs/design/aks-kubeconfig-rename.md](design/aks-kubeconfig-rename.md)

## AKS through Azure Bastion (az aks bastion)

`az aks bastion` opens an Azure Bastion tunnel to a private AKS API server and gives you a working kubeconfig in one command. It is built in and needs only `kubectl`.

**Differences from the Python CLI.** Python has this only in the `aks-preview` extension, as `az aks bastion tunnel`. It starts an interactive subshell and shells out to `az network bastion tunnel` (needs the `bastion` extension, which it installs on demand). Its kubeconfig comes from `az aks get-credentials`, so kubectl uses kubelogin on Entra ID clusters. Its `--kubeconfig-path` must be an existing file that it edits in place. Here the tunnel runs inside the process, you can run a command directly with `--cmd`, `--kubeconfig` writes a new file that is kept, and the kubeconfig pins `PATH` and `AZ_SESSION`. Python can discover the bastion when it is omitted; here `--bastion` is required. `--bastion` must be a full resource ID (Python also accepts a name), and the bastion must be in your default subscription. Python uses the subscription from the bastion ID.

```bash
# Run one command, then close the tunnel when it exits
az aks bastion -n my-cluster -g my-rg \
  --bastion /subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Network/bastionHosts/my-bastion \
  --cmd "kubectl get pods -A"

# Open k9s through the tunnel
az aks bastion -n my-cluster -g my-rg --bastion <bastion-id> --cmd k9s

# No --cmd: prints `export KUBECONFIG=...`, copies it to the clipboard,
# and keeps the tunnel open until Ctrl+C
az aks bastion -n my-cluster -g my-rg --bastion <bastion-id>

# Keep the kubeconfig after exit, on a fixed local port
az aks bastion -n my-cluster -g my-rg --bastion <bastion-id> \
  --kubeconfig ./bastion.kubeconfig --port 50443

# Rename the context (same flags as `az aks get-credentials`)
az aks bastion -n appcluster-prod-usw2-k8s-20251209 -g my-rg --bastion <bastion-id> \
  --context-regex '^appcluster' --context-replacement 'acme'

# Local testing with a different `az` earlier on PATH
./bin/az/az aks bastion -n my-cluster -g my-rg --bastion <bastion-id> \
  --absolute-path --cmd "kubectl get nodes"
```

Notes:

- With `--cmd`, it first runs `kubectl get nodes --request-timeout=10s` as a warm-up (its errors are ignored), then runs your command with `KUBECONFIG` set. Tokens come silently from your `az login` session. Run `az login` first, because nothing here prompts.
- `-f/--kubeconfig PATH` writes a new kubeconfig to PATH (overwriting any existing file) and leaves it after exit. The default is a temp file deleted when the command returns normally. Ctrl+C during a `--cmd` run, or within the first ~2 seconds, kills az without cleanup and leaves the `az-aks-bastion-*` temp directory behind.
- The generated kubeconfig runs `az aks get-token` with `PATH` (this binary's directory first) and, if set, `AZ_SESSION` pinned in the exec env, so kubectl keeps using the right session from any shell. `--absolute-path` embeds this binary's full path instead of `az`.
- `-p/--port` defaults to 0, which picks a random port in 49152-65535. The `--conn-read-buffer`, `--conn-write-buffer`, `--chunk-read-buffer` and `--chunk-write-buffer` flags tune buffers in KB (defaults 32, 32, 8, 8).
- `--cmd` is split on whitespace with no shell and no quote handling, so pipes, redirects, quoted arguments and `sh -c '...'` do not work. For anything beyond a plain command, run without `--cmd` and use the printed `export KUBECONFIG=...` in another shell (or keep it with `-f`).
- If `kubectl` is missing, a warning is printed; with `--cmd` the warm-up then fails and the command aborts with `authentication failed: failed to start kubectl`.
- `--admin` is accepted but currently has no effect.

Design: [docs/design/aks-bastion.md](design/aks-bastion.md)

## Bastion tunnel tuning

`az network bastion tunnel`, `az network bastion ssh` and `az aks bastion` share one in-process WebSocket tunnel. It keeps long-lived sessions (k9s, kubectl, ssh) alive through Azure Bastion and lets you tune its buffer sizes.

**Differs from the Python CLI:** the Python `bastion` extension's `tunnel` takes `--name`, `--resource-group`, `--target-resource-id`/`--target-ip-address`, `--resource-port`, `--port` and `--timeout`, with no buffer options, no WebSocket keepalive and no token-exchange retry. Here, like Python, each connection exchanges a new token, but the exchange always sends an empty `token` field so sessions are not linked (Python sends the previous token and node ID). The exchange is also retried (3 attempts, 1s then 2s apart), and a WebSocket ping is sent every 30s, dropping the connection if no pong arrives within 60s. The Go tunnel does not support `--timeout` or `--target-ip-address`.

```bash
# Tunnel with defaults (32 KB connection buffers, 8 KB chunk buffers)
az network bastion tunnel -n my-bastion -g my-rg \
  --target-resource-id <vm-resource-id> --resource-port 22 -p 2222

# Larger buffers for bulk transfers
az network bastion tunnel -n my-bastion -g my-rg \
  --target-resource-id <vm-resource-id> --resource-port 22 -p 2222 \
  --conn-read-buffer 64 --conn-write-buffer 64 \
  --chunk-read-buffer 16 --chunk-write-buffer 16

# SSH through Bastion with custom buffers
az network bastion ssh -n my-bastion -g my-rg \
  --target-resource-id <vm-resource-id> --auth-type AAD \
  --conn-read-buffer 64 --conn-write-buffer 64

# AKS through Bastion
az aks bastion -n my-aks -g my-rg --bastion <bastion-resource-id> \
  --cmd "k9s" --chunk-read-buffer 16 --chunk-write-buffer 16
```

Flags (all values in KB): `--conn-read-buffer` (default 32), `--conn-write-buffer` (32), `--chunk-read-buffer` (8), `--chunk-write-buffer` (8). The `conn` flags size the WebSocket connection's read/write I/O buffers (gorilla Dialer ReadBufferSize/WriteBufferSize); the `chunk` flags size the per-connection copy buffers.

Notes:
- Keepalive interval, pong timeout and retry count are fixed; they are not flags.
- Retry covers the token exchange only, with a linear backoff (1s, 2s). There is no automatic WebSocket reconnect: if a connection dies, the local TCP connection is closed. A new token is exchanged only when the client opens a new connection (kubectl does this per request; an ssh session just ends).
- Flag values are not validated, so use positive values.
- Add `--debug` to see buffer sizes, pings/pongs and retry attempts (not available for `az network bastion ssh`, which turns off debug logging once ssh starts).

Design: [docs/design/bastion-tunnel-tuning.md](design/bastion-tunnel-tuning.md)



## Privileged Identity Management (az pim)

`az pim` lists your eligible Azure resource role and Entra group PIM assignments (current activations are not listed) and activates Azure resource roles and Entra group memberships on demand.

The Python azure-cli has no PIM command group (not in core, not in any extension); there you have to call the PIM REST API through `az rest`. Here it runs in the same binary and credential, so `AZ_SESSION` isolation applies automatically.

```bash
# List eligible assignments (table by default)
az pim list
az pim list --type resource        # or: --type group
az pim list --output json

# Activate a resource role
az pim activate resource \
  --role Contributor \
  --scope "Acme Production" \
  --ticket Jira:TEC-1234 \
  --justification "Investigating incident INC-9999" \
  --duration 60 \
  --set-subscription

# Activate an Entra group membership
az pim activate group --name customer-acme-admins \
  --justification "Customer hand-off" --duration 120

# Fail instead of prompting when a flag is missing
az pim activate group --name customer-acme-admins --no-input
```

Notes and limitations:

- `--scope` takes an ARM path, a subscription UUID, `<tenant-id>/<subscription-name>[/rg]`, or a bare subscription name (must match exactly one eligible entry). It can be omitted if `--role` matches a single eligible assignment.
- Azure's validation endpoint is called before submitting; a rejection exits non-zero. A `Pending*` status (approval required) exits 0. With `-o table` it prints `Pending approval; request <id>`; the default JSON output shows `status: Pending...` and `requestId`.
- On a TTY, missing flags are prompted for. With `--no-input` they become `missing required flag: --x, --y` errors. With piped stdin and no `--no-input`, the command fails with `prompt required but running non-interactively`.
- `--set-subscription` rewrites the session's profile so the activated subscription becomes the default.
- Entra directory roles, approvals and scheduled activations are not supported.

Full guide: [docs/pim.md](pim.md)

Design: [docs/design/az-pim.md](design/az-pim.md)



## Support tickets (az support)

`az support` opens a full-screen terminal UI to list, read, reply to and create Azure support tickets for any subscription or tenant. It also has flag commands with the same names and flags as the Python `support` extension, for scripts.

Python's `support` extension has no interactive mode. Filing a ticket there means finding the service and problem type GUIDs yourself; here you search a local list by typing words from your title.

```bash
az support                       # TUI on a terminal; help otherwise
az support tui                   # force the TUI (errors without a terminal)
az support --tenant <tenant-id>  # start at tenant level
az support --subscription <id>   # start in another subscription
az support --no-input            # never start the TUI
```

The starting scope is the active profile's default subscription, or the one from `--subscription` or `--tenant`. `t` on the list switches scope, including tenant-level tickets (no subscription) and other tenants you can reach. Switching never changes the profile's default subscription.

Keys:

- List: `n` new ticket, `f` open/all (last 30 days), `t` scope, `/` search, `enter` open, `ctrl+r` reload, `q` quit.
- Detail: `r` reply, `a` attach a file, `s` close or reopen, `v` severity, `ctrl+r` reload, `esc` back.
- Reply: `tab` next field, `ctrl+s` send, `esc` cancel.
- New ticket: `tab` next field, `ctrl+n` next step, `esc` back one step (values are kept), `enter` picks from a list or moves on. On the contact step, `enter` on country, time zone and language opens a picker, and `space` toggles the contact method.
- Everywhere: `ctrl+l` signs in to the current tenant (MFA), `ctrl+c` quits.

Contact details are pre-filled so the common case is a few keystrokes: name, email and country from Microsoft Graph `/me` (`givenName`, `surname`, `mail`, `usageLocation`); the time zone from your local zone; the language from `LC_ALL`, `LC_MESSAGES` or `LANG`. If Graph fails (a guest tenant, for example) the name and email start empty, the rest is still filled, and the flow continues.

Limits:

- A Basic support plan can only file billing, subscription-management and quota tickets. The API error is shown verbatim for anything else.
- Quota tickets are not filed in the TUI. Use `az quota` or `--quota-change-*` on the flag command.
- Critical and highest critical impact need a plan that allows them, and a phone number.
- Every ticket this CLI creates gets a file workspace named after the ticket, so attachments work later. Tickets made elsewhere may have none; the UI says so instead of failing.

Flag examples:

```bash
az support in-subscription tickets list
az support in-subscription tickets show --ticket-name <guid>
az support in-subscription communication create --ticket-name <guid> \
  --communication-name <guid> --communication-sender me@example.com \
  --communication-subject "Re: ..." --communication-body "..."
az support in-subscription file upload --file-workspace-name <guid> --file-path ./log.txt
az support services problem-classifications list --service-name <service-guid>
az support no-subscription tickets list
```

Differences from the Python extension:

- Flags that take a list of objects (`--secondary-consent`, `--quota-change-requests`) take JSON, or `@file`.
- `--contact-additional-emails` is a repeatable or comma-separated list.
- `tickets create` creates a file workspace unless `--file-workspace` / `--file-workspace-name` is given.
- Blocked upload extensions match case-insensitively (`RUN.EXE` is rejected).
- List commands have `--max-items` but no `--next-token`.

Design: [docs/design/az-support.md](design/az-support.md)



## Terraform-compatible az version

`az version` prints one JSON line with this CLI's own version, and reports a fixed compatible `azure-cli` value so Terraform's `azurerm` provider (which parses that field and enforces a supported version range) accepts it.

**How it differs from the Python CLI.** Both print an `azure-cli` field, but here it is always `2.67.0` and is not the real version. The real version is in `azure-cli-go`, with `azure-cli-go-commit` and `azure-cli-go-build-date` alongside. Python's output instead lists `azure-cli`, `azure-cli-core`, `azure-cli-telemetry` and an `extensions` map; those keys are absent here. There is no `--version` flag.

```bash
az version
# {"azure-cli": "2.67.0", "azure-cli-go": "v1.7.0", "azure-cli-go-commit": "adb8f9f...", "azure-cli-go-build-date": "2026-08-12T07:55:58Z"}

# Real version of this CLI
az version | jq -r '."azure-cli-go"'

# Tell which CLI you have: only this one prints azure-cli-go
az version | grep -q azure-cli-go && echo "azure-go-cli"
```

Notes:
- `az --version` fails with `unknown flag: --version`; use `az version`.
- `--output` and `--query` are accepted but ignored; the output is always this JSON line.
- A binary built with plain `go build` reports `dev` / `unknown` / `unknown`. `make build` stamps the real values, but with a short commit and a `YYYY-MM-DD_HH:MM:SS` date. Release binaries carry the full commit SHA and an RFC3339 date, as shown above.
- Do not use `azure-cli` to detect which CLI is installed. Check for `azure-cli-go`.

Design: [docs/design/version-terraform-compat.md](design/version-terraform-compat.md)
