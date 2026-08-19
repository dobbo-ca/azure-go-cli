package repos

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// policySetup points cmd at srv and fills in the globals every policy
// command needs, hermetically (no real az login / git detection).
//
// ado.ResolveProject's validateOrg requires an org that looks like a real
// "https://dev.azure.com/..." or "*.visualstudio.com" URL (context.go), so
// an httptest URL can't be passed as --organization directly. Instead we
// keep a real-looking org and redirect the process-wide default transport
// to srv - the same host-override problem and the same fix as
// policyTestADOClient below, just applied at the http.DefaultTransport level
// because these commands build their *ado.Client internally.
func policySetup(t *testing.T, cmd *cobra.Command, srv *httptest.Server) {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")
	// Point pkg/config at a profile file that can't exist, so
	// azure.GetCredential fails fast on "not logged in" instead of
	// finding this machine's real cached az-login profile and making a
	// live AAD network call (pkg/config/profile.go's GetConfigPath honours
	// AZ_SESSION for exactly this kind of per-session isolation).
	t.Setenv("AZ_SESSION", "policy-test-hermetic")

	orig := http.DefaultTransport
	http.DefaultTransport = policyRedirectTransport{target: srv.Listener.Addr().String(), next: orig}
	t.Cleanup(func() { http.DefaultTransport = orig })

	policyMustSet(t, cmd, "organization", "https://dev.azure.com/myorg")
	policyMustSet(t, cmd, "project", "MyProj")
	policyMustSet(t, cmd, "detect", "false")
}

func policyMustSet(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set --%s=%s: %v", name, value, err)
	}
}

func policyDecodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal body %s: %v", b, err)
	}
	return m
}

// TestPolicyRequiredReviewerCreateRequiresMessage: policy.py:154-156 gives
// `message` no default, so create_policy_required_reviewer requires it
// (update, policy.py:178, correctly leaves it optional).
func TestPolicyRequiredReviewerCreateRequiresMessage(t *testing.T) {
	cmd := newPolicyRequiredReviewerCreateCmd()
	args := []string{
		"--repository-id", "repo1",
		"--branch", "master",
		"--blocking", "true",
		"--enabled", "true",
		"--required-reviewer-ids", "alice@contoso.com",
	}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if err := cmd.ValidateRequiredFlags(); err == nil {
		t.Errorf("--message must be required on create")
	}

	updateCmd := newPolicyRequiredReviewerUpdateCmd()
	if err := updateCmd.ValidateRequiredFlags(); err != nil {
		t.Errorf("--message must stay optional on update, got %v", err)
	}
}

