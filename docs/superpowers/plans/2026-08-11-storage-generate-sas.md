# `az storage generate-sas` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `az storage account generate-sas`, `az storage container generate-sas` and `az storage blob generate-sas`, matching the Python `azure-cli` flag-for-flag.

**Architecture:** A new `internal/storage/sas` package holds everything the three commands share: expiry parsing, permission validation, the account-key resolution chain, and a hand-rolled account-SAS signer. The container and blob commands sign through `azblob`'s `sas.BlobSignatureValues`. The account command cannot, because the SDK pins the services field to `"b"`, so it signs itself.

**Tech Stack:** Go, `cobra`, `azblob@v1.6.3`, `armstorage@v1.8.1`, stdlib `crypto/hmac` + `crypto/sha256`.

**Spec:** `docs/superpowers/specs/2026-08-11-storage-generate-sas-design.md`

## Global Constraints

- Build with `make build`. The binary lands at `bin/az/az`. Never call `go build` directly.
- New Go files use tabs (standard `gofmt`). The repo mixes tab and 2-space files; run `gofmt -w` **only on files you create or edit**, never across the tree.
- All Azure SDK calls take a `context.Context`.
- Commands output JSON by default via `pkg/output`.
- Module path is `github.com/cdobbyn/azure-go-cli`.
- Work on branch `storage-generate-sas-4c1e`. Verify with `git branch --show-current` before every commit.
- Conventional commits. `feat:` for the three commands.
- Run `make test` before each commit.

## Permission letters (canonical order, from the SDK's own `String()` methods)

| Scope | Letters |
| --- | --- |
| account | `rwdxylacupfti` |
| container | `racwdxltfmeopi` |
| blob | `racwdxyltmeopi` |

Container has no `y`. That is a known gap, tracked in `azure-go-cli-gig`.

## File structure

| File | Responsibility |
| --- | --- |
| `internal/storage/sas/sas.go` | Time parsing, permission validation, canonical ordering, IP range, Python-compatible quoting |
| `internal/storage/sas/account_sig.go` | Hand-rolled account SAS signer |
| `internal/storage/sas/credential.go` | Account name/key resolution, connection-string parsing, ARM key fetch |
| `internal/storage/account/generate_sas.go` | `az storage account generate-sas` |
| `internal/storage/container/generate_sas.go` | `az storage container generate-sas` |
| `internal/storage/blob/generate_sas.go` | `az storage blob generate-sas` |

---

### Task 1: Shared SAS helpers

**Files:**
- Create: `internal/storage/sas/sas.go`
- Test: `internal/storage/sas/sas_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func ParseTime(s string) (time.Time, error)`
  - `const AccountPerms = "rwdxylacupfti"`, `ContainerPerms = "racwdxltfmeopi"`, `BlobPerms = "racwdxyltmeopi"`
  - `func Canonical(input, order, label string) (string, error)`
  - `func ParseIPRange(s string) (azsas.IPRange, error)` where `azsas` is `azblob/sas`
  - `func Quote(s, safe string) string`
  - `func OutputFormat(cmd *cobra.Command) string`

- [ ] **Step 1: Write the failing test**

```go
package sas

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2017-12-31T01:11:59Z", "2017-12-31T01:11:59Z"},
		{"2017-12-31T01:11Z", "2017-12-31T01:11:00Z"},
		{"2017-12-31T01Z", "2017-12-31T01:00:00Z"},
		{"2017-12-31", "2017-12-31T00:00:00Z"},
	}
	for _, c := range cases {
		got, err := ParseTime(c.in)
		if err != nil {
			t.Fatalf("ParseTime(%q) returned error: %v", c.in, err)
		}
		if got.Format("2006-01-02T15:04:05Z") != c.want {
			t.Errorf("ParseTime(%q) = %s, want %s", c.in, got.Format("2006-01-02T15:04:05Z"), c.want)
		}
	}
}

func TestParseTimeRejectsGarbage(t *testing.T) {
	if _, err := ParseTime("tomorrow"); err == nil {
		t.Fatal("expected an error for unparseable input")
	}
}

func TestCanonicalReordersAndDedupes(t *testing.T) {
	got, err := Canonical("wr", AccountPerms, "permission")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "rw" {
		t.Errorf("Canonical(\"wr\") = %q, want \"rw\"", got)
	}

	got, err = Canonical("rrw", AccountPerms, "permission")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "rw" {
		t.Errorf("Canonical(\"rrw\") = %q, want \"rw\"", got)
	}
}

func TestCanonicalRejectsUnknownLetter(t *testing.T) {
	// 'y' is valid on a blob but not on a container. See azure-go-cli-gig.
	if _, err := Canonical("ry", ContainerPerms, "permission"); err == nil {
		t.Fatal("expected 'y' to be rejected for a container")
	}
	if _, err := Canonical("ry", BlobPerms, "permission"); err != nil {
		t.Fatalf("expected 'y' to be accepted for a blob, got %v", err)
	}
}

func TestParseIPRange(t *testing.T) {
	r, err := ParseIPRange("168.1.5.60-168.1.5.70")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.String() != "168.1.5.60-168.1.5.70" {
		t.Errorf("got %q", r.String())
	}

	r, err = ParseIPRange("168.1.5.60")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.String() != "168.1.5.60" {
		t.Errorf("got %q", r.String())
	}

	if _, err := ParseIPRange("not-an-ip"); err == nil {
		t.Fatal("expected an error for a non-IP value")
	}
}

func TestQuoteMatchesPythonSafeSet(t *testing.T) {
	// Python: quote(token, safe="&%()$=',~")
	got := Quote("a=b&c/d+e%2Ff", "&%()$=',~")
	if got != "a=b&c%2Fd%2Be%2Ff" {
		t.Errorf("Quote = %q, want %q", got, "a=b&c%2Fd%2Be%2Ff")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/sas/ -run 'TestParseTime|TestCanonical|TestParseIPRange|TestQuote' -v`
Expected: FAIL to build — `undefined: ParseTime`, `undefined: Canonical`, etc.

- [ ] **Step 3: Write minimal implementation**

