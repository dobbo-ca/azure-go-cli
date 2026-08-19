package devops

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

// wikiCapturedRequest is what each fake server handler records for a test to
// assert on.
type wikiCapturedRequest struct {
	Method string
	URL    string // raw request-target, escaped path + query, as sent on the wire
	Body   map[string]any
	Header http.Header
}

// wikiTestServer replays responses in order (one per captured request),
// looping the last one if more requests arrive than responses provided.
func wikiTestServer(t *testing.T, statuses []int, respBodies []string, etags []string) (*httptest.Server, *[]wikiCapturedRequest) {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	captured := &[]wikiCapturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := len(*captured)
		if i >= len(statuses) {
			i = len(statuses) - 1
		}

		req := wikiCapturedRequest{Method: r.Method, URL: r.URL.RequestURI(), Header: r.Header.Clone()}
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			if len(b) > 0 {
				_ = json.Unmarshal(b, &req.Body)
			}
		}
		*captured = append(*captured, req)

		w.Header().Set("Content-Type", "application/json")
		if i < len(etags) && etags[i] != "" {
			w.Header().Set("ETag", etags[i])
		}
		w.WriteHeader(statuses[i])
		w.Write([]byte(respBodies[i]))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func TestWikiCreate_Codewiki_CallSequenceAndBody(t *testing.T) {
	srv, captured := wikiTestServer(t,
		[]int{200, 200, 200},
		[]string{
			`{"id":"repo-guid","name":"myrepo"}`,
			`{"id":"proj-guid","name":"MyProj"}`,
			`{"id":"wiki-1","name":"code.wiki","type":"codewiki"}`,
		},
		nil,
	)

	cmd := wikiCreateCmd()
	dctx := ado.Context{Org: srv.URL, Project: "MyProj", Repo: "myrepo"}

	if err := wikiCreate(context.Background(), cmd, dctx, "codewiki", "code.wiki", "/", "main"); err != nil {
		t.Fatalf("wikiCreate: %v", err)
	}

	if len(*captured) != 3 {
		t.Fatalf("got %d requests, want 3 (repo lookup, project lookup, create)", len(*captured))
	}
	reqs := *captured

	if reqs[0].Method != http.MethodGet || reqs[0].URL != "/MyProj/_apis/git/repositories/myrepo?api-version=5.0" {
		t.Errorf("request 0 (repo lookup) = %s %s", reqs[0].Method, reqs[0].URL)
	}
	if reqs[1].Method != http.MethodGet || reqs[1].URL != "/_apis/projects/MyProj?api-version=5.0" {
		t.Errorf("request 1 (project lookup) = %s %s", reqs[1].Method, reqs[1].URL)
	}
	if reqs[2].Method != http.MethodPost || reqs[2].URL != "/MyProj/_apis/wiki/wikis?api-version=5.0" {
		t.Errorf("request 2 (create) = %s %s", reqs[2].Method, reqs[2].URL)
	}

	body := reqs[2].Body
	if body["type"] != "codewiki" {
		t.Errorf("body type = %v, want codewiki", body["type"])
	}
	if body["projectId"] != "proj-guid" {
		t.Errorf("body projectId = %v, want proj-guid", body["projectId"])
	}
	if body["repositoryId"] != "repo-guid" {
		t.Errorf("body repositoryId = %v, want repo-guid", body["repositoryId"])
	}
	if body["mappedPath"] != "/" {
		t.Errorf("body mappedPath = %v, want /", body["mappedPath"])
	}
	ver, _ := body["version"].(map[string]any)
	if ver["version"] != "main" {
		t.Errorf("body version.version = %v, want main", ver["version"])
	}
}

func TestWikiCreate_Projectwiki_SkipsRepoLookupAndOmitsUnsetFields(t *testing.T) {
	srv, captured := wikiTestServer(t,
		[]int{200, 200},
		[]string{
			`{"id":"proj-guid","name":"MyProj"}`,
			`{"id":"wiki-1","name":"MyProj.wiki","type":"projectwiki"}`,
		},
		nil,
	)

	cmd := wikiCreateCmd()
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := wikiCreate(context.Background(), cmd, dctx, "projectwiki", "", "", ""); err != nil {
		t.Fatalf("wikiCreate: %v", err)
	}

	if len(*captured) != 2 {
		t.Fatalf("got %d requests, want 2 (project lookup, create only — no repo lookup)", len(*captured))
	}
	body := (*captured)[1].Body
	if _, ok := body["name"]; ok {
		t.Errorf("body has name = %v, want omitted when --name not given", body["name"])
	}
	if _, ok := body["repositoryId"]; ok {
		t.Errorf("body has repositoryId = %v, want omitted for projectwiki", body["repositoryId"])
	}
	if _, ok := body["version"]; ok {
		t.Errorf("body has version = %v, want omitted when --version not given", body["version"])
	}
}

