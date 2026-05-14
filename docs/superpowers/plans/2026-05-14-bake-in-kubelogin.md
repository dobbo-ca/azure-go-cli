# Bake kubelogin into az binary — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the external `kubelogin` binary with built-in `az aks get-token` (kubectl exec credential plugin) and `az aks convert-kubeconfig` (kubeconfig rewriter); update `az aks get-credentials`, `az aks bastion`, and `az aks install-cli` accordingly.

**Architecture:** New `internal/aks/credplugin/` package owns ExecCredential rendering, the get-token entry point, and the kubeconfig converter. Two new cobra commands (`get-token`, `convert-kubeconfig`) and one new flag (`--absolute-path`) on `get-credentials` and `bastion`. Token minting uses the existing `pkg/azure.GetCredential()` chain — no new auth code paths.

**Tech Stack:** Go 1.25, `gopkg.in/yaml.v3` (already in go.mod), `github.com/Azure/azure-sdk-for-go/sdk/azcore`, `github.com/spf13/cobra`. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-05-14-bake-in-kubelogin-design.md`

---

## File Structure

**New files (create):**

- `internal/aks/credplugin/types.go` — ExecCredential JSON struct
- `internal/aks/credplugin/plugin.go` — `RenderExecCredential`, `DetermineAPIVersion`, `GetToken`
- `internal/aks/credplugin/plugin_test.go` — unit tests for the three functions above
- `internal/aks/credplugin/convert.go` — `Convert(yaml, opts) (out, changed, err)`
- `internal/aks/credplugin/convert_test.go` — fixture-driven unit tests
- `internal/aks/credplugin/testdata/legacy_azure_input.yaml` — fixture: legacy `auth-provider: azure`
- `internal/aks/credplugin/testdata/legacy_azure_expected.yaml` — fixture: expected output
- `internal/aks/credplugin/testdata/kubelogin_exec_input.yaml` — fixture: existing kubelogin exec entry
- `internal/aks/credplugin/testdata/kubelogin_exec_expected.yaml` — fixture: expected output
- `internal/aks/credplugin/testdata/multi_user_input.yaml` — fixture: cert + AAD users mixed
- `internal/aks/credplugin/testdata/multi_user_expected.yaml` — fixture: only AAD user rewritten
- `internal/aks/credplugin/testdata/admin_only.yaml` — fixture: cert-only kubeconfig (unchanged)
- `internal/aks/credplugin/testdata/already_converted.yaml` — fixture: already points at `az`
- `internal/aks/gettoken.go` — cobra command for `az aks get-token`
- `internal/aks/convertkubeconfig.go` — cobra command for `az aks convert-kubeconfig`

**Files to modify:**

- `internal/aks/commands.go` — register `get-token` and `convert-kubeconfig`; thread `--absolute-path` through `get-credentials` and `bastion`; update `install-cli` help text
- `internal/aks/kubeconfig.go` — `WriteKubeconfig` takes `absolutePath bool`; emits `command: az` (or absolute path) + args `[aks, get-token, --server-id, <id>]` instead of `kubelogin`
- `internal/aks/kubeconfig_test.go` — update existing tests for new signature; add test for command field
- `internal/aks/credentials.go` — `GetCredentialsOptions` gains `AbsolutePath bool`; pipe kubeconfig through `credplugin.Convert` before write/merge/stdout
- `internal/aks/bastion.go` — `BastionOptions` gains `AbsolutePath bool`; pass through to `WriteKubeconfig`; update dependency-check warning to drop kubelogin reference
- `internal/aks/install.go` — delete `installKubelogin` function and its caller; remove `"kubelogin"` from `CheckDependencies`; update success message

---

### Task 1: Hand-roll ExecCredential JSON types

**Files:**
- Create: `internal/aks/credplugin/types.go`

- [ ] **Step 1: Write the type definitions**

```go
package credplugin

import "time"

// API versions accepted in KUBERNETES_EXEC_INFO and emitted on stdout.
const (
	APIVersionV1Beta1 = "client.authentication.k8s.io/v1beta1"
	APIVersionV1      = "client.authentication.k8s.io/v1"

	// AKSServerIDDefault is the first-party AKS AAD application ID. Used as the
	// fallback --server-id when neither the legacy auth-provider config nor an
	// existing exec entry supplies one.
	AKSServerIDDefault = "6dae42f8-4368-4678-94ff-3960e28e3630"
)

// ExecCredential is the wire shape kubectl expects from an exec credential
// plugin (client-go clientauthentication v1 / v1beta1). We hand-roll it to
// avoid vendoring k8s.io/client-go for a 3-field payload.
type ExecCredential struct {
	Kind       string               `json:"kind"`
	APIVersion string               `json:"apiVersion"`
	Status     ExecCredentialStatus `json:"status"`
}

type ExecCredentialStatus struct {
	Token               string    `json:"token"`
	ExpirationTimestamp time.Time `json:"expirationTimestamp"`
}

// execInfoEnvelope is just the apiVersion field we read out of
// KUBERNETES_EXEC_INFO. Anything else in the envelope is ignored.
type execInfoEnvelope struct {
	APIVersion string `json:"apiVersion"`
}
```

- [ ] **Step 2: Build to confirm it compiles**

Run: `make build`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/aks/credplugin/types.go
git commit -m "feat(aks): scaffold credplugin package types"
```

---

### Task 2: Implement `DetermineAPIVersion`

**Files:**
- Create: `internal/aks/credplugin/plugin.go`
- Create: `internal/aks/credplugin/plugin_test.go`

- [ ] **Step 1: Write the failing test**

