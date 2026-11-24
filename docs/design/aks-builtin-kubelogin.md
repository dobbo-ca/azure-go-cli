# Built-in kubelogin (`az aks get-token`, `convert-kubeconfig`, `--absolute-path`)

## Purpose

This binary is its own kubectl exec credential plugin, so Microsoft's external `kubelogin` is not needed for AKS Entra ID auth.

Python azure-cli differs:

- It has no `get-token` or `convert-kubeconfig` command (neither core nor aks-preview).
- `az aks get-credentials` writes the server's kubeconfig as-is. Only for devicecode configs does it shell out to an external `kubelogin convert-kubeconfig -l azurecli`, warning if kubelogin is missing.
- `az aks install-cli` installs kubelogin too.

Here, `get-credentials` rewrites the kubeconfig itself, `install-cli` installs only kubectl, and the exec entry calls back into this binary.

History and goals: [2026-05-14-bake-in-kubelogin-design.md](../superpowers/specs/2026-05-14-bake-in-kubelogin-design.md) and the [plan](../superpowers/plans/2026-05-14-bake-in-kubelogin.md). This doc describes the code as it stands.

## Data flow

```
az aks get-credentials / convert-kubeconfig / bastion
        |
        v   credplugin.Convert / WriteKubeconfig
kubeconfig user.exec: command=az|<abs path>, args=[aks get-token --server-id ...]
        |
kubectl --(KUBERNETES_EXEC_INFO env)--> az aks get-token
                                          |  credplugin.GetToken
                                          |  azure.GetCredential() -> token for <server-id>/.default
                                          v
                              ExecCredential JSON on stdout
```

## Components

| What | Where |
|------|-------|
| `get-token` cobra command, flags `--server-id` (required), `--tenant-id`, `--client-id` | `internal/aks/gettoken.go` |
| `GetToken`, `DetermineAPIVersion`, `RenderExecCredential` | `internal/aks/credplugin/plugin.go` |
| `ExecCredential` types, API version constants, `AKSServerIDDefault` | `internal/aks/credplugin/types.go` |
| `Convert`, `commandField`, `rewriteLegacyAuthProvider`, `rewriteKubeloginExec`, `buildExecEntry` | `internal/aks/credplugin/convert.go` |
| `convert-kubeconfig` cobra command | `internal/aks/convertkubeconfig.go` |
| `get-credentials` integration | `internal/aks/credentials.go` (calls `Convert` after context renaming, before stdout/write/merge) |
| Bastion temp kubeconfig | `internal/aks/kubeconfig.go` (`WriteKubeconfig`) |
| `--absolute-path` flag registration | `internal/aks/commands.go` (get-credentials, bastion); `convertkubeconfig.go` |
| `install-cli` (kubectl only) | `internal/aks/install.go` |

### get-token

1. `--server-id` empty is an error.
2. `DetermineAPIVersion` reads `KUBERNETES_EXEC_INFO` (or `GetTokenOptions.ExecInfoEnv`). Empty or missing `apiVersion` gives `client.authentication.k8s.io/v1beta1`. `v1` and `v1beta1` are echoed back. Anything else, or malformed JSON, is an error.
3. The credential comes from `azure.GetCredential()`, the same chain as every other command. `AZ_SESSION` therefore selects the MSAL cache (`pkg/azure/msalcache.go`).
4. The token is requested with scope `<server-id>/.default` and `TenantID` from `--tenant-id`. Only the `--use-azure-cli` credential honours it; the MSAL and broker credentials always use the default subscription's tenant.
5. `ExecCredential` is written with `status.token` and `status.expirationTimestamp = token.ExpiresOn`. kubectl caches on that timestamp.

`--client-id` is carried in `GetTokenOptions.ClientID` but ignored. The token is always minted with the Azure CLI public client from the `az login` session. `AZURE_CLIENT_ID` is not read.

### Conversion rules (`credplugin.Convert`)

For each entry in `users`:

- `user.auth-provider.name == azure`: replaced by an exec entry. `apiserver-id` (default `AKSServerIDDefault`), `tenant-id` and `client-id` come from its `config`.
- `user.exec.command == "kubelogin"` (exact match): replaced, carrying `--server-id/--tenant-id/--client-id` from the old args (server-id defaults likewise).
- Anything else, including admin certificate users and already-converted entries, is untouched.

If nothing changed, the original bytes are returned and `changed=false`. Otherwise the whole document is re-marshalled with yaml.v3.

The generated entry is `apiVersion: v1beta1`, `args: [aks get-token --server-id ... [--tenant-id ...] [--client-id ...]]`, `interactiveMode: IfAvailable`, `provideClusterInfo: false`.

### convert-kubeconfig

- `-f/--file`, default `~/.kube/config`. `KUBECONFIG` is deliberately ignored: kubectl treats it as a merge list, which is ambiguous to rewrite in place.
- Not changed: prints `No convertible entries found in <file>; nothing to do.` and leaves the file alone.
- Changed: `os.WriteFile(..., 0600)` followed by an explicit `os.Chmod(..., 0600)`, since `WriteFile` applies the mode only on creation. Result is always 0600.

### --absolute-path

Exists on `get-credentials`, `bastion` and `convert-kubeconfig`. It writes `os.Executable()` as `exec.command` instead of `az` (`commandField` in `convert.go`; the equivalent branch in `WriteKubeconfig`).

Reason: kubectl resolves `az` via PATH, so another `az` earlier on PATH (Homebrew `az-go`, Python az) can shadow the binary under test and get invoked for `aks get-token` (`unknown flag: --server-id`). See [.claude/rules/aks-kubeconfig-testing.md](../../.claude/rules/aks-kubeconfig-testing.md).

### bastion

`WriteKubeconfig` builds the kubeconfig from a template (it does not use `Convert`). Its exec `env` prepends the binary's directory to `PATH` and pins `AZ_SESSION` when set, so the token subprocess reads the same MSAL profile.

## Decisions

- **Reuse `azure.GetCredential()`** rather than kubelogin's login modes. The credential is always the saved `az login` session; service-principal, managed-identity and workload-identity env vars are not read. No `--login` flag.
- **Hand-rolled `ExecCredential`** (3 fields) to avoid vendoring `k8s.io/client-go`.
- **yaml.v3 over `map[string]interface{}`.** Unknown fields are preserved, but comments and key order are not.
- **Conversion in `get-credentials` is silent and unconditional**, so users never see a legacy entry.

## Limitations and edge cases

- No kubelogin `--login` modes, PoP tokens, SPN/MSI/workload-identity env-var credentials (only the saved `az login` session is used), or sovereign-cloud-specific handling beyond what the credential chain supports.
- Only `command == "kubelogin"` literally is rewritten; a path such as `/usr/local/bin/kubelogin` is not.
- Re-marshalling drops YAML comments and may reorder keys (`convert-kubeconfig` and `get-credentials`).
- `convert-kubeconfig` handles one file; use `-f` per file for `KUBECONFIG` lists.
- `--absolute-path` bakes in a machine-specific path; do not use it for kubeconfigs shared across machines.
- `install-cli` installs only kubectl and prints a note that kubelogin is no longer needed. It needs root and supports darwin and linux.

## Tests

- `internal/aks/credplugin/plugin_test.go`: API version detection, ExecCredential rendering, `GetToken` happy path and errors.
- `internal/aks/credplugin/convert_test.go`: legacy provider, kubelogin exec, multi-user, admin unchanged, already converted, malformed YAML, absolute path.
- `internal/aks/kubeconfig_test.go`: bastion kubeconfig (`get-token` args, `AZ_SESSION` pinning, `--absolute-path`).
- `internal/aks/install_test.go`: kubectl version-string validation (`kubeVersionRe`) only.

Run with `make test`.