func TestWikiCreate_ProjectAlreadyUUID_SkipsProjectLookup(t *testing.T) {
	const guid = "11111111-2222-3333-4444-555555555555"
	srv, captured := wikiTestServer(t,
		[]int{200},
		[]string{`{"id":"wiki-1","name":"MyProj.wiki","type":"projectwiki"}`},
		nil,
	)

	cmd := wikiCreateCmd()
	dctx := ado.Context{Org: srv.URL, Project: guid}

	if err := wikiCreate(context.Background(), cmd, dctx, "projectwiki", "", "", ""); err != nil {
		t.Fatalf("wikiCreate: %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf("got %d requests, want 1 (create only — project is already a GUID)", len(*captured))
	}
	if (*captured)[0].Body["projectId"] != guid {
		t.Errorf("body projectId = %v, want %s", (*captured)[0].Body["projectId"], guid)
	}
}

func TestWikiPageCreate_NoIfMatchAndCapturesETag(t *testing.T) {
	srv, captured := wikiTestServer(t,
		[]int{201},
		[]string{`{"path":"/abc","order":0,"isParentPage":false}`},
		[]string{`"etag-value-1"`},
	)

	cmd := wikiPageCreateCmd()
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	out := wikiCaptureStdout(t, func() {
		if err := wikiPageCreate(context.Background(), cmd, dctx, "myprojectwiki", "abc", "Created a wiki page", "hello"); err != nil {
			t.Fatalf("wikiPageCreate: %v", err)
		}
	})

	req := (*captured)[0]
	if req.Method != http.MethodPut {
		t.Errorf("Method = %q, want PUT", req.Method)
	}
	wantURL := "/MyProj/_apis/wiki/wikis/myprojectwiki/pages?api-version=5.0&comment=Created+a+wiki+page&path=abc"
	if req.URL != wantURL {
		t.Errorf("URL = %q, want %q", req.URL, wantURL)
	}
	if req.Body["content"] != "hello" {
		t.Errorf("body content = %v, want hello", req.Body["content"])
	}
	if got := req.Header.Get("If-Match"); got != "" {
		t.Errorf("If-Match = %q, want none on create", got)
	}

	// The ETag lives only in the response header (never the JSON body,
	// recording-verified) — assert wikiDoPage's transport capture actually
	// surfaces it in the printed output, since that's the whole point of the
	// workaround in wiki_page.go.
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	if got["eTag"] != `"etag-value-1"` {
		t.Errorf("output eTag = %v, want the captured ETag header value", got["eTag"])
	}
}

func TestWikiPageUpdate_SendsIfMatchHeader(t *testing.T) {
	srv, captured := wikiTestServer(t,
		[]int{200},
		[]string{`{"path":"/abc","order":0,"isParentPage":false}`},
		[]string{`"etag-value-2"`},
	)

	cmd := wikiPageUpdateCmd()
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := wikiPageUpdate(context.Background(), cmd, dctx, "myprojectwiki", "abc", "Updated the page using Azure DevOps CLI", "new content", "b54c50ea"); err != nil {
		t.Fatalf("wikiPageUpdate: %v", err)
	}

	req := (*captured)[0]
	if req.Method != http.MethodPut {
		t.Errorf("Method = %q, want PUT (update reuses the create route)", req.Method)
	}
	if got := req.Header.Get("If-Match"); got != "b54c50ea" {
		t.Errorf("If-Match = %q, want b54c50ea", got)
	}
}

func TestWikiPageShow_VersionSetsQueryParamInsteadOfCrashing(t *testing.T) {
	srv, captured := wikiTestServer(t,
		[]int{200},
		[]string{`{"path":"/abc","order":0,"isParentPage":false,"content":"hi"}`},
		[]string{`"etag-value-3"`},
	)

	cmd := wikiPageShowCmd()
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := wikiPageShow(context.Background(), cmd, dctx, "myprojectwiki", "abc", "etag-abc", false, true, ""); err != nil {
		t.Fatalf("wikiPageShow: %v", err)
	}

	req := (*captured)[0]
	wantURL := "/MyProj/_apis/wiki/wikis/myprojectwiki/pages?api-version=5.0&includeContent=true&path=abc&versionDescriptor.version=etag-abc"
	if req.URL != wantURL {
		t.Errorf("URL = %q, want %q", req.URL, wantURL)
	}
}

func TestWikiPageShow_NoVersionOmitsDescriptorParams(t *testing.T) {
	srv, captured := wikiTestServer(t,
		[]int{200},
		[]string{`{"path":"/abc","order":0,"isParentPage":false}`},
		nil,
	)

	cmd := wikiPageShowCmd()
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := wikiPageShow(context.Background(), cmd, dctx, "myprojectwiki", "abc", "", false, true, ""); err != nil {
		t.Fatalf("wikiPageShow: %v", err)
	}

	wantURL := "/MyProj/_apis/wiki/wikis/myprojectwiki/pages?api-version=5.0&includeContent=true&path=abc"
	if (*captured)[0].URL != wantURL {
		t.Errorf("URL = %q, want %q (no versionDescriptor.* params)", (*captured)[0].URL, wantURL)
	}
}

func TestWikiList_ScopeOrganizationOmitsProjectSegmentAndSortsByName(t *testing.T) {
	srv, captured := wikiTestServer(t,
		[]int{200},
		[]string{`{"count":2,"value":[{"id":"2","name":"beta"},{"id":"1","name":"Alpha"}]}`},
		nil,
	)

	// _get_wiki_key sorting (_format.py:279-283) is wired only as this
	// command's table_transformer (commands.py:175), applied by knack for
	// -o table with no --query.
	cmd := wikiListCmd()
	cmd.Flags().String("output", "table", "")
	cmd.Flags().String("query", "", "")
	dctx := ado.Context{Org: srv.URL}

	out := wikiCaptureStdout(t, func() {
		if err := wikiList(context.Background(), cmd, dctx, "organization"); err != nil {
			t.Fatalf("wikiList: %v", err)
		}
	})

	wantURL := "/_apis/wiki/wikis?api-version=5.0"
	if (*captured)[0].URL != wantURL {
		t.Errorf("URL = %q, want %q (no project segment for org scope)", (*captured)[0].URL, wantURL)
	}

	// Table output, not JSON: header line + dash separator + one row per
	// wiki, in sorted order.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (header + separator + 2 rows)\noutput: %s", len(lines), out)
	}
	if !strings.Contains(lines[2], "Alpha") || !strings.Contains(lines[3], "beta") {
		t.Fatalf("rows = %q, %q, want [Alpha, beta] sorted case-insensitively", lines[2], lines[3])
	}
}

