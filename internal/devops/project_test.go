package devops

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// projectTestClient builds a Client against srv with a hermetic, network-free
// auth path: $HOME is redirected so azure.GetCredential's config.Load()
// finds no real az-login profile (fails fast, no network) and resolveAuth
// falls through to AZURE_DEVOPS_EXT_PAT. Mirrors ado's own client_test.go
// newTestClient helper — this package can't reach ado's unexported
// getCredential seam, so redirecting $HOME is the equivalent from outside.
// org is deliberately NOT run through ado.Resolve's real-org-host
// validation (httptest URLs are http://127.0.0.1:port, which validateOrg
// would reject) — these tests exercise projectCreate/projectDelete/
// projectList directly, the same split point runProject* calls after
// resolving org and building the client.
func projectTestClient(t *testing.T, org string) *ado.Client {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	c, err := ado.NewClient(context.Background(), org)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestProjectCreate_CallSequenceAndBody(t *testing.T) {
	var seq []string
	var createBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seq = append(seq, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/org/_apis/process/processes":
			w.Write([]byte(`{"count":2,"value":[{"id":"proc-basic","name":"Basic","isDefault":true},{"id":"proc-agile","name":"Agile","isDefault":false}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/org/_apis/projects":
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &createBody)
			w.Write([]byte(`{"id":"op-1","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/org/_apis/operations/op-1":
			w.Write([]byte(`{"id":"op-1","status":"succeeded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/org/_apis/projects/MyProj":
			if r.URL.Query().Get("includeCapabilities") != "true" {
				t.Errorf("expected includeCapabilities=true, got %q", r.URL.RawQuery)
			}
			w.Write([]byte(`{"id":"proj-1","name":"MyProj","visibility":"private"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := projectTestClient(t, srv.URL+"/org")
	project, err := projectCreate(context.Background(), client, projectCreateParams{
		Name:          "MyProj",
		Process:       "agile",
		SourceControl: "git",
		Visibility:    "private",
	})
	if err != nil {
		t.Fatalf("projectCreate: %v", err)
	}
	if project["id"] != "proj-1" {
		t.Errorf("returned project = %#v, want id proj-1", project)
	}

	wantSeq := []string{
		"GET /org/_apis/process/processes",
		"POST /org/_apis/projects",
		"GET /org/_apis/operations/op-1",
		"GET /org/_apis/projects/MyProj",
	}
	if len(seq) != len(wantSeq) {
		t.Fatalf("call sequence = %v, want %v", seq, wantSeq)
	}
	for i, want := range wantSeq {
		if seq[i] != want {
			t.Errorf("call %d = %q, want %q", i, seq[i], want)
		}
	}

	if createBody["name"] != "MyProj" || createBody["visibility"] != "private" {
		t.Errorf("create body = %#v", createBody)
	}
	caps, _ := createBody["capabilities"].(map[string]any)
	proc, _ := caps["processTemplate"].(map[string]any)
	if proc["templateTypeId"] != "proc-agile" {
		t.Errorf("resolved process id = %v, want proc-agile (case-insensitive --process name match)", proc["templateTypeId"])
	}
	vc, _ := caps["versioncontrol"].(map[string]any)
	if vc["sourceControlType"] != "git" {
		t.Errorf("sourceControlType = %v, want git", vc["sourceControlType"])
	}
}

func TestProjectCreate_ProcessNotFoundUsesProjectNameInError(t *testing.T) {
	// project.py:59: the error message interpolates the *project* name, not
	// the unmatched --process value — a Python quirk this port preserves.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":1,"value":[{"id":"proc-basic","name":"Basic","isDefault":true}]}`))
	}))
	defer srv.Close()

	client := projectTestClient(t, srv.URL+"/org")
	_, err := projectCreate(context.Background(), client, projectCreateParams{
		Name:          "MyProj",
		Process:       "NoSuchProcess",
		SourceControl: "git",
		Visibility:    "private",
	})

	wantMsg := `Could not find a process template with name: "MyProj"`
	if err == nil || err.Error() != wantMsg {
		t.Fatalf("error = %v, want %q", err, wantMsg)
	}
}

func TestProjectDelete_CallSequence(t *testing.T) {
	var seq []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seq = append(seq, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/org/_apis/projects/proj-123":
			w.Write([]byte(`{"id":"op-9","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/org/_apis/operations/op-9":
			w.Write([]byte(`{"id":"op-9","status":"succeeded"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := projectTestClient(t, srv.URL+"/org")
	op, err := projectDelete(context.Background(), client, "proj-123")
	if err != nil {
		t.Fatalf("projectDelete: %v", err)
	}
	if op["id"] != "op-9" {
		t.Errorf("returned operation = %#v, want id op-9", op)
	}

	want := []string{"DELETE /org/_apis/projects/proj-123", "GET /org/_apis/operations/op-9"}
	if len(seq) != len(want) {
		t.Fatalf("call sequence = %v, want %v", seq, want)
	}
	for i := range want {
		if seq[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, seq[i], want[i])
		}
	}
}

func TestProjectDelete_FailedStatusIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodDelete:
			w.Write([]byte(`{"id":"op-9","status":"queued"}`))
		default:
			w.Write([]byte(`{"id":"op-9","status":"failed"}`))
		}
	}))
	defer srv.Close()

	client := projectTestClient(t, srv.URL+"/org")
	_, err := projectDelete(context.Background(), client, "proj-123")
	if err == nil || err.Error() != "Project deletion failed." {
		t.Fatalf("error = %v, want %q", err, "Project deletion failed.")
	}
}

func TestProjectList_NoAutoPageAndAPIVersion(t *testing.T) {
	// list_projects (project.py:123-148) makes a single GET and does not
	// follow X-MS-ContinuationToken, unlike every other list command in
	// this port; it also targets api-version 5.1, not 5.0.
	calls := 0
	var gotQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-MS-ContinuationToken", "next-page-token")
		w.Write([]byte(`{"count":1,"value":[{"id":"p1","name":"Zeta","visibility":"private"}]}`))
	}))
	defer srv.Close()

	client := projectTestClient(t, srv.URL+"/org")
	projects, err := projectList(context.Background(), client, projectListParams{StateFilter: "all"})
	if err != nil {
		t.Fatalf("projectList: %v", err)
	}
	if len(projects) != 1 || projects[0]["name"] != "Zeta" {
		t.Errorf("projects = %#v", projects)
	}

	if calls != 1 {
		t.Fatalf("expected exactly 1 request despite a continuation token in the response, got %d", calls)
	}
	if !strings.Contains(gotQuery, "api-version=5.1") {
		t.Errorf("query = %q, want api-version=5.1", gotQuery)
	}
	if !strings.Contains(gotQuery, "stateFilter=all") {
		t.Errorf("query = %q, want stateFilter=all", gotQuery)
	}
}

