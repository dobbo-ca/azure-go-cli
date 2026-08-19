package repos

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// refMustParseQuery parses a raw query string, failing the test on error.
func refMustParseQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	v, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", raw, err)
	}
	return v
}

// refTestClient builds a real ado.Client pointed at srv, bypassing
// ado.ResolveProject's org validation (httptest URLs are http://127.0.0.1,
// which validateOrg rejects) — the same seam ado's own client_test.go uses.
func refTestClient(t *testing.T, srv *httptest.Server) (*ado.Client, ado.Context) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	org := srv.URL + "/myorg"
	client, err := ado.NewClient(context.Background(), org)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, ado.Context{Org: org, Project: "MyProj", Repo: "myrepo"}
}

// refTestCmd builds cmdFn's cobra.Command and parses args into its flags
// (no Execute — RunE, and therefore ado.ResolveProject, never runs).
func refTestCmd(t *testing.T, cmdFn func() *cobra.Command, args ...string) *cobra.Command {
	t.Helper()
	cmd := cmdFn()
	if err := cmd.Flags().Parse(args); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return cmd
}

// TestRefCreateBody verifies the exact POST body shape (array, refs/-prefixed
// name, zero-sentinel oldObjectId, explicit isLocked:false) and that a
// server-signalled logical failure (success:false) surfaces as an error even
// though the HTTP status is 200 — the trickiest behaviour in create_ref.
func TestRefCreateBody(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Path != "/myorg/MyProj/_apis/git/repositories/myrepo/refs" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"count":1,"value":[{"name":"refs/heads/my_branch","oldObjectId":"0000000000000000000000000000000000000000","newObjectId":"abc123","isLocked":false,"updateStatus":"succeeded","success":true}]}`))
	}))
	t.Cleanup(srv.Close)
	client, dctx := refTestClient(t, srv)
	cmd := refTestCmd(t, newRefCreateCmd, "--name", "heads/my_branch", "--object-id", "abc123")

	if err := refCreateExec(context.Background(), cmd, client, dctx); err != nil {
		t.Fatalf("refCreateExec: %v", err)
	}

	var body []map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("body is not a JSON array: %v (%s)", err, capturedBody)
	}
	if len(body) != 1 {
		t.Fatalf("want 1 ref update, got %d", len(body))
	}
	got := body[0]
	if got["name"] != "refs/heads/my_branch" {
		t.Errorf("name = %v, want refs/heads/my_branch (resolve_git_refs must prefix refs/)", got["name"])
	}
	if got["newObjectId"] != "abc123" {
		t.Errorf("newObjectId = %v, want abc123", got["newObjectId"])
	}
	if got["oldObjectId"] != refZeroObjectID {
		t.Errorf("oldObjectId = %v, want the zero-SHA sentinel", got["oldObjectId"])
	}
	if locked, ok := got["isLocked"]; !ok || locked != false {
		t.Errorf("isLocked = %v, want explicit false", got["isLocked"])
	}
}

func TestRefCreateLogicalFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200) // the bulk-refs endpoint answers 200 even on logical failure
		_, _ = w.Write([]byte(`{"count":1,"value":[{"name":"refs/heads/my_branch","success":false,"customMessage":"ref already exists"}]}`))
	}))
	t.Cleanup(srv.Close)
	client, dctx := refTestClient(t, srv)
	cmd := refTestCmd(t, newRefCreateCmd, "--name", "heads/my_branch", "--object-id", "abc123")

	err := refCreateExec(context.Background(), cmd, client, dctx)
	if err == nil || err.Error() != "ref already exists" {
		t.Fatalf("want error %q, got %v", "ref already exists", err)
	}
}

// TestRefDeleteResolvesObjectID covers delete_ref's two-call sequence when
// --object-id is omitted: a GET filtered by the RAW (unprefixed) name, then
// a POST whose body uses the refs/-prefixed name but carries no isLocked
// key at all (unlike create).
func TestRefDeleteResolvesObjectID(t *testing.T) {
	var bodies [][]byte
	var queries []string
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, b)
		queries = append(queries, r.URL.RawQuery)
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_, _ = w.Write([]byte(`{"count":1,"value":[{"name":"refs/heads/my_branch","objectId":"resolved-id"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"count":1,"value":[{"name":"refs/heads/my_branch","oldObjectId":"resolved-id","newObjectId":"0000000000000000000000000000000000000000","success":true}]}`))
	}))
	t.Cleanup(srv.Close)
	client, dctx := refTestClient(t, srv)
	cmd := refTestCmd(t, newRefDeleteCmd, "--name", "heads/my_branch")

	if err := refDeleteExec(context.Background(), cmd, client, dctx); err != nil {
		t.Fatalf("refDeleteExec: %v", err)
	}

	if len(methods) != 2 || methods[0] != "GET" || methods[1] != "POST" {
		t.Fatalf("want GET then POST, got %v", methods)
	}
	if got := refMustParseQuery(t, queries[0]).Get("filter"); got != "heads/my_branch" {
		t.Errorf("GET filter query = %q, want the raw (unprefixed) name", got)
	}

	var postBody []map[string]any
	if err := json.Unmarshal(bodies[1], &postBody); err != nil {
		t.Fatalf("POST body is not a JSON array: %v", err)
	}
	got := postBody[0]
	if got["name"] != "refs/heads/my_branch" {
		t.Errorf("POST name = %v, want refs/heads/my_branch", got["name"])
	}
	if got["oldObjectId"] != "resolved-id" {
		t.Errorf("oldObjectId = %v, want the id resolved from the GET", got["oldObjectId"])
	}
	if got["newObjectId"] != refZeroObjectID {
		t.Errorf("newObjectId = %v, want the zero-SHA sentinel", got["newObjectId"])
	}
	if _, ok := got["isLocked"]; ok {
		t.Errorf("POST body must not carry isLocked at all, got %v", got["isLocked"])
	}
}

// TestRefLockUnlockUnprefixedFilter covers the lock/unlock asymmetry: unlike
// create/delete, the name is sent as the raw filter query value without the
// refs/ prefix, and the body is a bare {"isLocked":bool} object via PATCH.
func TestRefLockUnlockUnprefixedFilter(t *testing.T) {
	tests := []struct {
		locked bool
	}{
		{locked: true},
		{locked: false},
	}

	for _, tt := range tests {
		t.Run(map[bool]string{true: "lock", false: "unlock"}[tt.locked], func(t *testing.T) {
			var method, query string
			var body []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method = r.Method
				query = r.URL.RawQuery
				body, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"name":"refs/heads/my_branch","objectId":"abc123","isLocked":true}`))
			}))
			t.Cleanup(srv.Close)
			client, dctx := refTestClient(t, srv)
			cmd := refTestCmd(t, newRefLockCmd, "--name", "heads/my_branch")

			if err := refUpdateLockExec(context.Background(), cmd, client, dctx, tt.locked); err != nil {
				t.Fatalf("refUpdateLockExec: %v", err)
			}

			if method != "PATCH" {
				t.Errorf("method = %s, want PATCH", method)
			}
			if got := refMustParseQuery(t, query).Get("filter"); got != "heads/my_branch" {
				t.Errorf("filter query = %q, want the raw (unprefixed) name", got)
			}

			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("body is not a JSON object: %v", err)
			}
			if got["isLocked"] != tt.locked {
				t.Errorf("isLocked = %v, want %v", got["isLocked"], tt.locked)
			}
		})
	}
}