// TestWikiList_JSONKeepsServerOrder guards B7/T10: -o json/tsv must keep the
// server's order, unlike -o table.
func TestWikiList_JSONKeepsServerOrder(t *testing.T) {
	srv, _ := wikiTestServer(t,
		[]int{200},
		[]string{`{"count":2,"value":[{"id":"2","name":"beta"},{"id":"1","name":"Alpha"}]}`},
		nil,
	)

	cmd := wikiListCmd()
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	dctx := ado.Context{Org: srv.URL}

	out := wikiCaptureStdout(t, func() {
		if err := wikiList(context.Background(), cmd, dctx, "organization"); err != nil {
			t.Fatalf("wikiList: %v", err)
		}
	})

	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	if len(got) != 2 || got[0]["name"] != "beta" || got[1]["name"] != "Alpha" {
		t.Fatalf("got %v, want [beta, Alpha] (server order preserved)", got)
	}
}

// TestWikiRunList_OrganizationScopeKeepsExplicitProject guards AUTH-04:
// wiki.py:88-96 calls get_all_wikis(project=project) unconditionally, so an
// explicit --project alongside --scope organization still scopes the
// request — the org branch only skips resolving a project, it doesn't clear
// one the caller passed. Uses serviceendpointTestEnv/-Org (serviceendpoint_
// test.go, same package) for a dev.azure.com-shaped org that ado.Resolve's
// validateOrg accepts, since wikiRunList (unlike wikiList) goes through it.
func TestWikiRunList_OrganizationScopeKeepsExplicitProject(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":0,"value":[]}`))
	}))
	defer srv.Close()
	serviceendpointTestEnv(t, srv)

	cmd := wikiListCmd()
	cmd.SetArgs([]string{
		"--organization", serviceendpointTestOrg,
		"--scope", "organization",
		"--project", "MyProj",
	})
	cmd.SetOut(os.Stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	wantURL := "/myorg/MyProj/_apis/wiki/wikis?api-version=5.0"
	if gotURL != wantURL {
		t.Errorf("URL = %q, want %q (explicit --project kept for org scope)", gotURL, wantURL)
	}
}

// wikiCaptureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything it wrote, same idiom as team_test.go's teamCaptureStdout.
func wikiCaptureStdout(t *testing.T, fn func()) string {
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

	b, _ := io.ReadAll(r)
	return string(b)
}
