# AKS kubeconfig context regex rename — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--context-regex` / `--context-replacement` flags to `az aks get-credentials` and `az aks bastion` that anchor a regex on the cluster name and propagate the rename to every kubeconfig identifier.

**Architecture:** A new pure helper `pkg/kubeconfig.RenameByRegex` parses kubeconfig YAML, finds the cluster name at `clusters[0].name`, derives a new name via `pattern.ReplaceAllString`, and substring-replaces the old name across every identifier field. `get-credentials` calls the helper after Azure returns the YAML (and before stdout/write/merge). `bastion` owns its template, so it computes the renamed name up front and passes it to `WriteKubeconfig`. A small flag-validation helper in the aks package is reused by both commands.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `regexp`, `github.com/spf13/cobra`.

---

## File Structure

**New files:**
- `pkg/kubeconfig/rename.go` — `RenameByRegex` helper that parses kubeconfig YAML and applies the cluster-name-anchored substring rename.
- `pkg/kubeconfig/rename_test.go` — unit tests for `RenameByRegex`.
- `internal/aks/contextregex.go` — flag-validation helper `parseContextRegexFlags` returning `(*regexp.Regexp, string, error)`, shared by `get-credentials` and `bastion`.

**Modified files:**
- `internal/aks/credentials.go` — accept regex + replacement, apply `RenameByRegex` to fetched kubeconfig, fix existing literal `--context` to apply in write/merge modes too.
- `internal/aks/bastion.go` — accept regex + replacement, compute effective name, pass to `WriteKubeconfig`.
- `internal/aks/commands.go` — register `--context-regex` and `--context-replacement` on both `getCredsCmd` and `bastionCmd`, wire validation helper.
- `internal/aks/kubeconfig_test.go` — extend with a test that `WriteKubeconfig` honors an effective name distinct from the underlying cluster name.

---

## Task 1: `RenameByRegex` helper — failing test

**Files:**
- Create: `pkg/kubeconfig/rename_test.go`

This task establishes the contract for the helper before implementation.

- [ ] **Step 1: Create the test file with a basic substring-rename test**

Create `pkg/kubeconfig/rename_test.go`:

```go
package kubeconfig

import (
	"regexp"
	"strings"
	"testing"
)

const sampleUserKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io:443
    certificate-authority-data: AAAA
  name: appcluster-prod-usw2-k8s-20251209
contexts:
- context:
    cluster: appcluster-prod-usw2-k8s-20251209
    user: clusterUser_appcluster-prod-usw2-k8s-20251209
  name: appcluster-prod-usw2-k8s-20251209
current-context: appcluster-prod-usw2-k8s-20251209
users:
- name: clusterUser_appcluster-prod-usw2-k8s-20251209
  user:
    token: redacted
`