```go
// Package sas builds Azure Storage shared access signatures.
//
// It exists because all three generate-sas commands need the same expiry
// parsing, permission validation and credential resolution, and because the
// account-scope signature cannot go through the SDK (see account_sig.go).
package sas

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// NOTE: this package is itself named sas, so the SDK's sas package MUST be
// imported under the azsas alias in every file here. Importing it unaliased
// compiles but shadows the package's own name and reads as a bug.

// Permission letters in the order the service expects them. These mirror the
// String() methods on sas.AccountPermissions, sas.ContainerPermissions and
// sas.BlobPermissions, so validation here cannot drift from signing there.
const (
	AccountPerms   = "rwdxylacupfti"
	ContainerPerms = "racwdxltfmeopi"
	BlobPerms      = "racwdxyltmeopi"
)

// timeLayouts are the four forms azure-cli accepts for --expiry and --start,
// tried in this order. See _validators.py:get_datetime_type.
var timeLayouts = []string{
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04Z",
	"2006-01-02T15Z",
	"2006-01-02",
}

// ParseTime parses a --expiry or --start value as UTC.
func ParseTime(s string) (time.Time, error) {
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("input '%s' not valid. Valid example: 2000-12-31T12:59:59Z", s)
}

// Canonical validates every character of input against order, then returns the
// characters sorted into that order with duplicates removed. The service
// requires these strings in a fixed order; label names the flag for errors.
func Canonical(input, order, label string) (string, error) {
	for _, r := range input {
		if !strings.ContainsRune(order, r) {
			return "", fmt.Errorf("invalid %s '%c': valid values are %s or a combination thereof", label, r, order)
		}
	}
	var b strings.Builder
	for _, r := range order {
		if strings.ContainsRune(input, r) {
			b.WriteRune(r)
		}
	}
	return b.String(), nil
}

// ParseIPRange parses a single IPv4 address or an "ip1-ip2" range.
func ParseIPRange(s string) (azsas.IPRange, error) {
	if s == "" {
		return azsas.IPRange{}, nil
	}
	startStr, endStr, hasEnd := strings.Cut(s, "-")
	start := net.ParseIP(strings.TrimSpace(startStr))
	if start == nil {
		return azsas.IPRange{}, fmt.Errorf("invalid --ip value %q: expected an IPv4 address or a range like 168.1.5.60-168.1.5.70", s)
	}
	r := azsas.IPRange{Start: start}
	if hasEnd {
		end := net.ParseIP(strings.TrimSpace(endStr))
		if end == nil {
			return azsas.IPRange{}, fmt.Errorf("invalid --ip value %q: expected an IPv4 address or a range like 168.1.5.60-168.1.5.70", s)
		}
		r.End = end
	}
	return r, nil
}

// alwaysSafe are the characters Python's urllib.parse.quote never escapes.
const alwaysSafe = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_.-~"

// Quote reproduces Python's urllib.parse.quote(s, safe=safe), which is what
// azure-cli applies to a blob SAS token (operations/blob.py:906). Go's
// url.QueryEscape uses a different safe set and cannot be substituted.
func Quote(s, safe string) string {
	var b bytes.Buffer
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(alwaysSafe, c) >= 0 || strings.IndexByte(safe, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// OutputFormat reads the global --output flag for a SAS token result.
//
// A SAS token is a bare string, and output.PrintFormatted only renders json
// and tsv, so "table" is mapped to "json" the way internal/role/list.go does.
func OutputFormat(cmd *cobra.Command) string {
	format, _ := cmd.Flags().GetString("output")
	if format == "table" || format == "" {
		return "json"
	}
	return format
}
```

Add `"github.com/spf13/cobra"` to the imports for `OutputFormat`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/sas/ -v`
Expected: PASS, all tests.

If `TestParseTime` fails on the `2017-12-31T01Z` case, that means Go read the trailing `Z` as a zone indicator rather than a literal. Confirm by printing the parsed value. The fix is to keep the layout as written — a lone `Z` not followed by `07` is a literal in Go — so a failure here means the input string, not the layout, is wrong.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/sas/sas.go internal/storage/sas/sas_test.go
git commit -m "feat(storage): add shared SAS helpers for generate-sas"
```

---

### Task 2: Hand-rolled account SAS signer

The SDK cannot sign an account SAS for anything but blob: `sas/account.go:71` writes a literal `"b"` into the string-to-sign and `sas/account.go:96` sets `services: "b", // will always be "b"`. `--services bqtf` therefore needs its own signer.

**Files:**
- Create: `internal/storage/sas/account_sig.go`
- Test: `internal/storage/sas/account_sig_test.go`

**Interfaces:**
- Consumes: `Canonical`, `ParseIPRange` from Task 1.
- Produces:
  - `type AccountOptions struct { AccountName, Permissions, Services, ResourceTypes, IPRange, Protocol, EncryptionScope string; Start, Expiry time.Time }`
  - `func SignAccount(o AccountOptions, accountKey string) (string, error)`

- [ ] **Step 1: Write the failing test**

The strongest available check is a differential test. For `--services b`, our signer must produce a byte-identical token to the SDK's own, because that is the one case both can express. If they agree there, the only difference for `bqtf` is the `ss` value fed into the same code path.

```go
package sas

import (
	"net/url"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// A syntactically valid but fake key. base64 of 32 bytes.
const testKey = "bXlzdXBlcnNlY3JldHRlc3RrZXkxMjM0NTY3ODkwYWI="

func TestSignAccountMatchesSDKForBlobOnly(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	cred, err := azblob.NewSharedKeyCredential("myaccount", testKey)
	if err != nil {
		t.Fatalf("NewSharedKeyCredential: %v", err)
	}
	sdkParams, err := azsas.AccountSignatureValues{
		Protocol:      azsas.ProtocolHTTPS,
		StartTime:     start,
		ExpiryTime:    expiry,
		Permissions:   "rl",
		ResourceTypes: "sco",
	}.SignWithSharedKey(cred)
	if err != nil {
		t.Fatalf("SDK SignWithSharedKey: %v", err)
	}

	ours, err := SignAccount(AccountOptions{
		AccountName:   "myaccount",
		Permissions:   "rl",
		Services:      "b",
		ResourceTypes: "sco",
		Start:         start,
		Expiry:        expiry,
		Protocol:      "https",
	}, testKey)
	if err != nil {
		t.Fatalf("SignAccount: %v", err)
	}

	sdkSig := mustQuery(t, sdkParams.Encode()).Get("sig")
	ourSig := mustQuery(t, ours).Get("sig")
	if sdkSig != ourSig {
		t.Errorf("signature mismatch\n SDK: %s\nours: %s", sdkSig, ourSig)
	}
}

func TestSignAccountCarriesAllServices(t *testing.T) {
	ours, err := SignAccount(AccountOptions{
		AccountName:   "myaccount",
		Permissions:   "rl",
		Services:      "tqbf", // deliberately out of order
		ResourceTypes: "oc",   // deliberately out of order
		Expiry:        time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}, testKey)
	if err != nil {
		t.Fatalf("SignAccount: %v", err)
	}
	q := mustQuery(t, ours)
	if q.Get("ss") != "bqtf" {
		t.Errorf("ss = %q, want \"bqtf\" (this is the regression this signer exists to prevent)", q.Get("ss"))
	}
	if q.Get("srt") != "co" {
		t.Errorf("srt = %q, want \"co\"", q.Get("srt"))
	}
	if q.Get("sig") == "" {
		t.Error("sig is empty")
	}
}

func TestSignAccountRejectsBadServiceLetter(t *testing.T) {
	_, err := SignAccount(AccountOptions{
		AccountName:   "myaccount",
		Permissions:   "r",
		Services:      "bz",
		ResourceTypes: "o",
		Expiry:        time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}, testKey)
	if err == nil {
		t.Fatal("expected 'z' to be rejected as a service")
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	v, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", raw, err)
	}
	return v
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/sas/ -run TestSignAccount -v`
Expected: FAIL to build — `undefined: SignAccount`, `undefined: AccountOptions`.

- [ ] **Step 3: Write minimal implementation**

The field order below is copied from `sas/account.go:60-78` in `azblob@v1.6.3`. The trailing empty element is deliberate: the account SAS string-to-sign ends with a newline.

