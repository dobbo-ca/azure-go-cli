package boards

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// queryTestClient builds an *ado.Client against srv with a hermetic,
// network-free auth path (a fake PAT via env var; AZ_SESSION points the
// foundation's config.Load() at a profile file that can't exist, so the AAD
// attempt fails fast with no network call).
func queryTestClient(t *testing.T, srv *httptest.Server) *ado.Client {
	t.Helper()
	t.Setenv("AZ_SESSION", "boards-query-test-"+t.Name())
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	client, err := ado.NewClient(context.Background(), srv.URL+"/testorg")
	if err != nil {
		t.Fatalf("ado.NewClient: %v", err)
	}
	return client
}

func queryTestCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	return cmd
}

// queryCaptureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written to it, per pkg/output/output_test.go's captureStdout.
func queryCaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	w.Close()
	b, _ := io.ReadAll(r)
	return string(b)
}

// TestQueryQuery_ByWiql exercises the --wiql path: POST wit/wiql with the
// {"query": ...} body, then a single hydration GET whose ids/fields come
// from the query result, and asserts the hydrated items come back re-sorted
// into the original query order even though the server returns them
// reversed.
func TestQueryQuery_ByWiql(t *testing.T) {
	var calls []string
	var gotWiqlBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/wit/wiql"):
			b := map[string]string{}
			_ = json.NewDecoder(r.Body).Decode(&b)
			gotWiqlBody = b
			fmt.Fprint(w, `{"asOf":"2020-01-01T00:00:00Z","columns":[{"name":"ID","referenceName":"System.Id"},{"name":"Title","referenceName":"System.Title"}],"workItems":[{"id":1},{"id":2}]}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/wit/workitems"):
			if got := r.URL.Query().Get("ids"); got != "1,2" {
				t.Errorf("hydration ids = %q, want 1,2", got)
			}
			if got := r.URL.Query().Get("fields"); got != "System.Id,System.Title" {
				t.Errorf("hydration fields = %q, want System.Id,System.Title", got)
			}
			// server returns them out of order -- the client must re-sort.
			fmt.Fprint(w, `{"count":2,"value":[{"id":2,"fields":{"System.Title":"second"}},{"id":1,"fields":{"System.Title":"first"}}]}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL)
		}
	}))
	defer srv.Close()

	client := queryTestClient(t, srv)
	cmd := queryTestCmd()
	dctx := ado.Context{Org: srv.URL + "/testorg"}

	var runErr error
	stdout := queryCaptureStdout(t, func() {
		runErr = queryQuery(context.Background(), cmd, client, dctx, "select * from workitems", "", "")
	})
	if runErr != nil {
		t.Fatalf("queryQuery: %v", runErr)
	}

	var printed []map[string]any
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		t.Fatalf("unmarshal printed output %q: %v", stdout, err)
	}
	if len(printed) != 2 {
		t.Fatalf("printed %d items, want 2", len(printed))
	}
	// server answered id 2 before id 1; the query result said 1 then 2 --
	// the printed order must follow the query result, not the server's
	// hydration response order.
	if printed[0]["id"].(float64) != 1 || printed[1]["id"].(float64) != 2 {
		t.Errorf("printed order = %v, want [id=1, id=2] (original query order)", printed)
	}

	if gotWiqlBody["query"] != "select * from workitems" {
		t.Errorf("wiql body query = %q, want %q", gotWiqlBody["query"], "select * from workitems")
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v, want 2 requests", calls)
	}
	if !strings.HasPrefix(calls[0], "POST ") || !strings.Contains(calls[0], "/testorg/_apis/wit/wiql?") {
		t.Errorf("first call = %q, want POST to org-scoped wit/wiql", calls[0])
	}
}

// TestQueryQuery_ByID skips the wiql POST entirely and goes straight to
// GET wit/wiql/{id}.
func TestQueryQuery_ByID(t *testing.T) {
	var calls []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/wit/wiql/my-query-id"):
			fmt.Fprint(w, `{"asOf":"","columns":[],"workItems":[]}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL)
		}
	}))
	defer srv.Close()

	client := queryTestClient(t, srv)
	cmd := queryTestCmd()
	dctx := ado.Context{Org: srv.URL + "/testorg"}

	if err := queryQuery(context.Background(), cmd, client, dctx, "", "my-query-id", ""); err != nil {
		t.Fatalf("queryQuery: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %v, want exactly 1 request (no hydration for zero work items)", calls)
	}
	if calls[0] != "GET /testorg/_apis/wit/wiql/my-query-id" {
		t.Errorf("call = %q, want GET .../wit/wiql/my-query-id", calls[0])
	}
}

// TestQueryQuery_ByPath resolves the saved-query path against the
// project-scoped wit/queries endpoint first, then runs the resolved id
// against the org-scoped (no project segment) wiql endpoint -- project is
// never passed to query_by_id/query_by_wiql in Python.
func TestQueryQuery_ByPath(t *testing.T) {
	var calls []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		// r.URL.Path is percent-decoded by net/http (net/url), so the
		// query path segments come through with spaces, not "%20" -- the
		// %-encoding only exists on the wire (see r.URL.String() in the
		// other tests' "unexpected request" fallthrough).
		case strings.Contains(r.URL.Path, "/testorg/MyProj/_apis/wit/queries/Shared Queries/My Query"):
			fmt.Fprint(w, `{"id":"resolved-id"}`)
		case strings.Contains(r.URL.Path, "/testorg/_apis/wit/wiql/resolved-id"):
			fmt.Fprint(w, `{"asOf":"","columns":[],"workItems":[]}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL)
		}
	}))
	defer srv.Close()

	client := queryTestClient(t, srv)
	cmd := queryTestCmd()
	dctx := ado.Context{Org: srv.URL + "/testorg", Project: "MyProj"}

	if err := queryQuery(context.Background(), cmd, client, dctx, "", "", "Shared Queries/My Query"); err != nil {
		t.Fatalf("queryQuery: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v, want 2 requests", calls)
	}
	if !strings.Contains(calls[0], "/testorg/MyProj/_apis/wit/queries/") {
		t.Errorf("first call = %q, want project-scoped wit/queries", calls[0])
	}
	if calls[1] != "GET /testorg/_apis/wit/wiql/resolved-id" {
		t.Errorf("second call = %q, want org-scoped (no project) wit/wiql/resolved-id", calls[1])
	}
}