func TestRenameByRegex_BasicSubstring(t *testing.T) {
	pattern := regexp.MustCompile(`appcluster`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "acme")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)

	mustContain := []string{
		"name: acme-prod-usw2-k8s-20251209",
		"cluster: acme-prod-usw2-k8s-20251209",
		"user: clusterUser_acme-prod-usw2-k8s-20251209",
		"current-context: acme-prod-usw2-k8s-20251209",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("expected output to contain %q\n--- got ---\n%s", s, got)
		}
	}
	if strings.Contains(got, "appcluster") {
		t.Errorf("expected all occurrences of 'appcluster' to be replaced\n--- got ---\n%s", got)
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test ./pkg/kubeconfig/ -run TestRenameByRegex_BasicSubstring`
Expected: build failure — `undefined: RenameByRegex`.

- [ ] **Step 3: Commit the failing test**

```bash
git add pkg/kubeconfig/rename_test.go
git commit -m "test(kubeconfig): add failing test for RenameByRegex basic substring rename"
```

---

## Task 2: `RenameByRegex` helper — minimal implementation

**Files:**
- Create: `pkg/kubeconfig/rename.go`

- [ ] **Step 1: Implement the helper**

Create `pkg/kubeconfig/rename.go`:

```go
package kubeconfig

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// RenameByRegex applies pattern.ReplaceAllString to the kubeconfig's cluster
// name (clusters[0].name) to derive a new name, then replaces every substring
// occurrence of the old name with the new name across identifier fields:
// current-context, clusters[].name, contexts[].name, contexts[].context.cluster,
// contexts[].context.user, users[].name.
//
// If the input has no cluster name or the regex does not transform it, the
// kubeconfig is returned unchanged (semantically; YAML formatting may differ
// only if the input itself was non-canonical).
func RenameByRegex(kubeConfig []byte, pattern *regexp.Regexp, replacement string) ([]byte, error) {
	if pattern == nil {
		return kubeConfig, nil
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(kubeConfig, &cfg); err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}

	clusters, _ := cfg["clusters"].([]interface{})
	if len(clusters) == 0 {
		return kubeConfig, nil
	}
	firstCluster, _ := clusters[0].(map[string]interface{})
	oldName, _ := firstCluster["name"].(string)
	if oldName == "" {
		return kubeConfig, nil
	}

	newName := pattern.ReplaceAllString(oldName, replacement)
	if newName == oldName {
		return kubeConfig, nil
	}

	replace := func(s string) string {
		return strings.ReplaceAll(s, oldName, newName)
	}

	if cc, ok := cfg["current-context"].(string); ok {
		cfg["current-context"] = replace(cc)
	}

	for _, key := range []string{"clusters", "contexts", "users"} {
		list, _ := cfg[key].([]interface{})
		for _, item := range list {
			m, _ := item.(map[string]interface{})
			if m == nil {
				continue
			}
			if n, ok := m["name"].(string); ok {
				m["name"] = replace(n)
			}
			if ctx, ok := m["context"].(map[string]interface{}); ok {
				if c, ok := ctx["cluster"].(string); ok {
					ctx["cluster"] = replace(c)
				}
				if u, ok := ctx["user"].(string); ok {
					ctx["user"] = replace(u)
				}
			}
		}
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal kubeconfig: %w", err)
	}
	return out, nil
}
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `go test ./pkg/kubeconfig/ -run TestRenameByRegex_BasicSubstring -v`
Expected: `PASS`.

- [ ] **Step 3: Commit**

```bash
git add pkg/kubeconfig/rename.go
git commit -m "feat(kubeconfig): add RenameByRegex helper for context renaming"
```

---

## Task 3: `RenameByRegex` — capture groups, anchored patterns, admin prefix, no-match, empty replacement

**Files:**
- Modify: `pkg/kubeconfig/rename_test.go`

- [ ] **Step 1: Add the additional tests**

Append the following tests to `pkg/kubeconfig/rename_test.go`:

```go
const sampleAdminKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io:443
    certificate-authority-data: AAAA
  name: appcluster-prod-usw2-k8s-20251209
contexts:
- context:
    cluster: appcluster-prod-usw2-k8s-20251209
    user: clusterAdmin_appcluster-prod-usw2-k8s-20251209
  name: appcluster-prod-usw2-k8s-20251209-admin
current-context: appcluster-prod-usw2-k8s-20251209-admin
users:
- name: clusterAdmin_appcluster-prod-usw2-k8s-20251209
  user:
    token: redacted
`

func TestRenameByRegex_CaptureGroup(t *testing.T) {
	pattern := regexp.MustCompile(`^appcluster-(.+)$`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "acme-$1")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "acme-prod-usw2-k8s-20251209") {
		t.Errorf("expected capture-group replacement to produce acme-prod-usw2-k8s-20251209\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, "clusterUser_acme-prod-usw2-k8s-20251209") {
		t.Errorf("expected user prefix preserved with renamed suffix\n--- got ---\n%s", got)
	}
}

func TestRenameByRegex_AnchoredPattern(t *testing.T) {
	// Anchored pattern matches the bare cluster name only. It must still
	// propagate to user/context fields that contain that name as a substring.
	pattern := regexp.MustCompile(`^appcluster-prod-usw2-k8s-20251209$`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "mycluster")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)
	mustContain := []string{
		"name: mycluster",
		"cluster: mycluster",
		"user: clusterUser_mycluster",
		"current-context: mycluster",
		"name: clusterUser_mycluster",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("anchored pattern: expected %q\n--- got ---\n%s", s, got)
		}
	}
}