// TestRefListFilter covers the --filter query param and that list uses
// ado.Client.List (unwraps {count,value}).
func TestRefListFilter(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":1,"value":[{"name":"refs/heads/my_branch","objectId":"abc123"}]}`))
	}))
	t.Cleanup(srv.Close)
	client, dctx := refTestClient(t, srv)
	cmd := refTestCmd(t, newRefListCmd, "--filter", "heads/")

	if err := refListExec(context.Background(), cmd, client, dctx); err != nil {
		t.Fatalf("refListExec: %v", err)
	}
	if got := refMustParseQuery(t, query).Get("filter"); got != "heads/" {
		t.Errorf("filter query = %q, want heads/", got)
	}
}

// TestRefListTableQueryKeepsServerOrder: _format.py:262-266's sort lives
// only in the table transformer, which knack never runs when --query is
// set (query.py:49 applies the query to the raw, un-sorted result). So
// `-o table --query` must see server order, same as repo_list.go's guard.
func TestRefListTableQueryKeepsServerOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":2,"value":[{"name":"refs/heads/zeta","objectId":"1"},{"name":"refs/heads/alpha","objectId":"2"}]}`))
	}))
	t.Cleanup(srv.Close)
	client, dctx := refTestClient(t, srv)
	cmd := newRefListCmd()
	cmd.Flags().String("output", "table", "")
	cmd.Flags().String("query", "[0].name", "")

	out := refCaptureStdout(t, func() {
		if err := refListExec(context.Background(), cmd, client, dctx); err != nil {
			t.Fatalf("refListExec: %v", err)
		}
	})
	if !strings.Contains(out, "refs/heads/zeta") {
		t.Errorf("-o table --query output = %q, want server-order first element (zeta), not sorted (alpha)", out)
	}
}

// refCaptureStdout redirects os.Stdout for the duration of fn.
func refCaptureStdout(t *testing.T, fn func()) string {
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

// TestRefDeleteNoConfirmation asserts `repos ref delete` neither registers
// --yes nor prompts for confirmation (commands.py:134 registers delete_ref
// with no confirmation=, unlike delete_repo/delete_policy). Run with no
// --organization: if runRefDelete still called ado.Confirm first, a non-TTY
// test process would fail on the confirmation's "Use --yes" error instead
// of the org-resolution error asserted here.
func TestRefDeleteNoConfirmation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	cmd := refTestCmd(t, newRefDeleteCmd, "--name", "heads/my_branch")
	if f := cmd.Flags().Lookup("yes"); f != nil {
		t.Errorf("ref delete must not register --yes")
	}

	err := runRefDelete(context.Background(), cmd)
	if err == nil || !strings.Contains(err.Error(), "must be specified") {
		t.Fatalf("want an org/project-resolution error (proof no confirmation ran first), got %v", err)
	}
}