```go
package sas

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// Service and resource-type letters, in the order the service expects.
const (
	AccountServices      = "bqtf"
	AccountResourceTypes = "sco"
)

// AccountOptions are the inputs to an account-scope SAS.
type AccountOptions struct {
	AccountName     string
	Permissions     string
	Services        string
	ResourceTypes   string
	IPRange         string
	Protocol        string
	EncryptionScope string
	Start           time.Time
	Expiry          time.Time
}

// SignAccount builds an account SAS token and returns it as a query string.
//
// This does not use sas.AccountSignatureValues because that type has no
// Services field and hardcodes "b" into the signature, which would silently
// reduce --services bqtf to blob-only.
func SignAccount(o AccountOptions, accountKey string) (string, error) {
	perms, err := Canonical(o.Permissions, AccountPerms, "permission")
	if err != nil {
		return "", err
	}
	services, err := Canonical(o.Services, AccountServices, "service")
	if err != nil {
		return "", err
	}
	resourceTypes, err := Canonical(o.ResourceTypes, AccountResourceTypes, "resource type")
	if err != nil {
		return "", err
	}
	ipRange, err := ParseIPRange(o.IPRange)
	if err != nil {
		return "", err
	}

	start := ""
	if !o.Start.IsZero() {
		start = o.Start.UTC().Format(azsas.TimeFormat)
	}
	expiry := ""
	if !o.Expiry.IsZero() {
		expiry = o.Expiry.UTC().Format(azsas.TimeFormat)
	}

	stringToSign := strings.Join([]string{
		o.AccountName,
		perms,
		services,
		resourceTypes,
		start,
		expiry,
		ipRange.String(),
		o.Protocol,
		azsas.Version,
		o.EncryptionScope,
		"", // the account SAS requires a terminating newline
	}, "\n")

	key, err := base64.StdEncoding.DecodeString(accountKey)
	if err != nil {
		return "", fmt.Errorf("the account key is not valid base64: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Emitted in azure-cli's field order. Order is cosmetic; the service
	// recomputes the signature from the values it receives.
	pairs := []struct{ k, v string }{
		{"sv", azsas.Version},
		{"ss", services},
		{"srt", resourceTypes},
		{"sp", perms},
		{"se", expiry},
		{"st", start},
		{"sip", ipRange.String()},
		{"spr", o.Protocol},
		{"ses", o.EncryptionScope},
		{"sig", signature},
	}
	var b strings.Builder
	for _, p := range pairs {
		if p.v == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.k)
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(p.v))
	}
	return b.String(), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/sas/ -v`
Expected: PASS.

If `TestSignAccountMatchesSDKForBlobOnly` fails, the string-to-sign differs from the SDK's. Re-read `sas/account.go:60-78` in the module cache and compare element by element. Do **not** adjust the test to match the implementation — the SDK is the reference here.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/sas/account_sig.go internal/storage/sas/account_sig_test.go
git commit -m "feat(storage): add account SAS signer with full --services support"
```

---

### Task 3: Credential resolution

**Files:**
- Create: `internal/storage/sas/credential.go`
- Test: `internal/storage/sas/credential_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `type Creds struct { AccountName, AccountKey string }`
  - `func ParseConnectionString(cs string) map[string]string`
  - `func ResolveInputs(accountName, accountKey, connectionString string) Creds`
  - `func FetchAccountKey(ctx context.Context, accountName string) (string, error)`
  - `func Resolve(ctx context.Context, accountName, accountKey, connectionString string) (Creds, error)`

- [ ] **Step 1: Write the failing test**

```go
package sas

import "testing"

func TestParseConnectionStringKeepsBase64Padding(t *testing.T) {
	cs := "DefaultEndpointsProtocol=https;AccountName=acct;AccountKey=YWJjZA==;EndpointSuffix=core.windows.net"
	got := ParseConnectionString(cs)
	if got["AccountName"] != "acct" {
		t.Errorf("AccountName = %q", got["AccountName"])
	}
	// The key contains '=' padding. Splitting on every '=' would truncate it.
	if got["AccountKey"] != "YWJjZA==" {
		t.Errorf("AccountKey = %q, want \"YWJjZA==\"", got["AccountKey"])
	}
}

func TestResolveInputsPrefersConnectionString(t *testing.T) {
	got := ResolveInputs("flagname", "flagkey",
		"AccountName=csname;AccountKey=cskey")
	if got.AccountName != "csname" || got.AccountKey != "cskey" {
		t.Errorf("connection string should win, got %+v", got)
	}
}

func TestResolveInputsFallsBackToEnv(t *testing.T) {
	t.Setenv("AZURE_STORAGE_ACCOUNT", "envname")
	t.Setenv("AZURE_STORAGE_KEY", "envkey")
	got := ResolveInputs("", "", "")
	if got.AccountName != "envname" || got.AccountKey != "envkey" {
		t.Errorf("expected env fallback, got %+v", got)
	}
}

func TestResolveInputsFlagsBeatEnv(t *testing.T) {
	t.Setenv("AZURE_STORAGE_ACCOUNT", "envname")
	t.Setenv("AZURE_STORAGE_KEY", "envkey")
	got := ResolveInputs("flagname", "flagkey", "")
	if got.AccountName != "flagname" || got.AccountKey != "flagkey" {
		t.Errorf("expected flags to win, got %+v", got)
	}
}

func TestResolveInputsReadsConnectionStringFromEnv(t *testing.T) {
	t.Setenv("AZURE_STORAGE_CONNECTION_STRING", "AccountName=csname;AccountKey=cskey")
	got := ResolveInputs("", "", "")
	if got.AccountName != "csname" || got.AccountKey != "cskey" {
		t.Errorf("expected env connection string, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/sas/ -run 'TestParseConnectionString|TestResolveInputs' -v`
Expected: FAIL to build — `undefined: ParseConnectionString`, `undefined: ResolveInputs`.

- [ ] **Step 3: Write minimal implementation**

