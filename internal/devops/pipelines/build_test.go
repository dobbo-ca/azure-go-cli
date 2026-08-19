package pipelines

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// buildCapturedRequest is one HTTP request seen by a test server, decoded
// enough to assert routing and body shape. Mirrors internal/devops/repos'
// repoCapturedRequest.
type buildCapturedRequest struct {
	Method string
	Path   string
	Query  string
	Body   any
}

func buildTestServer(t *testing.T, handlers ...func(r *buildCapturedRequest) (int, any)) (*httptest.Server, *[]buildCapturedRequest) {
	t.Helper()
	var got []buildCapturedRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body any
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &body)
		}
		rec := buildCapturedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body}
		got = append(got, rec)

		if len(got) > len(handlers) {
			t.Fatalf("unexpected extra request #%d: %s %s", len(got), r.Method, r.URL.Path)
		}
		status, resp := handlers[len(got)-1](&rec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

// buildTestClient builds an *ado.Client against srv with a hermetic,
// network-free auth path — same approach as ado's own client_test.go and
// internal/devops/repos' repoTestClient.
func buildTestClient(t *testing.T, srv *httptest.Server) *ado.Client {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	client, err := ado.NewClient(context.Background(), srv.URL+"/myorg")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

// buildTestCmd returns real (a command built by one of this file's
// newBuild*Cmd constructors, so it carries the production flag set),
// additionally registers the inherited --output/--query persistent flags,
// and applies sets.
func buildTestCmd(t *testing.T, real *cobra.Command, output string, sets map[string]string) *cobra.Command {
	t.Helper()
	real.Flags().String("output", output, "")
	real.Flags().String("query", "", "")
	for k, v := range sets {
		if err := real.Flags().Set(k, v); err != nil {
			t.Fatalf("set --%s=%q: %v", k, v, err)
		}
	}
	return real
}

// TestBuildQueue covers the id-vs-name resolution call, the sourceBranch
// prefix normalisation, the --queue-id string->int wire conversion, and the
// --variables -> JSON-string parameters body.
func TestBuildQueue(t *testing.T) {
	srv, got := buildTestServer(t,
		// 1: name -> id lookup (GET Definitions?name=)
		func(r *buildCapturedRequest) (int, any) {
			return 200, map[string]any{"count": 1, "value": []any{map[string]any{"id": 42}}}
		},
		// 2: POST Builds
		func(r *buildCapturedRequest) (int, any) {
			return 200, map[string]any{"id": 638, "project": map[string]any{"name": "myproj"}}
		},
	)
	client := buildTestClient(t, srv)
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj"}
	cmd := buildTestCmd(t, newBuildQueueCmd(), "json", map[string]string{
		"definition-name": "MyDef",
		"branch":          "main",
		"queue-id":        "42",
	})
	// --variables is a repeatable StringArray flag (build_queue.go), not a
	// comma-splitting StringSlice: set it twice, and give one value a comma
	// to prove it survives intact instead of being split into "b".
	if err := cmd.Flags().Set("variables", "a=1"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("variables", "msg=x,y"); err != nil {
		t.Fatal(err)
	}

	if err := buildQueue(context.Background(), cmd, client, dctx); err != nil {
		t.Fatalf("buildQueue: %v", err)
	}

	reqs := *got
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests, got %d", len(reqs))
	}
	if reqs[0].Method != http.MethodGet || reqs[0].Path != "/myorg/myproj/_apis/build/Definitions" {
		t.Errorf("lookup request = %+v", reqs[0])
	}
	if reqs[0].Query != "api-version=5.0&name=MyDef" {
		t.Errorf("lookup query = %q", reqs[0].Query)
	}

	if reqs[1].Method != http.MethodPost || reqs[1].Path != "/myorg/myproj/_apis/build/Builds" {
		t.Errorf("queue request = %+v", reqs[1])
	}
	body, ok := reqs[1].Body.(map[string]any)
	if !ok {
		t.Fatalf("queue body is not an object: %#v", reqs[1].Body)
	}
	def, _ := body["definition"].(map[string]any)
	if def["id"] != float64(42) {
		t.Errorf("definition.id = %v, want 42 (resolved from name)", def["id"])
	}
	if body["sourceBranch"] != "refs/heads/main" {
		t.Errorf("sourceBranch = %v, want refs/heads/main", body["sourceBranch"])
	}
	queue, _ := body["queue"].(map[string]any)
	if queue["id"] != float64(42) {
		t.Errorf("queue.id = %v, want 42 as a number, not a string", queue["id"])
	}
	params, ok := body["parameters"].(string)
	if !ok {
		t.Fatalf("parameters is not a JSON string: %#v", body["parameters"])
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(params), &decoded); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}
	if decoded["a"] != "1" || decoded["msg"] != "x,y" {
		t.Errorf("parameters = %v, want {a:1 msg:x,y}", decoded)
	}
}

// TestBuildQueueRequiresIDOrName checks the exact ValueError-equivalent
// message (build.py:42-44) when neither --definition-id nor
// --definition-name is given, with no HTTP call at all.
func TestBuildQueueRequiresIDOrName(t *testing.T) {
	srv, got := buildTestServer(t)
	client := buildTestClient(t, srv)
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj"}
	cmd := buildTestCmd(t, newBuildQueueCmd(), "json", nil)

	err := buildQueue(context.Background(), cmd, client, dctx)
	const want = "Either the --definition-id argument or the --definition-name argument must be supplied for this command."
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if len(*got) != 0 {
		t.Errorf("expected no HTTP calls, got %d", len(*got))
	}
}

// TestBuildTagAddRouting covers the single-tag PUT vs multi-tag POST branch
// and the no-whitespace-trim comma split (build.py:156-160).
func TestBuildTagAddRouting(t *testing.T) {
	t.Run("single tag uses PUT with the tag in the path", func(t *testing.T) {
		srv, got := buildTestServer(t, func(r *buildCapturedRequest) (int, any) {
			return 200, map[string]any{"count": 1, "value": []any{"tag1"}}
		})
		client := buildTestClient(t, srv)
		dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj"}
		cmd := buildTestCmd(t, newBuildTagAddCmd(), "json", map[string]string{
			"build-id": "637",
			"tags":     "tag1",
		})

		if err := buildTagAdd(context.Background(), cmd, client, dctx); err != nil {
			t.Fatalf("buildTagAdd: %v", err)
		}

		reqs := *got
		if len(reqs) != 1 {
			t.Fatalf("want 1 request, got %d", len(reqs))
		}
		if reqs[0].Method != http.MethodPut || reqs[0].Path != "/myorg/myproj/_apis/build/builds/637/tags/tag1" {
			t.Errorf("request = %+v", reqs[0])
		}
	})

	t.Run("multiple tags uses POST with a JSON array body, no trim", func(t *testing.T) {
		srv, got := buildTestServer(t, func(r *buildCapturedRequest) (int, any) {
			return 200, map[string]any{"count": 2, "value": []any{"a", " b"}}
		})
		client := buildTestClient(t, srv)
		dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj"}
		cmd := buildTestCmd(t, newBuildTagAddCmd(), "json", map[string]string{
			"build-id": "637",
			"tags":     "a, b",
		})

		if err := buildTagAdd(context.Background(), cmd, client, dctx); err != nil {
			t.Fatalf("buildTagAdd: %v", err)
		}

		reqs := *got
		if len(reqs) != 1 {
			t.Fatalf("want 1 request, got %d", len(reqs))
		}
		if reqs[0].Method != http.MethodPost || reqs[0].Path != "/myorg/myproj/_apis/build/builds/637/tags" {
			t.Errorf("request = %+v", reqs[0])
		}
		tags, ok := reqs[0].Body.([]any)
		if !ok || len(tags) != 2 {
			t.Fatalf("body = %#v, want a 2-element array", reqs[0].Body)
		}
		if tags[0] != "a" || tags[1] != " b" {
			t.Errorf("tags = %#v, want [\"a\", \" b\"] (leading space preserved)", tags)
		}
	})
}

// TestBuildDefinitionListNoRShorthand covers that `pipelines build
// definition list --repository` has no `-r` alias: Python registers that
// shorthand only in the `devops`/`repos` argument contexts, not `pipelines`.
// TestBuildDefinitionColumns_DraftOnlyWhenPresent ports
// transform_definitions_table_output/transform_definition_table_output
// (_format.py:64-98): the Draft column only appears when at least one row
// being rendered has quality=="draft".
func TestBuildDefinitionColumns_DraftOnlyWhenPresent(t *testing.T) {
	hasHeader := func(cols []ado.Column, header string) bool {
		for _, c := range cols {
			if c.Header == header {
				return true
			}
		}
		return false
	}

	noDraft := buildDefinitionColumns([]map[string]any{{"id": 1, "quality": "definition"}})
	if hasHeader(noDraft, "Draft") {
		t.Errorf("columns = %+v, want no Draft column when no row is a draft", noDraft)
	}

	withDraft := buildDefinitionColumns([]map[string]any{
		{"id": 1, "quality": "definition"},
		{"id": 2, "quality": "draft"},
	})
	if !hasHeader(withDraft, "Draft") {
		t.Errorf("columns = %+v, want a Draft column when any row is a draft", withDraft)
	}
}

func TestBuildDefinitionListNoRShorthand(t *testing.T) {
	cmd := newBuildDefinitionListCmd()
	if f := cmd.Flags().ShorthandLookup("r"); f != nil {
		t.Errorf("-r is registered (for --%s), want no shorthand", f.Name)
	}
}

// TestBuildDefinitionList covers the repository-name -> id lookup call
// sequence and the always-forced repositoryType=TfsGit behaviour
// (build_definition.py:36-40) — --repository-type is accepted but has no
// effect on the request, matching the Python bug.
func TestBuildDefinitionList(t *testing.T) {
	srv, got := buildTestServer(t,
		// 1: GET git/repositories, to resolve "myrepo" -> its id
		func(r *buildCapturedRequest) (int, any) {
			return 200, map[string]any{"count": 1, "value": []any{
				map[string]any{"id": "repo-guid-1", "name": "myrepo"},
			}}
		},
		// 2: GET build/Definitions
		func(r *buildCapturedRequest) (int, any) {
			return 200, map[string]any{"count": 0, "value": []any{}}
		},
	)
	client := buildTestClient(t, srv)
	// dctx.Repo mirrors what ado.ResolveProject actually returns (either
	// the --repository flag or a git-detected repo); buildDefinitionList
	// reads dctx.Repo, not the raw flag, so tests must set both.
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj", Repo: "myrepo"}
	cmd := buildTestCmd(t, newBuildDefinitionListCmd(), "json", map[string]string{
		"repository":      "myrepo",
		"repository-type": "github", // accepted, must have no effect
	})

	if err := buildDefinitionList(context.Background(), cmd, client, dctx); err != nil {
		t.Fatalf("buildDefinitionList: %v", err)
	}

	reqs := *got
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests, got %d", len(reqs))
	}
	if reqs[0].Path != "/myorg/myproj/_apis/git/repositories" {
		t.Errorf("repository lookup path = %q", reqs[0].Path)
	}
	if reqs[1].Path != "/myorg/myproj/_apis/build/Definitions" {
		t.Errorf("definitions list path = %q", reqs[1].Path)
	}
	if reqs[1].Query != "api-version=5.0&queryOrder=DefinitionNameAscending&repositoryId=repo-guid-1&repositoryType=TfsGit" {
		t.Errorf("definitions list query = %q, want repositoryType forced to TfsGit regardless of --repository-type github", reqs[1].Query)
	}
}

