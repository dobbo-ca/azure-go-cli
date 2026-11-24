# az aks bastion

Maintainer design notes for `az aks bastion` (`internal/aks/bastion.go`, `internal/aks/kubeconfig.go`, flag wiring in `internal/aks/commands.go`).

## Purpose

Reach a private AKS API server through Azure Bastion with one command. The tunnel runs inside the `az` process, the kubeconfig is generated for it, and the user either gets a command run against the cluster (`--cmd`) or an `export KUBECONFIG=...` line to use in another shell.

### Difference from Python azure-cli

Python has this only in the `aks-preview` extension, as `az aks bastion tunnel` (`az aks bastion` became a group: enable/disable/update/tunnel).

| | Go `az aks bastion` | Python `az aks bastion tunnel` |
|---|---|---|
| Availability | Built in | Extension only |
| Tunnel | In-process (`bastion.Tunnel`), no `az network bastion` subprocess | Shells out to `az network bastion tunnel` (needs the `bastion` extension, installed on demand). Its kubeconfig comes from `az aks get-credentials`, so kubectl uses kubelogin on Entra ID clusters |
| After connect | Runs `--cmd`, or prints/copies `export KUBECONFIG=...` and waits for Ctrl+C | Starts an interactive subshell |
| Kubeconfig | `-f/--kubeconfig PATH` writes a new file and keeps it; default is a temp file deleted on exit | `--kubeconfig-path` must be an existing file; its server URL is rewritten in place |
| Session pinning | Exec env pins `PATH` and `AZ_SESSION` | None |
| Rename / buffer flags | `--context-regex`, `--context-replacement`, buffer flags | None |
| Bastion discovery | `--bastion` is required | Discovers managed/node-resource-group bastion when omitted |
| Bastion identity | Full resource ID only; looked up in the default subscription, the subscription in the ID is ignored | Accepts a name or ID; uses the subscription from the ID |

## Flow

```
az aks bastion -n -g --bastion ID [--cmd C] [-f PATH]
  |
  +- port = --port, or random in 49152-65535 when 0
  +- ManagedClusters.Get -> cluster.ID, Properties.Fqdn
  +- kubeconfig: -f PATH  -> WriteKubeconfig (kept)
  |              default  -> CreateTempKubeconfig (dir removed on return)
  +- parseBastionResourceID(--bastion) -> name, resource group
  +- goroutine: bastion.Tunnel(..., cluster.ID, 443, port, BufferConfig)
  +- sleep 2s; if the tunnel already returned an error -> fail
  |
  +- --cmd set:  AuthenticateKubeconfig  (kubectl get nodes warm-up, errors ignored)
  |              RunCommand (KUBECONFIG=..., stdio attached)
  |              return -> deferred cancelTunnel() closes tunnel
  +- else:       print "export KUBECONFIG=PATH", copy to clipboard,
                 wait for tunnel error or SIGINT/SIGTERM -> cancelTunnel()
```

## Key pieces

- `Bastion` (`bastion.go`): orchestration. `BastionOptions` carries the flags.
- `WriteKubeconfig(path, clusterName, server, port, absolutePath)` (`kubeconfig.go`): writes mode 0600 YAML. Cluster server is `https://127.0.0.1:<port>` with `insecure-skip-tls-verify: true` (the API server cert does not match 127.0.0.1). The user is an exec plugin: `az aks get-token --server-id <credplugin.AKSServerIDDefault>`, `interactiveMode: IfAvailable`, `provideClusterInfo: false`.
- `CreateTempKubeconfig`: `os.MkdirTemp("az-aks-bastion-*")/.kube/config`. `Bastion` removes the temp root (two levels up from the file) on return.
- `AuthenticateKubeconfig`: runs `kubectl get nodes --request-timeout=10s`, as a warm-up. kubectl errors are ignored here; the real command surfaces them. It also scans stderr for a 9-char `[A-Z0-9]` code and an `https://` URL and runs `handleDeviceCodeFlow`, but this is dead code left over from kubelogin: the built-in `get-token` is silent (`AcquireTokenSilent`, non-interactive broker or Azure CLI credential) and never emits a device code. Run `az login` first.
- `RunCommand`: splits `--cmd` with `strings.Fields`, resolves the executable with `exec.LookPath`, runs it with `KUBECONFIG` appended to the environment and stdin/stdout/stderr attached.
- `parseBastionResourceID`: accepts `.../bastionHosts/<n>` or `.../virtualNetworks/<n>` (the vnet name is then taken as the bastion name); requires `Microsoft.Network`. Returns name and resource group.
- Flags (`commands.go`): `-n/--name`, `-g/--resource-group`, `--bastion` (all required), `-a/--admin`, `-p/--port` (0 = random), `--cmd`, `-f/--kubeconfig`, `--conn-read-buffer` (32), `--conn-write-buffer` (32), `--chunk-read-buffer` (8), `--chunk-write-buffer` (8) in KB, `--absolute-path`, `--context-regex`/`--context-replacement` (`contextregex.go`, same semantics as `get-credentials`; see [context regex spec](../superpowers/specs/2026-05-11-aks-kubeconfig-context-regex-design.md)). `--subscription` (or the default) is used for the cluster lookup only. The bastion is always looked up in the configured default subscription (`bastion.Tunnel` calls `config.GetDefaultSubscription()`); the subscription in the `--bastion` ID is ignored.