```go
package sas

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// MissingCredentialsError reproduces the message azure-cli prints at
// operations/account.py:39. It is the most common failure for this command,
// so it names every accepted way to supply credentials.
const MissingCredentialsError = `missing or invalid credentials to access the storage service. The following variations are accepted:
    (1) account name and key (--account-name and --account-key options, or
        set AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_KEY environment variables)
    (2) account name (--account-name option or AZURE_STORAGE_ACCOUNT environment variable;
        this will query for a storage account key using your login credentials)
    (3) connection string (--connection-string option or
        set AZURE_STORAGE_CONNECTION_STRING environment variable)`

// Creds is a resolved storage account name and shared key.
type Creds struct {
	AccountName string
	AccountKey  string
}

// ParseConnectionString splits a storage connection string into its parts.
// It cuts on the first '=' only, so a base64 AccountKey keeps its padding.
func ParseConnectionString(cs string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(cs, ";") {
		k, v, found := strings.Cut(part, "=")
		if found {
			out[strings.TrimSpace(k)] = v
		}
	}
	return out
}

// ResolveInputs applies the flag-then-environment precedence chain from
// _validators.py:validate_client_parameters. A connection string, from either
// source, overrides the separately supplied name and key, as it does there.
func ResolveInputs(accountName, accountKey, connectionString string) Creds {
	if connectionString == "" {
		connectionString = os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	}
	if connectionString != "" {
		parts := ParseConnectionString(connectionString)
		return Creds{AccountName: parts["AccountName"], AccountKey: parts["AccountKey"]}
	}
	if accountName == "" {
		accountName = os.Getenv("AZURE_STORAGE_ACCOUNT")
	}
	if accountKey == "" {
		accountKey = os.Getenv("AZURE_STORAGE_KEY")
	}
	return Creds{AccountName: accountName, AccountKey: accountKey}
}

// FetchAccountKey looks up a storage account's first key over ARM. The
// resource group is discovered by listing the subscription's storage accounts
// and matching on name, so the user does not have to pass -g.
func FetchAccountKey(ctx context.Context, accountName string) (string, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return "", err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return "", err
	}
	client, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create storage accounts client: %w", err)
	}

	resourceGroup := ""
	pager := client.NewListPager(nil)
	for pager.More() && resourceGroup == "" {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list storage accounts: %w", err)
		}
		for _, acct := range page.Value {
			if acct.Name != nil && *acct.Name == accountName && acct.ID != nil {
				resourceGroup = resourceGroupFromID(*acct.ID)
				break
			}
		}
	}
	if resourceGroup == "" {
		return "", fmt.Errorf("storage account %q was not found in the current subscription", accountName)
	}

	resp, err := client.ListKeys(ctx, resourceGroup, accountName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list keys for storage account %q: %w", accountName, err)
	}
	for _, k := range resp.Keys {
		if k.Value != nil && *k.Value != "" {
			return *k.Value, nil
		}
	}
	return "", fmt.Errorf("storage account %q returned no keys", accountName)
}

// resourceGroupFromID pulls the resource group out of an ARM resource ID.
func resourceGroupFromID(id string) string {
	parts := strings.Split(id, "/")
	for i, p := range parts {
		if strings.EqualFold(p, "resourceGroups") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// Resolve runs the full chain, falling back to an ARM key lookup when a name
// is known but no key was supplied. azure-cli warns and continues on lookup
// failure rather than aborting, and so does this.
func Resolve(ctx context.Context, accountName, accountKey, connectionString string) (Creds, error) {
	creds := ResolveInputs(accountName, accountKey, connectionString)
	if creds.AccountName == "" {
		return creds, fmt.Errorf("%s", MissingCredentialsError)
	}
	if creds.AccountKey != "" {
		return creds, nil
	}

	fmt.Fprintln(os.Stderr, "There are no credentials provided in your command and environment, we will query for account key for your storage account.")
	fmt.Fprintln(os.Stderr, "It is recommended to provide --connection-string or --account-key in your command as credentials.")

	key, err := FetchAccountKey(ctx, creds.AccountName)
	if err != nil {
		return creds, fmt.Errorf("%s\n\nquerying the account key failed: %v", MissingCredentialsError, err)
	}
	creds.AccountKey = key
	return creds, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/sas/ -v`
Expected: PASS. Then `make build` to confirm the ARM imports resolve.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/sas/credential.go internal/storage/sas/credential_test.go
git commit -m "feat(storage): add account key resolution chain for generate-sas"
```

---

### Task 4: `az storage account generate-sas`

**Files:**
- Create: `internal/storage/account/generate_sas.go`
- Modify: `internal/storage/account/commands.go` — add the subcommand to the `cmd.AddCommand(...)` call at the end

**Interfaces:**
- Consumes: `sas.SignAccount`, `sas.AccountOptions`, `sas.Resolve`, `sas.ParseTime` from Tasks 1-3.
- Produces: `func NewGenerateSASCommand() *cobra.Command`

This command has **no** `--auth-mode`, `--as-user` or `--full-uri`. Python registers it with `storage_custom_command`, not the `_oauth` variant (`commands.py:128`), so those flags do not exist there.

- [ ] **Step 1: Write the implementation**

```go
package account

import (
	"context"
	"fmt"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/storage/sas"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewGenerateSASCommand builds `az storage account generate-sas`.
func NewGenerateSASCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-sas",
		Short: "Generate a shared access signature for the storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateSAS(context.Background(), cmd)
		},
	}

	cmd.Flags().String("services", "", "The storage services the SAS is applicable for. Allowed values: (b)lob (f)ile (q)ueue (t)able. Can be combined")
	cmd.Flags().String("resource-types", "", "The resource types the SAS is applicable for. Allowed values: (s)ervice (c)ontainer (o)bject. Can be combined")
	cmd.Flags().String("permissions", "", "The permissions the SAS grants. Allowed values: "+sas.AccountPerms+". Can be combined")
	cmd.Flags().String("expiry", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes invalid")
	cmd.Flags().String("start", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes valid. Defaults to the time of the request")
	cmd.Flags().String("ip", "", "IP address or range of IP addresses from which to accept requests. IPv4 only")
	cmd.Flags().Bool("https-only", false, "Only permit requests made with the HTTPS protocol")
	cmd.Flags().String("account-name", "", "Storage account name. Environment variable: AZURE_STORAGE_ACCOUNT")
	cmd.Flags().String("account-key", "", "Storage account key. Environment variable: AZURE_STORAGE_KEY")
	cmd.Flags().String("connection-string", "", "Storage account connection string. Environment variable: AZURE_STORAGE_CONNECTION_STRING")
	cmd.Flags().String("encryption-scope", "", "A predefined encryption scope used to encrypt the data on the service")

	cmd.MarkFlagRequired("services")
	cmd.MarkFlagRequired("resource-types")
	cmd.MarkFlagRequired("permissions")
	cmd.MarkFlagRequired("expiry")

	return cmd
}

func runGenerateSAS(ctx context.Context, cmd *cobra.Command) error {
	services, _ := cmd.Flags().GetString("services")
	resourceTypes, _ := cmd.Flags().GetString("resource-types")
	permissions, _ := cmd.Flags().GetString("permissions")
	expiryStr, _ := cmd.Flags().GetString("expiry")
	startStr, _ := cmd.Flags().GetString("start")
	ip, _ := cmd.Flags().GetString("ip")
	httpsOnly, _ := cmd.Flags().GetBool("https-only")
	accountName, _ := cmd.Flags().GetString("account-name")
	accountKey, _ := cmd.Flags().GetString("account-key")
	connectionString, _ := cmd.Flags().GetString("connection-string")
	encryptionScope, _ := cmd.Flags().GetString("encryption-scope")

	expiry, err := sas.ParseTime(expiryStr)
	if err != nil {
		return fmt.Errorf("--expiry: %w", err)
	}
	var start time.Time
	if startStr != "" {
		start, err = sas.ParseTime(startStr)
		if err != nil {
			return fmt.Errorf("--start: %w", err)
		}
	}

	creds, err := sas.Resolve(ctx, accountName, accountKey, connectionString)
	if err != nil {
		return err
	}

	protocol := ""
	if httpsOnly {
		protocol = "https"
	}

	token, err := sas.SignAccount(sas.AccountOptions{
		AccountName:     creds.AccountName,
		Permissions:     permissions,
		Services:        services,
		ResourceTypes:   resourceTypes,
		IPRange:         ip,
		Protocol:        protocol,
		EncryptionScope: encryptionScope,
		Start:           start,
		Expiry:          expiry,
	}, creds.AccountKey)
	if err != nil {
		return err
	}

	return output.PrintFormatted(cmd, token, sas.OutputFormat(cmd))
}
```

- [ ] **Step 2: Register the command**

In `internal/storage/account/commands.go`, change the final `cmd.AddCommand(...)` line to include the new command:

```go
	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, NewGenerateSASCommand())