func TestPolicyApproverCountCreate(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotBody = policyDecodeBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"p1","isBlocking":true,"isEnabled":true,"settings":{"scope":[{"repositoryId":"repo1"}]},"type":{"id":"` + policyTypeApproverCount + `"}}`))
	}))
	defer srv.Close()

	cmd := newPolicyApproverCountCreateCmd()
	policySetup(t, cmd, srv)
	for k, v := range map[string]string{
		"repository-id":          "repo1",
		"branch":                 "main",
		"blocking":               "true",
		"enabled":                "true",
		"minimum-approver-count": "2",
		"creator-vote-counts":    "false",
		"allow-downvotes":        "true",
		"reset-on-source-push":   "false",
	} {
		policyMustSet(t, cmd, k, v)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	wantPath := "/myorg/MyProj/_apis/policy/configurations"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotQuery != "api-version=5.0" {
		t.Errorf("query = %q, want api-version=5.0", gotQuery)
	}

	if gotBody["isBlocking"] != true || gotBody["isEnabled"] != true {
		t.Errorf("isBlocking/isEnabled = %v/%v, want true/true", gotBody["isBlocking"], gotBody["isEnabled"])
	}
	typ, _ := gotBody["type"].(map[string]any)
	if typ["id"] != policyTypeApproverCount {
		t.Errorf("type.id = %v, want %s", typ["id"], policyTypeApproverCount)
	}
	settings, _ := gotBody["settings"].(map[string]any)
	if settings["minimumApproverCount"] != "2" {
		t.Errorf("minimumApproverCount = %v (%T), want string \"2\"", settings["minimumApproverCount"], settings["minimumApproverCount"])
	}
	if settings["creatorVoteCounts"] != false || settings["allowDownvotes"] != true || settings["resetOnSourcePush"] != false {
		t.Errorf("tri-state settings = %v", settings)
	}
	scope, _ := settings["scope"].([]any)
	if len(scope) != 1 {
		t.Fatalf("scope len = %d, want 1", len(scope))
	}
	s0 := scope[0].(map[string]any)
	if s0["repositoryId"] != "repo1" || s0["refName"] != "refs/heads/main" || s0["matchKind"] != "exact" {
		t.Errorf("scope = %v", s0)
	}
}

func TestPolicyCaseEnforcementCreateLiteralStringSetting(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = policyDecodeBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cmd := newPolicyCaseEnforcementCreateCmd()
	policySetup(t, cmd, srv)
	for k, v := range map[string]string{"repository-id": "repo1", "blocking": "true", "enabled": "true"} {
		policyMustSet(t, cmd, k, v)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	settings, _ := gotBody["settings"].(map[string]any)
	// policy.py:582-583 sends the literal string "true", not a JSON boolean.
	if v, ok := settings["enforceConsistentCase"].(string); !ok || v != "true" {
		t.Errorf("enforceConsistentCase = %#v, want string \"true\"", settings["enforceConsistentCase"])
	}
	scope, _ := settings["scope"].([]any)
	s0 := scope[0].(map[string]any)
	if _, hasRef := s0["refName"]; hasRef {
		t.Errorf("scope has refName %v, want repo-wide scope with no branch", s0["refName"])
	}
}

func TestPolicyList(t *testing.T) {
	t.Run("repository-id and branch build a dash-stripped scope", func(t *testing.T) {
		var gotScope string
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			gotScope = r.URL.Query().Get("scope")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"count":0,"value":[]}`))
		}))
		defer srv.Close()

		cmd := newPolicyListCmd()
		policySetup(t, cmd, srv)
		policyMustSet(t, cmd, "repository-id", "e556f204-53c9-4153-9cd9-ef41a11e3345")
		policyMustSet(t, cmd, "branch", "main")

		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}
		if !called {
			t.Fatal("server was not called")
		}
		wantScope := policyStripDashes("e556f204-53c9-4153-9cd9-ef41a11e3345") + ":refs/heads/main"
		if gotScope != wantScope {
			t.Errorf("scope = %q, want %q", gotScope, wantScope)
		}
	})

	t.Run("no repository-id sends no scope param", func(t *testing.T) {
		var gotRawQuery string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotRawQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"count":0,"value":[]}`))
		}))
		defer srv.Close()

		cmd := newPolicyListCmd()
		policySetup(t, cmd, srv)

		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}
		if gotRawQuery != "api-version=5.0" {
			t.Errorf("query = %q, want just api-version", gotRawQuery)
		}
	})

	t.Run("branch without repository-id is a client-side error", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		defer srv.Close()

		cmd := newPolicyListCmd()
		policySetup(t, cmd, srv)
		policyMustSet(t, cmd, "branch", "main")

		if err := cmd.RunE(cmd, nil); err == nil {
			t.Fatal("want error, got nil")
		}
		if called {
			t.Error("server should not have been called")
		}
	})
}

func policyStripDashes(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '-' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

// TestPolicyMergeStrategyUseSquashMergeHidden covers arguments.py:66-71:
// use_squash_merge is wrapped in context.deprecate(hide=True), so it must
// not appear in --help while remaining functional (still settable and
// still processed).
func TestPolicyMergeStrategyUseSquashMergeHidden(t *testing.T) {
	for _, cmdFn := range []func() *cobra.Command{newPolicyMergeStrategyCreateCmd, newPolicyMergeStrategyUpdateCmd} {
		cmd := cmdFn()
		f := cmd.Flags().Lookup("use-squash-merge")
		if f == nil {
			t.Fatalf("%s: --use-squash-merge must still be registered", cmd.Use)
		}
		if !f.Hidden {
			t.Errorf("%s: --use-squash-merge must be hidden from help", cmd.Use)
		}
		if f.Deprecated == "" {
			t.Errorf("%s: --use-squash-merge must carry a deprecation message", cmd.Use)
		}
	}
}

func TestPolicyMergeStrategyUpdateStateMachine(t *testing.T) {
	t.Run("new-style flag on a legacy policy inherits useSquashMerge as allowSquash", func(t *testing.T) {
		var putBody map[string]any
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodGet:
				w.Write([]byte(`{"isBlocking":true,"isEnabled":true,"settings":{"useSquashMerge":true,"scope":[{"repositoryId":"repo1","refName":"refs/heads/main","matchKind":"exact"}]}}`))
			case http.MethodPut:
				putBody = policyDecodeBody(t, r)
				w.Write([]byte(`{}`))
			}
		}))
		defer srv.Close()

		cmd := newPolicyMergeStrategyUpdateCmd()
		policySetup(t, cmd, srv)
		policyMustSet(t, cmd, "policy-id", "p1")
		policyMustSet(t, cmd, "allow-rebase", "true")

		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}
		if calls != 2 {
			t.Fatalf("calls = %d, want 2 (GET then PUT)", calls)
		}
		settings, _ := putBody["settings"].(map[string]any)
		if settings["allowSquash"] != true {
			t.Errorf("allowSquash = %v, want true (inherited from legacy useSquashMerge)", settings["allowSquash"])
		}
		if settings["allowRebase"] != true {
			t.Errorf("allowRebase = %v, want true", settings["allowRebase"])
		}
		if settings["allowRebaseMerge"] != false || settings["allowNoFastForward"] != false {
			t.Errorf("unset new-style settings should coerce to false: %v", settings)
		}
		if _, has := settings["useSquashMerge"]; has {
			t.Errorf("useSquashMerge should not be present once switched to new-style: %v", settings)
		}
	})

	t.Run("legacy --use-squash-merge is rejected once the policy has any new-style value", func(t *testing.T) {
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"isBlocking":true,"isEnabled":true,"settings":{"allowSquash":true,"allowRebase":false,"allowRebaseMerge":false,"allowNoFastForward":false,"scope":[{"repositoryId":"repo1","refName":"refs/heads/main","matchKind":"exact"}]}}`))
		}))
		defer srv.Close()

		cmd := newPolicyMergeStrategyUpdateCmd()
		policySetup(t, cmd, srv)
		policyMustSet(t, cmd, "policy-id", "p1")
		policyMustSet(t, cmd, "use-squash-merge", "true")

		err := cmd.RunE(cmd, nil)
		if err == nil {
			t.Fatal("want error, got nil")
		}
		if got := err.Error(); got != policyMergeDeprecatedError {
			t.Errorf("error = %q, want %q", got, policyMergeDeprecatedError)
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1 (GET only, no PUT)", calls)
		}
	})

	t.Run("no merge-type flags on a legacy policy resend useSquashMerge unchanged", func(t *testing.T) {
		var putBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodGet:
				w.Write([]byte(`{"isBlocking":true,"isEnabled":true,"settings":{"useSquashMerge":true,"scope":[{"repositoryId":"repo1","refName":"refs/heads/main","matchKind":"exact"}]}}`))
			case http.MethodPut:
				putBody = policyDecodeBody(t, r)
				w.Write([]byte(`{}`))
			}
		}))
		defer srv.Close()

		cmd := newPolicyMergeStrategyUpdateCmd()
		policySetup(t, cmd, srv)
		policyMustSet(t, cmd, "policy-id", "p1")
		policyMustSet(t, cmd, "blocking", "false")

		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}
		settings, _ := putBody["settings"].(map[string]any)
		if len(settings) != 2 { // useSquashMerge + scope
			t.Errorf("settings = %v, want only useSquashMerge (+scope)", settings)
		}
		if settings["useSquashMerge"] != true {
			t.Errorf("useSquashMerge = %v, want true (unchanged)", settings["useSquashMerge"])
		}
		if putBody["isBlocking"] != false {
			t.Errorf("isBlocking = %v, want false", putBody["isBlocking"])
		}
	})
}