## Decisions

- **Built-in `get-token`, no kubelogin.** The exec entry calls this binary's `aks get-token`, so there is no external credential plugin. See [bake-in kubelogin spec](../superpowers/specs/2026-05-14-bake-in-kubelogin-design.md). Only `kubectl` is checked (`CheckDependencies`); a missing kubectl prints a warning and does not stop the tunnel. With `--cmd`, the kubectl warm-up then fails and the command aborts with `authentication failed: failed to start kubectl`.
- **Pin `PATH` and `AZ_SESSION` in the exec env.** `PATH` is `<dir of os.Executable()>:<current PATH>` so kubectl finds this `az` even when Python `az` is earlier on the user's PATH. `AZ_SESSION` is written only when set, so token minting in kubectl subprocesses reads the same profile/cache as the parent (see README, session isolation).
- **`--absolute-path`.** Replaces `command: az` with `os.Executable()`. Needed for local testing when another `az` shadows the build; see `.claude/rules/aks-kubeconfig-testing.md`.
- **Random port** uses `crypto/rand` in 49152-65535 to avoid collisions between concurrent sessions. The port is not checked for availability.
- **`--cmd` runs a kubectl warm-up first** so token acquisition and connection errors happen before the command starts, rather than inside a TUI such as k9s. Nothing prompts; tokens come silently from the `az login` session.
- **`-f` writes a fresh file** (`os.WriteFile`, 0600; parent created with `MkdirAll` 0700) rather than editing an existing kubeconfig. It overwrites whatever is at the path, and the file is left in place after exit.
- **Lifetime is tied to a context.** The tunnel goroutine uses `tunnelCtx`; returning from `Bastion` cancels it.

## Edge cases and limitations

- `--bastion` is required; there is no discovery. It must be a full resource ID, and the bastion must be in the default subscription (the ID's subscription is ignored).
- `--admin` is parsed and stored in `BastionOptions.Admin`, but `Bastion` never reads it. It currently has no effect.
- Tunnel start is detected by a fixed `time.Sleep(2s)` plus a non-blocking read of the error channel. A tunnel that fails later is only noticed in non-`--cmd` mode (the `select` on `tunnelErrCh`); in `--cmd` mode the command just fails to connect.
- Interrupt handling (`SIGINT`/`SIGTERM`) is installed only in the non-`--cmd` branch. In `--cmd` mode SIGINT reaches both az and the child. az has no handler, so Go exits at once and the deferred temp-dir removal does not run; the same happens on Ctrl+C during the initial 2s sleep in either mode.
- The `PATH` value in `WriteKubeconfig` is joined with a literal `:`, which is wrong for Windows `;` lists.
- `RunCommand` splits on whitespace: no shell, pipes, redirects or quote handling; quoted arguments, including `sh -c '...'`, are split too and break.
- Clipboard and browser helpers exist only for macOS and Linux utilities; failures are logged at debug level and the flow continues.
- `insecure-skip-tls-verify: true` is intentional; traffic reaches the API server only via the local loopback tunnel.
- If the cluster has no `Fqdn`, the empty string is passed to `WriteKubeconfig` as `server`, which is unused (the kubeconfig always targets `127.0.0.1`).

## Tests

`internal/aks/kubeconfig_test.go`: `TestKubeconfigPinsAZSession`, `TestKubeconfigOmitsAZSessionWhenUnset`, `TestWriteKubeconfig_EffectiveNameRenamesAllPositions`, `TestWriteKubeconfig_UsesAzGetToken`, `TestWriteKubeconfig_AbsolutePath`. `parseContextRegexFlags`, `parseBastionResourceID`, the tunnel itself and the `--cmd` path have no unit tests; verify them manually with `--absolute-path` against a real cluster. Run with `make test`.
