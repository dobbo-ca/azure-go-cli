# Bake kubelogin into az binary — Design

**Date:** 2026-05-14
**Status:** Approved (pending implementation plan)
**Scope:** Replace the external `kubelogin` dependency by implementing the kubectl exec credential plugin and the legacy-auth kubeconfig converter directly in this binary. Covers `az aks bastion`, `az aks get-credentials`, and provides a standalone `az aks convert-kubeconfig` for pre-existing kubeconfigs on disk.

## Motivation

Users of this CLI currently must install Microsoft's `kubelogin` separately to use `az aks bastion` and to consume kubeconfigs produced by `az aks get-credentials`. `kubelogin` is an external Go binary whose only relevant job — for the user-facing flows we care about — is minting an AAD bearer token for the AKS server-id and emitting a kubectl `ExecCredential` JSON object. We already have an Azure credential chain (`pkg/azure.GetCredential()`) and the JSON-writing machinery (`internal/account/token.go`). Re-implementing the exec plugin and converter inline removes the external dependency, eliminates the `sudo az aks install-cli` step for kubelogin, and gives us a single binary to ship and version.

## Goals

- `kubectl` works against AKS clusters using only this binary (no `kubelogin` in PATH).
- `az aks get-credentials` produces a kubeconfig whose exec entry points at this binary.
- `az aks bastion` produces a temp kubeconfig whose exec entry points at this binary.
- `az aks convert-kubeconfig` rewrites any pre-existing kubeconfig (legacy `azure` auth-provider or existing `kubelogin` exec entry) in place.
- `az aks install-cli` no longer attempts to install `kubelogin` (kubectl install remains).

## Non-goals

- No support for kubelogin's `--login spn|msi|workloadidentity|devicecode|interactive` modes as explicit flags. The exec plugin uses `azure.GetCredential()` — the same credential chain every other command in this binary uses. `DefaultAzureCredential` already covers SPN/MSI/workload-identity via standard env vars (`AZURE_CLIENT_ID`, `AZURE_FEDERATED_TOKEN_FILE`, etc.); no plugin-specific flags needed.
- No PoP (proof-of-possession) tokens.
- No support for sovereign clouds beyond what the binary already supports (AzurePublicCloud).
- No persistent token cache beyond what `azidentity` / MSAL already do. kubectl caches `ExecCredential` responses based on the `expirationTimestamp` field we emit, so a single mint per token lifetime is the steady state.

## Behavior

### New command: `az aks get-token`