```

- [ ] **Step 3: Build and check the help output**

```bash
make build
./bin/az/az storage account generate-sas --help
```

Expected: every flag above appears; `--services`, `--resource-types`, `--permissions` and `--expiry` are marked required.

- [ ] **Step 4: Verify the token shape without an Azure account**

```bash
AZURE_STORAGE_ACCOUNT=myaccount \
AZURE_STORAGE_KEY=bXlzdXBlcnNlY3JldHRlc3RrZXkxMjM0NTY3ODkwYWI= \
./bin/az/az storage account generate-sas \
  --services bqtf --resource-types sco --permissions rl \
  --expiry 2030-01-01 --https-only -o tsv
```

Expected: a bare token on stdout containing `ss=bqtf`, `srt=sco`, `sp=rl`, `spr=https` and `sig=`. No network call happens, because the key is supplied.

Then confirm the default output is a quoted JSON string:

```bash
AZURE_STORAGE_ACCOUNT=myaccount \
AZURE_STORAGE_KEY=bXlzdXBlcnNlY3JldHRlc3RrZXkxMjM0NTY3ODkwYWI= \
./bin/az/az storage account generate-sas \
  --services b --resource-types o --permissions r --expiry 2030-01-01
```

Expected: the same token wrapped in double quotes.

- [ ] **Step 5: Commit**

```bash
make test
git add internal/storage/account/
git commit -m "feat(storage): add az storage account generate-sas"
```

---

### Task 5: `az storage container generate-sas`

**Files:**
- Create: `internal/storage/sas/blob_sig.go` — the signer shared with Task 6
- Create: `internal/storage/container/generate_sas.go`
- Modify: `internal/storage/sas/sas.go` — add `NewSharedKey`
- Modify: `internal/storage/sas/sas_test.go` — add `TestValidateAsUser`
- Modify: `internal/storage/container/commands.go` — add to the final `cmd.AddCommand(...)`

**Interfaces:**
- Consumes: `sas.ParseTime`, `sas.Canonical`, `sas.ParseIPRange`, `sas.Resolve`, `sas.ResolveInputs`, `sas.ContainerPerms`, `sas.OutputFormat`.
- Produces, all in package `sas` and all reused verbatim by Task 6:
  - `type BlobScopeOptions struct` — fields listed in Step 1
  - `func SignBlobScope(ctx context.Context, o BlobScopeOptions, accountKey string, asUser bool) (string, error)`
  - `func ValidateAsUser(asUser bool, authMode, expiry string, expiryTime, now time.Time) error`
  - `func NewSharedKey(accountName, accountKey string) (*azsas.SharedKeyCredential, error)`
  - `const UserDelegationExpiryLimit = 7 * 24 * time.Hour`
  - and in package `container`: `func NewGenerateSASCommand() *cobra.Command`

`SignBlobScope` picks the permission set from whether `BlobName` is empty, so Task 6 needs no separate signer.

Here `--name`/`-n` is the **container** name (`_params.py:1570`).

- [ ] **Step 1: Add the shared blob-scope signer to the sas package**

Create `internal/storage/sas/blob_sig.go`:

```go
package sas

