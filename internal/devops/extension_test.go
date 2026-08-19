package devops

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// extensionTestClient builds an *ado.Client whose HTTP transport dials srv
// regardless of the host in the request URL — needed because these commands
// set Request.Host: "extmgmt", which ado.Client rewrites to a subdomain
// (e.g. "extmgmt.dev.azure.com") that would not otherwise resolve to the
// httptest server.
func extensionTestClient(srv *httptest.Server) *ado.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return net.Dial(network, srv.Listener.Addr().String())
		},
	}
	return &ado.Client{Org: "http://dev.azure.com/myorg", HTTP: &http.Client{Transport: transport}}
}

// withExtensionTestClient substitutes the extensionNewClient seam (see
// extension.go) so a command's real ado.NewClient/auth call is replaced with
// a client pointed at srv.
func withExtensionTestClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := extensionNewClient
	extensionNewClient = func(ctx context.Context, org string) (*ado.Client, error) {
		return extensionTestClient(srv), nil
	}
	t.Cleanup(func() { extensionNewClient = orig })
}

func extensionCaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestExtensionSearch_RequestBody(t *testing.T) {
	var gotMethod, gotContentType, gotAccept string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"extensions":[{"publisher":{"publisherName":"pub1"},"extensionName":"ext1","displayName":"Ext One"}]}]}`))
	}))
	defer srv.Close()

	origURL := extensionMarketplaceURL
	extensionMarketplaceURL = srv.URL
	defer func() { extensionMarketplaceURL = origURL }()

	cmd := newExtensionSearchCmd()
	out := extensionCaptureStdout(t, func() {
		if err := runExtensionSearch(context.Background(), cmd, "myterm"); err != nil {
			t.Fatalf("runExtensionSearch: %v", err)
		}
	})

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotContentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	if gotAccept != "application/json;api-version=5.0-preview.1" {
		t.Errorf("Accept = %q", gotAccept)
	}

	filters, _ := gotBody["filters"].([]any)
	if len(filters) != 1 {
		t.Fatalf("filters = %v, want 1 entry", filters)
	}
	filter := filters[0].(map[string]any)
	if filter["pageSize"] != float64(50) || filter["pageNumber"] != float64(1) {
		t.Errorf("filter paging = %v", filter)
	}
	criteria, _ := filter["criteria"].([]any)
	if len(criteria) != 9 {
		t.Fatalf("criteria = %d entries, want 9", len(criteria))
	}
	searchCriterion := criteria[7].(map[string]any)
	if searchCriterion["filterType"] != float64(10) || searchCriterion["value"] != "myterm" {
		t.Errorf("search criterion = %v, want filterType 10 value \"myterm\"", searchCriterion)
	}
	targetCriterion := criteria[8].(map[string]any)
	if targetCriterion["filterType"] != float64(12) || targetCriterion["value"] != "37888" {
		t.Errorf("target criterion = %v, want filterType 12 value \"37888\"", targetCriterion)
	}

	if !strings.Contains(out, "Ext One") {
		t.Errorf("stdout = %q, want it to contain the extension row", out)
	}
}

func TestExtensionSearch_EmptyResultsDoesNotPanic(t *testing.T) {
	// Python indexes results[0] unconditionally and crashes with an
	// IndexError here; the port fixes that (see extension_search.go).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	origURL := extensionMarketplaceURL
	extensionMarketplaceURL = srv.URL
	defer func() { extensionMarketplaceURL = origURL }()

	cmd := newExtensionSearchCmd()
	if err := runExtensionSearch(context.Background(), cmd, "x"); err != nil {
		t.Fatalf("runExtensionSearch: %v", err)
	}
}

func TestExtensionList_URLAndClientSideFilter(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":2,"value":[
			{"extensionId":"e1","extensionName":"Ext One","flags":"eventCallbacksBypassed"},
			{"extensionId":"e2","extensionName":"Ext Two","flags":"builtIn, trusted"}
		]}`))
	}))
	defer srv.Close()
	withExtensionTestClient(t, srv)

	cmd := newExtensionListCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("include-built-in", "false")

	out := extensionCaptureStdout(t, func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}
	})

	wantPath := "/myorg/_apis/extensionmanagement/installedextensions"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if !strings.Contains(gotQuery, "includeDisabledExtensions=true") {
		t.Errorf("query = %q, want includeDisabledExtensions=true (unset --include-disabled defaults true)", gotQuery)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0]["extensionId"] != "e1" {
		t.Errorf("rows = %v, want only e1 (e2 has flags containing builtIn and --include-built-in=false)", rows)
	}
}

