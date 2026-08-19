package devops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// withCoreStdin redirects os.Stdin to content for the duration of fn,
// restoring it afterward. Exercises PromptSecret's non-TTY "read one line"
// path, which is what a piped `echo $PAT | az devops login` and (here) a
// test both hit.
func withCoreStdin(t *testing.T, content string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig })

	go func() {
		_, _ = w.WriteString(content)
		w.Close()
	}()

	fn()
}

func TestRunCoreLogin(t *testing.T) {
	t.Run("valid token with organization verifies, stores the PAT, and sets the default org", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())

		var gotMethod, gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path + "?" + r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{
				"authenticatedUser": map[string]any{"id": "real-user-guid"},
			})
		}))
		defer srv.Close()

		cmd := newCoreLoginCmd()
		if err := cmd.Flags().Set("organization", srv.URL); err != nil {
			t.Fatal(err)
		}

		withCoreStdin(t, "s3cr3t-pat\n", func() {
			if err := runCoreLogin(context.Background(), cmd); err != nil {
				t.Fatalf("runCoreLogin: %v", err)
			}
		})

		if gotMethod != http.MethodGet {
			t.Errorf("verification request method = %q, want GET", gotMethod)
		}
		if gotPath != "/_apis/connectionData?api-version=5.0-preview.1" {
			t.Errorf("verification request path = %q", gotPath)
		}
		if got := ado.GetPAT(srv.URL); got != "s3cr3t-pat" {
			t.Errorf("GetPAT = %q, want s3cr3t-pat", got)
		}
		org, _, _ := ado.ConfigDefaults()
		if org != srv.URL {
			t.Errorf("default org = %q, want %q (first login auto-sets it)", org, srv.URL)
		}
	})

	t.Run("anonymous user id fails verification and does not store the PAT", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"authenticatedUser": map[string]any{"id": coreAnonymousUserID},
			})
		}))
		defer srv.Close()

		cmd := newCoreLoginCmd()
		_ = cmd.Flags().Set("organization", srv.URL)

		var err error
		withCoreStdin(t, "bad-pat\n", func() {
			err = runCoreLogin(context.Background(), cmd)
		})

		if err == nil || err.Error() != "Failed to authenticate using the supplied token." {
			t.Fatalf("err = %v, want anonymous-user failure", err)
		}
		if got := ado.GetPAT(srv.URL); got != "" {
			t.Errorf("GetPAT = %q, want empty (not stored)", got)
		}
	})

	t.Run("no organization performs zero REST calls and stores the fallback PAT", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())

		cmd := newCoreLoginCmd()

		withCoreStdin(t, "fallback-pat\n", func() {
			if err := runCoreLogin(context.Background(), cmd); err != nil {
				t.Fatalf("runCoreLogin: %v", err)
			}
		})

		if got := ado.GetPAT("https://dev.azure.com/anything"); got != "fallback-pat" {
			t.Errorf("GetPAT = %q, want fallback-pat (default key fallback)", got)
		}
	})
}

