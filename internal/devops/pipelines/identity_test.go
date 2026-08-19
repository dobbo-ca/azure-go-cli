package pipelines

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// identityTestClient builds a real *ado.Client hermetically, then redirects
// every request (regardless of Host override, e.g. vssps) to srv - same
// tradeoff as repos/policy_test.go's policyTestADOClient, needed because
// httptest.Server can't bind an alternate hostname.
func identityTestClient(t *testing.T, srv *httptest.Server) *ado.Client {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	client, err := ado.NewClient(context.Background(), "https://dev.azure.com/myorg")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.HTTP.Transport = identityRedirectTransport{target: srv.Listener.Addr().String(), next: http.DefaultTransport}
	return client
}

type identityRedirectTransport struct {
	target string
	next   http.RoundTripper
}

func (rt identityRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = rt.target
	return rt.next.RoundTrip(req)
}

// TestPipelinesResolveIdentityID covers build.py:122 / pipeline_run.py:75's
// resolve_identity_as_id: empty passes through, a UUID short-circuits, "me"
// resolves via ConnectionData, anything else via the vssps Identities
// lookup. Without this resolution, `--requested-for me`/email/alias values
// were sent to the server unresolved (build_list.go/runs_list.go used to
// set requestedFor to the raw flag value).
func TestPipelinesResolveIdentityID(t *testing.T) {
	t.Run("empty passes through without any HTTP call", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
		defer srv.Close()
		client := identityTestClient(t, srv)

		id, err := pipelinesResolveIdentityID(context.Background(), client, "")
		if err != nil || id != "" {
			t.Fatalf("id=%q err=%v, want empty/nil", id, err)
		}
		if called {
			t.Error("server should not have been called for an empty filter")
		}
	})

	t.Run("UUID input short-circuits without any HTTP call", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
		defer srv.Close()
		client := identityTestClient(t, srv)

		id, err := pipelinesResolveIdentityID(context.Background(), client, "e556f204-53c9-4153-9cd9-ef41a11e3345")
		if err != nil {
			t.Fatalf("pipelinesResolveIdentityID: %v", err)
		}
		if id != "e556f204-53c9-4153-9cd9-ef41a11e3345" {
			t.Errorf("id = %q, want the input echoed back", id)
		}
		if called {
			t.Error("server should not have been called for a UUID input")
		}
	})

	t.Run(`"me" resolves via ConnectionData`, func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"authenticatedUser":{"id":"current-user-id"}}`))
		}))
		defer srv.Close()
		client := identityTestClient(t, srv)

		id, err := pipelinesResolveIdentityID(context.Background(), client, "me")
		if err != nil {
			t.Fatalf("pipelinesResolveIdentityID: %v", err)
		}
		if id != "current-user-id" {
			t.Errorf("id = %q, want current-user-id", id)
		}
		if gotPath != "/myorg/_apis/ConnectionData" {
			t.Errorf("path = %q, want .../_apis/ConnectionData", gotPath)
		}
	})

	t.Run("email/alias resolves via vssps Identities lookup", func(t *testing.T) {
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
		client := identityTestClient(t, srv)

		id, err := pipelinesResolveIdentityID(context.Background(), client, "alice@contoso.com")
		if err != nil {
			t.Fatalf("pipelinesResolveIdentityID: %v", err)
		}
		if id != "identity-1" {
			t.Errorf("id = %q, want identity-1", id)
		}
		if len(searchFilters) != 1 || searchFilters[0] != "General" {
			t.Errorf("searchFilters = %v, want [General] (found on first try)", searchFilters)
		}
	})

	t.Run("no match errors", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"count":0,"value":[]}`))
		}))
		defer srv.Close()
		client := identityTestClient(t, srv)

		_, err := pipelinesResolveIdentityID(context.Background(), client, "nobody@contoso.com")
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})

	// identities.py:60: `identity_filter.find(' ') > 0 or identity_filter.find('@') > 0`
	// — a match at index 0 does NOT count, so a filter starting with "@"
	// must still try DirectoryAlias first. strings.Contains would wrongly
	// flip to General-first for this input.
	t.Run(`filter starting with "@" tries DirectoryAlias first (index-0 match doesn't count)`, func(t *testing.T) {
		var searchFilters []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sf := r.URL.Query().Get("searchFilter")
			searchFilters = append(searchFilters, sf)
			w.Header().Set("Content-Type", "application/json")
			if sf == "DirectoryAlias" {
				w.Write([]byte(`{"count":1,"value":[{"id":"identity-1"}]}`))
				return
			}
			w.Write([]byte(`{"count":0,"value":[]}`))
		}))
		defer srv.Close()
		client := identityTestClient(t, srv)

		id, err := pipelinesResolveIdentityID(context.Background(), client, "@contoso")
		if err != nil {
			t.Fatalf("pipelinesResolveIdentityID: %v", err)
		}
		if id != "identity-1" {
			t.Errorf("id = %q, want identity-1", id)
		}
		if len(searchFilters) != 1 || searchFilters[0] != "DirectoryAlias" {
			t.Errorf("searchFilters = %v, want [DirectoryAlias] (found on first try)", searchFilters)
		}
	})
}