func TestProjectList_OptionalFiltersOmittedWhenUnset(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":0,"value":[]}`))
	}))
	defer srv.Close()

	client := projectTestClient(t, srv.URL+"/org")
	if _, err := projectList(context.Background(), client, projectListParams{StateFilter: "all"}); err != nil {
		t.Fatalf("projectList: %v", err)
	}

	for _, absent := range []string{"$top", "$skip", "continuationToken", "getDefaultTeamImageUrl"} {
		if strings.Contains(gotQuery, absent+"=") {
			t.Errorf("query = %q, did not expect %s to be present when its flag was unset", gotQuery, absent)
		}
	}
}

// TestProjectListOutput_Envelope guards M6: a bare array made
// --continuation-token/--query "value[]..." unreachable; non-table output
// must be {"value": [...], "continuationToken": ...} (core_client.py:245-260).
func TestProjectListOutput_Envelope(t *testing.T) {
	projects := []map[string]any{{"id": "p1", "name": "Zeta"}}

	v := projectListOutput(projects, false)
	env, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("non-table output = %#v, want a map envelope", v)
	}
	if _, ok := env["continuationToken"]; !ok {
		t.Error("envelope missing continuationToken key")
	}
	value, ok := env["value"].([]map[string]any)
	if !ok || len(value) != 1 || value[0]["name"] != "Zeta" {
		t.Errorf("envelope value = %#v", env["value"])
	}
}