func TestRunCoreLogout(t *testing.T) {
	t.Run("nothing stored errors", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		cmd := newCoreLogoutCmd()
		err := runCoreLogout(cmd)
		if err == nil || err.Error() != "No credentials were found." {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("clears a specific org and its matching default, preserving the default project", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		org := "https://dev.azure.com/myorg"
		if err := ado.SetPAT(org, "p1"); err != nil {
			t.Fatal(err)
		}
		if err := ado.SetConfigDefaults(org, "myproj"); err != nil {
			t.Fatal(err)
		}

		cmd := newCoreLogoutCmd()
		_ = cmd.Flags().Set("organization", org)
		if err := runCoreLogout(cmd); err != nil {
			t.Fatalf("runCoreLogout: %v", err)
		}

		if got := ado.GetPAT(org); got != "" {
			t.Errorf("GetPAT after logout = %q, want empty", got)
		}
		gotOrg, gotProject, _ := ado.ConfigDefaults()
		if gotOrg != "" {
			t.Errorf("default org = %q, want cleared", gotOrg)
		}
		if gotProject != "myproj" {
			t.Errorf("default project = %q, want preserved", gotProject)
		}
	})

	t.Run("clearing an unknown specific org reports the credential-not-found wording", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		if err := ado.SetPAT("https://dev.azure.com/other", "p"); err != nil {
			t.Fatal(err)
		}

		cmd := newCoreLogoutCmd()
		_ = cmd.Flags().Set("organization", "https://dev.azure.com/missing")
		err := runCoreLogout(cmd)
		if err == nil || err.Error() != "The credential was not found" {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRunCoreConfigureDefaults(t *testing.T) {
	t.Run("no flags at all is a usage error", func(t *testing.T) {
		cmd := newCoreConfigureCmd()
		err := cmd.RunE(cmd, nil)
		want := "usage error: atleast one of the options must be specified.For list of supported options see help using -h flag."
		if err == nil || err.Error() != want {
			t.Fatalf("err = %v, want %q", err, want)
		}
	})

	t.Run("sets organization and project together", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		cmd := newCoreConfigureCmd()
		// Two --defaults tokens, matching a real "--defaults
		// organization=... project=..." space-separated call (StringArray
		// accumulates one value per Set/flag occurrence; configure.py's
		// --defaults takes each token verbatim, no comma-splitting).
		if err := cmd.Flags().Set("defaults", "organization=https://dev.azure.com/myorg"); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Flags().Set("defaults", "project=myproj"); err != nil {
			t.Fatal(err)
		}
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("configure: %v", err)
		}
		org, project, _ := ado.ConfigDefaults()
		if org != "https://dev.azure.com/myorg" || project != "myproj" {
			t.Errorf("org=%q project=%q", org, project)
		}
	})

	// TestRunCoreConfigureDefaults/nargs-star-space-form (below) guards M3:
	// a single "--defaults organization=X project=Y" call only lets pflag
	// consume "organization=X" as --defaults's value, leaving "project=Y" a
	// stray positional — RunE must fold it back in.
	t.Run("nargs-star-space-form: a single flag occurrence plus a leftover positional sets both", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		cmd := newCoreConfigureCmd()
		if err := cmd.Flags().Set("defaults", "organization=https://dev.azure.com/myorg"); err != nil {
			t.Fatal(err)
		}
		if err := cmd.RunE(cmd, []string{"project=myproj"}); err != nil {
			t.Fatalf("configure: %v", err)
		}
		org, project, _ := ado.ConfigDefaults()
		if org != "https://dev.azure.com/myorg" || project != "myproj" {
			t.Errorf("org=%q project=%q", org, project)
		}
	})

	t.Run("invalid key errors verbatim", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		cmd := newCoreConfigureCmd()
		_ = cmd.Flags().Set("defaults", "badkey=x")
		err := cmd.RunE(cmd, nil)
		want := "usage error: invalid default value setup. Supported values are ['organization', 'project']."
		if err == nil || err.Error() != want {
			t.Fatalf("err = %v, want %q", err, want)
		}
	})

	t.Run("an earlier valid pair persists even when a later pair in the same call fails", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		cmd := newCoreConfigureCmd()
		_ = cmd.Flags().Set("defaults", "project=myproj")
		_ = cmd.Flags().Set("defaults", "badkey=x")
		if err := cmd.RunE(cmd, nil); err == nil {
			t.Fatal("expected an error from the second pair")
		}
		_, project, _ := ado.ConfigDefaults()
		if project != "myproj" {
			t.Errorf("project = %q, want myproj (first pair should have already been written)", project)
		}
	})
}

func TestCoreInvokeRoute(t *testing.T) {
	tests := []struct {
		name      string
		route     []string
		wantScope string
		wantPath  string
	}{
		{
			name:      "project route parameter becomes scope",
			route:     []string{"project=MyProj"},
			wantScope: "MyProj",
			wantPath:  "wiki/wikis",
		},
		{
			name:      "non-project route parameters append as path segments in order",
			route:     []string{"project=MyProj", "wikiIdentifier=abc", "pageId=42"},
			wantScope: "MyProj",
			wantPath:  "wiki/wikis/abc/42",
		},
		{
			name:      "no route parameters at all",
			route:     nil,
			wantScope: "",
			wantPath:  "wiki/wikis",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv, err := coreParseKV(tt.route)
			if err != nil {
				t.Fatal(err)
			}
			scope, path := coreInvokeRoute("wiki", "wikis", kv)
			if scope != tt.wantScope || path != tt.wantPath {
				t.Errorf("coreInvokeRoute() = (%q, %q), want (%q, %q)", scope, path, tt.wantScope, tt.wantPath)
			}
		})
	}
}

func TestCoreParseKV(t *testing.T) {
	if _, err := coreParseKV([]string{"noequalssign"}); err == nil {
		t.Fatal("expected an error for a token without '='")
	} else if err.Error() != "noequalssign is not valid it needs to be in format param=value" {
		t.Errorf("err = %v", err)
	}
}

// TestCoreAppendNargsStar guards M2: pflag only ever consumes one token per
// "--route-parameters"/"--query-parameters" occurrence, so a space-separated
// "--route-parameters project=X wikiIdentifier=Y" leaves "wikiIdentifier=Y"
// as a leftover positional (arguments.py:91-94 registers nargs='*').
func TestCoreAppendNargsStar(t *testing.T) {
	t.Run("leftovers fold into route-parameters when only it was given", func(t *testing.T) {
		route, query := coreAppendNargsStar([]string{"project=X"}, nil, []string{"wikiIdentifier=Y"})
		if len(route) != 2 || route[0] != "project=X" || route[1] != "wikiIdentifier=Y" {
			t.Errorf("route = %v", route)
		}
		if len(query) != 0 {
			t.Errorf("query = %v, want empty", query)
		}
	})

	t.Run("leftovers fold into query-parameters when only it was given", func(t *testing.T) {
		route, query := coreAppendNargsStar(nil, []string{"$top=5"}, []string{"continuationToken=abc"})
		if len(route) != 0 {
			t.Errorf("route = %v, want empty", route)
		}
		if len(query) != 2 || query[0] != "$top=5" || query[1] != "continuationToken=abc" {
			t.Errorf("query = %v", query)
		}
	})

	t.Run("no leftovers is a no-op", func(t *testing.T) {
		route, query := coreAppendNargsStar([]string{"project=X"}, []string{"a=1"}, nil)
		if len(route) != 1 || len(query) != 1 {
			t.Errorf("route=%v query=%v", route, query)
		}
	})
}