// TestQueryQuery_PathWithoutProject reproduces work_item.py:257-258's
// project check: --path without --project is a CLIError before any request
// is sent.
func TestQueryQuery_PathWithoutProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request %s %s", r.Method, r.URL)
	}))
	defer srv.Close()

	client := queryTestClient(t, srv)
	cmd := queryTestCmd()
	dctx := ado.Context{Org: srv.URL + "/testorg"}

	err := queryQuery(context.Background(), cmd, client, dctx, "", "", "Shared Queries/My Query")
	if err == nil || !strings.Contains(err.Error(), "--project argument must be specified") {
		t.Fatalf("err = %v, want a --project-required error", err)
	}
}

// TestQueryComputeBatches asserts the client-side batching mirrors Python's
// boundary rules: a small URL-length budget splits well under 200 ids, and
// the 1000-item hard cap drops the remainder.
func TestQueryComputeBatches(t *testing.T) {
	t.Run("batch size cap", func(t *testing.T) {
		ids := make([]int, 250)
		for i := range ids {
			ids[i] = i + 1
		}
		// a short org keeps the URL-length budget generous, so only the
		// 200-id batch size should bind.
		batches := queryComputeBatches("https://dev.azure.com/o", 0, ids)
		if len(batches) != 2 {
			t.Fatalf("got %d batches, want 2", len(batches))
		}
		if len(batches[0]) != 200 {
			t.Errorf("batches[0] len = %d, want 200", len(batches[0]))
		}
		if len(batches[1]) != 50 {
			t.Errorf("batches[1] len = %d, want 50", len(batches[1]))
		}
	})

	t.Run("1000 item hard cap", func(t *testing.T) {
		ids := make([]int, 1200)
		for i := range ids {
			ids[i] = i + 1
		}
		batches := queryComputeBatches("https://dev.azure.com/o", 0, ids)
		total := 0
		for _, b := range batches {
			total += len(b)
		}
		if total != 1000 {
			t.Errorf("total ids batched = %d, want 1000", total)
		}
	})

	t.Run("tiny url budget forces single-id batches", func(t *testing.T) {
		// A very long "organization" value drives the remaining budget
		// negative immediately, so every id closes its own batch.
		longOrg := strings.Repeat("x", 2000)
		batches := queryComputeBatches(longOrg, 0, []int{1, 2, 3})
		if len(batches) != 3 {
			t.Fatalf("got %d batches, want 3 (one id each)", len(batches))
		}
	})
}

// TestQueryCellValue covers the three special-cased columns plus the
// falsy-zero quirk (_format.py:93-119).
func TestQueryCellValue(t *testing.T) {
	tests := []struct {
		name  string
		field queryFieldRef
		row   map[string]any
		want  string
	}{
		{
			name:  "missing field renders a blank marker",
			field: queryFieldRef{Name: "State", ReferenceName: "System.State"},
			row:   map[string]any{"fields": map[string]any{}},
			want:  " ",
		},
		{
			name:  "numeric zero renders as literal 0",
			field: queryFieldRef{Name: "Remaining Work", ReferenceName: "Microsoft.VSTS.Scheduling.RemainingWork"},
			row:   map[string]any{"fields": map[string]any{"Microsoft.VSTS.Scheduling.RemainingWork": float64(0)}},
			want:  "0",
		},
		{
			name:  "title truncates at 70 chars",
			field: queryFieldRef{Name: "Title", ReferenceName: "System.Title"},
			row:   map[string]any{"fields": map[string]any{"System.Title": strings.Repeat("a", 80)}},
			want:  strings.Repeat("a", 67) + "...",
		},
		{
			name:  "assigned to unwraps to uniqueName",
			field: queryFieldRef{Name: "Assigned To", ReferenceName: "System.AssignedTo"},
			row:   map[string]any{"fields": map[string]any{"System.AssignedTo": map[string]any{"uniqueName": "fabrikam@example.com", "displayName": "Fabrikam"}}},
			want:  "fabrikam@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := queryCellValue(tt.field)(tt.row)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
