# Rule: testing `az aks` commands that generate kubeconfigs

Applies to `az aks get-credentials`, `az aks bastion`, and `az aks convert-kubeconfig` — any command that writes a kubeconfig with a `command:` exec entry.

## Use `--absolute-path` for local end-to-end tests

When testing one of these commands locally, pass `--absolute-path` so the generated kubeconfig's exec entry points at the exact binary under test:

```bash
./bin/az/az aks bastion ... --absolute-path --cmd "kubectl get nodes"
```

## Why

The kubeconfig exec entry defaults to `command: az`, which kubectl resolves via `PATH` when it mints a token. During local dev, another `az` earlier in `PATH` (e.g. the Homebrew `az-go` at `/opt/homebrew/bin/az`) shadows the fresh `./bin/az/az` build. kubectl then invokes the *wrong* binary for `az aks get-token`, producing:

```
Error: unknown flag: --server-id
```

because the shadowing binary predates the `get-token` subcommand. The new binary generates a correct kubeconfig, but the old binary gets called to mint the token. (The bastion temp kubeconfig also prepends the running binary's dir to `PATH`, but that still resolves wrong if `os.Executable()` lands in a dir whose `az` is the old binary — e.g. when invoked via a Homebrew symlink that resolves into the Cellar dir.)

`--absolute-path` swaps `command: az` for the `os.Executable()` absolute path, bypassing `PATH` entirely.

## When it is NOT needed

For real end users who install our binary *as* `az` (replacing any other `az`), the default `command: az` resolves correctly. `--absolute-path` exists for the dev/test shadowing case and for users who keep multiple `az` binaries.

## Where the logic lives

- `internal/aks/credplugin/convert.go` — `commandField(absolute bool)`
- `internal/aks/kubeconfig.go` — `WriteKubeconfig(..., absolutePath bool)`
