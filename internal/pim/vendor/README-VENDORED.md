# Vendored: netr0m/az-pim-cli

Source: <https://github.com/netr0m/az-pim-cli>
Upstream commit: `63d8f2ce47be44d61d15e92d964a1b35558e29f5` (release 1.14.0)
License: MIT (see `LICENSE` in this directory)

## Local modifications

The following changes are applied across Tasks 1–4 of `docs/superpowers/plans/2026-05-14-pim.md`. As of this commit, items 1, 3, 4, and 5 are applied; item 2 (slog→pkg/logger) is still pending. To re-sync from upstream after all four tasks land, re-apply these in order:

1. `client.go`, `utils.go`: all `os.Exit(1)` calls replaced with returned errors. Exported functions that previously returned `*T` now return `(*T, error)`. The `Client` interface and `AzureClient` methods were updated to match.
2. `client.go`, `utils.go`: `log/slog` calls replaced with our `pkg/logger` equivalents (`logger.Debug`, `logger.Info`, `logger.Error`).
3. `client.go`: `GetAccessToken` no longer constructs `azidentity.NewAzureCLICredential`. The `Client` interface's `GetAccessToken` is satisfied by `internal/pim/tokencred.go` instead. The default `AzureClient.GetAccessToken` was deleted.
4. `common.go`: created locally to absorb the one type we need from upstream `pkg/common` (`common.Error`). Upstream `InitLogger` is not copied — we use our own logger.
5. Import paths rewritten:
   - `github.com/netr0m/az-pim-cli/pkg/common` → `(removed; types live in this package)`
   - `github.com/netr0m/az-pim-cli/pkg/pim` → `(this package)`
6. `utils_test.go`: `TestParseDateTime` patched to compute the expected timezone offset from the parsed date (Dec 31 2024) rather than `time.Now()`. The upstream version fails whenever the machine clock is in a different DST window than the parsed date.