// TestExtensionList_IncludeBuiltInSpaceForm guards B6: a space-separated
// "--include-built-in false" leaves "false" as a stray positional (pflag
// has no lookahead for a string flag's value) that RunE must fold back in.
func TestExtensionList_IncludeBuiltInSpaceForm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":2,"value":[
			{"extensionId":"e1","extensionName":"Ext One","flags":"eventCallbacksBypassed"},
			{"extensionId":"e2","extensionName":"Ext Two","flags":"builtIn, trusted"}
		]}`))
	}))
	defer srv.Close()
	withExtensionTestClient(t, srv)

	cmd := newExtensionListCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	// Bare "--include-built-in" binds to NoOptDefVal "true"; the
	// space-separated "false" is what RunE must pick up from args.
	cmd.Flags().Set("include-built-in", "true")

	out := extensionCaptureStdout(t, func() {
		if err := cmd.RunE(cmd, []string{"false"}); err != nil {
			t.Fatalf("RunE: %v", err)
		}
	})

	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0]["extensionId"] != "e1" {
		t.Errorf("rows = %v, want only e1 (e2 excluded once --include-built-in resolves to false)", rows)
	}
}

func TestExtensionShowInstallUninstall_URLMethod(t *testing.T) {
	tests := []struct {
		name       string
		newCmd     func() *cobra.Command
		wantMethod string
	}{
		{"show", newExtensionShowCmd, http.MethodGet},
		{"install", newExtensionInstallCmd, http.MethodPost},
		{"uninstall", newExtensionUninstallCmd, http.MethodDelete},
	}

	wantPath := "/myorg/_apis/extensionmanagement/installedextensionsbyname/pub1/ext1"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"extensionId":"e1","publisherId":"pub1"}`))
			}))
			defer srv.Close()
			withExtensionTestClient(t, srv)

			cmd := tt.newCmd()
			cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
			cmd.Flags().Set("detect", "false")
			cmd.Flags().Set("publisher-id", "pub1")
			cmd.Flags().Set("extension-id", "ext1")
			if yes := cmd.Flags().Lookup("yes"); yes != nil {
				cmd.Flags().Set("yes", "true")
			}

			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if gotMethod != tt.wantMethod {
				t.Errorf("method = %q, want %q", gotMethod, tt.wantMethod)
			}
			if gotPath != wantPath {
				t.Errorf("path = %q, want %q", gotPath, wantPath)
			}
		})
	}
}

func TestExtensionEnableDisable_TwoStepSequence(t *testing.T) {
	t.Run("enable success", func(t *testing.T) {
		var calls []string
		var patchBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, r.Method+" "+r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodGet:
				w.Write([]byte(`{"extensionId":"e1","installState":{"flags":"disabled, eventCallbacksBypassed"}}`))
			case http.MethodPatch:
				_ = json.NewDecoder(r.Body).Decode(&patchBody)
				w.Write([]byte(`{"extensionId":"e1","installState":{"flags":"eventCallbacksBypassed"}}`))
			}
		}))
		defer srv.Close()
		withExtensionTestClient(t, srv)

		cmd := newExtensionEnableCmd()
		cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
		cmd.Flags().Set("detect", "false")
		cmd.Flags().Set("publisher-id", "pub1")
		cmd.Flags().Set("extension-id", "ext1")

		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}

		wantCalls := []string{
			"GET /myorg/_apis/extensionmanagement/installedextensionsbyname/pub1/ext1",
			"PATCH /myorg/_apis/extensionmanagement/installedextensions",
		}
		if len(calls) != 2 || calls[0] != wantCalls[0] || calls[1] != wantCalls[1] {
			t.Errorf("calls = %v, want %v", calls, wantCalls)
		}

		state, _ := patchBody["installState"].(map[string]any)
		if state["flags"] != "eventCallbacksBypassed" {
			t.Errorf("PATCH installState.flags = %v, want %q", state["flags"], "eventCallbacksBypassed")
		}
	})

	t.Run("enable already-enabled errors without a PATCH", func(t *testing.T) {
		patchCalled := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPatch {
				patchCalled = true
			}
			w.Write([]byte(`{"extensionId":"e1","installState":{"flags":"eventCallbacksBypassed"}}`))
		}))
		defer srv.Close()
		withExtensionTestClient(t, srv)

		cmd := newExtensionEnableCmd()
		cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
		cmd.Flags().Set("detect", "false")
		cmd.Flags().Set("publisher-id", "pub1")
		cmd.Flags().Set("extension-id", "ext1")

		err := cmd.RunE(cmd, nil)
		if err == nil || err.Error() != "Extension is already in enabled state" {
			t.Errorf("err = %v, want \"Extension is already in enabled state\"", err)
		}
		if patchCalled {
			t.Errorf("PATCH must not be sent when the extension is already enabled")
		}
	})

	t.Run("disable success appends the disabled token", func(t *testing.T) {
		var patchBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodGet:
				w.Write([]byte(`{"extensionId":"e1","installState":{"flags":"eventCallbacksBypassed"}}`))
			case http.MethodPatch:
				_ = json.NewDecoder(r.Body).Decode(&patchBody)
				w.Write([]byte(`{"extensionId":"e1","installState":{"flags":"eventCallbacksBypassed, disabled"}}`))
			}
		}))
		defer srv.Close()
		withExtensionTestClient(t, srv)

		cmd := newExtensionDisableCmd()
		cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
		cmd.Flags().Set("detect", "false")
		cmd.Flags().Set("publisher-id", "pub1")
		cmd.Flags().Set("extension-id", "ext1")

		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}

		state, _ := patchBody["installState"].(map[string]any)
		if state["flags"] != "eventCallbacksBypassed, disabled" {
			t.Errorf("PATCH installState.flags = %v, want %q", state["flags"], "eventCallbacksBypassed, disabled")
		}
	})
}

// TestExtensionTrim_RuneTruncation guards against byte-slicing a multi-byte
// value: Python's trim_for_display (dev/common/format.py:11-22) slices code
// points -- text[0:max_length] + '...' -- and must survive non-ASCII input
// without splitting a rune into mojibake.
func TestExtensionTrim_RuneTruncation(t *testing.T) {
	text := strings.Repeat("路", 10) // 10 code points, 3 bytes each
	got := extensionTrim(text, 7)

	if !utf8.ValidString(got) {
		t.Fatalf("got %q, not valid UTF-8", got)
	}
	want := string([]rune(text)[:7]) + "..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if n := utf8.RuneCountInString(got); n != 10 {
		t.Errorf("rune count = %d, want 10 (Python: text[0:7] + '...')", n)
	}
}
