# az version and Terraform azurerm compatibility

## Purpose

`az version` prints a single JSON line. Terraform's `azurerm` provider shells out to `az version`, parses the `azure-cli` field and requires `>= MinimumVersion` (2.0.81) and `< NextMajorVersion` (3.0.0) (go-azure-sdk `sdk/internal/azurecli/versions.go`). This CLI is not Microsoft's `azure-cli`, so its own version (`v1.x`, or `dev`) would fail that check. The command therefore reports a fixed, compatible `azure-cli` value and exposes the real version in separate fields.

## Output

```
{"azure-cli": "2.67.0", "azure-cli-go": "<version>", "azure-cli-go-commit": "<commit>", "azure-cli-go-build-date": "<date>"}
```

Single line, newline-terminated, always JSON. `--output`, `--query` and the other root persistent flags are accepted but ignored; the output is a hard-coded `fmt.Printf`, not routed through the output formatter.

## How it works

```
make build / CI  --ldflags -X main.version/commit/date-->  vars in cmd/az/main.go
                                                              |
                          az version  -->  RunE  -->  fmt.Printf(JSON)  -->  stdout
```

- `cmd/az/main.go:40-45`: package vars `version="dev"`, `commit="unknown"`, `date="unknown"`, overridden at link time.
- `cmd/az/main.go:67-78`: the `version` cobra command, registered directly on `rootCmd` (not a domain package).
- `Makefile:12-18`: `VERSION` from `git describe --tags --always --dirty` (falls back to `dev`), `COMMIT` from `git rev-parse --short HEAD`, `BUILD_DATE` from `date -u '+%Y-%m-%d_%H:%M:%S'`.
- `.github/workflows/msi-smoke.yml:51`: stamps the same three vars for the Windows MSI smoke build.
- `.github/workflows/release.yml:25-32` -> dobbo-ca/.github `release-go.yml` (`version-pkg: main`): stamps release binaries with the tag version, the full `GITHUB_SHA` commit and an RFC3339 UTC date (`%Y-%m-%dT%H:%M:%SZ`). Note that `make build` uses a short SHA and a `YYYY-MM-DD_HH:MM:SS` date instead.

## Decisions

- **Fixed `azure-cli` = `2.67.0`.** Required so the azurerm version-range check passes. It is a compatibility claim, not our version. If azurerm's supported range changes, bump the literal at `cmd/az/main.go:74`; it must stay below azurerm's `NextMajorVersion`.
- **Separate `azure-cli-go*` keys.** Keeps the real version, commit and build date available without disturbing the field Terraform parses. They are also the documented way to tell the two CLIs apart, since both print `azure-cli`.
- **No `--version` flag.** Only the `version` subcommand exists; `az --version` fails with `unknown flag: --version`.

## Differences from Python azure-cli

Python's `get_az_version_json` (azure-cli-core `util.py`) returns `azure-cli` and `azure-cli-core` (both the hard-coded `azure.cli.core.__version__`), `azure-cli-telemetry`, and an `extensions` map of installed extensions. It also supports `az --version`, which prints a human-readable table. This CLI emits none of `azure-cli-core`, `azure-cli-telemetry` or `extensions`, and has no `--version`. Anything parsing those keys will not find them.

## Edge cases and limitations

- Values are interpolated into the JSON with `%s`, unescaped. Version/commit/date strings containing `"` or `\` would produce invalid JSON; the build sources above do not produce them.
- A binary built with plain `go build` reports `dev` / `unknown` / `unknown`.
- `azure-cli` is not a truthful statement about parity with Python azure-cli 2.67.0.
- Because both CLIs report `azure-cli`, detect this one by the presence of `azure-cli-go`.

## Tests

No Go unit test covers the command. CI check: `.github/workflows/msi-smoke.yml:115-118` runs `az version` from the installed MSI, requires exit 0 and that the output matches `azure-cli-go`.

## See also

User-facing install notes: [windows-install.md](../windows-install.md) (verifying the install, and telling this CLI from Microsoft's).