// TestProjectListOutput_TableSortsOnlyForTable guards that -o json/tsv keep
// the server's order (_format.py:53-57 sorts only inside the table
// transformer) while -o table (no --query) sorts by name.lower().
func TestProjectListOutput_TableSortsOnlyForTable(t *testing.T) {
	projects := []map[string]any{{"name": "Zeta"}, {"name": "alpha"}}

	nonTable := projectListOutput(projects, false)
	env := nonTable.(map[string]any)
	value := env["value"].([]map[string]any)
	if value[0]["name"] != "Zeta" {
		t.Errorf("non-table order = %v, want server order preserved", value)
	}

	tableRows := projectListOutput(projects, true).([]map[string]any)
	if tableRows[0]["name"] != "alpha" || tableRows[1]["name"] != "Zeta" {
		t.Errorf("table order = %v, want sorted by name.lower()", tableRows)
	}
}

// TestProjectListColumns_OmitsCapabilitiesColumns guards T3: list never
// requests capabilities (project.py:123-148), so its table columns must not
// include Process/Source Control (only project_show.go's includeCapabilities
// GET legitimately fills them).
func TestProjectListColumns_OmitsCapabilitiesColumns(t *testing.T) {
	for _, c := range projectListColumns {
		if c.Header == "Process" || c.Header == "Source Control" {
			t.Errorf("projectListColumns unexpectedly includes %q", c.Header)
		}
	}
	if len(projectListColumns) != 3 {
		t.Errorf("projectListColumns = %d columns, want 3 (ID, Name, Visibility)", len(projectListColumns))
	}
}

// TestProjectNormalizeStateFilter guards m6: arguments.py:72 registers
// state_filter via get_enum_type(), whose CaseInsensitiveList choices match
// case-insensitively and normalize to the canonical-cased value.
func TestProjectNormalizeStateFilter(t *testing.T) {
	got, ok := projectNormalizeStateFilter("createpending")
	if !ok || got != "createPending" {
		t.Errorf("projectNormalizeStateFilter(\"createpending\") = (%q, %v), want (\"createPending\", true)", got, ok)
	}
	if _, ok := projectNormalizeStateFilter("bogus"); ok {
		t.Error("expected ok=false for an unknown state filter")
	}
}

// TestProjectListParseFlags_GetDefaultImageURLSpaceForm guards m4: a plain
// Bool flag made "--get-default-team-image-url false" send
// getDefaultTeamImageUrl=true (arguments.py:77-78 is three-state).
func TestProjectListParseFlags_GetDefaultImageURLSpaceForm(t *testing.T) {
	cmd := newProjectListCmd()
	if err := cmd.Flags().Set("get-default-team-image-url", "true"); err != nil {
		t.Fatal(err)
	}
	// Simulates what cobra hands RunE for "--get-default-team-image-url
	// false": pflag only consumes "true" as the NoOptDefVal-bound bare-flag
	// value, leaving "false" as a leftover positional.
	p, err := projectListParseFlags(cmd, []string{"false"})
	if err != nil {
		t.Fatalf("projectListParseFlags: %v", err)
	}
	if !p.HasGetDefaultImgURL || p.GetDefaultImageURL {
		t.Errorf("GetDefaultImageURL = %v (has=%v), want false", p.GetDefaultImageURL, p.HasGetDefaultImgURL)
	}
}