// TestCoreNormalizeEnum guards m6: arguments.py:95 registers http_method via
// get_enum_type(), whose CaseInsensitiveList choices match
// case-insensitively and normalize to the canonical-cased value.
func TestCoreNormalizeEnum(t *testing.T) {
	got, ok := coreNormalizeEnum(coreHTTPMethods, "post")
	if !ok || got != "POST" {
		t.Errorf("coreNormalizeEnum(..., \"post\") = (%q, %v), want (\"POST\", true)", got, ok)
	}
	if _, ok := coreNormalizeEnum(coreHTTPMethods, "bogus"); ok {
		t.Error("expected ok=false for an unknown method")
	}
}

// TestInvokeRequestConstruction exercises the actual routing pieces
// runCoreInvoke drives (coreParseKV + coreInvokeRoute feeding an
// ado.Request) against a real httptest server, proving the URL, method and
// query string are assembled correctly. It builds the Client directly
// (ado.NewClient) rather than going through ado.Resolve/cmd.RunE, because
// Resolve's org validation requires a dev.azure.com/visualstudio.com-shaped
// URL that an httptest server can't provide — the same constraint
// ado's own client_test.go works around.
func TestInvokeRequestConstruction(t *testing.T) {
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	var gotMethod, gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "wiki-1"})
	}))
	defer srv.Close()

	routeParams, err := coreParseKV([]string{"project=MyProj", "wikiIdentifier=abc"})
	if err != nil {
		t.Fatal(err)
	}
	queryParams, err := coreParseKV([]string{"includeContent=true"})
	if err != nil {
		t.Fatal(err)
	}
	scope, path := coreInvokeRoute("wiki", "wikis", routeParams)

	q := url.Values{}
	for _, kv := range queryParams {
		q.Set(kv.Key, kv.Value)
	}

	client, err := ado.NewClient(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	result := map[string]any{}
	if err := client.Do(context.Background(), ado.Request{
		Method:     http.MethodGet,
		Scope:      scope,
		Path:       path,
		APIVersion: "5.0",
		Query:      q,
	}, &result); err != nil {
		t.Fatalf("Do: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	wantURL := "/MyProj/_apis/wiki/wikis/abc?api-version=5.0&includeContent=true"
	if gotURL != wantURL {
		t.Errorf("url = %q, want %q", gotURL, wantURL)
	}
	if result["id"] != "wiki-1" {
		t.Errorf("result = %v, want id=wiki-1", result)
	}
}

func TestCoreDecodeUTF16(t *testing.T) {
	// "hi" in UTF-16LE and UTF-16BE.
	le := []byte{'h', 0, 'i', 0}
	be := []byte{0, 'h', 0, 'i'}

	gotLE, err := coreDecodeUTF16(le, false)
	if err != nil || string(gotLE) != "hi" {
		t.Errorf("utf-16le: got %q, err %v", gotLE, err)
	}
	gotBE, err := coreDecodeUTF16(be, true)
	if err != nil || string(gotBE) != "hi" {
		t.Errorf("utf-16be: got %q, err %v", gotBE, err)
	}
	if _, err := coreDecodeUTF16([]byte{1, 2, 3}, false); err == nil {
		t.Error("expected an error for odd byte length")
	}
}

func TestCoreParseThreeState(t *testing.T) {
	tests := []struct {
		in      string
		want    bool
		wantErr bool
	}{
		{"", true, false},
		{"true", true, false},
		{"TRUE", true, false},
		{"false", false, false},
		{"nope", false, true},
	}
	for _, tt := range tests {
		got, err := coreParseThreeState(tt.in)
		if (err != nil) != tt.wantErr || (err == nil && got != tt.want) {
			t.Errorf("coreParseThreeState(%q) = (%v, %v), want (%v, err=%v)", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

// TestCoreGitAliasesConfigured_GitMissingIsCLIError guards X-08: git.py:186-187
// re-raises an OSError (binary missing) as a CLIError, distinct from a
// CalledProcessError (git installed, alias just unset) which means "No".
// configure --list silently printed "Use git alias = No" for both.
func TestCoreGitAliasesConfigured_GitMissingIsCLIError(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // a PATH with no "git" on it anywhere

	_, err := coreGitAliasesConfigured()
	want := "Checking the git config values failed. Ensure git is installed and in your path."
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestCoreLoginBadPATStatusIsFailure(t *testing.T) {
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cmd := newCoreLoginCmd()
	_ = cmd.Flags().Set("organization", srv.URL)

	var err error
	withCoreStdin(t, "bad-pat\n", func() {
		err = runCoreLogin(context.Background(), cmd)
	})

	if err == nil || !strings.Contains(err.Error(), "Failed to authenticate using the supplied token.") {
		t.Fatalf("err = %v", err)
	}
}
