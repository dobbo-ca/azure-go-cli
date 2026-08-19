package devops

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// userCaptureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything it wrote, same idiom as team_test.go's teamCaptureStdout.
func userCaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig

	var b strings.Builder
	io.Copy(&b, r)
	return b.String()
}

// userRequest is one HTTP call captured by userTestServer.
type userRequest struct {
	Method      string
	Host        string
	Path        string
	Query       url.Values
	ContentType string
	Body        []byte
}

// userRedirectTransport rewrites every outbound request's dial target to a
// local httptest.Server while preserving the Host header ado.Client.url()
// computed (e.g. "vsaex.dev.azure.com"). This is the only way to exercise
// the vsaex/vssps resource-area hosts against httptest: ado.Client.url()
// genuinely rewrites the hostname per request (client.go hostFor), and a
// real DNS lookup for "vsaex.127.0.0.1:PORT" would fail.
type userRedirectTransport struct {
	addr string
	next http.RoundTripper
}

func (t userRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Host = req.URL.Host
	req2.URL.Host = t.addr
	req2.URL.Scheme = "http"
	return t.next.RoundTrip(req2)
}

// userTestServer starts a hermetic fake Azure DevOps server: responder is
// called once per request, in order (n is 0-based), and returns the status
// and JSON body to answer with. Returns the organization URL to pass as
// --organization and a pointer to the requests captured so far.
//
// ponytail: swaps the process-wide http.DefaultTransport for the test's
// duration (ado.NewClient's *http.Client has no Transport of its own to
// inject from outside package ado) — fine because these subtests run
// serially, never t.Parallel().
func userTestServer(t *testing.T, responder func(n int, req userRequest) (int, string)) (string, *[]userRequest) {
	t.Helper()

	requests := &[]userRequest{}
	n := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		req := userRequest{
			Method:      r.Method,
			Host:        r.Host,
			Path:        r.URL.Path,
			Query:       r.URL.Query(),
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		}
		*requests = append(*requests, req)
		status, respBody := responder(n, req)
		n++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)

	origTransport := http.DefaultTransport
	http.DefaultTransport = userRedirectTransport{addr: srv.Listener.Addr().String(), next: origTransport}
	t.Cleanup(func() { http.DefaultTransport = origTransport })

	// config.Load() (via azure.GetCredential -> ado's AAD attempt) fails
	// fast off a HOME with no azureProfile.json, so auth falls through to
	// the PAT below with no real network call, on any machine.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	// A URL that passes ado's validateOrg (must be https://dev.azure.com/...
	// or *.visualstudio.com); userRedirectTransport ignores the hostname
	// entirely and always dials srv, so this never touches the network.
	return "https://dev.azure.com/myorg", requests
}

// userExecute runs cmd (one of the userNew*Cmd() constructors) through a
// minimal root carrying the inherited -o/--query persistent flags, exactly
// as cmd/az/main.go wires them, so ado.Print behaves the same as in
// production.
func userExecute(t *testing.T, cmd *cobra.Command, org string, extraArgs ...string) error {
	t.Helper()
	root := &cobra.Command{Use: "az"}
	root.PersistentFlags().StringP("output", "o", "json", "")
	root.PersistentFlags().String("query", "", "")
	root.AddCommand(cmd)
	// No SetOut: pkg/output.PrintFormatted now writes via cmd.OutOrStdout(),
	// which a child command inherits from root, so discarding root's output
	// here would swallow the very JSON userCaptureStdout's tests need to see.
	root.SetArgs(append([]string{cmd.Use, "--organization", org}, extraArgs...))
	return root.Execute()
}

func TestUserRunList(t *testing.T) {
	t.Run("default top, no skip", func(t *testing.T) {
		org, reqs := userTestServer(t, func(n int, r userRequest) (int, string) {
			return 200, `{"members":[{"id":"u1","user":{"displayName":"Alice","mailAddress":"alice@example.com"},"accessLevel":{"accountLicenseType":"express","licenseDisplayName":"Basic","status":"active"}}]}`
		})

		if err := userExecute(t, userNewListCmd(), org); err != nil {
			t.Fatalf("execute: %v", err)
		}

		if len(*reqs) != 1 {
			t.Fatalf("got %d requests, want 1", len(*reqs))
		}
		r := (*reqs)[0]
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.Path != "/myorg/_apis/userentitlements" {
			t.Errorf("path = %q", r.Path)
		}
		if got := r.Host; len(got) < 6 || got[:6] != "vsaex." {
			t.Errorf("host = %q, want vsaex. prefix", got)
		}
		if r.Query.Get("api-version") != "5.0-preview.2" {
			t.Errorf("api-version = %q", r.Query.Get("api-version"))
		}
		if r.Query.Get("top") != "100" {
			t.Errorf("top = %q, want 100 (python's own default)", r.Query.Get("top"))
		}
		if r.Query.Has("skip") {
			t.Errorf("skip present but --skip was never passed; python only sends it when non-None")
		}
	})

	t.Run("explicit top and skip", func(t *testing.T) {
		org, reqs := userTestServer(t, func(n int, r userRequest) (int, string) {
			return 200, `{"members":[]}`
		})

		if err := userExecute(t, userNewListCmd(), org, "--top", "5", "--skip", "10"); err != nil {
			t.Fatalf("execute: %v", err)
		}

		r := (*reqs)[0]
		if r.Query.Get("top") != "5" || r.Query.Get("skip") != "10" {
			t.Errorf("top=%q skip=%q, want 5/10", r.Query.Get("top"), r.Query.Get("skip"))
		}
	})
}