// TestBuildDefinitionList_UsesGitDetectedRepo ports build_definition.py:32-33
// (resolve_instance_project_and_repo auto-detects the repo from the git
// remote when --repository is omitted, services.py:346-348): with no
// --repository flag, buildDefinitionList must still filter by dctx.Repo
// rather than listing the whole project.
func TestBuildDefinitionList_UsesGitDetectedRepo(t *testing.T) {
	srv, got := buildTestServer(t,
		func(r *buildCapturedRequest) (int, any) {
			return 200, map[string]any{"count": 1, "value": []any{
				map[string]any{"id": "repo-guid-1", "name": "detected-repo"},
			}}
		},
		func(r *buildCapturedRequest) (int, any) {
			return 200, map[string]any{"count": 0, "value": []any{}}
		},
	)
	client := buildTestClient(t, srv)
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj", Repo: "detected-repo"}
	cmd := buildTestCmd(t, newBuildDefinitionListCmd(), "json", nil)

	if err := buildDefinitionList(context.Background(), cmd, client, dctx); err != nil {
		t.Fatalf("buildDefinitionList: %v", err)
	}

	reqs := *got
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests (repository lookup, then filtered definitions list), got %d", len(reqs))
	}
	if reqs[1].Query != "api-version=5.0&queryOrder=DefinitionNameAscending&repositoryId=repo-guid-1&repositoryType=TfsGit" {
		t.Errorf("definitions list query = %q, want filtered by the git-detected repo", reqs[1].Query)
	}
}

