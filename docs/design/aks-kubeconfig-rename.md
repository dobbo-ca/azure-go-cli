# AKS kubeconfig identifier rename

`az aks get-credentials` and `az aks bastion` can rename every identifier in the
kubeconfig they produce. This is a Go-CLI-only capability.

Original design history: [2026-05-11 spec](../superpowers/specs/2026-05-11-aks-kubeconfig-context-regex-design.md),
[plan](../superpowers/plans/2026-05-11-aks-kubeconfig-context-regex.md).

## Purpose

Customer clusters often share one name (for example `appcluster-prod-usw2-k8s-20251209`).
Merging several of them into `~/.kube/config` collides: `Merge` keys clusters, contexts
and users by `name`, so the last one wins. Renaming at generation time gives each a
distinct set of identifiers.

## Flags

| Flag | Commands | Meaning |
|------|----------|---------|
| `--context <name>` | `get-credentials` only | Literal rename of the cluster name |
| `--context-regex <re>` | `get-credentials`, `bastion` | Go `regexp` applied to the cluster name |
| `--context-replacement <str>` | `get-credentials`, `bastion` | Replacement; `$1`, `$2` supported |

Constraints (`parseContextRegexFlags`, `internal/aks/contextregex.go`):

- regex and replacement must both be set or both unset (checked with `Flags().Changed`, so an explicit empty replacement is valid and deletes the match);
- `--context-regex` cannot be combined with `--context`;
- an empty `--context-regex` and an invalid pattern are errors.

Validation runs in the cobra `RunE` before any Azure call (`internal/aks/commands.go`). `bastion` has no `--context`, so it passes `""` for `literalContext`.

## Difference from Python azure-cli

- No regex rename exists in Python.
- Python `--context` (`acs/_params.py:1118`) only rewrites `contexts[0].name`, `contexts[0].context.cluster`, `clusters[0].name` and `current-context`. It leaves the user entry alone and is ignored for `-f -` (`acs/custom.py:2183-2191, 2265-2272`).
- Here `--context` goes through the same all-identifier rename as the regex path, including `users[].name` and `contexts[].context.user`, and it applies to stdout output too.
- Python's equivalent, `az aks bastion tunnel` (aks-preview `_params.py:4083-4093`, `aks bastion tunnel`), has no context flag.
- With `--admin`, Python also appends `-admin` to the context name (`acs/custom.py:2193-2200`, for example `acme-prod-admin`). This CLI has no such suffix logic, so the admin context is named exactly as renamed and admin and user credentials for the same cluster overwrite each other on merge.

## How it works

```
get-credentials:
  ARM kubeconfig bytes
    -> RenameByRegex (regex) | UpdateContext (--context)      pkg/kubeconfig
    -> credplugin.Convert                                     internal/aks/credplugin
    -> -f -  : print          (renamed)
       --overwrite-existing : write file
       default : kubeconfig.Merge into file                   merge keyed by name

bastion:
  clusterName -> effectiveName = regex.ReplaceAllString(clusterName, repl)
    -> WriteKubeconfig / CreateTempKubeconfig(effectiveName)  templated, no YAML rewrite
```

Key code:

- `pkg/kubeconfig/rename.go` `RenameByRegex(kubeConfig, pattern, replacement)`: parses YAML, takes `clusters[0].name` as the old name, computes `pattern.ReplaceAllString(old, replacement)`, and calls `renameClusterIdentifier`. Returns the input bytes untouched when the pattern is nil, there is no cluster, the name is empty, or the result equals the old name.
- `pkg/kubeconfig/rename.go` `renameClusterIdentifier(cfg, old, new)`: `strings.ReplaceAll` over `current-context`, `clusters[].name`, `contexts[].name`, `contexts[].context.cluster`, `contexts[].context.user`, `users[].name`. Mutates the map in place.
- `pkg/kubeconfig/merge.go` `UpdateContext(kubeConfig, name)`: the literal `--context` path; same shape, reuses `renameClusterIdentifier`.
- `internal/aks/credentials.go` `GetCredentials`: renames before `credplugin.Convert` and before the stdout / overwrite / merge branches, so every output mode sees the renamed identifiers. The "Merged ..." message reports `effectiveContext`.
- `internal/aks/bastion.go` `Bastion`: computes `effectiveName` and passes it to `WriteKubeconfig` / `CreateTempKubeconfig` (`internal/aks/kubeconfig.go`), which template the name into all six positions (`clusterUser_<name>` for the user).

## Decisions

- **Anchor on the cluster name, then substring-replace.** Applying the regex per field would break anchored patterns (`^name$` would not match `clusterUser_name`) and force users to account for the `clusterUser_` / `clusterAdmin_` prefix. Substring replacement keeps the prefix automatically. Cost: the regex must match the cluster name to do anything.
- **One shared helper for `--context` and regex**, so both rename the same fields.
- **Rename before `credplugin.Convert` and before output branching**, so stdout, overwrite and merge are consistent.
- **Bastion renames by templating**, not by rewriting YAML, because the bastion kubeconfig is generated locally from the name.
- **Rename is by `clusters[0]`**: an AKS credential response contains one cluster.

## Edge cases and limitations

- No match, or a result equal to the original: no-op, the original bytes are passed through.
- When a rename does occur, the kubeconfig is re-marshalled through `map[string]interface{}`, so key order and comments are not preserved (YAML is rewritten in canonical order).
- Substring replace also hits other occurrences of the old name inside identifier fields (intended for the user prefix). It does not touch other fields such as `server`.
- Regex is Go RE2 syntax, not Python `re`; no lookaheads or backreferences in the pattern.
- `$1` is expanded by the Go regexp engine (`ReplaceAllString`); in a shell, single-quote the replacement so `$1` is not expanded by the shell.
- `$name` is read greedily over letters, digits and underscore, so `$1_eu` is parsed as a group named `1_eu`, which does not exist, and expands to an empty string with no error (every identifier becomes empty). Write `${1}_eu` when the next character is a letter, digit or underscore.
- If the replacement leaves the name empty (for example the regex matches the whole name and the replacement is empty), `RenameByRegex` only short-circuits on `newName == oldName`, so no error is raised and the kubeconfig is written with empty cluster, context and `current-context` names and a user of `clusterUser_<rg>_`. Bastion templates an empty name the same way.
- `Merge` replaces same-name entries and sets `current-context` to the new one. Merged entry order is not stable (map iteration).
- `--context` is not available on `bastion`.
- Testing a bastion or get-credentials flow locally: see `.claude/rules/aks-kubeconfig-testing.md` (`--absolute-path`).

## Tests

- `pkg/kubeconfig/rename_test.go`: substring, capture groups, anchored pattern, admin prefix, no-match, empty replacement, nil pattern.
- `pkg/kubeconfig/merge_test.go`: `UpdateContext` renames all identifiers; no-clusters passthrough.
- `internal/aks/kubeconfig_test.go` `TestWriteKubeconfig_EffectiveNameRenamesAllPositions`: bastion template.
- There is no unit test for `parseContextRegexFlags` or the `GetCredentials` output ordering.

Run: `make test`.