// TestUserRunList_JSONKeepsWholeWrapper guards M7: get_user_entitlements
// returns the whole PagedGraphMemberList wrapper (member_entitlement_
// management/models.py:939-952 + PagedList base :342-357), not just the bare
// members array — -o json must not drop the wrapper's other keys.
func TestUserRunList_JSONKeepsWholeWrapper(t *testing.T) {
	org, _ := userTestServer(t, func(n int, r userRequest) (int, string) {
		return 200, `{"members":[{"id":"u1"}],"continuationToken":"tok-1","totalCount":1}`
	})

	out := userCaptureStdout(t, func() {
		if err := userExecute(t, userNewListCmd(), org); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if got["continuationToken"] != "tok-1" {
		t.Errorf("continuationToken = %v, want tok-1", got["continuationToken"])
	}
	if got["totalCount"] != float64(1) {
		t.Errorf("totalCount = %v, want 1", got["totalCount"])
	}
	members, _ := got["members"].([]any)
	if len(members) != 1 {
		t.Errorf("members = %v", got["members"])
	}
}

func TestUserRunShow_EmailResolution(t *testing.T) {
	// user.py:34: '@' present => resolve_identity_as_id => resolve_identity
	// tries searchFilter=General first, falls back to DirectoryAlias when
	// General finds nothing (common/identities.py:59-70) — a genuine
	// three-call sequence for a single `user show --user <email>`.
	org, reqs := userTestServer(t, func(n int, r userRequest) (int, string) {
		switch n {
		case 0:
			return 200, `{"count":0,"value":[]}`
		case 1:
			return 200, `{"count":1,"value":[{"id":"identity-1","properties":{}}]}`
		default:
			return 200, `{"id":"identity-1","user":{"displayName":"Alice","mailAddress":"alice@example.com"},"accessLevel":{"accountLicenseType":"express","licenseDisplayName":"Basic","status":"active"}}`
		}
	})

	if err := userExecute(t, userNewShowCmd(), org, "--user", "alice@example.com"); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(*reqs) != 3 {
		t.Fatalf("got %d requests, want 3 (General search, DirectoryAlias fallback, GET by id)", len(*reqs))
	}

	for i, wantFilter := range []string{"General", "DirectoryAlias"} {
		r := (*reqs)[i]
		if r.Path != "/myorg/_apis/identities" {
			t.Errorf("request %d path = %q", i, r.Path)
		}
		if got := r.Host; len(got) < 6 || got[:6] != "vssps." {
			t.Errorf("request %d host = %q, want vssps. prefix", i, got)
		}
		if r.Query.Get("searchFilter") != wantFilter {
			t.Errorf("request %d searchFilter = %q, want %q", i, r.Query.Get("searchFilter"), wantFilter)
		}
		if r.Query.Get("filterValue") != "alice@example.com" {
			t.Errorf("request %d filterValue = %q", i, r.Query.Get("filterValue"))
		}
	}

	last := (*reqs)[2]
	if last.Path != "/myorg/_apis/userentitlements/identity-1" {
		t.Errorf("final request path = %q, want resolved id in path", last.Path)
	}
	if got := last.Host; len(got) < 6 || got[:6] != "vsaex." {
		t.Errorf("final request host = %q, want vsaex. prefix", got)
	}
}

// TestUserNormalizeLicenseType guards m6: arguments.py:110 registers
// license_type via get_enum_type(), whose CaseInsensitiveList choices match
// case-insensitively and normalize to the canonical-cased value.
func TestUserNormalizeLicenseType(t *testing.T) {
	got, err := userNormalizeLicenseType("EXPRESS")
	if err != nil || got != "express" {
		t.Errorf("userNormalizeLicenseType(\"EXPRESS\") = (%q, %v), want (\"express\", nil)", got, err)
	}
	if _, err := userNormalizeLicenseType("bogus"); err == nil {
		t.Error("expected an error for an unknown license type")
	}
}

func TestUserRunUpdate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		org, reqs := userTestServer(t, func(n int, r userRequest) (int, string) {
			return 200, `{"isSuccess":true,"userEntitlement":{"id":"u1","user":{"displayName":"Alice","mailAddress":"alice@example.com"},"accessLevel":{"accountLicenseType":"stakeholder","licenseDisplayName":"Stakeholder","status":"active"}},"operationResults":[{"errors":[null]}]}`
		})

		if err := userExecute(t, userNewUpdateCmd(), org, "--user", "u1", "--license-type", "stakeholder"); err != nil {
			t.Fatalf("execute: %v", err)
		}

		r := (*reqs)[0]
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		if r.Path != "/myorg/_apis/userentitlements/u1" {
			t.Errorf("path = %q", r.Path)
		}
		if r.ContentType != "application/json-patch+json" {
			t.Errorf("content-type = %q, want application/json-patch+json", r.ContentType)
		}

		var body []map[string]any
		if err := json.Unmarshal(r.Body, &body); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if len(body) != 1 || body[0]["op"] != "replace" || body[0]["path"] != "/accessLevel" {
			t.Fatalf("body = %s, want a single replace /accessLevel op", r.Body)
		}
		value, _ := body[0]["value"].(map[string]any)
		if value["accountLicenseType"] != "stakeholder" {
			t.Errorf("body value = %v, want accountLicenseType=stakeholder", value)
		}
	})

	t.Run("server-reported operation error is surfaced as the error message", func(t *testing.T) {
		org, _ := userTestServer(t, func(n int, r userRequest) (int, string) {
			return 200, `{"isSuccess":false,"operationResults":[{"errors":[{"key":0,"value":"boom"}]}]}`
		})

		err := userExecute(t, userNewUpdateCmd(), org, "--user", "u1", "--license-type", "stakeholder")
		if err == nil || err.Error() != "boom" {
			t.Fatalf("err = %v, want \"boom\" (user.py:71-73)", err)
		}
	})
}