The kubectl exec credential plugin endpoint. Kubectl invokes this on every API call (until the previous response's `expirationTimestamp` passes).

```
az aks get-token --server-id <id> [--tenant-id <id>] [--client-id <id>]
```

Flags:

- `--server-id <id>` (required): AAD application ID of the AKS API server. Used as `<id>/.default` scope.
- `--tenant-id <id>` (optional): AAD tenant ID. If omitted, the credential chain uses its own discovery.
- `--client-id <id>` (optional): AAD client ID. If omitted, the credential chain uses its own.

Behavior:

1. Read `KUBERNETES_EXEC_INFO` env var. Parse JSON to extract `apiVersion`. Default to `client.authentication.k8s.io/v1beta1` if unset or empty.
2. Call `azure.GetCredential()` and request a token at scope `<server-id>/.default`.
3. Emit `ExecCredential` JSON to stdout with `status.token` and `status.expirationTimestamp = token.ExpiresOn` (RFC3339).
4. On failure: write `error: <message>` to stderr and exit 1. kubectl surfaces the stderr message to the user.

Output shape (v1beta1; v1 differs only in apiVersion string):

```json
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "status": {
    "token": "<bearer>",
    "expirationTimestamp": "2026-05-14T15:30:00Z"
  }
}
```

Interactive device-code handling mirrors the existing UX in `internal/aks/kubeconfig.go:handleDeviceCodeFlow` — browser open + clipboard copy — but only if the credential chain triggers an interactive flow.

### New command: `az aks convert-kubeconfig`

In-place rewriter for kubeconfigs that already exist on disk (e.g., produced by Python `az` CLI or by previous runs that depended on `kubelogin`).

```
az aks convert-kubeconfig [--file <path>] [--absolute-path]
```

Flags:

- `--file <path>` (optional): kubeconfig file to convert. Defaults to `~/.kube/config`. The `KUBECONFIG` env var (which kubectl supports as a colon-separated merge list) is intentionally ignored to avoid ambiguity; users with `KUBECONFIG` set must pass `--file` explicitly.
- `--absolute-path` (optional): emit the absolute path from `os.Executable()` as `command:` instead of `az`.

Behavior:

1. Read and parse the kubeconfig YAML.
2. For each entry in `users[]`:
   - If `user.auth-provider.name == "azure"`: extract `apiserver-id`, `tenant-id`, `client-id`, `environment` from `auth-provider.config`. Drop the `auth-provider` key and add an `exec` entry pointing at this binary. All other fields on the user entry are preserved untouched.
   - Else if `user.exec.command == "kubelogin"`: scan `user.exec.args` for `--server-id`, `--tenant-id`, `--client-id` values. Replace `user.exec` with a fresh exec entry pointing at this binary. Other fields on the user entry are preserved.
   - Else if `user.exec.command == "az"` (exact string match): leave alone. This is the idempotency check — best-effort, not strict. With `--absolute-path` the check is exact string match against `os.Executable()`.
   - Else: leave alone (cert auth, other plugins, etc.).
3. If anything changed, write back to the same path with mode 0600. If nothing changed, exit 0 without writing.

### Modified: `az aks get-credentials`

Adds:

- `--absolute-path` (optional): same semantics as on `convert-kubeconfig`.

Behavior change: after fetching the kubeconfig from the AKS API and before any write/merge/stdout output, the bytes pass through the same converter `convert-kubeconfig` uses. Admin kubeconfigs use cert auth and pass through unchanged.

### Modified: `az aks bastion`

Adds:

- `--absolute-path` (optional): same semantics. Default is `command: az`.

Behavior change: `WriteKubeconfig` emits an exec entry pointing at this binary instead of `kubelogin`. The existing PATH-prepend env trick (`internal/aks/kubeconfig.go:60-69`, the "ensure it's used instead of Python CLI" comment) is preserved — that env block ensures kubectl finds our `az` even if Python `az` is earlier in the user's PATH at invocation time. `AZ_SESSION` pinning into the env block stays.

### Modified: `az aks install-cli`

The `installKubelogin` step is deleted entirely. `installKubectl` remains. Help text updated to say only kubectl is installed.

### Modified: `CheckDependencies`

`internal/aks/install.go:CheckDependencies` returns missing tools from a list. Drop `"kubelogin"` from the list. `kubectl` stays.

## Architecture

### New package: `internal/aks/credplugin/`

Encapsulates exec-plugin logic and kubeconfig conversion. Kept in `internal/aks/` rather than at top-level because the scope is AKS-specific (server-id is the AKS first-party app); promoting it to a shared package can come later if another command needs the exec-plugin shape.

Files:

- `plugin.go` — `GetToken(ctx, opts) error`. Reads `KUBERNETES_EXEC_INFO`, mints token, writes JSON to stdout. Exported so `internal/aks/gettoken.go` (the cobra command) can call it.
- `types.go` — Hand-rolled `ExecCredential` Go struct (avoids vendoring `k8s.io/client-go`). Three fields nested: TypeMeta + Status + ExpirationTimestamp.
- `convert.go` — `Convert(yaml []byte, opts ConvertOptions) ([]byte, bool, error)`. Returns rewritten bytes, a `changed` flag, and any error. `ConvertOptions` carries `AbsolutePath bool`.
- `*_test.go` — fixture-driven tests.

### New command wrappers

- `internal/aks/gettoken.go` — cobra command, parses flags, calls `credplugin.GetToken`.
- `internal/aks/convertkubeconfig.go` — cobra command, parses flags, reads file, calls `credplugin.Convert`, writes back if changed.

Registered in `internal/aks/commands.go`.

### Modified files

- `internal/aks/kubeconfig.go` — `WriteKubeconfig` rewrites the exec entry template. Args become `[aks, get-token, --server-id, <id>]`. Existing PATH env-block logic preserved verbatim. AKS first-party server-id `6dae42f8-4368-4678-94ff-3960e28e3630` becomes a named constant in `credplugin` and is imported here.
- `internal/aks/credentials.go` — `GetCredentials` calls `credplugin.Convert(kubeConfig, ConvertOptions{AbsolutePath: opts.AbsolutePath})` after the context-rename block and before any output branch. `GetCredentialsOptions` gains `AbsolutePath bool`.
- `internal/aks/bastion.go` — `BastionOptions` gains `AbsolutePath bool`. Passed to `WriteKubeconfig`. The dependency-check warning ("Please install them using: sudo az aks install-cli") is updated since kubelogin is no longer needed.
- `internal/aks/install.go` — `installKubelogin` and its call site deleted. `CheckDependencies` updated.

## Self-path resolution

Default: `command: az` (PATH lookup). Portable kubeconfigs, no install-location baked in.

With `--absolute-path`: `command: <os.Executable() result>`. Robust against PATH issues. Breaks if the binary is moved (loudly — kubectl surfaces the error), but rerunning `get-credentials` / `convert-kubeconfig` regenerates.

The bastion temp kubeconfig uses `command: az` plus the existing PATH-prepend env block, so the Python-`az` shadow case is handled the way it is today.

## Server-ID discovery in `Convert`

The converter needs to know the server-id to emit it as an arg. Sources in priority order:

| Source                                | Field                              |
| ------------------------------------- | ---------------------------------- |
| Legacy `auth-provider: azure` config  | `apiserver-id`                     |
| Existing `exec` entry args            | value following `--server-id`      |
| Neither                               | default `6dae42f8-4368-4678-94ff-3960e28e3630` (AKS first-party app) |

Same lookup pattern for `--tenant-id` and `--client-id`: extract if present, otherwise omit from the emitted args.

## Generated exec entry shape

For `get-credentials` and `convert-kubeconfig` output the `env` key is omitted entirely. For the bastion temp kubeconfig the `env` key is populated with the existing PATH-prepend entry and the optional `AZ_SESSION` entry.

```yaml
exec:
  apiVersion: client.authentication.k8s.io/v1beta1
  command: az
  args:
    - aks
    - get-token
    - --server-id
    - 6dae42f8-4368-4678-94ff-3960e28e3630
    # --tenant-id <val>   # only if known
    # --client-id <val>   # only if known
  # env:                  # omitted for get-credentials/convert; populated for bastion temp kubeconfig
  interactiveMode: IfAvailable
  provideClusterInfo: false
```

## Idempotency

`Convert` is idempotent: a kubeconfig whose `users[*].exec.command` already resolves to this binary is returned unchanged with `changed=false`. Repeated `get-credentials` runs against the same merge target don't churn the file.

## Error handling

- `get-token` failures (credential chain returns error, network failure, server-id missing) → `fmt.Fprintf(os.Stderr, "error: %v\n", err)`, exit 1. kubectl surfaces stderr verbatim.
- `convert-kubeconfig` on malformed YAML → error to stderr, exit 1, file untouched.
- `convert-kubeconfig` on a kubeconfig with no convertible entries → no-op, exit 0, no write.
- `get-credentials` Convert failure → existing error path (returned to cobra, rendered as `Error: ...`). The fetched kubeconfig is not written.

## Testing

### Unit

- `credplugin/convert_test.go` — fixtures covering:
  - Legacy `auth-provider: azure` block → exec entry (assert exact YAML output).
  - Existing `kubelogin` exec entry → exec entry with this binary (server-id, tenant-id, client-id preserved).
  - Multi-user kubeconfig (one cert-auth user, one AAD user) — only the AAD user is rewritten.
  - Already-converted kubeconfig → unchanged, `changed=false`.
  - Admin kubeconfig (client cert only) → unchanged.
  - Malformed YAML → error.
  - `--absolute-path` true vs false → command field differs.
- `credplugin/plugin_test.go` — fake `KUBERNETES_EXEC_INFO`:
  - Unset → v1beta1 output.
  - v1 set → v1 output.
  - v1beta1 set → v1beta1 output.
  - Unknown apiVersion → error.
  - Token mint failure → error with non-zero exit (test the formatter, not the exit).

### Manual integration

Documented in the PR description, not automated:

- `az aks get-credentials` against a real AAD-enabled cluster with `kubelogin` not in PATH → `kubectl get nodes` succeeds.
- `az aks convert-kubeconfig` against a kubeconfig produced by Python `az` CLI → diff shows only the exec entry changed → `kubectl get nodes` succeeds.
- `az aks bastion` against a real bastion'd cluster with `kubelogin` not in PATH → tunnel up, `kubectl get nodes` succeeds, ctrl-C closes cleanly.
- `az aks install-cli` on a clean machine → installs kubectl only, no kubelogin step.

## Out of scope

- Token caching beyond what kubectl does via `expirationTimestamp`.
- PoP tokens (`--pop-enabled` in kubelogin).
- Cloud environments other than AzurePublicCloud.
- Migration of pre-existing kubeconfigs on disk other than via `convert-kubeconfig` (no auto-scan).
- Removing `kubectl` as a dependency. (Different problem; `kubectl` is the standard k8s client and not Microsoft-specific.)