// TestBuildList covers client-side dedup of --definition-ids/--tags and the
// exact query parameter names get_builds sends (definitions, tagFilters,
// branchName, resultFilter/statusFilter/reasonFilter, requestedFor).
func TestBuildList(t *testing.T) {
	srv, got := buildTestServer(t, func(r *buildCapturedRequest) (int, any) {
		return 200, map[string]any{"count": 0, "value": []any{}}
	})
	client := buildTestClient(t, srv)
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj"}
	cmd := buildTestCmd(t, newBuildListCmd(), "json", map[string]string{
		"branch": "main",
		"result": "succeeded",
		// a UUID short-circuits identity resolution (no extra HTTP call),
		// keeping this test's single recorded request meaningful; identity
		// resolution itself (email/alias/"me") is covered by
		// TestPipelinesResolveIdentityID in identity_test.go.
		"requested-for": "11111111-1111-1111-1111-111111111111",
	})
	// --definition-ids/--tags are repeatable StringArray flags
	// (build_list.go), not comma-splitting StringSlice: set each twice with
	// a duplicate to prove client-side dedup still runs.
	for _, v := range []string{"1", "2", "1"} {
		if err := cmd.Flags().Set("definition-ids", v); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []string{"x", "y", "x"} {
		if err := cmd.Flags().Set("tags", v); err != nil {
			t.Fatal(err)
		}
	}

	if err := buildList(context.Background(), cmd, client, dctx); err != nil {
		t.Fatalf("buildList: %v", err)
	}

	reqs := *got
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	want := "api-version=5.0&branchName=refs%2Fheads%2Fmain&definitions=1%2C2&requestedFor=11111111-1111-1111-1111-111111111111&resultFilter=succeeded&tagFilters=x%2Cy"
	if reqs[0].Query != want {
		t.Errorf("query = %q, want %q", reqs[0].Query, want)
	}
}

// TestBuildListDefinitionIDsRejectsNonInteger ports arguments.py:34's
// `type=int` on --definition-ids (argparse rejects a non-integer value at
// parse time instead of sending it straight through on the wire).
func TestBuildListDefinitionIDsRejectsNonInteger(t *testing.T) {
	srv, got := buildTestServer(t)
	client := buildTestClient(t, srv)
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj"}
	cmd := buildTestCmd(t, newBuildListCmd(), "json", nil)
	if err := cmd.Flags().Set("definition-ids", "abc"); err != nil {
		t.Fatal(err)
	}

	if err := buildList(context.Background(), cmd, client, dctx); err == nil {
		t.Fatal("expected an error for a non-integer --definition-ids value")
	}
	if len(*got) != 0 {
		t.Errorf("expected no HTTP calls, got %d", len(*got))
	}
}

// TestBuildListInvalidResult checks client-side enum validation returns
// before any HTTP call.
func TestBuildListInvalidResult(t *testing.T) {
	srv, got := buildTestServer(t)
	client := buildTestClient(t, srv)
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj"}
	cmd := buildTestCmd(t, newBuildListCmd(), "json", map[string]string{"result": "bogus"})

	if err := buildList(context.Background(), cmd, client, dctx); err == nil {
		t.Fatal("expected an error for an invalid --result value")
	}
	if len(*got) != 0 {
		t.Errorf("expected no HTTP calls, got %d", len(*got))
	}
}