func TestRenameByRegex_AdminPrefixPreserved(t *testing.T) {
	pattern := regexp.MustCompile(`appcluster`)
	out, err := RenameByRegex([]byte(sampleAdminKubeconfig), pattern, "acme")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "clusterAdmin_acme-prod-usw2-k8s-20251209") {
		t.Errorf("expected clusterAdmin_ prefix preserved\n--- got ---\n%s", got)
	}
	if strings.Contains(got, "appcluster") {
		t.Errorf("expected all 'appcluster' replaced\n--- got ---\n%s", got)
	}
}

func TestRenameByRegex_NoMatchReturnsSemanticEquivalent(t *testing.T) {
	pattern := regexp.MustCompile(`nonexistent`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "whatever")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	if !strings.Contains(string(out), "appcluster-prod-usw2-k8s-20251209") {
		t.Errorf("expected original cluster name unchanged when regex does not match")
	}
}

func TestRenameByRegex_EmptyReplacement(t *testing.T) {
	pattern := regexp.MustCompile(`^appcluster-`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "name: prod-usw2-k8s-20251209") {
		t.Errorf("expected 'appcluster-' prefix stripped\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, "clusterUser_prod-usw2-k8s-20251209") {
		t.Errorf("expected user prefix preserved with stripped cluster name\n--- got ---\n%s", got)
	}
}

func TestRenameByRegex_NilPatternIsNoop(t *testing.T) {
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), nil, "ignored")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	if string(out) != sampleUserKubeconfig {
		t.Errorf("nil pattern should return input unchanged")
	}
}
```

- [ ] **Step 2: Run the new tests**

Run: `go test ./pkg/kubeconfig/ -v`
Expected: all tests pass (including the existing `TestRenameByRegex_BasicSubstring`).

- [ ] **Step 3: Commit**

```bash
git add pkg/kubeconfig/rename_test.go
git commit -m "test(kubeconfig): cover capture groups, anchors, admin prefix, no-match"
```

---

## Task 4: Flag validation helper

**Files:**
- Create: `internal/aks/contextregex.go`

- [ ] **Step 1: Write the flag-validation helper**

Create `internal/aks/contextregex.go`:

```go
package aks

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
)

// addContextRegexFlags registers --context-regex and --context-replacement on cmd.
func addContextRegexFlags(cmd *cobra.Command) {
	cmd.Flags().String("context-regex", "",
		"Regex matched against the cluster name; the replacement is propagated to every kubeconfig identifier. Requires --context-replacement.")
	cmd.Flags().String("context-replacement", "",
		"Replacement string for --context-regex (supports $1, $2 capture group references).")
}