func TestUserRunAdd(t *testing.T) {
	t.Run("default invite (unset --send-email-invite means true, so doNotSendInviteForNewUsers=false)", func(t *testing.T) {
		org, reqs := userTestServer(t, func(n int, r userRequest) (int, string) {
			return 200, `{"haveResultsSucceeded":true,"results":[{"result":{"id":"new1","user":{"displayName":"Bob","mailAddress":"bob@example.com"},"accessLevel":{"accountLicenseType":"express","licenseDisplayName":"Basic","status":"pending"}}}]}`
		})

		if err := userExecute(t, userNewAddCmd(), org, "--email-id", "bob@example.com", "--license-type", "express"); err != nil {
			t.Fatalf("execute: %v", err)
		}

		r := (*reqs)[0]
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		if r.Path != "/myorg/_apis/userentitlements" {
			t.Errorf("path = %q, want the bulk collection route (no id)", r.Path)
		}
		if r.Query.Get("doNotSendInviteForNewUsers") != "false" {
			t.Errorf("doNotSendInviteForNewUsers = %q, want false (user.py:85-87 double negative)", r.Query.Get("doNotSendInviteForNewUsers"))
		}

		var body []map[string]any
		if err := json.Unmarshal(r.Body, &body); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		value, _ := body[0]["value"].(map[string]any)
		user, _ := value["user"].(map[string]any)
		if user["principalName"] != "bob@example.com" || user["subjectKind"] != "user" {
			t.Errorf("body user = %v", user)
		}
	})

	t.Run("--send-email-invite=false inverts to doNotSendInviteForNewUsers=true", func(t *testing.T) {
		org, reqs := userTestServer(t, func(n int, r userRequest) (int, string) {
			return 200, `{"haveResultsSucceeded":true,"results":[{"result":{"id":"new1"}}]}`
		})

		// "--send-email-invite=false" (one token): NoOptDefVal means a
		// space-separated "--send-email-invite false" would parse as the
		// bare flag (true) plus a stray positional "false" instead, the
		// same string-flag limitation ado.go documents for --detect.
		if err := userExecute(t, userNewAddCmd(), org, "--email-id", "bob@example.com", "--license-type", "express", "--send-email-invite=false"); err != nil {
			t.Fatalf("execute: %v", err)
		}

		r := (*reqs)[0]
		if r.Query.Get("doNotSendInviteForNewUsers") != "true" {
			t.Errorf("doNotSendInviteForNewUsers = %q, want true", r.Query.Get("doNotSendInviteForNewUsers"))
		}
	})

	t.Run("bulk operation error is surfaced from the results[] envelope", func(t *testing.T) {
		org, _ := userTestServer(t, func(n int, r userRequest) (int, string) {
			return 200, `{"haveResultsSucceeded":false,"results":[{"errors":[{"key":0,"value":"already exists"}]}]}`
		})

		err := userExecute(t, userNewAddCmd(), org, "--email-id", "bob@example.com", "--license-type", "express")
		if err == nil || err.Error() != "already exists" {
			t.Fatalf("err = %v, want \"already exists\" (user.py:104-105)", err)
		}
	})
}
