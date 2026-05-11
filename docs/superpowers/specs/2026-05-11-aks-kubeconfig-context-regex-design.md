# AKS kubeconfig context regex rename — Design

**Date:** 2026-05-11
**Status:** Approved (pending implementation plan)
**Scope:** Add `--context-regex` / `--context-replacement` flags to `az aks get-credentials` and `az aks bastion`, applying a regex transform to the cluster name and propagating the result to every identifier in the resulting kubeconfig.

## Motivation

Customer clusters deployed on customer infrastructure are all named identically (e.g., `proscia-prod-usw2-k8s-20251209`) so customers see a consistent product name. Internal support staff manage many of these clusters and cannot tell them apart from the kubeconfig identifiers alone. They need a way to rewrite the cluster's identifiers into a disambiguating form (e.g., `acme-prod-usw2-k8s-20251209`) at kubeconfig generation time.

A literal `--context <name>` flag already exists on `get-credentials`, but it only takes effect when output is `-f -` (stdout). It does not apply to write or merge modes, where support staff actually consume the kubeconfig. The new regex flags fix that and add pattern-based rewriting.

## Behavior

### New flags

Available on both `az aks get-credentials` and `az aks bastion`:

```
--context-regex <pattern>        Go regexp pattern matched against the cluster name.
                                 Required together with --context-replacement.
--context-replacement <string>   Replacement string. Supports Go regexp $1, $2 capture
                                 group references.
```

### Validation

- `--context-regex` and `--context-replacement` must be supplied together (both or neither).
- `--context-regex` is mutually exclusive with the existing literal `--context` flag.
- Invalid regex returns an error before any Azure call or file write.
- An empty `--context-replacement` is allowed (effectively a "remove the matched portion" operation).

### Example

```
az aks get-credentials \
  -n proscia-prod-usw2-k8s-20251209 -g rg \
  --context-regex '^proscia-(.+)$' \
  --context-replacement 'acme-$1'
```

The resulting kubeconfig (whether stdout, file, or merged into `~/.kube/config`) contains `acme-prod-usw2-k8s-20251209` in place of the original name across every identifier field.

## Rename algorithm

The regex is anchored on the **cluster name**, not applied independently to every field:

1. Take the existing cluster name from the kubeconfig (`clusters[0].name`).
2. Apply `pattern.ReplaceAllString(oldName, replacement)` to derive `newName`.
3. If `newName == oldName`, no-op (preserve input bytes' semantics).
4. Walk the YAML and replace every substring occurrence of `oldName` with `newName` in these fields:
   - `current-context`
   - `clusters[].name`
   - `contexts[].name`
   - `contexts[].context.cluster`
   - `contexts[].context.user`
   - `users[].name`

### Why anchor on the cluster name (not per-field)

Per-field regex application has two failure modes that anchoring avoids:

- **Anchored patterns break:** A user pattern like `^proscia-prod-usw2-k8s-20251209$` only matches when applied to the bare cluster name. Applied independently to `clusterUser_proscia-prod-usw2-k8s-20251209`, it would not match (the field doesn't equal the cluster name), leaving the user entry untouched and the kubeconfig internally inconsistent.
- **User name prefixes:** `clusterUser_<name>` and `clusterAdmin_<name>` carry a fixed prefix. Anchoring on the cluster name and then substring-replacing across fields preserves the prefix automatically: `clusterUser_proscia-...` → `clusterUser_acme-...` without the user having to write a regex aware of the prefix.

The cost of anchoring is that the regex must match somewhere in the cluster name to do anything. That is the intended UX — the user is renaming a cluster, not arbitrarily rewriting kubeconfig YAML.

## Code structure

### New package helper

**`pkg/kubeconfig/rename.go`** (new file):

```go
// RenameByRegex applies pattern.ReplaceAllString to the kubeconfig's cluster
// name (clusters[0].name) to derive a new name, then replaces every substring
// occurrence of the old name with the new name across identifier fields:
// current-context, clusters[].name, contexts[].name, contexts[].context.cluster,
// contexts[].context.user, users[].name.
//
// Returns the input unchanged if the regex does not transform the cluster name.
func RenameByRegex(kubeConfig []byte, pattern *regexp.Regexp, replacement string) ([]byte, error)
```

### get-credentials path

`internal/aks/credentials.go` and `internal/aks/commands.go`:

- Add `ContextRegex *regexp.Regexp` and `ContextReplacement string` to `GetCredentialsOptions`.
- After fetching `kubeConfig` from Azure and before any output branch, if `ContextRegex != nil`, call `RenameByRegex` to transform the bytes.
- The renamed bytes flow through all three output branches (stdout, overwrite, merge) unchanged. This is also where we **fix the existing `--context` literal flag** so it applies in write/merge modes too, not just stdout.

### bastion path

`internal/aks/bastion.go` and `internal/aks/kubeconfig.go`:

- Add `ContextRegex *regexp.Regexp` and `ContextReplacement string` to `BastionOptions`.
- Bastion owns its kubeconfig template (it is built locally from a format string, not returned by Azure). Compute `effectiveName := pattern.ReplaceAllString(clusterName, replacement)` up front and pass `effectiveName` to `WriteKubeconfig` as the cluster name. No YAML round-trip required.
- The user-facing `"Merged %q as current context in %s"` message uses `effectiveName`.

### Flag wiring

`internal/aks/commands.go`:

- Register `--context-regex` and `--context-replacement` on both `getCredsCmd` and `bastionCmd`.
- A shared validation helper compiles the regex and enforces the pair-required and mutual-exclusion rules. Returns a `*regexp.Regexp` (or nil) plus an error.

## Tests

**`pkg/kubeconfig/rename_test.go`** (new):

- Basic substring rename touches all six identifier fields.
- Capture-group replacement (`$1`) works.
- Anchored pattern (`^cluster-name$`) works against the bare cluster name.
- No-match input returns semantically equivalent output (cluster name unchanged in all fields).
- `clusterUser_` prefix preserved through rename.
- `clusterAdmin_` prefix preserved through rename (admin credentials kubeconfig).
- Empty replacement string allowed.

**`internal/aks/kubeconfig_test.go`** (extend):

- `WriteKubeconfig` with an effective name different from the underlying cluster name produces a template with the effective name in every position.

## Out of scope

- Renaming based on Azure resource tags or external lookup (e.g., "customer name from a tag").
- Per-field independent regex rules.
- Backwards-compatibility shims for the existing `--context` stdout-only behavior — we are fixing the quirk, not preserving it.