// parseContextRegexFlags compiles --context-regex / --context-replacement and
// enforces the pair-required and mutual-exclusion constraints. Returns a nil
// pattern when neither flag is set. `literalContext` is the value of the
// existing --context flag (pass "" if the command does not register it).
func parseContextRegexFlags(cmd *cobra.Command, literalContext string) (*regexp.Regexp, string, error) {
	pattern, _ := cmd.Flags().GetString("context-regex")
	replacement, _ := cmd.Flags().GetString("context-replacement")

	regexSet := cmd.Flags().Changed("context-regex")
	replSet := cmd.Flags().Changed("context-replacement")

	if regexSet != replSet {
		return nil, "", fmt.Errorf("--context-regex and --context-replacement must be supplied together")
	}
	if !regexSet {
		return nil, "", nil
	}
	if literalContext != "" {
		return nil, "", fmt.Errorf("--context is mutually exclusive with --context-regex")
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, "", fmt.Errorf("invalid --context-regex: %w", err)
	}
	return compiled, replacement, nil
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./internal/aks/...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/aks/contextregex.go
git commit -m "feat(aks): add --context-regex/--context-replacement flag helpers"
```

---

## Task 5: Wire regex into `bastion` — failing test

**Files:**
- Modify: `internal/aks/kubeconfig_test.go`

The bastion path calls `WriteKubeconfig(path, clusterName, fqdn, port)`. We will change the call site to pass an `effectiveName` (the regex-transformed name). `WriteKubeconfig`'s signature does not change — the caller decides what to pass. This task adds a test that locks in the behavior: passing a different name to `WriteKubeconfig` renames every position in the template.

- [ ] **Step 1: Append the failing test**

Append to `internal/aks/kubeconfig_test.go`:

```go
func TestWriteKubeconfig_EffectiveNameRenamesAllPositions(t *testing.T) {
	tmp := t.TempDir() + "/config"
	// Pretend the caller already applied a regex transform: pass the renamed
	// name to WriteKubeconfig. Every position in the template must use it.
	if err := WriteKubeconfig(tmp, "acme-prod-usw2-k8s-20251209", "myfqdn", 12345); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	mustContain := []string{
		"name: acme-prod-usw2-k8s-20251209",
		"cluster: acme-prod-usw2-k8s-20251209",
		"user: clusterUser_acme-prod-usw2-k8s-20251209",
		"current-context: acme-prod-usw2-k8s-20251209",
		"- name: clusterUser_acme-prod-usw2-k8s-20251209",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("expected %q in kubeconfig:\n%s", s, got)
		}
	}
}
```

- [ ] **Step 2: Run the test to confirm it passes already**

Run: `go test ./internal/aks/ -run TestWriteKubeconfig_EffectiveNameRenamesAllPositions -v`
Expected: PASS. The current `WriteKubeconfig` already substitutes the supplied name into every position, so this test acts as a regression guard for the wiring change.

If it fails, stop and inspect `internal/aks/kubeconfig.go` `WriteKubeconfig`. Each of the five positions must use the supplied `clusterName` argument; if one was hardcoded or skipped, fix it.

- [ ] **Step 3: Commit**

```bash
git add internal/aks/kubeconfig_test.go
git commit -m "test(aks): assert WriteKubeconfig honors effective name in all template positions"
```

---

## Task 6: Wire regex into `bastion` — apply rename

**Files:**
- Modify: `internal/aks/bastion.go`

- [ ] **Step 1: Add fields to `BastionOptions`**

In `internal/aks/bastion.go`, locate the `BastionOptions` struct (around line 21). Add two fields and a `regexp` import.

Add to imports (alongside the existing imports at the top of the file):

```go
	"regexp"
```

Update the struct:

```go
// BastionOptions contains options for the bastion tunnel
type BastionOptions struct {
	ClusterName          string
	ResourceGroup        string
	BastionResourceID    string
	SubscriptionOverride string
	Admin                bool
	Port                 int
	Command              string // Command to run with KUBECONFIG set (e.g., "k9s" or "kubectl get pods")
	KubeconfigPath       string // If set, write kubeconfig to this path instead of a temp file (and don't delete it on exit)
	ContextRegex         *regexp.Regexp
	ContextReplacement   string
	BufferConfig         bastion.BufferConfig
}
```

- [ ] **Step 2: Compute `effectiveName` and use it for kubeconfig writes**

In the same file, inside `Bastion(ctx context.Context, opts BastionOptions) error`, locate the block that begins `clusterName := opts.ClusterName`. Immediately after, derive the effective name:

```go
	clusterName := opts.ClusterName
	effectiveName := clusterName
	if opts.ContextRegex != nil {
		effectiveName = opts.ContextRegex.ReplaceAllString(clusterName, opts.ContextReplacement)
	}