```go
package credplugin

import "testing"

func TestDetermineAPIVersion(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		want    string
		wantErr bool
	}{
		{name: "empty env defaults to v1beta1", env: "", want: APIVersionV1Beta1},
		{name: "explicit v1beta1", env: `{"apiVersion":"client.authentication.k8s.io/v1beta1","kind":"ExecCredential"}`, want: APIVersionV1Beta1},
		{name: "explicit v1", env: `{"apiVersion":"client.authentication.k8s.io/v1","kind":"ExecCredential"}`, want: APIVersionV1},
		{name: "envelope without apiVersion defaults to v1beta1", env: `{"kind":"ExecCredential"}`, want: APIVersionV1Beta1},
		{name: "unknown apiVersion errors", env: `{"apiVersion":"bogus/v9"}`, wantErr: true},
		{name: "malformed json errors", env: `{not json`, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetermineAPIVersion(tc.env)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test, confirm it fails to compile**

Run: `go test ./internal/aks/credplugin/...`
Expected: build error (`DetermineAPIVersion` undefined)

- [ ] **Step 3: Write the implementation**

```go
package credplugin

import (
	"encoding/json"
	"fmt"
)

// DetermineAPIVersion inspects the KUBERNETES_EXEC_INFO env value kubectl
// supplies (may be empty) and returns the apiVersion to emit. Defaults to
// v1beta1 for empty input or an envelope missing apiVersion (kubectl <1.22
// behavior). Returns an error for malformed JSON or an unrecognized version.
func DetermineAPIVersion(execInfoEnv string) (string, error) {
	if execInfoEnv == "" {
		return APIVersionV1Beta1, nil
	}
	var env execInfoEnvelope
	if err := json.Unmarshal([]byte(execInfoEnv), &env); err != nil {
		return "", fmt.Errorf("malformed KUBERNETES_EXEC_INFO: %w", err)
	}
	switch env.APIVersion {
	case "":
		return APIVersionV1Beta1, nil
	case APIVersionV1, APIVersionV1Beta1:
		return env.APIVersion, nil
	default:
		return "", fmt.Errorf("unsupported KUBERNETES_EXEC_INFO apiVersion: %s", env.APIVersion)
	}
}
```

- [ ] **Step 4: Run test, confirm pass**

Run: `go test ./internal/aks/credplugin/... -run TestDetermineAPIVersion -v`
Expected: all subtests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/aks/credplugin/plugin.go internal/aks/credplugin/plugin_test.go
git commit -m "feat(aks): add DetermineAPIVersion for credplugin"
```

---

### Task 3: Implement `RenderExecCredential`

**Files:**
- Modify: `internal/aks/credplugin/plugin.go`
- Modify: `internal/aks/credplugin/plugin_test.go`

- [ ] **Step 1: Write the failing test (append to plugin_test.go)**

```go
import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

func TestRenderExecCredential(t *testing.T) {
	expiry := time.Date(2026, 5, 14, 15, 30, 0, 0, time.UTC)
	token := azcore.AccessToken{Token: "abc.def.ghi", ExpiresOn: expiry}

	cases := []struct {
		name       string
		apiVersion string
	}{
		{"v1beta1", APIVersionV1Beta1},
		{"v1", APIVersionV1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := RenderExecCredential(token, tc.apiVersion, &buf); err != nil {
				t.Fatalf("RenderExecCredential: %v", err)
			}
			var got ExecCredential
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("output not valid JSON: %v\noutput=%s", err, buf.String())
			}
			if got.Kind != "ExecCredential" {
				t.Errorf("kind=%q, want ExecCredential", got.Kind)
			}
			if got.APIVersion != tc.apiVersion {
				t.Errorf("apiVersion=%q, want %q", got.APIVersion, tc.apiVersion)
			}
			if got.Status.Token != "abc.def.ghi" {
				t.Errorf("token=%q, want abc.def.ghi", got.Status.Token)
			}
			if !got.Status.ExpirationTimestamp.Equal(expiry) {
				t.Errorf("expirationTimestamp=%v, want %v", got.Status.ExpirationTimestamp, expiry)
			}
		})
	}
}
```

- [ ] **Step 2: Run test, confirm fail (undefined RenderExecCredential)**

Run: `go test ./internal/aks/credplugin/... -run TestRenderExecCredential`
Expected: build error

- [ ] **Step 3: Append implementation to plugin.go**

```go
import (
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// RenderExecCredential writes the ExecCredential JSON envelope kubectl expects
// to w. apiVersion is whatever DetermineAPIVersion returned.
func RenderExecCredential(token azcore.AccessToken, apiVersion string, w io.Writer) error {
	cred := ExecCredential{
		Kind:       "ExecCredential",
		APIVersion: apiVersion,
		Status: ExecCredentialStatus{
			Token:               token.Token,
			ExpirationTimestamp: token.ExpiresOn,
		},
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(&cred); err != nil {
		return fmt.Errorf("failed to encode ExecCredential: %w", err)
	}
	return nil
}
```

Note: merge the new `io` and `azcore` imports into the existing `import (` block at the top of plugin.go. The existing `encoding/json` and `fmt` imports stay.

- [ ] **Step 4: Run test, confirm pass**

Run: `go test ./internal/aks/credplugin/... -run TestRenderExecCredential -v`
Expected: both subtests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/aks/credplugin/plugin.go internal/aks/credplugin/plugin_test.go
git commit -m "feat(aks): add RenderExecCredential for kubectl exec plugin output"
```

---

### Task 4: Implement `GetToken` (composes mint + render)

**Files:**
- Modify: `internal/aks/credplugin/plugin.go`
- Modify: `internal/aks/credplugin/plugin_test.go`

- [ ] **Step 1: Write the failing test**

```go
import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// fakeCred is an azcore.TokenCredential we can rig to return any access token
// or error from GetToken — enough to exercise the GetToken composition logic.
type fakeCred struct {
	token azcore.AccessToken
	err   error
	gotScopes []string
}

func (f *fakeCred) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	f.gotScopes = opts.Scopes
	return f.token, f.err
}