// TestPolicyTriStateExplicitEmptyErrors: arguments.py:36-39's
// get_three_state_flag() restricts blocking/enabled to true/false via
// argparse choices, so `--blocking=` is rejected there. Without this check,
// policyTriState returns (nil, nil) for an explicitly-empty value even
// though the flag is Changed+required, and every create command
// dereferences that as *blocking — a nil-pointer panic instead of a clean
// error.
func TestPolicyTriStateExplicitEmptyErrors(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	policyAddTriStateFlag(cmd, "blocking", "")
	if err := cmd.Flags().Set("blocking", ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := policyTriState(cmd, "blocking")
	if err == nil {
		t.Fatalf("want error for --blocking=, got (%v, nil)", got)
	}
}

// TestPolicyNameValueNullDisplayName covers _get_policy_display_name
// (_format.py:42-46): presence of the "displayName" key in settings decides
// the branch, not its type — a JSON null there must render blank, not fall
// back to type.displayName.
func TestPolicyNameValueNullDisplayName(t *testing.T) {
	row := map[string]any{
		"settings": map[string]any{"displayName": nil},
		"type":     map[string]any{"displayName": "Fallback Type Name"},
	}
	if got := policyNameValue(row); got != "" {
		t.Errorf("policyNameValue = %q, want empty (null displayName present, no fallback)", got)
	}
}

// TestPolicyBranchValueNullRefName covers the refName branch of
// _transform_repo_policy_request_row (_format.py:35-38): presence of
// "refName" decides the branch, not its type — a JSON null there must
// render blank, not "All Branches".
func TestPolicyBranchValueNullRefName(t *testing.T) {
	row := map[string]any{
		"settings": map[string]any{
			"scope": []any{map[string]any{"refName": nil}},
		},
	}
	if got := policyBranchValue(row); got != "" {
		t.Errorf("policyBranchValue = %q, want empty (null refName present, not \"All Branches\")", got)
	}
}

func TestPolicyResolveIdentityID(t *testing.T) {
	t.Run("UUID input short-circuits without any HTTP call", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		defer srv.Close()
		client := policyTestADOClient(t, srv)

		id, err := policyResolveIdentityID(context.Background(), client, "e556f204-53c9-4153-9cd9-ef41a11e3345")
		if err != nil {
			t.Fatalf("policyResolveIdentityID: %v", err)
		}
		if id != "e556f204-53c9-4153-9cd9-ef41a11e3345" {
			t.Errorf("id = %q, want the input echoed back", id)
		}
		if called {
			t.Error("server should not have been called for a UUID input")
		}
	})

	t.Run("email input tries General before DirectoryAlias", func(t *testing.T) {
		var searchFilters []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sf := r.URL.Query().Get("searchFilter")
			searchFilters = append(searchFilters, sf)
			w.Header().Set("Content-Type", "application/json")
			if sf == "General" {
				w.Write([]byte(`{"count":1,"value":[{"id":"identity-1"}]}`))
				return
			}
			w.Write([]byte(`{"count":0,"value":[]}`))
		}))
		defer srv.Close()
		client := policyTestADOClient(t, srv)

		id, err := policyResolveIdentityID(context.Background(), client, "alice@contoso.com")
		if err != nil {
			t.Fatalf("policyResolveIdentityID: %v", err)
		}
		if id != "identity-1" {
			t.Errorf("id = %q, want identity-1", id)
		}
		if len(searchFilters) != 1 || searchFilters[0] != "General" {
			t.Errorf("searchFilters = %v, want [General] (found on first try)", searchFilters)
		}
	})

	t.Run("alias input falls back from DirectoryAlias to General when empty", func(t *testing.T) {
		var searchFilters []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sf := r.URL.Query().Get("searchFilter")
			searchFilters = append(searchFilters, sf)
			w.Header().Set("Content-Type", "application/json")
			if sf == "General" {
				w.Write([]byte(`{"count":1,"value":[{"id":"identity-2"}]}`))
				return
			}
			w.Write([]byte(`{"count":0,"value":[]}`))
		}))
		defer srv.Close()
		client := policyTestADOClient(t, srv)

		id, err := policyResolveIdentityID(context.Background(), client, "jsmith")
		if err != nil {
			t.Fatalf("policyResolveIdentityID: %v", err)
		}
		if id != "identity-2" {
			t.Errorf("id = %q, want identity-2", id)
		}
		if len(searchFilters) != 2 || searchFilters[0] != "DirectoryAlias" || searchFilters[1] != "General" {
			t.Errorf("searchFilters = %v, want [DirectoryAlias General]", searchFilters)
		}
	})

	t.Run("me resolves the caller via ConnectionData", func(t *testing.T) {
		var paths []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"authenticatedUser":{"id":"me-id"}}`))
		}))
		defer srv.Close()
		client := policyTestADOClient(t, srv)

		id, err := policyResolveIdentityID(context.Background(), client, "me")
		if err != nil {
			t.Fatalf("policyResolveIdentityID: %v", err)
		}
		if id != "me-id" {
			t.Errorf("id = %q, want me-id (identities.py:17-19 ME branch)", id)
		}
		for _, p := range paths {
			if strings.Contains(p, "Identities") {
				t.Errorf("must not hit the vssps Identities search for \"me\": %v", paths)
			}
		}
	})

	t.Run("leading @ filter tries DirectoryAlias before General", func(t *testing.T) {
		// identities.py:60 uses find(' ')>0 / find('@')>0 (strict index>0),
		// so a filter starting with '@' does NOT take the General-first
		// order a naive Contains check would give it.
		var searchFilters []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sf := r.URL.Query().Get("searchFilter")
			searchFilters = append(searchFilters, sf)
			w.Header().Set("Content-Type", "application/json")
			if sf == "DirectoryAlias" {
				w.Write([]byte(`{"count":1,"value":[{"id":"identity-3"}]}`))
				return
			}
			w.Write([]byte(`{"count":0,"value":[]}`))
		}))
		defer srv.Close()
		client := policyTestADOClient(t, srv)

		id, err := policyResolveIdentityID(context.Background(), client, "@contoso")
		if err != nil {
			t.Fatalf("policyResolveIdentityID: %v", err)
		}
		if id != "identity-3" {
			t.Errorf("id = %q, want identity-3", id)
		}
		if len(searchFilters) != 1 || searchFilters[0] != "DirectoryAlias" {
			t.Errorf("searchFilters = %v, want [DirectoryAlias] (found on first try)", searchFilters)
		}
	})

	t.Run("no match errors", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"count":0,"value":[]}`))
		}))
		defer srv.Close()
		client := policyTestADOClient(t, srv)

		_, err := policyResolveIdentityID(context.Background(), client, "nobody@contoso.com")
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})
}

// policyTestADOClient builds a real *ado.Client hermetically, then swaps its
// transport so requests to any host land on srv - the only way to exercise
// the vssps host override against httptest, which cannot bind an alternate
// hostname (see foundation-spec.md §10, TestHostFor, for the same tradeoff
// made in the foundation package's own tests).
func policyTestADOClient(t *testing.T, srv *httptest.Server) *ado.Client {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")
	t.Setenv("AZ_SESSION", "policy-test-hermetic")

	client, err := ado.NewClient(context.Background(), "https://dev.azure.com/myorg")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.HTTP.Transport = policyRedirectTransport{target: srv.Listener.Addr().String(), next: http.DefaultTransport}
	return client
}

// policyRedirectTransport rewrites every request's scheme/host to target before
// delegating to next - the only way to exercise a Host-overridden request
// (e.g. the vssps subdomain) against an httptest.Server, which can't bind an
// alternate hostname.
type policyRedirectTransport struct {
	target string
	next   http.RoundTripper
}

func (t policyRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = t.target
	return t.next.RoundTrip(req)
}