import (
	"context"
	"fmt"
	"time"

	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

// BlobScopeOptions are the inputs shared by container-scope and blob-scope SAS.
// Leave BlobName empty for a container SAS.
type BlobScopeOptions struct {
	AccountName        string
	ContainerName      string
	BlobName           string
	Permissions        string
	Identifier         string
	IPRange            string
	Protocol           string
	EncryptionScope    string
	CacheControl       string
	ContentDisposition string
	ContentEncoding    string
	ContentLanguage    string
	ContentType        string
	Snapshot           string
	AuthorizedObjectID string
	Start              time.Time
	Expiry             time.Time
}

// UserDelegationExpiryLimit is the service cap on a user delegation key.
const UserDelegationExpiryLimit = 7 * 24 * time.Hour

// SnapshotTimeFormat mirrors the unexported exported.SnapshotTimeFormat in the
// SDK, which is not importable from outside the module.
const SnapshotTimeFormat = "2006-01-02T15:04:05.0000000Z07:00"

// ValidateAsUser applies the --as-user rules from _validators.py:1541.
func ValidateAsUser(asUser bool, authMode, expiry string, expiryTime time.Time, now time.Time) error {
	if authMode == "login" && !asUser {
		return fmt.Errorf("incorrect usage: specify --as-user when --auth-mode login is used to get user delegation key")
	}
	if !asUser {
		return nil
	}
	if expiry == "" {
		return fmt.Errorf("incorrect usage: specify --expiry when --as-user is enabled")
	}
	if authMode != "login" {
		return fmt.Errorf("incorrect usage: specify '--auth-mode login' when --as-user is enabled")
	}
	if expiryTime.After(now.Add(UserDelegationExpiryLimit)) {
		return fmt.Errorf("incorrect usage: --expiry should be within 7 days from now")
	}
	return nil
}

// SignBlobScope signs a container-scope or blob-scope SAS. When asUser is true
// it fetches a user delegation key over AAD; otherwise it signs with accountKey.
func SignBlobScope(ctx context.Context, o BlobScopeOptions, accountKey string, asUser bool) (string, error) {
	allowed := ContainerPerms
	label := "container permission"
	if o.BlobName != "" {
		allowed = BlobPerms
		label = "blob permission"
	}
	perms, err := Canonical(o.Permissions, allowed, label)
	if err != nil {
		return "", err
	}
	ipRange, err := ParseIPRange(o.IPRange)
	if err != nil {
		return "", err
	}

	// A snapshot SAS signs resource "bs" rather than "b". The SDK switches on
	// SnapshotTime being non-zero, so the opaque --snapshot value is parsed here.
	var snapshotTime time.Time
	if o.Snapshot != "" {
		snapshotTime, err = time.Parse(SnapshotTimeFormat, o.Snapshot)
		if err != nil {
			return "", fmt.Errorf("invalid --snapshot value %q: %w", o.Snapshot, err)
		}
	}

	values := azsas.BlobSignatureValues{
		SnapshotTime:       snapshotTime,
		Protocol:           azsas.Protocol(o.Protocol),
		StartTime:          o.Start,
		ExpiryTime:         o.Expiry,
		Permissions:        perms,
		IPRange:            ipRange,
		Identifier:         o.Identifier,
		ContainerName:      o.ContainerName,
		BlobName:           o.BlobName,
		CacheControl:       o.CacheControl,
		ContentDisposition: o.ContentDisposition,
		ContentEncoding:    o.ContentEncoding,
		ContentLanguage:    o.ContentLanguage,
		ContentType:        o.ContentType,
		AuthorizedObjectID: o.AuthorizedObjectID,
		EncryptionScope:    o.EncryptionScope,
	}

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", o.AccountName)

	if asUser {
		cred, err := azure.GetCredential()
		if err != nil {
			return "", err
		}
		client, err := service.NewClient(serviceURL, cred, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create blob service client: %w", err)
		}
		start := o.Start
		if start.IsZero() {
			start = time.Now().UTC()
		}
		startStr := start.UTC().Format(azsas.TimeFormat)
		expiryStr := o.Expiry.UTC().Format(azsas.TimeFormat)
		udc, err := client.GetUserDelegationCredential(ctx, service.KeyInfo{
			Start:  &startStr,
			Expiry: &expiryStr,
		}, nil)
		if err != nil {
			return "", fmt.Errorf("failed to get a user delegation key: %w", err)
		}
		params, err := values.SignWithUserDelegation(udc)
		if err != nil {
			return "", err
		}
		return params.Encode(), nil
	}

	sharedKey, err := NewSharedKey(o.AccountName, accountKey)
	if err != nil {
		return "", err
	}
	params, err := values.SignWithSharedKey(sharedKey)
	if err != nil {
		return "", err
	}
	return params.Encode(), nil
}
```

Add to `internal/storage/sas/sas.go`:

```go
// NewSharedKey wraps the SDK constructor with a clearer error.
func NewSharedKey(accountName, accountKey string) (*azsas.SharedKeyCredential, error) {
	cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("invalid storage account key: %w", err)
	}
	return cred, nil
}
```

with `"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"` added to its imports.

- [ ] **Step 2: Write the failing test for the --as-user rules**

Add to `internal/storage/sas/sas_test.go`:

```go
func TestValidateAsUser(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	within := now.Add(24 * time.Hour)
	beyond := now.Add(8 * 24 * time.Hour)

	if err := ValidateAsUser(false, "login", "", time.Time{}, now); err == nil {
		t.Error("--auth-mode login without --as-user must fail")
	}
	if err := ValidateAsUser(true, "login", "", time.Time{}, now); err == nil {
		t.Error("--as-user without --expiry must fail")
	}
	if err := ValidateAsUser(true, "key", "2026-01-02", within, now); err == nil {
		t.Error("--as-user without --auth-mode login must fail")
	}
	if err := ValidateAsUser(true, "login", "2026-01-09", beyond, now); err == nil {
		t.Error("--as-user beyond 7 days must fail")
	}
	if err := ValidateAsUser(true, "login", "2026-01-02", within, now); err != nil {
		t.Errorf("valid --as-user usage must pass, got %v", err)
	}
	if err := ValidateAsUser(false, "key", "", time.Time{}, now); err != nil {
		t.Errorf("plain key usage must pass, got %v", err)
	}
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/storage/sas/ -run TestValidateAsUser -v`
Expected: FAIL first (`undefined: ValidateAsUser`) until Step 1's file is in place, then PASS.

- [ ] **Step 4: Write the command**

```go
package container

import (
	"context"
	"fmt"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/storage/sas"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewGenerateSASCommand builds `az storage container generate-sas`.
func NewGenerateSASCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-sas",
		Short: "Generate a SAS token for a storage container",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateSAS(context.Background(), cmd)
		},
	}

	cmd.Flags().StringP("name", "n", "", "The container name")
	cmd.Flags().String("permissions", "", "The permissions the SAS grants. Allowed values: "+sas.ContainerPerms+". Can be combined")
	cmd.Flags().String("expiry", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes invalid")
	cmd.Flags().String("start", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes valid. Defaults to the time of the request")
	cmd.Flags().String("ip", "", "IP address or range of IP addresses from which to accept requests. IPv4 only")
	cmd.Flags().Bool("https-only", false, "Only permit requests made with the HTTPS protocol")
	cmd.Flags().String("policy-name", "", "The name of a stored access policy within the container's ACL")
	cmd.Flags().Bool("as-user", false, "Return the SAS signed with the user delegation key. Requires --expiry and --auth-mode login")
	cmd.Flags().String("auth-mode", "key", "The mode in which to run the command. Allowed values: key, login")
	cmd.Flags().String("user-delegation-oid", "", "Entra ID of the user authorized to use the resulting SAS URL. Requires --as-user")
	cmd.Flags().String("cache-control", "", "Response header value for Cache-Control when the resource is accessed using this SAS")
	cmd.Flags().String("content-disposition", "", "Response header value for Content-Disposition when the resource is accessed using this SAS")
	cmd.Flags().String("content-encoding", "", "Response header value for Content-Encoding when the resource is accessed using this SAS")
	cmd.Flags().String("content-language", "", "Response header value for Content-Language when the resource is accessed using this SAS")
	cmd.Flags().String("content-type", "", "Response header value for Content-Type when the resource is accessed using this SAS")
	cmd.Flags().String("account-name", "", "Storage account name. Environment variable: AZURE_STORAGE_ACCOUNT")
	cmd.Flags().String("account-key", "", "Storage account key. Environment variable: AZURE_STORAGE_KEY")
	cmd.Flags().String("connection-string", "", "Storage account connection string. Environment variable: AZURE_STORAGE_CONNECTION_STRING")
	cmd.Flags().String("encryption-scope", "", "A predefined encryption scope used to encrypt the data on the service")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("permissions")

	return cmd
}

func runGenerateSAS(ctx context.Context, cmd *cobra.Command) error {
	containerName, _ := cmd.Flags().GetString("name")
	permissions, _ := cmd.Flags().GetString("permissions")
	expiryStr, _ := cmd.Flags().GetString("expiry")
	startStr, _ := cmd.Flags().GetString("start")
	ip, _ := cmd.Flags().GetString("ip")
	httpsOnly, _ := cmd.Flags().GetBool("https-only")
	policyName, _ := cmd.Flags().GetString("policy-name")
	asUser, _ := cmd.Flags().GetBool("as-user")
	authMode, _ := cmd.Flags().GetString("auth-mode")
	delegationOID, _ := cmd.Flags().GetString("user-delegation-oid")
	cacheControl, _ := cmd.Flags().GetString("cache-control")
	contentDisposition, _ := cmd.Flags().GetString("content-disposition")
	contentEncoding, _ := cmd.Flags().GetString("content-encoding")
	contentLanguage, _ := cmd.Flags().GetString("content-language")
	contentType, _ := cmd.Flags().GetString("content-type")
	accountName, _ := cmd.Flags().GetString("account-name")
	accountKey, _ := cmd.Flags().GetString("account-key")
	connectionString, _ := cmd.Flags().GetString("connection-string")
	encryptionScope, _ := cmd.Flags().GetString("encryption-scope")

	var expiry time.Time
	var err error
	if expiryStr != "" {
		expiry, err = sas.ParseTime(expiryStr)
		if err != nil {
			return fmt.Errorf("--expiry: %w", err)
		}
	}
	var start time.Time
	if startStr != "" {
		start, err = sas.ParseTime(startStr)
		if err != nil {
			return fmt.Errorf("--start: %w", err)
		}
	}

	if err := sas.ValidateAsUser(asUser, authMode, expiryStr, expiry, time.Now().UTC()); err != nil {
		return err
	}
	if delegationOID != "" && !asUser {
		return fmt.Errorf("incorrect usage: need to specify '--as-user' when '--user-delegation-oid' is provided")
	}

	protocol := ""
	if httpsOnly {
		protocol = "https"
	}

	opts := sas.BlobScopeOptions{
		ContainerName:      containerName,
		Permissions:        permissions,
		Identifier:         policyName,
		IPRange:            ip,
		Protocol:           protocol,
		EncryptionScope:    encryptionScope,
		CacheControl:       cacheControl,
		ContentDisposition: contentDisposition,
		ContentEncoding:    contentEncoding,
		ContentLanguage:    contentLanguage,
		ContentType:        contentType,
		AuthorizedObjectID: delegationOID,
		Start:              start,
		Expiry:             expiry,
	}

	var key string
	if asUser {
		opts.AccountName = sas.ResolveInputs(accountName, accountKey, connectionString).AccountName
		if opts.AccountName == "" {
			return fmt.Errorf("--account-name is required (or set AZURE_STORAGE_ACCOUNT)")
		}
	} else {
		creds, err := sas.Resolve(ctx, accountName, accountKey, connectionString)
		if err != nil {
			return err
		}
		opts.AccountName = creds.AccountName
		key = creds.AccountKey
	}

	token, err := sas.SignBlobScope(ctx, opts, key, asUser)
	if err != nil {
		return err
	}

	return output.PrintFormatted(cmd, token, sas.OutputFormat(cmd))
}
```

- [ ] **Step 5: Register, build, verify**

In `internal/storage/container/commands.go`, add `NewGenerateSASCommand()` to the final `cmd.AddCommand(...)`.

```bash
make build
./bin/az/az storage container generate-sas --help
AZURE_STORAGE_ACCOUNT=myaccount \
AZURE_STORAGE_KEY=bXlzdXBlcnNlY3JldHRlc3RrZXkxMjM0NTY3ODkwYWI= \
./bin/az/az storage container generate-sas -n mycontainer \
  --permissions rl --expiry 2030-01-01 --https-only -o tsv
```

Expected: a bare token containing `sp=rl`, `spr=https`, `se=`, `sig=`, `sr=c`.

Then confirm the guard rails fire:

```bash
./bin/az/az storage container generate-sas -n c --permissions r --auth-mode login --expiry 2030-01-01
```
Expected: `incorrect usage: specify --as-user when --auth-mode login is used to get user delegation key`

```bash
AZURE_STORAGE_ACCOUNT=a AZURE_STORAGE_KEY=bXlzdXBlcnNlY3JldHRlc3RrZXkxMjM0NTY3ODkwYWI= \
./bin/az/az storage container generate-sas -n c --permissions ry --expiry 2030-01-01
```
Expected: an error naming `y` and listing `racwdxltfmeopi`. This is the documented gap `azure-go-cli-gig`.

- [ ] **Step 6: Commit**

```bash
make test
git add internal/storage/sas/blob_sig.go internal/storage/sas/sas.go internal/storage/sas/sas_test.go internal/storage/container/
git commit -m "feat(storage): add az storage container generate-sas"
```

---

### Task 6: `az storage blob generate-sas`

**Files:**
- Create: `internal/storage/blob/generate_sas.go`
- Create: `internal/storage/blob/generate_sas_test.go`
- Modify: `internal/storage/blob/commands.go` — add to the final `cmd.AddCommand(...)`

**Interfaces:**
- Consumes: everything from Tasks 1-5, especially `sas.SignBlobScope` and `sas.Quote`.
- Produces: `func NewGenerateSASCommand() *cobra.Command`, `func fullURI(accountName, containerName, blobName, token string) string`

Here `--name`/`-n` is the **blob** name and `--container-name`/`-c` is the container (`_params.py:955`). This is the only one of the three with `--full-uri` and `--snapshot`.

- [ ] **Step 1: Write the failing test**

```go
package blob

import "testing"

func TestFullURI(t *testing.T) {
	got := fullURI("myaccount", "mycontainer", "my blob.txt", "se=2030-01-01&sig=abc%2Fdef")
	want := "https://myaccount.blob.core.windows.net/mycontainer/my%20blob.txt?se=2030-01-01&sig=abc%2Fdef"
	if got != want {
		t.Errorf("fullURI() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/blob/ -run TestFullURI -v`
Expected: FAIL to build — `undefined: fullURI`.

- [ ] **Step 3: Write the implementation**

Create `internal/storage/blob/generate_sas.go` in full:

```go
package blob

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/cdobbyn/azure-go-cli/internal/storage/sas"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewGenerateSASCommand builds `az storage blob generate-sas`.
func NewGenerateSASCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-sas",
		Short: "Generate a shared access signature for the blob",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateSAS(context.Background(), cmd)
		},
	}

	cmd.Flags().StringP("name", "n", "", "The blob name")
	cmd.Flags().StringP("container-name", "c", "", "The container name")
	cmd.Flags().String("permissions", "", "The permissions the SAS grants. Allowed values: "+sas.BlobPerms+". Can be combined")
	cmd.Flags().String("expiry", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes invalid")
	cmd.Flags().String("start", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes valid. Defaults to the time of the request")
	cmd.Flags().String("ip", "", "IP address or range of IP addresses from which to accept requests. IPv4 only")
	cmd.Flags().Bool("https-only", false, "Only permit requests made with the HTTPS protocol")
	cmd.Flags().String("policy-name", "", "The name of a stored access policy within the container's ACL")
	cmd.Flags().Bool("as-user", false, "Return the SAS signed with the user delegation key. Requires --expiry and --auth-mode login")
	cmd.Flags().String("auth-mode", "key", "The mode in which to run the command. Allowed values: key, login")
	cmd.Flags().String("user-delegation-oid", "", "Entra ID of the user authorized to use the resulting SAS URL. Requires --as-user")
	cmd.Flags().Bool("full-uri", false, "Indicates that this command return the full blob URI and the shared access signature token")
	cmd.Flags().String("snapshot", "", "An optional blob snapshot ID. Opaque DateTime value that, when present, specifies the blob snapshot to grant permission")
	cmd.Flags().String("blob-url", "", "The full endpoint URL to the blob. An alternative to --name plus --container-name")
	cmd.Flags().String("cache-control", "", "Response header value for Cache-Control when the resource is accessed using this SAS")
	cmd.Flags().String("content-disposition", "", "Response header value for Content-Disposition when the resource is accessed using this SAS")
	cmd.Flags().String("content-encoding", "", "Response header value for Content-Encoding when the resource is accessed using this SAS")
	cmd.Flags().String("content-language", "", "Response header value for Content-Language when the resource is accessed using this SAS")
	cmd.Flags().String("content-type", "", "Response header value for Content-Type when the resource is accessed using this SAS")
	cmd.Flags().String("account-name", "", "Storage account name. Environment variable: AZURE_STORAGE_ACCOUNT")
	cmd.Flags().String("account-key", "", "Storage account key. Environment variable: AZURE_STORAGE_KEY")
	cmd.Flags().String("connection-string", "", "Storage account connection string. Environment variable: AZURE_STORAGE_CONNECTION_STRING")
	cmd.Flags().String("encryption-scope", "", "A predefined encryption scope used to encrypt the data on the service")

	cmd.MarkFlagRequired("permissions")

	return cmd
}

func runGenerateSAS(ctx context.Context, cmd *cobra.Command) error {
	blobName, _ := cmd.Flags().GetString("name")
	containerName, _ := cmd.Flags().GetString("container-name")
	permissions, _ := cmd.Flags().GetString("permissions")
	expiryStr, _ := cmd.Flags().GetString("expiry")
	startStr, _ := cmd.Flags().GetString("start")
	ip, _ := cmd.Flags().GetString("ip")
	httpsOnly, _ := cmd.Flags().GetBool("https-only")
	policyName, _ := cmd.Flags().GetString("policy-name")
	asUser, _ := cmd.Flags().GetBool("as-user")
	authMode, _ := cmd.Flags().GetString("auth-mode")
	delegationOID, _ := cmd.Flags().GetString("user-delegation-oid")
	fullURIFlag, _ := cmd.Flags().GetBool("full-uri")
	snapshot, _ := cmd.Flags().GetString("snapshot")
	blobURL, _ := cmd.Flags().GetString("blob-url")
	cacheControl, _ := cmd.Flags().GetString("cache-control")
	contentDisposition, _ := cmd.Flags().GetString("content-disposition")
	contentEncoding, _ := cmd.Flags().GetString("content-encoding")
	contentLanguage, _ := cmd.Flags().GetString("content-language")
	contentType, _ := cmd.Flags().GetString("content-type")
	accountName, _ := cmd.Flags().GetString("account-name")
	accountKey, _ := cmd.Flags().GetString("account-key")
	connectionString, _ := cmd.Flags().GetString("connection-string")
	encryptionScope, _ := cmd.Flags().GetString("encryption-scope")

	// --blob-url is an alternative to naming the blob and container. It also
	// carries the account name, so it wins over --account-name.
	if blobURL != "" {
		parts, err := azblob.ParseURL(blobURL)
		if err != nil {
			return fmt.Errorf("invalid --blob-url: %w", err)
		}
		accountName = strings.Split(parts.Host, ".")[0]
		containerName = parts.ContainerName
		blobName = parts.BlobName
		if snapshot == "" {
			snapshot = parts.Snapshot
		}
	}
	if containerName == "" || blobName == "" {
		return fmt.Errorf("specify --name and --container-name, or --blob-url")
	}

	var expiry time.Time
	var err error
	if expiryStr != "" {
		expiry, err = sas.ParseTime(expiryStr)
		if err != nil {
			return fmt.Errorf("--expiry: %w", err)
		}
	}
	var start time.Time
	if startStr != "" {
		start, err = sas.ParseTime(startStr)
		if err != nil {
			return fmt.Errorf("--start: %w", err)
		}
	}

	if err := sas.ValidateAsUser(asUser, authMode, expiryStr, expiry, time.Now().UTC()); err != nil {
		return err
	}
	if delegationOID != "" && !asUser {
		return fmt.Errorf("incorrect usage: need to specify '--as-user' when '--user-delegation-oid' is provided")
	}

	protocol := ""
	if httpsOnly {
		protocol = "https"
	}

	opts := sas.BlobScopeOptions{
		ContainerName:      containerName,
		BlobName:           blobName,
		Permissions:        permissions,
		Identifier:         policyName,
		IPRange:            ip,
		Protocol:           protocol,
		EncryptionScope:    encryptionScope,
		CacheControl:       cacheControl,
		ContentDisposition: contentDisposition,
		ContentEncoding:    contentEncoding,
		ContentLanguage:    contentLanguage,
		ContentType:        contentType,
		Snapshot:           snapshot,
		AuthorizedObjectID: delegationOID,
		Start:              start,
		Expiry:             expiry,
	}

	var key string
	if asUser {
		opts.AccountName = sas.ResolveInputs(accountName, accountKey, connectionString).AccountName
		if opts.AccountName == "" {
			return fmt.Errorf("--account-name is required (or set AZURE_STORAGE_ACCOUNT)")
		}
	} else {
		creds, err := sas.Resolve(ctx, accountName, accountKey, connectionString)
		if err != nil {
			return err
		}
		opts.AccountName = creds.AccountName
		key = creds.AccountKey
	}

	token, err := sas.SignBlobScope(ctx, opts, key, asUser)
	if err != nil {
		return err
	}

	// azure-cli percent-encodes the blob token with this exact safe set
	// (operations/blob.py:906). The container command does not.
	quoted := sas.Quote(token, "&%()$=',~")
	if fullURIFlag {
		return output.PrintFormatted(cmd, fullURI(opts.AccountName, containerName, blobName, quoted), sas.OutputFormat(cmd))
	}
	return output.PrintFormatted(cmd, quoted, sas.OutputFormat(cmd))
}

// fullURI assembles the blob URL with the SAS token appended, matching
// operations/blob.py:902-905. The path is escaped but the token is not, since
// the caller has already quoted it.
func fullURI(accountName, containerName, blobName, token string) string {
	return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s?%s",
		accountName, url.PathEscape(containerName), url.PathEscape(blobName), token)
}
```

Note: `--name` and `--container-name` are deliberately **not** marked required,
because `--blob-url` can supply both. The check after parsing enforces it.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/... -v`
Expected: PASS.

- [ ] **Step 5: Register, build, verify**

In `internal/storage/blob/commands.go`, add `NewGenerateSASCommand()` to the final `cmd.AddCommand(...)`.

```bash
make build
./bin/az/az storage blob generate-sas --help

AZURE_STORAGE_ACCOUNT=myaccount \
AZURE_STORAGE_KEY=bXlzdXBlcnNlY3JldHRlc3RrZXkxMjM0NTY3ODkwYWI= \
./bin/az/az storage blob generate-sas -c mycontainer -n myblob \
  --permissions r --expiry 2030-01-01 --https-only -o tsv
```
Expected: a bare token containing `sr=b`, `sp=r`, `spr=https`, `sig=`.

```bash
AZURE_STORAGE_ACCOUNT=myaccount \
AZURE_STORAGE_KEY=bXlzdXBlcnNlY3JldHRlc3RrZXkxMjM0NTY3ODkwYWI= \
./bin/az/az storage blob generate-sas -c mycontainer -n myblob \
  --permissions r --expiry 2030-01-01 --full-uri -o tsv
```
Expected: `https://myaccount.blob.core.windows.net/mycontainer/myblob?...`

Confirm `y` IS accepted here, unlike on a container:
```bash
AZURE_STORAGE_ACCOUNT=a AZURE_STORAGE_KEY=bXlzdXBlcnNlY3JldHRlc3RrZXkxMjM0NTY3ODkwYWI= \
./bin/az/az storage blob generate-sas -c c -n b --permissions ry --expiry 2030-01-01 -o tsv
```
Expected: a token with `sp=ry`.

- [ ] **Step 6: Commit**

```bash
make test
gofmt -l internal/storage/sas internal/storage/account/generate_sas.go internal/storage/container/generate_sas.go internal/storage/blob/generate_sas.go
git add internal/storage/blob/
git commit -m "feat(storage): add az storage blob generate-sas"
```

`gofmt -l` must print nothing. Do not run `gofmt -w` across the repo; it mixes tab and 2-space files by design.

---

## Final verification

- [ ] `make test` passes.
- [ ] `make build` succeeds.
- [ ] All three `--help` outputs list the flags from the spec's flag tables.
- [ ] `git branch --show-current` reads `storage-generate-sas-4c1e`.
- [ ] Update `docs/implemented-commands.txt` if it tracks the command list.

## Live verification (needs a real storage account, do by hand)

These cannot run offline. Report the results rather than assuming them.

1. Account SAS with `--services bqtf`, used against blob and queue endpoints.
2. Container SAS round-trip: generate, then `az storage blob upload --sas-token`.
3. Blob SAS round-trip: generate with `--full-uri`, then `curl` the URL.
4. `--as-user --auth-mode login` against a real user delegation key. Confirm the token carries `skoid=`, `sktid=`, `skt=` and `skv=`.
5. Cross-check one token against the Python CLI's output for identical inputs.