func TestGetToken_HappyPath(t *testing.T) {
	expiry := time.Date(2026, 5, 14, 15, 30, 0, 0, time.UTC)
	cred := &fakeCred{token: azcore.AccessToken{Token: "tok", ExpiresOn: expiry}}

	var buf bytes.Buffer
	opts := GetTokenOptions{
		ServerID: "server-id-x",
		CredentialFactory: func() (azcore.TokenCredential, error) { return cred, nil },
		Stdout: &buf,
		ExecInfoEnv: `{"apiVersion":"client.authentication.k8s.io/v1"}`,
	}
	if err := GetToken(context.Background(), opts); err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	if len(cred.gotScopes) != 1 || cred.gotScopes[0] != "server-id-x/.default" {
		t.Errorf("scopes=%v, want [server-id-x/.default]", cred.gotScopes)
	}
	if !strings.Contains(buf.String(), `"apiVersion":"client.authentication.k8s.io/v1"`) {
		t.Errorf("output missing v1 apiVersion: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"token":"tok"`) {
		t.Errorf("output missing token: %s", buf.String())
	}
}

func TestGetToken_RequiresServerID(t *testing.T) {
	err := GetToken(context.Background(), GetTokenOptions{
		CredentialFactory: func() (azcore.TokenCredential, error) { return &fakeCred{}, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "server-id") {
		t.Fatalf("want server-id required error, got %v", err)
	}
}

func TestGetToken_CredentialFactoryError(t *testing.T) {
	err := GetToken(context.Background(), GetTokenOptions{
		ServerID: "x",
		CredentialFactory: func() (azcore.TokenCredential, error) { return nil, fmt.Errorf("boom") },
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want credential error surfaced, got %v", err)
	}
}

func TestGetToken_MintError(t *testing.T) {
	cred := &fakeCred{err: fmt.Errorf("mint failed")}
	err := GetToken(context.Background(), GetTokenOptions{
		ServerID: "x",
		CredentialFactory: func() (azcore.TokenCredential, error) { return cred, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "mint failed") {
		t.Fatalf("want mint error surfaced, got %v", err)
	}
}
```

- [ ] **Step 2: Run test, confirm fail**

Run: `go test ./internal/aks/credplugin/... -run TestGetToken`
Expected: build error (undefined `GetTokenOptions`, `GetToken`)

- [ ] **Step 3: Add the implementation to plugin.go**

```go
import (
	"context"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// GetTokenOptions configures GetToken. ServerID is required. CredentialFactory,
// Stdout, and ExecInfoEnv are optional and default to production values; tests
// inject fakes via the same fields.
type GetTokenOptions struct {
	ServerID string
	TenantID string
	ClientID string

	// CredentialFactory returns the TokenCredential used to mint the access
	// token. Defaults to azure.GetCredential().
	CredentialFactory func() (azcore.TokenCredential, error)

	// Stdout is where the ExecCredential JSON is written. Defaults to os.Stdout.
	Stdout io.Writer

	// ExecInfoEnv is the value of $KUBERNETES_EXEC_INFO. If empty, GetToken
	// reads os.Getenv("KUBERNETES_EXEC_INFO").
	ExecInfoEnv string
}

// GetToken is the kubectl exec credential plugin entrypoint. It mints an AKS
// access token at scope <server-id>/.default using the configured credential
// chain and writes an ExecCredential envelope to the writer.
func GetToken(ctx context.Context, opts GetTokenOptions) error {
	if opts.ServerID == "" {
		return fmt.Errorf("--server-id is required")
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	execInfo := opts.ExecInfoEnv
	if execInfo == "" {
		execInfo = os.Getenv("KUBERNETES_EXEC_INFO")
	}

	apiVersion, err := DetermineAPIVersion(execInfo)
	if err != nil {
		return err
	}

	credFactory := opts.CredentialFactory
	if credFactory == nil {
		return fmt.Errorf("CredentialFactory is required (production callers should pass azure.GetCredential)")
	}
	cred, err := credFactory()
	if err != nil {
		return fmt.Errorf("failed to obtain Azure credential: %w", err)
	}

	scope := opts.ServerID + "/.default"
	reqOpts := policy.TokenRequestOptions{
		Scopes:   []string{scope},
		TenantID: opts.TenantID,
	}

	token, err := cred.GetToken(ctx, reqOpts)
	if err != nil {
		return fmt.Errorf("failed to mint token: %w", err)
	}

	return RenderExecCredential(token, apiVersion, stdout)
}
```

Notes: leave the cobra wrapper (the layer that supplies the real `azure.GetCredential` factory) to Task 6. `ClientID` is accepted but unused by the AKS server-id flow at this layer — it's preserved on `GetTokenOptions` for symmetry with `--client-id` in args; the credential chain reads `AZURE_CLIENT_ID` from env when relevant.

- [ ] **Step 4: Run all credplugin tests**

Run: `go test ./internal/aks/credplugin/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/aks/credplugin/plugin.go internal/aks/credplugin/plugin_test.go
git commit -m "feat(aks): add GetToken exec-credential plugin entrypoint"
```

---

### Task 5: Implement `Convert` — legacy `auth-provider: azure` rewrite

**Files:**
- Create: `internal/aks/credplugin/convert.go`
- Create: `internal/aks/credplugin/convert_test.go`
- Create: `internal/aks/credplugin/testdata/legacy_azure_input.yaml`
- Create: `internal/aks/credplugin/testdata/legacy_azure_expected.yaml`

- [ ] **Step 1: Write the input fixture (`testdata/legacy_azure_input.yaml`)**

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io
    certificate-authority-data: REDACTED
  name: example
contexts:
- context:
    cluster: example
    user: clusterUser_rg_example
  name: example
current-context: example
users:
- name: clusterUser_rg_example
  user:
    auth-provider:
      name: azure
      config:
        apiserver-id: 6dae42f8-4368-4678-94ff-3960e28e3630
        client-id: 80faf920-1908-4b52-b5ef-a8e7bedfc67a
        tenant-id: contoso-tenant-id
        environment: AzurePublicCloud
        config-mode: "1"
```

- [ ] **Step 2: Write the expected fixture (`testdata/legacy_azure_expected.yaml`)**

```yaml
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: REDACTED
    server: https://example.hcp.eastus.azmk8s.io
  name: example
contexts:
- context:
    cluster: example
    user: clusterUser_rg_example
  name: example
current-context: example
kind: Config
users:
- name: clusterUser_rg_example
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
      - aks
      - get-token
      - --server-id
      - 6dae42f8-4368-4678-94ff-3960e28e3630
      - --tenant-id
      - contoso-tenant-id
      - --client-id
      - 80faf920-1908-4b52-b5ef-a8e7bedfc67a
      command: az
      interactiveMode: IfAvailable
      provideClusterInfo: false
```

Note: the field order in the expected output reflects how `gopkg.in/yaml.v3` marshals `map[string]interface{}` (alphabetical at each level). Match exactly.

- [ ] **Step 3: Write the failing test**

```go
package credplugin

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("loadFixture(%s): %v", name, err)
	}
	return data
}

func TestConvert_LegacyAzureAuthProvider(t *testing.T) {
	in := loadFixture(t, "legacy_azure_input.yaml")
	want := loadFixture(t, "legacy_azure_expected.yaml")

	got, changed, err := Convert(in, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !changed {
		t.Fatal("changed=false, want true")
	}
	if string(got) != string(want) {
		t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
```

- [ ] **Step 4: Run test, confirm fail**

Run: `go test ./internal/aks/credplugin/... -run TestConvert_LegacyAzureAuthProvider`
Expected: build error (Convert undefined)

- [ ] **Step 5: Implement `Convert` covering the legacy branch**

`internal/aks/credplugin/convert.go`:

```go
package credplugin

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ConvertOptions controls how Convert emits the exec entry.
type ConvertOptions struct {
	// AbsolutePath, when true, uses os.Executable() result as the exec
	// command field instead of the bare string "az".
	AbsolutePath bool
}

// Convert rewrites a kubeconfig in-memory, replacing legacy `auth-provider: azure`
// blocks and existing `kubelogin` exec entries with exec entries pointing at
// this binary. Returns the new bytes, a flag indicating whether anything
// changed, and any parse/marshal error.
func Convert(kubeConfig []byte, opts ConvertOptions) ([]byte, bool, error) {
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(kubeConfig, &cfg); err != nil {
		return nil, false, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	command, err := commandField(opts.AbsolutePath)
	if err != nil {
		return nil, false, err
	}

	users, _ := cfg["users"].([]interface{})
	changed := false
	for _, item := range users {
		userEntry, _ := item.(map[string]interface{})
		if userEntry == nil {
			continue
		}
		userMap, _ := userEntry["user"].(map[string]interface{})
		if userMap == nil {
			continue
		}
		if rewriteLegacyAuthProvider(userMap, command) {
			changed = true
		}
	}

	if !changed {
		return kubeConfig, false, nil
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}
	return out, true, nil
}

// commandField returns the string to use for the exec.command field.
func commandField(absolute bool) (string, error) {
	if !absolute {
		return "az", nil
	}
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}
	return p, nil
}

// rewriteLegacyAuthProvider replaces a `user.auth-provider: azure` block with a
// matching `user.exec` block. Returns true if the user was rewritten.
func rewriteLegacyAuthProvider(userMap map[string]interface{}, command string) bool {
	ap, _ := userMap["auth-provider"].(map[string]interface{})
	if ap == nil {
		return false
	}
	if name, _ := ap["name"].(string); name != "azure" {
		return false
	}
	cfg, _ := ap["config"].(map[string]interface{})

	serverID := AKSServerIDDefault
	if v, _ := cfg["apiserver-id"].(string); v != "" {
		serverID = v
	}
	tenantID, _ := cfg["tenant-id"].(string)
	clientID, _ := cfg["client-id"].(string)

	delete(userMap, "auth-provider")
	userMap["exec"] = buildExecEntry(command, serverID, tenantID, clientID)
	return true
}

// buildExecEntry constructs the standard exec entry pointing at this binary.
// env is left nil; the bastion temp-kubeconfig path populates env directly
// (see internal/aks/kubeconfig.go), not via Convert.
func buildExecEntry(command, serverID, tenantID, clientID string) map[string]interface{} {
	args := []interface{}{"aks", "get-token", "--server-id", serverID}
	if tenantID != "" {
		args = append(args, "--tenant-id", tenantID)
	}
	if clientID != "" {
		args = append(args, "--client-id", clientID)
	}
	return map[string]interface{}{
		"apiVersion":         APIVersionV1Beta1,
		"command":            command,
		"args":               args,
		"interactiveMode":    "IfAvailable",
		"provideClusterInfo": false,
	}
}
```

- [ ] **Step 6: Run test, confirm pass**

Run: `go test ./internal/aks/credplugin/... -run TestConvert_LegacyAzureAuthProvider -v`
Expected: PASS

If the comparison fails because of YAML key ordering, regenerate the expected fixture by running the test with a small writer-output dump and pasting the exact bytes. yaml.v3 emits map keys alphabetically.

- [ ] **Step 7: Commit**

```bash
git add internal/aks/credplugin/convert.go internal/aks/credplugin/convert_test.go internal/aks/credplugin/testdata/
git commit -m "feat(aks): convert legacy azure auth-provider kubeconfigs"
```

---

### Task 6: `Convert` — existing kubelogin exec entries

**Files:**
- Modify: `internal/aks/credplugin/convert.go`
- Modify: `internal/aks/credplugin/convert_test.go`
- Create: `internal/aks/credplugin/testdata/kubelogin_exec_input.yaml`
- Create: `internal/aks/credplugin/testdata/kubelogin_exec_expected.yaml`

- [ ] **Step 1: Write input fixture**

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io
    certificate-authority-data: REDACTED
  name: example
contexts:
- context:
    cluster: example
    user: clusterUser_rg_example
  name: example
current-context: example
users:
- name: clusterUser_rg_example
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: kubelogin
      args:
      - get-token
      - --login
      - azurecli
      - --server-id
      - 6dae42f8-4368-4678-94ff-3960e28e3630
      - --tenant-id
      - contoso-tenant-id
      - --client-id
      - 80faf920-1908-4b52-b5ef-a8e7bedfc67a
      interactiveMode: IfAvailable
      provideClusterInfo: false
```

- [ ] **Step 2: Write expected fixture**

```yaml
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: REDACTED
    server: https://example.hcp.eastus.azmk8s.io
  name: example
contexts:
- context:
    cluster: example
    user: clusterUser_rg_example
  name: example
current-context: example
kind: Config
users:
- name: clusterUser_rg_example
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
      - aks
      - get-token
      - --server-id
      - 6dae42f8-4368-4678-94ff-3960e28e3630
      - --tenant-id
      - contoso-tenant-id
      - --client-id
      - 80faf920-1908-4b52-b5ef-a8e7bedfc67a
      command: az
      interactiveMode: IfAvailable
      provideClusterInfo: false
```

- [ ] **Step 3: Append failing test**

```go
func TestConvert_ExistingKubeloginExec(t *testing.T) {
	in := loadFixture(t, "kubelogin_exec_input.yaml")
	want := loadFixture(t, "kubelogin_exec_expected.yaml")

	got, changed, err := Convert(in, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !changed {
		t.Fatal("changed=false, want true")
	}
	if string(got) != string(want) {
		t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
```

- [ ] **Step 4: Run test, confirm fail (output stays as kubelogin entry)**

Run: `go test ./internal/aks/credplugin/... -run TestConvert_ExistingKubeloginExec`
Expected: FAIL (changed=false, or output still says `command: kubelogin`)

- [ ] **Step 5: Extend `Convert` with kubelogin-exec rewrite**

In `convert.go`, add a second rewrite helper and call it from the user loop:

```go
// In Convert(), inside the users loop, after the rewriteLegacyAuthProvider call:
if rewriteKubeloginExec(userMap, command) {
	changed = true
}
```

Add the helper:

```go
// rewriteKubeloginExec replaces a `user.exec` block whose command is literally
// "kubelogin" with one pointing at this binary, carrying forward server/tenant/
// client IDs from the original args. Returns true if the user was rewritten.
func rewriteKubeloginExec(userMap map[string]interface{}, command string) bool {
	exec, _ := userMap["exec"].(map[string]interface{})
	if exec == nil {
		return false
	}
	if cmd, _ := exec["command"].(string); cmd != "kubelogin" {
		return false
	}
	serverID, tenantID, clientID := extractIDsFromArgs(exec["args"])
	if serverID == "" {
		serverID = AKSServerIDDefault
	}
	userMap["exec"] = buildExecEntry(command, serverID, tenantID, clientID)
	return true
}

// extractIDsFromArgs scans an args list (typed as []interface{} by yaml.v3) for
// --server-id / --tenant-id / --client-id and returns their values. Missing
// flags yield empty strings.
func extractIDsFromArgs(argsAny interface{}) (serverID, tenantID, clientID string) {
	args, _ := argsAny.([]interface{})
	for i := 0; i+1 < len(args); i++ {
		flag, _ := args[i].(string)
		val, _ := args[i+1].(string)
		switch flag {
		case "--server-id":
			serverID = val
		case "--tenant-id":
			tenantID = val
		case "--client-id":
			clientID = val
		}
	}
	return
}
```

- [ ] **Step 6: Run test, confirm pass; also rerun the legacy test**

Run: `go test ./internal/aks/credplugin/... -v`
Expected: both TestConvert_* PASS

- [ ] **Step 7: Commit**

```bash
git add internal/aks/credplugin/
git commit -m "feat(aks): convert existing kubelogin exec entries"
```

---

### Task 7: `Convert` — multi-user, admin-only, already-converted, malformed

**Files:**
- Modify: `internal/aks/credplugin/convert_test.go`
- Create: `internal/aks/credplugin/testdata/multi_user_input.yaml`
- Create: `internal/aks/credplugin/testdata/multi_user_expected.yaml`
- Create: `internal/aks/credplugin/testdata/admin_only.yaml`
- Create: `internal/aks/credplugin/testdata/already_converted.yaml`

- [ ] **Step 1: Write multi-user input fixture**

`testdata/multi_user_input.yaml`:

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io
  name: example
contexts:
- context:
    cluster: example
    user: clusterAdmin_rg_example
  name: example-admin
- context:
    cluster: example
    user: clusterUser_rg_example
  name: example
current-context: example
users:
- name: clusterAdmin_rg_example
  user:
    client-certificate-data: REDACTED
    client-key-data: REDACTED
- name: clusterUser_rg_example
  user:
    auth-provider:
      name: azure
      config:
        apiserver-id: 6dae42f8-4368-4678-94ff-3960e28e3630
        tenant-id: contoso-tenant-id
```

- [ ] **Step 2: Write multi-user expected fixture**

`testdata/multi_user_expected.yaml`:

```yaml
apiVersion: v1
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io
  name: example
contexts:
- context:
    cluster: example
    user: clusterAdmin_rg_example
  name: example-admin
- context:
    cluster: example
    user: clusterUser_rg_example
  name: example
current-context: example
kind: Config
users:
- name: clusterAdmin_rg_example
  user:
    client-certificate-data: REDACTED
    client-key-data: REDACTED
- name: clusterUser_rg_example
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
      - aks
      - get-token
      - --server-id
      - 6dae42f8-4368-4678-94ff-3960e28e3630
      - --tenant-id
      - contoso-tenant-id
      command: az
      interactiveMode: IfAvailable
      provideClusterInfo: false
```

- [ ] **Step 3: Write admin-only fixture**

`testdata/admin_only.yaml`:

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io
  name: example
contexts:
- context:
    cluster: example
    user: clusterAdmin_rg_example
  name: example
current-context: example
users:
- name: clusterAdmin_rg_example
  user:
    client-certificate-data: REDACTED
    client-key-data: REDACTED
```

- [ ] **Step 4: Write already-converted fixture**

`testdata/already_converted.yaml`:

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io
  name: example
contexts:
- context:
    cluster: example
    user: clusterUser_rg_example
  name: example
current-context: example
users:
- name: clusterUser_rg_example
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: az
      args:
      - aks
      - get-token
      - --server-id
      - 6dae42f8-4368-4678-94ff-3960e28e3630
      interactiveMode: IfAvailable
      provideClusterInfo: false
```

- [ ] **Step 5: Append the four new tests**

```go
func TestConvert_MultiUser_OnlyAADUserRewritten(t *testing.T) {
	in := loadFixture(t, "multi_user_input.yaml")
	want := loadFixture(t, "multi_user_expected.yaml")
	got, changed, err := Convert(in, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !changed {
		t.Fatal("changed=false, want true")
	}
	if string(got) != string(want) {
		t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestConvert_AdminOnly_Unchanged(t *testing.T) {
	in := loadFixture(t, "admin_only.yaml")
	got, changed, err := Convert(in, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if changed {
		t.Errorf("changed=true, want false")
	}
	if string(got) != string(in) {
		t.Errorf("admin-only kubeconfig should be returned byte-for-byte unchanged when changed=false")
	}
}

func TestConvert_AlreadyConverted_Unchanged(t *testing.T) {
	in := loadFixture(t, "already_converted.yaml")
	got, changed, err := Convert(in, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if changed {
		t.Errorf("changed=true, want false")
	}
	if string(got) != string(in) {
		t.Errorf("already-converted kubeconfig should be returned byte-for-byte unchanged")
	}
}

func TestConvert_MalformedYAML(t *testing.T) {
	_, _, err := Convert([]byte("not: valid: yaml: ::"), ConvertOptions{})
	if err == nil {
		t.Fatal("want parse error, got nil")
	}
}
```

- [ ] **Step 6: Run tests, confirm pass**

Run: `go test ./internal/aks/credplugin/... -v`
Expected: all PASS. The `AlreadyConverted` case works automatically because `rewriteLegacyAuthProvider` returns false (no auth-provider) and `rewriteKubeloginExec` returns false (`cmd != "kubelogin"`).

- [ ] **Step 7: Commit**

```bash
git add internal/aks/credplugin/
git commit -m "test(aks): cover multi-user, admin, idempotency, and malformed Convert cases"
```

---

### Task 8: `Convert` — `--absolute-path` option

**Files:**
- Modify: `internal/aks/credplugin/convert_test.go`

- [ ] **Step 1: Append failing test**

```go
import (
	"os"
	"strings"
)

func TestConvert_AbsolutePath(t *testing.T) {
	in := loadFixture(t, "legacy_azure_input.yaml")
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	got, changed, err := Convert(in, ConvertOptions{AbsolutePath: true})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !changed {
		t.Fatal("changed=false, want true")
	}
	if !strings.Contains(string(got), "command: "+exe) {
		t.Errorf("absolute-path output should contain %q\noutput:\n%s", "command: "+exe, got)
	}
	if strings.Contains(string(got), "command: az\n") {
		t.Errorf("absolute-path output should not contain bare `command: az`\noutput:\n%s", got)
	}
}
```

- [ ] **Step 2: Run test, confirm pass**

The `Convert` code already supports `AbsolutePath` via Task 5's `commandField`. This test should pass on first run.

Run: `go test ./internal/aks/credplugin/... -run TestConvert_AbsolutePath -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/aks/credplugin/convert_test.go
git commit -m "test(aks): verify Convert AbsolutePath option"
```

---

### Task 9: Cobra wrapper `az aks get-token`

**Files:**
- Create: `internal/aks/gettoken.go`
- Modify: `internal/aks/commands.go`

- [ ] **Step 1: Create the cobra wrapper**

`internal/aks/gettoken.go`:

```go
package aks

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/cdobbyn/azure-go-cli/internal/aks/credplugin"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/spf13/cobra"
)

func newGetTokenCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "get-token",
		Short: "kubectl exec credential plugin (mints AKS bearer tokens)",
		Long: `kubectl exec credential plugin endpoint.

Reads KUBERNETES_EXEC_INFO from the environment, mints an Azure AD access
token for the AKS server-id at the <server-id>/.default scope using this
binary's credential chain (the same chain used by every other 'az' command),
and writes a kubectl ExecCredential JSON object to stdout.

You normally invoke this through kubectl, not directly. See the exec entry
in the kubeconfig produced by 'az aks get-credentials'.`,
		// SilenceErrors so cobra doesn't print the error again with its
		// `Error: ` prefix — kubectl shows whatever lands on stderr.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			serverID, _ := cmd.Flags().GetString("server-id")
			tenantID, _ := cmd.Flags().GetString("tenant-id")
			clientID, _ := cmd.Flags().GetString("client-id")

			err := credplugin.GetToken(context.Background(), credplugin.GetTokenOptions{
				ServerID: serverID,
				TenantID: tenantID,
				ClientID: clientID,
				CredentialFactory: func() (azcore.TokenCredential, error) {
					return azure.GetCredential()
				},
				Stdout: os.Stdout,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			return err
		},
	}
	c.Flags().String("server-id", "", "AAD application ID of the AKS API server (required)")
	c.Flags().String("tenant-id", "", "AAD tenant ID")
	c.Flags().String("client-id", "", "AAD client ID")
	c.MarkFlagRequired("server-id")
	return c
}
```

- [ ] **Step 2: Register the command**

In `internal/aks/commands.go`, inside `NewAKSCommand()`, in the `cmd.AddCommand(...)` call (currently ending around line 245), add `newGetTokenCmd(),` before the package commands (e.g. right after `reconcileCmd,`):

```go
	cmd.AddCommand(
		getCredsCmd,
		bastionCmd,
		listCmd,
		showCmd,
		installCliCmd,
		deleteCmd,
		startCmd,
		stopCmd,
		abortCmd,
		reconcileCmd,
		newGetTokenCmd(),
		nodepool.NewNodePoolCommand(),
		...
	)
```

- [ ] **Step 3: Build and smoke test**

```bash
make build
./bin/az/az aks get-token --help
KUBERNETES_EXEC_INFO='{"apiVersion":"client.authentication.k8s.io/v1"}' \
  ./bin/az/az aks get-token --server-id 6dae42f8-4368-4678-94ff-3960e28e3630 | head -1
```

Expected: help output renders. Token command emits a JSON line beginning with `{"kind":"ExecCredential","apiVersion":"client.authentication.k8s.io/v1"...`. (Requires prior `az login`; if not logged in, an error to stderr is acceptable.)

- [ ] **Step 4: Commit**

```bash
git add internal/aks/gettoken.go internal/aks/commands.go
git commit -m "feat(aks): add 'az aks get-token' exec credential plugin command"
```

---

### Task 10: Cobra wrapper `az aks convert-kubeconfig`

**Files:**
- Create: `internal/aks/convertkubeconfig.go`
- Modify: `internal/aks/commands.go`

- [ ] **Step 1: Create the cobra wrapper**

`internal/aks/convertkubeconfig.go`:

```go
package aks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cdobbyn/azure-go-cli/internal/aks/credplugin"
	"github.com/spf13/cobra"
)

func newConvertKubeconfigCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "convert-kubeconfig",
		Short: "Rewrite an existing kubeconfig to use this binary instead of kubelogin",
		Long: `Rewrite an existing kubeconfig in place, replacing legacy 'auth-provider: azure'
blocks and 'kubelogin' exec entries with exec entries that call this binary's
'az aks get-token' subcommand.

Defaults to ~/.kube/config. The KUBECONFIG env var is intentionally ignored
(kubectl uses it as a merge list, which is ambiguous to rewrite); pass --file
explicitly if you have it set.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			absolute, _ := cmd.Flags().GetBool("absolute-path")

			if file == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to resolve home directory: %w", err)
				}
				file = filepath.Join(home, ".kube", "config")
			}

			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", file, err)
			}

			out, changed, err := credplugin.Convert(data, credplugin.ConvertOptions{AbsolutePath: absolute})
			if err != nil {
				return err
			}
			if !changed {
				fmt.Fprintf(os.Stderr, "No convertible entries found in %s; nothing to do.\n", file)
				return nil
			}

			if err := os.WriteFile(file, out, 0600); err != nil {
				return fmt.Errorf("failed to write %s: %w", file, err)
			}
			fmt.Fprintf(os.Stderr, "Rewrote %s\n", file)
			return nil
		},
	}
	c.Flags().StringP("file", "f", "", "Kubeconfig file to rewrite (default: ~/.kube/config)")
	c.Flags().Bool("absolute-path", false, "Use os.Executable() absolute path instead of 'az' for exec.command")
	return c
}
```

- [ ] **Step 2: Register the command**

In `internal/aks/commands.go`, add `newConvertKubeconfigCmd(),` to the `AddCommand` list next to `newGetTokenCmd()`:

```go
	cmd.AddCommand(
		...
		newGetTokenCmd(),
		newConvertKubeconfigCmd(),
		nodepool.NewNodePoolCommand(),
		...
	)
```

- [ ] **Step 3: Smoke test**

```bash
make build
./bin/az/az aks convert-kubeconfig --help
cp internal/aks/credplugin/testdata/legacy_azure_input.yaml /tmp/test-kubeconfig.yaml
./bin/az/az aks convert-kubeconfig -f /tmp/test-kubeconfig.yaml
diff -u internal/aks/credplugin/testdata/legacy_azure_input.yaml /tmp/test-kubeconfig.yaml | head -30
```

Expected: help renders; diff shows the auth-provider block replaced by an exec block pointing at `az aks get-token`.

- [ ] **Step 4: Commit**

```bash
git add internal/aks/convertkubeconfig.go internal/aks/commands.go
git commit -m "feat(aks): add 'az aks convert-kubeconfig' rewriter command"
```

---

### Task 11: `WriteKubeconfig` emits exec entry pointing at this binary

**Files:**
- Modify: `internal/aks/kubeconfig.go`
- Modify: `internal/aks/kubeconfig_test.go`

- [ ] **Step 1: Update the existing test for the new signature and new content**

Replace the body of `TestWriteKubeconfig_EffectiveNameRenamesAllPositions` and the two `AZ_SESSION` tests in `internal/aks/kubeconfig_test.go` so each call site supplies the new `absolutePath` argument:

```go
func TestKubeconfigPinsAZSession(t *testing.T) {
	t.Setenv("AZ_SESSION", "asdf")
	tmp := t.TempDir() + "/config"
	if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "name: AZ_SESSION") || !strings.Contains(got, `value: "asdf"`) {
		t.Fatalf("AZ_SESSION not pinned in kubeconfig:\n%s", got)
	}
}

func TestKubeconfigOmitsAZSessionWhenUnset(t *testing.T) {
	t.Setenv("AZ_SESSION", "")
	tmp := t.TempDir() + "/config"
	if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "AZ_SESSION") {
		t.Fatalf("AZ_SESSION should be absent when env var unset:\n%s", got)
	}
}

func TestWriteKubeconfig_EffectiveNameRenamesAllPositions(t *testing.T) {
	tmp := t.TempDir() + "/config"
	if err := WriteKubeconfig(tmp, "acme-prod-usw2-k8s-20251209", "myfqdn", 12345, false); err != nil {
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

- [ ] **Step 2: Add a new test for the exec entry shape (default + absolute)**

```go
func TestWriteKubeconfig_UsesAzGetToken(t *testing.T) {
	tmp := t.TempDir() + "/config"
	if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(tmp)
	got := string(data)

	mustContain := []string{
		"command: az",
		"- aks",
		"- get-token",
		"- --server-id",
		"- 6dae42f8-4368-4678-94ff-3960e28e3630",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("expected %q in kubeconfig:\n%s", s, got)
		}
	}
	if strings.Contains(got, "kubelogin") {
		t.Errorf("kubeconfig must not reference kubelogin:\n%s", got)
	}
}

func TestWriteKubeconfig_AbsolutePath(t *testing.T) {
	tmp := t.TempDir() + "/config"
	if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345, true); err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	data, _ := os.ReadFile(tmp)
	got := string(data)
	if !strings.Contains(got, "command: "+exe) {
		t.Errorf("expected absolute exe path in kubeconfig:\n%s", got)
	}
}
```

- [ ] **Step 3: Run tests, confirm fail**

Run: `go test ./internal/aks/... -run TestWriteKubeconfig -v`
Expected: build error (extra `false` arg, missing test functions resolve via new test file changes; compile error on signature mismatch with `WriteKubeconfig`)

- [ ] **Step 4: Update `WriteKubeconfig` signature and body**

In `internal/aks/kubeconfig.go`, replace the existing `WriteKubeconfig` (and its `CreateTempKubeconfig` caller) so it accepts `absolutePath bool` and emits the new exec entry. Full replacement:

```go
// CreateTempKubeconfig creates a temporary kubeconfig file for bastion tunnel
func CreateTempKubeconfig(ctx context.Context, clusterName, server string, port int, absolutePath bool) (string, error) {
	tmpDir, err := os.MkdirTemp("", "az-aks-bastion-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	kubeconfigDir := filepath.Join(tmpDir, ".kube")
	if err := os.MkdirAll(kubeconfigDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create .kube directory: %w", err)
	}

	kubeconfigPath := filepath.Join(kubeconfigDir, "config")
	if err := WriteKubeconfig(kubeconfigPath, clusterName, server, port, absolutePath); err != nil {
		return "", err
	}
	return kubeconfigPath, nil
}

// WriteKubeconfig writes a kubeconfig pointing at the local bastion tunnel to
// path. The parent directory must already exist. If absolutePath is true, the
// exec.command field is the absolute path returned by os.Executable() instead
// of the bare name "az".
func WriteKubeconfig(path, clusterName, server string, port int, absolutePath bool) error {
	localServer := fmt.Sprintf("https://127.0.0.1:%d", port)

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	currentPath := os.Getenv("PATH")
	customPath := fmt.Sprintf("%s:%s", exeDir, currentPath)

	command := "az"
	if absolutePath {
		command = exePath
	}

	logger.Debug("Writing kubeconfig at: %s", path)
	logger.Debug("Server URL: %s", localServer)
	logger.Debug("Using exec command: %s", command)
	logger.Debug("Prepending exe dir to PATH in exec env: %s", exeDir)

	// Pin PATH (so the right `az` is found even if Python `az` is earlier in
	// the user's shell PATH) and optionally AZ_SESSION (so subprocess token
	// minting reads the correct MSAL profile).
	envBlock := fmt.Sprintf("      - name: PATH\n        value: %s\n", customPath)
	if session := os.Getenv("AZ_SESSION"); session != "" {
		logger.Debug("Pinning AZ_SESSION=%s into kubeconfig", session)
		envBlock += fmt.Sprintf("      - name: AZ_SESSION\n        value: %q\n", session)
	}

	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
    insecure-skip-tls-verify: true
  name: %s
contexts:
- context:
    cluster: %s
    user: clusterUser_%s
  name: %s
current-context: %s
users:
- name: clusterUser_%s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: %s
      args:
      - aks
      - get-token
      - --server-id
      - %s
      env:
%s      interactiveMode: IfAvailable
      provideClusterInfo: false
`, localServer, clusterName, clusterName, clusterName, clusterName, clusterName, clusterName, command, credplugin.AKSServerIDDefault, envBlock)

	if err := os.WriteFile(path, []byte(kubeconfig), 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	logger.Debug("Kubeconfig created successfully")
	return nil
}
```

Add `"github.com/cdobbyn/azure-go-cli/internal/aks/credplugin"` to the import block at the top of `kubeconfig.go`.

- [ ] **Step 5: Run tests, confirm pass**

Run: `go test ./internal/aks/... -run TestWriteKubeconfig -v && go test ./internal/aks/... -run TestKubeconfig -v`
Expected: all PASS

- [ ] **Step 6: Build to catch other call sites**

Run: `make build`
Expected: error in `internal/aks/bastion.go` (CreateTempKubeconfig and WriteKubeconfig now need an extra argument). This is fixed in Task 13.

- [ ] **Step 7: Commit (test + signature change, even though build is broken; will fix in Task 13)**

```bash
git add internal/aks/kubeconfig.go internal/aks/kubeconfig_test.go
git commit -m "feat(aks): emit 'az aks get-token' exec entry in bastion kubeconfig"
```

Note: it is OK that the project does not build between Tasks 11 and 13 — they are committed back-to-back. If you prefer no broken intermediate states, hold off committing until Task 13.

---

### Task 12: `GetCredentials` pipes kubeconfig through `Convert`

**Files:**
- Modify: `internal/aks/credentials.go`
- Modify: `internal/aks/commands.go`

- [ ] **Step 1: Add `AbsolutePath` to `GetCredentialsOptions` and call `Convert`**

In `internal/aks/credentials.go`:

```go
import (
	// ... existing imports ...
	"github.com/cdobbyn/azure-go-cli/internal/aks/credplugin"
)

type GetCredentialsOptions struct {
	ClusterName        string
	ResourceGroup      string
	Admin              bool
	File               string
	Overwrite          bool
	Context            string
	ContextRegex       *regexp.Regexp
	ContextReplacement string
	AbsolutePath       bool
}
```

After the existing context-rename block (just before the `if opts.File == "-"` branch around line 87), insert:

```go
	// Rewrite legacy `auth-provider: azure` / kubelogin exec entries so that
	// kubectl talks to this binary instead of the external kubelogin tool.
	converted, _, err := credplugin.Convert(kubeConfig, credplugin.ConvertOptions{AbsolutePath: opts.AbsolutePath})
	if err != nil {
		return fmt.Errorf("failed to convert kubeconfig auth entries: %w", err)
	}
	kubeConfig = converted
```

- [ ] **Step 2: Expose `--absolute-path` on `get-credentials` in commands.go**

In `getCredsCmd` block:

```go
		RunE: func(cmd *cobra.Command, args []string) error {
			// ... existing GetString/GetBool calls ...
			absolutePath, _ := cmd.Flags().GetBool("absolute-path")

			// ... contextRegex/contextReplacement ...

			opts := GetCredentialsOptions{
				ClusterName:        clusterName,
				ResourceGroup:      resourceGroup,
				Admin:              admin,
				File:               file,
				Overwrite:          overwrite,
				Context:            contextName,
				ContextRegex:       contextRegex,
				ContextReplacement: contextReplacement,
				AbsolutePath:       absolutePath,
			}
			return GetCredentials(context.Background(), opts)
		},
```

And add the flag definition (after the existing `getCredsCmd.Flags()` block, before `addContextRegexFlags`):

```go
	getCredsCmd.Flags().Bool("absolute-path", false, "Embed the absolute path to this binary in the kubeconfig exec entry instead of the bare command 'az'")
```

- [ ] **Step 3: Build (still broken from Task 11) and run credentials test if any exists**

Run: `make build`
Expected: build still fails at `bastion.go` until Task 13.

Run: `go test ./internal/aks/... -run TestGetCredentials`
Expected: no test exists (skip; integration covered manually).

- [ ] **Step 4: Commit**

```bash
git add internal/aks/credentials.go internal/aks/commands.go
git commit -m "feat(aks): run get-credentials output through credplugin.Convert"
```

---

### Task 13: `Bastion` propagates `AbsolutePath`; drop kubelogin from CheckDependencies

**Files:**
- Modify: `internal/aks/bastion.go`
- Modify: `internal/aks/install.go`
- Modify: `internal/aks/commands.go`

- [ ] **Step 1: Update `BastionOptions` and call sites in bastion.go**

In `internal/aks/bastion.go`:

```go
type BastionOptions struct {
	ClusterName          string
	ResourceGroup        string
	BastionResourceID    string
	SubscriptionOverride string
	Admin                bool
	Port                 int
	Command              string
	KubeconfigPath       string
	ContextRegex         *regexp.Regexp
	ContextReplacement   string
	AbsolutePath         bool
	BufferConfig         bastion.BufferConfig
}
```

Update the two calls inside `Bastion()` (around lines 102 and 106) to pass `opts.AbsolutePath`:

```go
		if err := WriteKubeconfig(kubeconfigPath, effectiveName, clusterFQDN, port, opts.AbsolutePath); err != nil {
			return fmt.Errorf("failed to write kubeconfig: %w", err)
		}
	} else {
		kubeconfigPath, err = CreateTempKubeconfig(ctx, effectiveName, clusterFQDN, port, opts.AbsolutePath)
```

Update the dependency-check warning at the top of `Bastion()`:

```go
	// Check dependencies (kubectl only; kubelogin functionality is built in).
	missing := CheckDependencies()
	if len(missing) > 0 {
		fmt.Printf("Warning: The following required tools are not installed: %v\n", missing)
		fmt.Println("Please install kubectl using: sudo az aks install-cli")
		fmt.Println()
	}
```

- [ ] **Step 2: Update `CheckDependencies` in install.go**

In `internal/aks/install.go`, replace the `CheckDependencies` function body:

```go
// CheckDependencies checks if required CLI tools are installed
func CheckDependencies() (missing []string) {
	deps := []string{"kubectl"}
	for _, dep := range deps {
		if _, err := exec.LookPath(dep); err != nil {
			missing = append(missing, dep)
		}
	}
	return missing
}
```

- [ ] **Step 3: Expose `--absolute-path` on bastion in commands.go**

In `bastionCmd` `RunE`:

```go
			absolutePath, _ := cmd.Flags().GetBool("absolute-path")
			// ... existing code ...
			opts := BastionOptions{
				// ... existing fields ...
				AbsolutePath: absolutePath,
				BufferConfig: bufferConfig,
			}
```

And declare the flag near the other bastion flags:

```go
	bastionCmd.Flags().Bool("absolute-path", false, "Embed the absolute path to this binary in the temp kubeconfig exec entry")
```

Also update the bastion `Long` help text:

```go
		Long: `Open tunnel to AKS cluster through Azure Bastion.

Creates a temporary kubeconfig and establishes a secure tunnel to the cluster.
Dependencies: kubectl (install with: sudo az aks install-cli)`,
```

- [ ] **Step 4: Build, run all tests**

Run: `make build && go test ./...`
Expected: build succeeds; all tests PASS.

- [ ] **Step 5: Smoke test**

```bash
./bin/az/az aks get-credentials --help | grep absolute-path
./bin/az/az aks bastion --help | grep absolute-path
```

Expected: both show the new `--absolute-path` flag.

- [ ] **Step 6: Commit**

```bash
git add internal/aks/bastion.go internal/aks/install.go internal/aks/commands.go
git commit -m "feat(aks): drop kubelogin dependency from bastion and CheckDependencies"
```

---

### Task 14: Remove kubelogin install step from `az aks install-cli`

**Files:**
- Modify: `internal/aks/install.go`
- Modify: `internal/aks/commands.go`

- [ ] **Step 1: Delete `installKubelogin` and its call site**

In `internal/aks/install.go`, replace `InstallCLI`:

```go
// InstallCLI installs kubectl to /usr/local/bin
func InstallCLI(ctx context.Context) error {
	if os.Geteuid() != 0 {
		fmt.Println("This command requires sudo privileges to install to /usr/local/bin")
		fmt.Println("Please run: sudo az aks install-cli")
		return fmt.Errorf("requires sudo privileges")
	}

	fmt.Println("Installing kubectl...")

	osName := runtime.GOOS
	arch := runtime.GOARCH
	logger.Debug("OS: %s, Arch: %s", osName, arch)

	if err := installKubectl(ctx, osName, arch); err != nil {
		return fmt.Errorf("failed to install kubectl: %w", err)
	}

	fmt.Println("\nSuccessfully installed:")
	fmt.Println("  - kubectl")
	fmt.Println("\n(kubelogin is no longer needed — its functionality is built into this binary.)")
	fmt.Println("\nYou can now use 'az aks bastion' and 'kubectl' against AKS clusters.")
	return nil
}
```

Delete the `installKubelogin` function entirely (lines 92-140 of the original file).

- [ ] **Step 2: Update help text in commands.go**

In `installCliCmd`:

```go
	installCliCmd := &cobra.Command{
		Use:   "install-cli",
		Short: "Install kubectl",
		Long: `Install kubectl to /usr/local/bin.

This command requires sudo privileges to install to /usr/local/bin.
Run with: sudo az aks install-cli`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return InstallCLI(context.Background())
		},
	}
```

- [ ] **Step 3: Build, test**

Run: `make build && go test ./...`
Expected: PASS

- [ ] **Step 4: Smoke test help text**

```bash
./bin/az/az aks install-cli --help
```

Expected: short/long mention kubectl only.

- [ ] **Step 5: Commit**

```bash
git add internal/aks/install.go internal/aks/commands.go
git commit -m "feat(aks)!: drop kubelogin from install-cli (functionality is now built in)

BREAKING CHANGE: 'sudo az aks install-cli' no longer installs kubelogin.
The kubelogin functionality (exec credential plugin and kubeconfig
converter) is now built into this binary. Existing kubeconfigs that
reference kubelogin can be migrated with 'az aks convert-kubeconfig'."
```

---

### Task 15: End-to-end manual integration check (no commit)

**Files:** none modified.

- [ ] **Step 1: Verify `kubelogin` is not in PATH**

```bash
which kubelogin || echo "kubelogin not found — good"
```

If you have kubelogin installed locally, temporarily move it: `sudo mv $(which kubelogin) /tmp/kubelogin.bak` and restore after.

- [ ] **Step 2: `az aks get-credentials` against a real cluster**

```bash
./bin/az/az aks get-credentials -n <cluster> -g <rg>
grep -A2 "command:" ~/.kube/config | head -10
kubectl get nodes
```

Expected: kubeconfig shows `command: az`; `kubectl get nodes` succeeds.

- [ ] **Step 3: `az aks convert-kubeconfig` on a Python-CLI-produced kubeconfig**

```bash
# Produce one using Python az if available, or use one from a teammate
cp /path/to/python-azure-cli-kubeconfig /tmp/python-kc.yaml
./bin/az/az aks convert-kubeconfig -f /tmp/python-kc.yaml
KUBECONFIG=/tmp/python-kc.yaml kubectl get nodes
```

Expected: rewrite succeeds; `kubectl get nodes` succeeds.

- [ ] **Step 4: `az aks bastion` against a real bastion-fronted cluster**

```bash
./bin/az/az aks bastion -n <cluster> -g <rg> --bastion <bastion-resource-id> --cmd "kubectl get nodes"
```

Expected: tunnel up, nodes listed, tunnel closes cleanly.

- [ ] **Step 5: Restore kubelogin if it was moved**

```bash
sudo mv /tmp/kubelogin.bak $(which kubelogin || echo /usr/local/bin/kubelogin)
```

- [ ] **Step 6: Open the PR**

Use conventional commit prefixes already established. Final PR title:

```
feat(aks)!: bake kubelogin functionality into az binary
```

PR description references the design doc and lists the new commands and the BREAKING-CHANGE note.

---

## Self-Review Notes (run before handoff)

- [ ] Confirm `internal/aks/credplugin/types.go` exports `AKSServerIDDefault`, `APIVersionV1`, `APIVersionV1Beta1`.
- [ ] Confirm every fixture YAML in the plan exactly matches what `yaml.v3` will marshal (alphabetical map keys, two-space indent). If a fixture causes a test diff, regenerate by printing the actual output once and pasting.
- [ ] Confirm `WriteKubeconfig` signature is `(path, clusterName, server string, port int, absolutePath bool)` everywhere it's called: bastion.go (two sites), kubeconfig_test.go (four tests).
- [ ] Confirm `GetCredentialsOptions` and `BastionOptions` both have `AbsolutePath bool`.
- [ ] Confirm `commands.go` registers `newGetTokenCmd()` and `newConvertKubeconfigCmd()` in `AddCommand`.
- [ ] Confirm `CheckDependencies` returns only `kubectl`; no other place in the codebase still expects `kubelogin`.
- [ ] Confirm the breaking-change note in Task 14's commit message — uplift will pick this up and cut a major release.