```

Then replace the two `WriteKubeconfig` / `CreateTempKubeconfig` calls and the "Merged" log line so they use `effectiveName` instead of `clusterName`:

Change:
```go
		if err := WriteKubeconfig(kubeconfigPath, clusterName, clusterFQDN, port); err != nil {
```
to:
```go
		if err := WriteKubeconfig(kubeconfigPath, effectiveName, clusterFQDN, port); err != nil {
```

Change:
```go
		kubeconfigPath, err = CreateTempKubeconfig(ctx, clusterName, clusterFQDN, port)
```
to:
```go
		kubeconfigPath, err = CreateTempKubeconfig(ctx, effectiveName, clusterFQDN, port)
```

Change:
```go
	fmt.Printf("Merged \"%s\" as current context in %s\n", clusterName, kubeconfigPath)
```
to:
```go
	fmt.Printf("Merged \"%s\" as current context in %s\n", effectiveName, kubeconfigPath)
```

Leave the `"Opening tunnel to AKS cluster %s..."` line using `clusterName` — that message is about the Azure resource, not the kubeconfig context.

- [ ] **Step 3: Build to verify the changes compile**

Run: `make build`
Expected: clean build, binary at `bin/az/az`.

- [ ] **Step 4: Run aks tests**

Run: `go test ./internal/aks/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/aks/bastion.go
git commit -m "feat(aks): apply context regex to bastion kubeconfig identifiers"
```

---

## Task 7: Wire regex into `get-credentials` and fix `--context` for write/merge modes

**Files:**
- Modify: `internal/aks/credentials.go`

- [ ] **Step 1: Add fields to `GetCredentialsOptions`**

In `internal/aks/credentials.go`, add a `regexp` import alongside the existing imports:

```go
	"regexp"
```

Update the struct (around line 15):

```go
type GetCredentialsOptions struct {
	ClusterName        string
	ResourceGroup      string
	Admin              bool
	File               string
	Overwrite          bool
	Context            string
	ContextRegex       *regexp.Regexp
	ContextReplacement string
}
```

- [ ] **Step 2: Apply rename in a single place before the output branches**

In the same file, replace the existing rename block (currently scoped to stdout only) with one that runs in all modes.

Find this block:

```go
	if kubeConfig == nil {
		return fmt.Errorf("no kubeconfig data returned")
	}

	// If context is specified but file is "-", output to stdout with context name updated
	if opts.Context != "" && opts.File == "-" {
		kubeConfig, err = kubeconfig.UpdateContext(kubeConfig, opts.Context)
		if err != nil {
			return fmt.Errorf("failed to update context: %w", err)
		}
	}
```

Replace with:

```go
	if kubeConfig == nil {
		return fmt.Errorf("no kubeconfig data returned")
	}

	// Apply context renaming before any output branch so stdout, write, and
	// merge all observe the renamed identifiers.
	if opts.ContextRegex != nil {
		kubeConfig, err = kubeconfig.RenameByRegex(kubeConfig, opts.ContextRegex, opts.ContextReplacement)
		if err != nil {
			return fmt.Errorf("failed to apply context regex: %w", err)
		}
	} else if opts.Context != "" {
		kubeConfig, err = kubeconfig.UpdateContext(kubeConfig, opts.Context)
		if err != nil {
			return fmt.Errorf("failed to update context: %w", err)
		}
	}
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: clean build.

- [ ] **Step 4: Run aks tests**

Run: `go test ./internal/aks/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/aks/credentials.go
git commit -m "feat(aks): apply --context and --context-regex in all get-credentials output modes"
```

---

## Task 8: Register the new flags on both commands

**Files:**
- Modify: `internal/aks/commands.go`

- [ ] **Step 1: Register flags and wire validation on `getCredsCmd`**

In `internal/aks/commands.go`, find the `getCredsCmd` definition (around line 43). Inside its `RunE`, after `contextName, _ := cmd.Flags().GetString("context")`, add the validation call and wire the result into `GetCredentialsOptions`:

Replace this block:
```go
			contextName, _ := cmd.Flags().GetString("context")

			opts := GetCredentialsOptions{
				ClusterName:   clusterName,
				ResourceGroup: resourceGroup,
				Admin:         admin,
				File:          file,
				Overwrite:     overwrite,
				Context:       contextName,
			}

			return GetCredentials(context.Background(), opts)
```

with:
```go
			contextName, _ := cmd.Flags().GetString("context")

			contextRegex, contextReplacement, err := parseContextRegexFlags(cmd, contextName)
			if err != nil {
				return err
			}

			opts := GetCredentialsOptions{
				ClusterName:        clusterName,
				ResourceGroup:      resourceGroup,
				Admin:              admin,
				File:               file,
				Overwrite:          overwrite,
				Context:            contextName,
				ContextRegex:       contextRegex,
				ContextReplacement: contextReplacement,
			}

			return GetCredentials(context.Background(), opts)
```

Then, after the existing `getCredsCmd.Flags().String("context", ...)` line, register the new flags. Find:

```go
	getCredsCmd.Flags().String("context", "", "Set context name (only applicable with -f -)")
```

Replace the help text and add the regex flags:

```go
	getCredsCmd.Flags().String("context", "", "Set context name (literal rename of all identifiers)")
	addContextRegexFlags(getCredsCmd)
```

- [ ] **Step 2: Register flags and wire validation on `bastionCmd`**

Find the `bastionCmd` definition. Inside its `RunE`, after `kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")`, add validation:

Replace this block:
```go
			kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")

			bufferConfig := bastion.DefaultBufferConfig()
```

with:
```go
			kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")

			contextRegex, contextReplacement, err := parseContextRegexFlags(cmd, "")
			if err != nil {
				return err
			}

			bufferConfig := bastion.DefaultBufferConfig()
```

And update the `BastionOptions` literal a few lines below to pass the regex fields:

Replace:
```go
			opts := BastionOptions{
				ClusterName:          clusterName,
				ResourceGroup:        resourceGroup,
				BastionResourceID:    bastionResourceID,
				SubscriptionOverride: subscription,
				Admin:                admin,
				Port:                 port,
				Command:              cmdToRun,
				KubeconfigPath:       kubeconfigPath,
				BufferConfig:         bufferConfig,
			}
```

with:
```go
			opts := BastionOptions{
				ClusterName:          clusterName,
				ResourceGroup:        resourceGroup,
				BastionResourceID:    bastionResourceID,
				SubscriptionOverride: subscription,
				Admin:                admin,
				Port:                 port,
				Command:              cmdToRun,
				KubeconfigPath:       kubeconfigPath,
				ContextRegex:         contextRegex,
				ContextReplacement:   contextReplacement,
				BufferConfig:         bufferConfig,
			}
```

Then register the flags. Find the last `bastionCmd.Flags().Int(...)` line:

```go
	bastionCmd.Flags().Int("chunk-write-buffer", 8, "Streaming chunk write buffer size in KB (default 8)")
```

Immediately after that line, add:

```go
	addContextRegexFlags(bastionCmd)
```

- [ ] **Step 3: Build and check help output**

Run: `make build && ./bin/az/az aks get-credentials --help`
Expected: clean build; `--context-regex` and `--context-replacement` appear in the flag list.

Run: `./bin/az/az aks bastion --help`
Expected: `--context-regex` and `--context-replacement` appear in the flag list.

- [ ] **Step 4: Exercise validation manually**

Run: `./bin/az/az aks get-credentials -n x -g y --context-regex foo`
Expected: error `--context-regex and --context-replacement must be supplied together`.

Run: `./bin/az/az aks get-credentials -n x -g y --context me --context-regex foo --context-replacement bar`
Expected: error `--context is mutually exclusive with --context-regex`.

Run: `./bin/az/az aks get-credentials -n x -g y --context-regex '[' --context-replacement bar`
Expected: error mentioning `invalid --context-regex` and a regexp parse error.

(These commands fail before hitting Azure; no credentials required.)

- [ ] **Step 5: Commit**

```bash
git add internal/aks/commands.go
git commit -m "feat(aks): register --context-regex/--context-replacement flags"
```

---

## Task 9: Full verification

- [ ] **Step 1: Run the full test suite**

Run: `make test`
Expected: all tests pass.

- [ ] **Step 2: Confirm the binary is clean**

Run: `make build && ls -la bin/az/az`
Expected: clean build; binary exists.

- [ ] **Step 3: Confirm the help text reads well**

Run: `./bin/az/az aks get-credentials --help | grep -A1 'context'`
Expected: three flags listed — `--context`, `--context-regex`, `--context-replacement`, each with the descriptions written in Task 4 and Task 8.

Run: `./bin/az/az aks bastion --help | grep -A1 'context'`
Expected: `--context-regex` and `--context-replacement` listed (no literal `--context` on bastion — there never was one).

---

## Notes for the executor

- The plan does not modify `pkg/kubeconfig/merge.go` or the existing `UpdateContext` helper. Both stay in use for the literal `--context` path.
- The bastion command does not currently register `--context` (literal). We are not adding it as part of this work — out of scope.
- The renamed context is what appears in the bastion's "Merged \"X\" as current context" stderr message. The "Opening tunnel to AKS cluster X..." message keeps the Azure resource name. This distinction is intentional.
- `parseContextRegexFlags` takes the literal `--context` value rather than reading it via `cmd.Flags()` because not every command registers `--context`; passing `""` for commands without it keeps the helper general.
