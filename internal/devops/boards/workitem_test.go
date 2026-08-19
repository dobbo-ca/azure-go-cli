package boards

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// workitemCaptureStdout runs fn with os.Stdout redirected, and returns
// whatever it wrote -- needed because ado.Print writes straight to
// os.Stdout rather than through cmd.OutOrStdout().
func workitemCaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}

// workitemStubResponse is one canned server response, replayed in order.
type workitemStubResponse struct {
	Status int
	Body   string
}

// workitemCapturedRequest is what the fake server recorded for one request.
// Body is `any` (not map[string]any) because JSON Patch bodies are arrays,
// not objects.
type workitemCapturedRequest struct {
	Method string
	URL    string
	Body   any
}

// workitemTestServer replays responses in order, one per request, and
// records every request it receives -- needed here because several of these
// commands make more than one HTTP call in sequence.
func workitemTestServer(t *testing.T, responses []workitemStubResponse) (*httptest.Server, *[]workitemCapturedRequest) {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	captured := &[]workitemCapturedRequest{}
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := workitemCapturedRequest{Method: r.Method, URL: r.URL.RequestURI()}
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			if len(b) > 0 {
				_ = json.Unmarshal(b, &req.Body)
			}
		}
		*captured = append(*captured, req)

		if i >= len(responses) {
			t.Fatalf("unexpected request %d: %s %s", len(*captured)-1, r.Method, r.URL.String())
		}
		resp := responses[i]
		i++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.Status)
		w.Write([]byte(resp.Body))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func TestWorkitemCreate_PatchBodyAndURL(t *testing.T) {
	srv, captured := workitemTestServer(t, []workitemStubResponse{
		{Status: http.StatusOK, Body: `{"id":1,"fields":{"System.Title":"T1"}}`},
	})

	cmd := workitemCreateCmd()
	cmd.Flags().Set("description", "d")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := workitemCreate(context.Background(), cmd, dctx, "Bug", "T1", "d", "", "", "", "", "", nil, false); err != nil {
		t.Fatalf("workitemCreate: %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf("got %d requests, want 1", len(*captured))
	}
	req := (*captured)[0]
	if req.Method != http.MethodPost {
		t.Errorf("Method = %q, want POST", req.Method)
	}
	wantURL := "/MyProj/_apis/wit/workitems/$Bug?api-version=5.0"
	if req.URL != wantURL {
		t.Errorf("URL = %q, want %q", req.URL, wantURL)
	}
	ops, ok := req.Body.([]any)
	if !ok || len(ops) != 2 {
		t.Fatalf("body = %#v, want a 2-element patch array", req.Body)
	}
	first := ops[0].(map[string]any)
	if first["path"] != "/fields/System.Title" || first["value"] != "T1" {
		t.Errorf("op[0] = %v, want Title=T1", first)
	}
	second := ops[1].(map[string]any)
	if second["path"] != "/fields/System.Description" || second["value"] != "d" {
		t.Errorf("op[1] = %v, want Description=d", second)
	}
}

func TestWorkitemCreate_AssignedToMeResolvesViaConnectionData(t *testing.T) {
	srv, captured := workitemTestServer(t, []workitemStubResponse{
		{Status: http.StatusOK, Body: `{"authenticatedUser":{"id":"u1","properties":{"Account":{"$value":"me@example.com"}}}}`},
		{Status: http.StatusOK, Body: `{"id":2}`},
	})

	cmd := workitemCreateCmd()
	cmd.Flags().Set("assigned-to", "me")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := workitemCreate(context.Background(), cmd, dctx, "Bug", "T1", "", "me", "", "", "", "", nil, false); err != nil {
		t.Fatalf("workitemCreate: %v", err)
	}

	if len(*captured) != 2 {
		t.Fatalf("got %d requests, want 2 (connectionData then create)", len(*captured))
	}
	if (*captured)[0].URL != "/_apis/connectionData?api-version=5.0-preview.1" {
		t.Errorf("request 0 URL = %q, want connectionData", (*captured)[0].URL)
	}

	ops := (*captured)[1].Body.([]any)
	var sawAssignedTo bool
	for _, o := range ops {
		op := o.(map[string]any)
		if op["path"] == "/fields/System.AssignedTo" {
			sawAssignedTo = true
			if op["value"] != "me@example.com" {
				t.Errorf("AssignedTo value = %v, want the resolved account", op["value"])
			}
		}
	}
	if !sawAssignedTo {
		t.Errorf("no System.AssignedTo op in %v", ops)
	}
}

func TestWorkitemUpdate_OnlyChangedFieldsPatched(t *testing.T) {
	srv, captured := workitemTestServer(t, []workitemStubResponse{
		{Status: http.StatusOK, Body: `{"id":5}`},
	})

	cmd := workitemUpdateCmd()
	cmd.Flags().Set("state", "Active")
	dctx := ado.Context{Org: srv.URL}

	if err := workitemUpdate(context.Background(), cmd, dctx, 5, "", "", "", "Active", "", "", "", "", nil, false); err != nil {
		t.Fatalf("workitemUpdate: %v", err)
	}

	req := (*captured)[0]
	if req.Method != http.MethodPatch {
		t.Errorf("Method = %q, want PATCH", req.Method)
	}
	wantURL := "/_apis/wit/workitems/5?api-version=5.0"
	if req.URL != wantURL {
		t.Errorf("URL = %q, want %q (no {project} route segment)", req.URL, wantURL)
	}
	ops := req.Body.([]any)
	if len(ops) != 1 {
		t.Fatalf("ops = %v, want exactly 1 (only --state was changed)", ops)
	}
	op := ops[0].(map[string]any)
	if op["path"] != "/fields/System.State" || op["value"] != "Active" {
		t.Errorf("op = %v, want State=Active", op)
	}
}

func TestWorkitemDelete_DestroyAlwaysSentInQuery(t *testing.T) {
	for _, destroy := range []bool{false, true} {
		srv, captured := workitemTestServer(t, []workitemStubResponse{
			{Status: http.StatusOK, Body: `{"id":7}`},
		})

		cmd := workitemDeleteCmd()
		dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

		if err := workitemDelete(context.Background(), cmd, dctx, 7, destroy); err != nil {
			t.Fatalf("workitemDelete(destroy=%v): %v", destroy, err)
		}

		req := (*captured)[0]
		if req.Method != http.MethodDelete {
			t.Errorf("Method = %q, want DELETE", req.Method)
		}
		// url.Values.Encode() sorts keys, so api-version sorts before destroy.
		wantURL := "/MyProj/_apis/wit/workitems/7?api-version=5.0&destroy=false"
		if destroy {
			wantURL = "/MyProj/_apis/wit/workitems/7?api-version=5.0&destroy=true"
		}
		if req.URL != wantURL {
			t.Errorf("destroy=%v: URL = %q, want %q", destroy, req.URL, wantURL)
		}
	}
}

func TestWorkitemRelationAdd_WiqlThenPatchInResultOrder(t *testing.T) {
	srv, captured := workitemTestServer(t, []workitemStubResponse{
		{Status: http.StatusOK, Body: `{"count":1,"value":[{"name":"Parent","referenceName":"System.LinkTypes.Hierarchy-Reverse","attributes":{"enabled":true,"usage":"workItemLink"}}]}`},
		{Status: http.StatusOK, Body: `{"workItems":[{"id":2,"url":"https://example/2"},{"id":3,"url":"https://example/3"}]}`},
		{Status: http.StatusOK, Body: `{}`},
		{Status: http.StatusOK, Body: `{"relations":[{"rel":"System.LinkTypes.Hierarchy-Reverse","url":"https://example/2"},{"rel":"System.LinkTypes.Hierarchy-Reverse","url":"https://example/3"}]}`},
	})

	cmd := workitemRelationAddCmd()
	dctx := ado.Context{Org: srv.URL}

	if err := workitemRelationAdd(context.Background(), cmd, dctx, 1, "Parent", "2,3", ""); err != nil {
		t.Fatalf("workitemRelationAdd: %v", err)
	}

	if len(*captured) != 4 {
		t.Fatalf("got %d requests, want 4 (relationtypes, wiql, patch, get)", len(*captured))
	}

	wiqlReq := (*captured)[1]
	if wiqlReq.Method != http.MethodPost || wiqlReq.URL != "/_apis/wit/wiql?api-version=5.0" {
		t.Errorf("wiql request = %+v", wiqlReq)
	}
	wiqlBody := wiqlReq.Body.(map[string]any)
	wantQuery := "SELECT [System.Id] FROM WorkItems WHERE ([System.Id] = 2 OR [System.Id] = 3)"
	if wiqlBody["query"] != wantQuery {
		t.Errorf("wiql query = %q, want %q", wiqlBody["query"], wantQuery)
	}

	patchReq := (*captured)[2]
	if patchReq.Method != http.MethodPatch || patchReq.URL != "/_apis/wit/workitems/1?api-version=5.0" {
		t.Errorf("patch request = %+v", patchReq)
	}
	ops := patchReq.Body.([]any)
	if len(ops) != 2 {
		t.Fatalf("ops = %v, want 2", ops)
	}
	op0 := ops[0].(map[string]any)["value"].(map[string]any)
	if op0["rel"] != "System.LinkTypes.Hierarchy-Reverse" || op0["url"] != "https://example/2" {
		t.Errorf("op[0].value = %v, want target id 2 first (WIQL result order)", op0)
	}
}

func TestWorkitemRelationRemove_IndexesComputedFromPreFetchSnapshot(t *testing.T) {
	mainWorkItem := `{"relations":[` +
		`{"rel":"System.LinkTypes.Related","url":"https://example/9"},` +
		`{"rel":"System.LinkTypes.Hierarchy-Reverse","url":"https://example/2"},` +
		`{"rel":"System.LinkTypes.Hierarchy-Reverse","url":"https://example/3"}` +
		`]}`

	srv, captured := workitemTestServer(t, []workitemStubResponse{
		{Status: http.StatusOK, Body: `{"count":1,"value":[{"name":"Parent","referenceName":"System.LinkTypes.Hierarchy-Reverse","attributes":{"enabled":true,"usage":"workItemLink"}}]}`},
		{Status: http.StatusOK, Body: mainWorkItem},
		{Status: http.StatusOK, Body: `{"url":"https://example/2"}`},
		{Status: http.StatusOK, Body: `{"url":"https://example/3"}`},
		{Status: http.StatusOK, Body: `{}`},
		{Status: http.StatusOK, Body: `{"relations":[]}`},
	})

	cmd := workitemRelationRemoveCmd()
	dctx := ado.Context{Org: srv.URL}

	if err := workitemRelationRemove(context.Background(), cmd, dctx, 1, "Parent", "2,3"); err != nil {
		t.Fatalf("workitemRelationRemove: %v", err)
	}

	patchReq := (*captured)[4]
	if patchReq.Method != http.MethodPatch {
		t.Errorf("Method = %q, want PATCH", patchReq.Method)
	}
	ops := patchReq.Body.([]any)
	if len(ops) != 2 {
		t.Fatalf("ops = %v, want 2 removes", ops)
	}
	if ops[0].(map[string]any)["path"] != "/relations/1" {
		t.Errorf("op[0] = %v, want index 1 (target 2's position in the snapshot)", ops[0])
	}
	if ops[1].(map[string]any)["path"] != "/relations/2" {
		t.Errorf("op[1] = %v, want index 2 (target 3's position in the snapshot)", ops[1])
	}
}

func TestWorkitemRelationRemove_MismatchCountErrors(t *testing.T) {
	srv, _ := workitemTestServer(t, []workitemStubResponse{
		{Status: http.StatusOK, Body: `{"count":1,"value":[{"name":"Parent","referenceName":"System.LinkTypes.Hierarchy-Reverse","attributes":{"enabled":true,"usage":"workItemLink"}}]}`},
		{Status: http.StatusOK, Body: `{"relations":[]}`},
		{Status: http.StatusOK, Body: `{"url":"https://example/2"}`},
	})

	cmd := workitemRelationRemoveCmd()
	dctx := ado.Context{Org: srv.URL}

	err := workitemRelationRemove(context.Background(), cmd, dctx, 1, "Parent", "2")
	if err == nil || err.Error() != "Id(s) supplied in --target-id is not valid" {
		t.Fatalf("err = %v, want the CLIError text", err)
	}
}

// TestWorkitemCreateCmd_FieldsFlag ports nargs='*' semantics (arguments.py:16):
// commas inside a "field=value" pair are legal (split once on '=', not on
// pflag's CSV rule), and space-separated repeats of --fields are all
// collected, including ones pflag treats as trailing positional args (which
// the create RunE folds back into fields).
func TestWorkitemCreateCmd_FieldsFlag(t *testing.T) {
	cmd := workitemCreateCmd()
	if err := cmd.Flags().Parse([]string{
		"--type", "Bug", "--title", "T1",
		"--fields", "System.Tags=a,b", "x=1", "y=2",
	}); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	fields, err := cmd.Flags().GetStringArray("fields")
	if err != nil {
		t.Fatalf("GetStringArray: %v", err)
	}
	fields = append(fields, cmd.Flags().Args()...)

	ops, err := workitemParseFieldPairs(fields)
	if err != nil {
		t.Fatalf("workitemParseFieldPairs: %v", err)
	}
	want := map[string]any{
		"/fields/System.Tags": "a,b",
		"/fields/x":           "1",
		"/fields/y":           "2",
	}
	got := map[string]any{}
	for _, op := range ops {
		got[op["path"].(string)] = op["value"]
	}
	for path, val := range want {
		if got[path] != val {
			t.Errorf("op[%s] = %v, want %v (all ops: %v)", path, got[path], val, ops)
		}
	}
}

// TestWorkitemRelationAdd_OutputShape ports relations.py:70's `return
// work_item` vs commands.py:60's table_transformer=transform_work_item_relations:
// every format except -o table gets the full work item; only table gets the
// relations-only view.
func TestWorkitemRelationAdd_OutputShape(t *testing.T) {
	newServer := func() *httptest.Server {
		srv, _ := workitemTestServer(t, []workitemStubResponse{
			{Status: http.StatusOK, Body: `{"count":1,"value":[{"name":"Parent","referenceName":"System.LinkTypes.Hierarchy-Reverse","attributes":{"enabled":true,"usage":"workItemLink"}}]}`},
			{Status: http.StatusOK, Body: `{"workItems":[{"id":2,"url":"https://example/2"}]}`},
			{Status: http.StatusOK, Body: `{}`},
			{Status: http.StatusOK, Body: `{"id":1,"fields":{"System.Title":"T1"},"relations":[{"rel":"System.LinkTypes.Hierarchy-Reverse","url":"https://example/2"}]}`},
		})
		return srv
	}

	t.Run("json default is the full work item", func(t *testing.T) {
		srv := newServer()
		cmd := workitemRelationAddCmd()
		cmd.Flags().String("output", "json", "")
		cmd.Flags().String("query", "", "")
		dctx := ado.Context{Org: srv.URL}

		out := workitemCaptureStdout(t, func() {
			if err := workitemRelationAdd(context.Background(), cmd, dctx, 1, "Parent", "2", ""); err != nil {
				t.Fatalf("workitemRelationAdd: %v", err)
			}
		})
		if !strings.Contains(out, `"id": 1`) {
			t.Errorf("json output = %s, want the full work item (with id)", out)
		}
	})

	t.Run("table is the relations list only", func(t *testing.T) {
		srv := newServer()
		cmd := workitemRelationAddCmd()
		cmd.Flags().String("output", "table", "")
		cmd.Flags().String("query", "", "")
		dctx := ado.Context{Org: srv.URL}

		out := workitemCaptureStdout(t, func() {
			if err := workitemRelationAdd(context.Background(), cmd, dctx, 1, "Parent", "2", ""); err != nil {
				t.Fatalf("workitemRelationAdd: %v", err)
			}
		})
		if strings.Contains(out, `"id"`) || !strings.Contains(out, "Relation Type") {
			t.Errorf("table output = %q, want a relations table with no work item id", out)
		}
	})
}
