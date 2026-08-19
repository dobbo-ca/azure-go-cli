package pipelines

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// coreTestCmd builds a bare cobra command with just enough flags for
// ado.Print (output/query, via the error-discarding two-value form, so an
// absent flag behaves exactly like an empty one — json output, no query).
// Org/project resolution is bypassed entirely in these tests (see
// internal/devops/team_test.go for the same pattern): the coreX work
// functions take an already-resolved ado.Context, since ado.ResolveProject's
// org validation rejects a plain httptest URL.
func coreTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")
	return &cobra.Command{Use: "test"}
}

// coreRecordedRequest captures one inbound HTTP call for assertions.
type coreRecordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   map[string]any
}

func coreNewRecordingServer(t *testing.T, handler func(req coreRecordedRequest) any) (*httptest.Server, *[]coreRecordedRequest) {
	t.Helper()
	var reqs []coreRecordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &body)
		}
		// EscapedPath (not Path) so assertions see the on-wire percent-encoding
		// (e.g. "\" -> "%5C"), not net/url's decoded form.
		rr := coreRecordedRequest{Method: r.Method, Path: r.URL.EscapedPath(), Query: r.URL.RawQuery, Body: body}
		reqs = append(reqs, rr)

		w.Header().Set("Content-Type", "application/json")
		result := handler(rr)
		if result != nil {
			_ = json.NewEncoder(w).Encode(result)
		} else {
			w.Write([]byte("{}"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs
}

func TestFolderCreate_RequestShape(t *testing.T) {
	srv, reqs := coreNewRecordingServer(t, func(req coreRecordedRequest) any {
		return map[string]any{"path": req.Body["path"], "description": req.Body["description"]}
	})

	dctx := ado.Context{Org: srv.URL, Project: "proj"}
	if err := coreFolderCreate(context.Background(), coreTestCmd(t), dctx, `\MyFolder`, "desc"); err != nil {
		t.Fatalf("coreFolderCreate: %v", err)
	}

	if len(*reqs) != 1 {
		t.Fatalf("got %d requests, want 1", len(*reqs))
	}
	got := (*reqs)[0]
	if got.Method != http.MethodPut {
		t.Errorf("Method = %q, want PUT", got.Method)
	}
	if want := "/proj/_apis/build/folders/%5CMyFolder"; got.Path != want {
		t.Errorf("Path = %q, want %q", got.Path, want)
	}
	if want := "api-version=5.0-preview.2"; got.Query != want {
		t.Errorf("Query = %q, want %q", got.Query, want)
	}
	if got.Body["path"] != `\MyFolder` || got.Body["description"] != "desc" {
		t.Errorf("Body = %+v", got.Body)
	}
}

// TestFolderCreate_OmitsEmptyDescription ports pipeline_folders.py:29-31:
// folder.description stays None (dropped by msrest) when --description is
// omitted, so the request body must not carry a "description" key at all,
// not an explicit "".
func TestFolderCreate_OmitsEmptyDescription(t *testing.T) {
	srv, reqs := coreNewRecordingServer(t, func(req coreRecordedRequest) any {
		return map[string]any{"path": req.Body["path"]}
	})

	dctx := ado.Context{Org: srv.URL, Project: "proj"}
	if err := coreFolderCreate(context.Background(), coreTestCmd(t), dctx, `\MyFolder`, ""); err != nil {
		t.Fatalf("coreFolderCreate: %v", err)
	}

	got := (*reqs)[0]
	if _, ok := got.Body["description"]; ok {
		t.Errorf("Body = %+v, want no \"description\" key", got.Body)
	}
}

// TestFolderColumns_EmptyDescriptionStaysBlankCell covers the ado.Column
// Value convention: _transform_pipeline_folder_row (_format.py:392-401)
// always assigns table_row['Description'] (falling back to an empty string
// for None), so a folder with no description must render as " " (a kept,
// blank cell), not "" -- which ado.Print's cellValue would instead treat as
// "omit this column from this row", dropping the whole Description column
// from a `folder list` where every folder happens to lack one.
func TestFolderColumns_EmptyDescriptionStaysBlankCell(t *testing.T) {
	cols := coreFolderColumns()
	var desc ado.Column
	for _, c := range cols {
		if c.Header == "Description" {
			desc = c
		}
	}
	if desc.Value == nil {
		t.Fatalf("coreFolderColumns() has no Description column")
	}
	for _, row := range []map[string]any{
		{"path": `\MyFolder`},
		{"path": `\MyFolder`, "description": nil},
		{"path": `\MyFolder`, "description": ""},
	} {
		if got := desc.Value(row); got != " " {
			t.Errorf("Description.Value(%+v) = %q, want %q", row, got, " ")
		}
	}
}

func TestFolderUpdate_ListThenExactMatchThenPost(t *testing.T) {
	srv, reqs := coreNewRecordingServer(t, func(req coreRecordedRequest) any {
		if req.Method == http.MethodGet {
			return map[string]any{
				"count": 2,
				"value": []map[string]any{
					{"path": `\Other`, "description": "other"},
					{"path": `\MyFolder\Sub`, "description": "old"},
				},
			}
		}
		return req.Body
	})

	dctx := ado.Context{Org: srv.URL, Project: "proj"}
	if err := coreFolderUpdate(context.Background(), coreTestCmd(t), dctx, `\MyFolder\Sub\`, "", "new description"); err != nil {
		t.Fatalf("coreFolderUpdate: %v", err)
	}

	if len(*reqs) != 2 {
		t.Fatalf("got %d requests, want 2 (GET list, POST update)", len(*reqs))
	}

	list := (*reqs)[0]
	if list.Method != http.MethodGet || list.Query != "api-version=5.0-preview.2&queryOrder=folderAscending" {
		t.Errorf("list request = %+v", list)
	}

	update := (*reqs)[1]
	if update.Method != http.MethodPost {
		t.Errorf("Method = %q, want POST", update.Method)
	}
	// POST goes to the ORIGINAL path route value, not the (absent, since only
	// description changed) new path.
	if want := "/proj/_apis/build/folders/%5CMyFolder%5CSub%5C"; update.Path != want {
		t.Errorf("Path = %q, want %q", update.Path, want)
	}
	if update.Body["path"] != `\MyFolder\Sub` || update.Body["description"] != "new description" {
		t.Errorf("posted body = %+v, want the matched folder mutated in place", update.Body)
	}
}

func TestFolderUpdate_NoMatchErrors(t *testing.T) {
	srv, _ := coreNewRecordingServer(t, func(req coreRecordedRequest) any {
		return map[string]any{"count": 0, "value": []map[string]any{}}
	})

	dctx := ado.Context{Org: srv.URL, Project: "proj"}
	err := coreFolderUpdate(context.Background(), coreTestCmd(t), dctx, `\NoSuchFolder`, "\\NewPath", "")
	if err == nil {
		t.Fatal("expected an error when no folder matches")
	}
}

func TestRunPipeline_QueueBuildPath(t *testing.T) {
	srv, reqs := coreNewRecordingServer(t, func(req coreRecordedRequest) any {
		return map[string]any{"id": 99, "buildNumber": "20260101.1"}
	})

	dctx := ado.Context{Org: srv.URL, Project: "proj"}
	err := coreRunPipeline(context.Background(), coreTestCmd(t), dctx, 42, "", "master", "", "", []string{"foo=bar"}, nil, false)
	if err != nil {
		t.Fatalf("coreRunPipeline: %v", err)
	}

	if len(*reqs) != 1 {
		t.Fatalf("got %d requests, want 1", len(*reqs))
	}
	got := (*reqs)[0]
	if got.Method != http.MethodPost {
		t.Errorf("Method = %q, want POST", got.Method)
	}
	if want := "/proj/_apis/build/Builds"; got.Path != want {
		t.Errorf("Path = %q, want %q", got.Path, want)
	}
	if got.Query != "api-version=5.0" {
		t.Errorf("Query = %q, want api-version=5.0", got.Query)
	}
	def, _ := got.Body["definition"].(map[string]any)
	if def == nil || def["id"] != float64(42) {
		t.Errorf("definition = %+v, want {id:42}", def)
	}
	// resolve_git_ref_heads: bare branch name gets the refs/heads/ prefix.
	if got.Body["sourceBranch"] != "refs/heads/master" {
		t.Errorf("sourceBranch = %v, want refs/heads/master", got.Body["sourceBranch"])
	}
	// Build.parameters is a JSON-encoded STRING field, not a raw object.
	paramsStr, ok := got.Body["parameters"].(string)
	if !ok {
		t.Fatalf("parameters = %#v, want a JSON string", got.Body["parameters"])
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(paramsStr), &decoded); err != nil {
		t.Fatalf("parameters did not decode as JSON: %v", err)
	}
	if decoded["foo"] != "bar" {
		t.Errorf("decoded parameters = %+v, want {foo: bar}", decoded)
	}
}

func TestRunPipeline_ParametersPathUsesPipelinesAPI(t *testing.T) {
	srv, reqs := coreNewRecordingServer(t, func(req coreRecordedRequest) any {
		return map[string]any{"id": 7, "state": "inProgress"}
	})

	dctx := ado.Context{Org: srv.URL, Project: "proj"}
	err := coreRunPipeline(context.Background(), coreTestCmd(t), dctx, 42, "", "refs/heads/main", "", "abc123", []string{"v1=1"}, []string{"p1=hello"}, false)
	if err != nil {
		t.Fatalf("coreRunPipeline: %v", err)
	}

	if len(*reqs) != 1 {
		t.Fatalf("got %d requests, want 1", len(*reqs))
	}
	got := (*reqs)[0]
	if want := "/proj/_apis/pipelines/42/runs"; got.Path != want {
		t.Errorf("Path = %q, want %q", got.Path, want)
	}
	if got.Query != "api-version=6.0-preview.1" {
		t.Errorf("Query = %q, want api-version=6.0-preview.1", got.Query)
	}
	resources, _ := got.Body["resources"].(map[string]any)
	repos, _ := resources["repositories"].(map[string]any)
	self, _ := repos["self"].(map[string]any)
	// Path A does NOT normalise the branch through resolve_git_ref_heads.
	if self["refName"] != "refs/heads/main" {
		t.Errorf("refName = %v, want refs/heads/main (raw, unnormalised)", self["refName"])
	}
	if self["version"] != "abc123" {
		t.Errorf("version = %v, want abc123", self["version"])
	}
	templateParams, _ := got.Body["templateParameters"].(map[string]any)
	if templateParams["p1"] != "hello" {
		t.Errorf("templateParameters = %+v", templateParams)
	}
	// --variables on this path is wrapped as {"value": v}, unlike path B's raw string.
	variables, _ := got.Body["variables"].(map[string]any)
	v1, _ := variables["v1"].(map[string]any)
	if v1["value"] != "1" {
		t.Errorf("variables = %+v, want v1.value == \"1\"", variables)
	}
}

func TestList_RepositoryNameResolvedToID(t *testing.T) {
	srv, reqs := coreNewRecordingServer(t, func(req coreRecordedRequest) any {
		if req.Path == "/proj/_apis/git/repositories" {
			return map[string]any{"count": 1, "value": []map[string]any{{"id": "repo-guid", "name": "MyRepo"}}}
		}
		return map[string]any{"count": 0, "value": []map[string]any{}}
	})

	dctx := ado.Context{Org: srv.URL, Project: "proj"}
	if err := coreList(context.Background(), coreTestCmd(t), dctx, "", 0, "", "myrepo", "", ""); err != nil {
		t.Fatalf("coreList: %v", err)
	}

	if len(*reqs) != 2 {
		t.Fatalf("got %d requests, want 2 (repository lookup, then definitions list)", len(*reqs))
	}
	lookup := (*reqs)[0]
	if lookup.Path != "/proj/_apis/git/repositories" {
		t.Errorf("first request Path = %q", lookup.Path)
	}

	list := (*reqs)[1]
	if list.Path != "/proj/_apis/build/Definitions" {
		t.Errorf("second request Path = %q", list.Path)
	}
	if list.Query != "api-version=5.0&queryOrder=none&repositoryId=repo-guid&repositoryType=TfsGit" {
		t.Errorf("Query = %q", list.Query)
	}
}

// TestCoreRunColumns_QueuedTimeIsLocalNotRaw ports _format.py:250-251:
// `pipelines run`/`pipelines create`'s Queued Time cell must render local
// "YYYY-MM-DD HH:MM:SS", not the raw ISO-8601 UTC string.
func TestCoreRunColumns_QueuedTimeIsLocalNotRaw(t *testing.T) {
	cols := coreRunColumns()
	var cell ado.Column
	for _, c := range cols {
		if c.Header == "Queued Time" {
			cell = c
		}
	}
	if cell.Value == nil {
		t.Fatal("no Queued Time column")
	}
	got := cell.Value(map[string]any{"queueTime": "2021-01-02T03:04:05.6Z"})
	if got == "2021-01-02T03:04:05.6Z" {
		t.Errorf("got the raw ISO string %q, want it converted to local time", got)
	}
}

// TestCoreTruncate_RuneSafe covers coreTruncate slicing by rune (code
// point), not byte: a multi-byte-rune string truncated at a byte boundary
// would split a rune and render invalid UTF-8 / U+FFFD.
func TestCoreTruncate_RuneSafe(t *testing.T) {
	s := strings.Repeat("é", 10) // each 'é' is 2 bytes in UTF-8
	got := coreTruncate(s, 5, "..")
	want := strings.Repeat("é", 5) + ".."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !utf8.ValidString(got) {
		t.Errorf("got %q, not valid UTF-8", got)
	}
}

// TestCoreValidateChoice covers coreValidateChoice's case-insensitivity and
// canonical-casing normalisation, matching knack's enum_choice_list.
func TestCoreValidateChoice(t *testing.T) {
	got, err := coreValidateChoice("nameasc", "query-order", coreQueryOrderChoices)
	if err != nil || got != "NameAsc" {
		t.Errorf("got=%q err=%v, want NameAsc/nil", got, err)
	}

	if _, err := coreValidateChoice("bogus", "query-order", coreQueryOrderChoices); err == nil {
		t.Fatal("expected an error for an unrecognised --query-order value")
	}

	got, err = coreValidateChoice("", "query-order", coreQueryOrderChoices)
	if err != nil || got != "" {
		t.Errorf("got=%q err=%v, want empty/nil (unset is always allowed)", got, err)
	}
}

// TestList_InvalidQueryOrderErrors and TestList_InvalidRepositoryTypeErrors
// cover `pipelines list`'s choices validation (arguments.py:75-79), which
// this port previously lacked: bogus values fell through
// coreResolveQueryOrder's silent "none" default instead of erroring.
func TestList_InvalidQueryOrderErrors(t *testing.T) {
	dctx := ado.Context{Org: "https://dev.azure.com/org", Project: "proj"}
	err := coreList(context.Background(), coreTestCmd(t), dctx, "", 0, "bogus", "", "", "")
	if err == nil {
		t.Fatal("expected an error for an invalid --query-order value")
	}
}

func TestList_InvalidRepositoryTypeErrors(t *testing.T) {
	dctx := ado.Context{Org: "https://dev.azure.com/org", Project: "proj"}
	err := coreList(context.Background(), coreTestCmd(t), dctx, "", 0, "", "myrepo", "", "bogus")
	if err == nil {
		t.Fatal("expected an error for an invalid --repository-type value")
	}
}

// TestFolderList_InvalidQueryOrderErrors covers `pipelines folder list`'s
// choices validation (arguments.py:28,121).
func TestFolderList_InvalidQueryOrderErrors(t *testing.T) {
	dctx := ado.Context{Org: "https://dev.azure.com/org", Project: "proj"}
	err := coreFolderList(context.Background(), coreTestCmd(t), dctx, "", "bogus")
	if err == nil {
		t.Fatal("expected an error for an invalid --query-order value")
	}
}

// TestCreate_InvalidRepositoryTypeErrors and
// TestCreate_RepositoryTypeCaseInsensitive cover `pipelines create
// --repository-type`'s narrower choices (arguments.py:81-82: only
// tfsgit/github) plus str.lower normalisation.
func TestCreate_InvalidRepositoryTypeErrors(t *testing.T) {
	dctx := ado.Context{Org: "https://dev.azure.com/org", Project: "proj"}
	err := coreCreate(context.Background(), coreTestCmd(t), dctx, "p", "", "owner/repo", "main",
		"azure-pipelines.yml", "github enterprise", "sc1", "q1", "", true)
	if err == nil {
		t.Fatal("expected an error for a --repository-type value outside tfsgit/github")
	}
}

// TestCreate_ExplicitRepositoryNotForcedToAzureRepo ports the
// pipeline_create.py:83-93 behaviour: repository_name (and the resulting
// TfsGit override) is only derived from git-remote detection, never from an
// explicitly-passed --repository. ado/context.go's resolve() echoes the
// --repository flag back onto dctx.Repo, so coreCreate must not treat that
// echo as "detected" or it forces repositoryType=TfsGit and skips the
// GitHub repo id/URL wiring below, even with --repository-type github.
func TestCreate_ExplicitRepositoryNotForcedToAzureRepo(t *testing.T) {
	srv, reqs := coreNewRecordingServer(t, func(req coreRecordedRequest) any {
		if req.Method == http.MethodGet {
			return map[string]any{"count": 0, "value": []map[string]any{}}
		}
		return map[string]any{"id": 42, "name": "mypipeline"}
	})

	// dctx.Repo mirrors what ado.ResolveProject actually returns when
	// --repository is passed explicitly: the flag's own value, not a
	// git-detected one.
	dctx := ado.Context{Org: srv.URL, Project: "proj", Repo: "owner/ghrepo"}
	cmd := coreTestCmd(t)
	cmd.Flags().String("repository", "", "")
	if err := cmd.Flags().Set("repository", "owner/ghrepo"); err != nil {
		t.Fatal(err)
	}

	err := coreCreate(context.Background(), cmd, dctx, "mypipeline", "", "owner/ghrepo", "main",
		"azure-pipelines.yml", "github", "sc1", "q1", "", true)
	if err != nil {
		t.Fatalf("coreCreate: %v", err)
	}

	if len(*reqs) != 2 {
		t.Fatalf("got %d requests, want 2 (name-availability GET, definition POST); "+
			"a wrongly-forced TfsGit type would add a git/repositories lookup", len(*reqs))
	}
	created := (*reqs)[1].Body
	repo, _ := created["repository"].(map[string]any)
	if repo["type"] != "GitHub" {
		t.Errorf("repository.type = %v, want GitHub (must not be forced to TfsGit for an explicit --repository)", repo["type"])
	}
}

// TestCreate_DetectFalseSkipsGithubRemoteDetection ports
// pipeline_create.py:96,208-211 (_get_repository_url_from_local_repo only
// calls get_remote_url when should_detect(detect)). --detect false plus no
// --repository must error, never fall back to scanning the local git
// remotes for a GitHub URL — this repo's own origin remote is a real
// github.com URL, so the un-gated call would silently pick it up.
func TestCreate_DetectFalseSkipsGithubRemoteDetection(t *testing.T) {
	dctx := ado.Context{Org: "https://dev.azure.com/org", Project: "proj"}
	cmd := coreTestCmd(t)
	cmd.Flags().String("detect", "", "")
	if err := cmd.Flags().Set("detect", "false"); err != nil {
		t.Fatal(err)
	}

	err := coreCreate(context.Background(), cmd, dctx, "mypipeline", "", "", "main",
		"azure-pipelines.yml", "", "sc1", "q1", "", true)
	if err == nil {
		t.Fatal("expected an error requiring --repository, got nil (github-remote auto-detection ran despite --detect false)")
	}
	if want := "the following arguments are required: --repository"; err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
}
