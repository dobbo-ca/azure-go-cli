package boards

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
)

// areaiterationTestRequest is one recorded request against the fake server.
type areaiterationTestRequest struct {
	Method string
	URL    string // raw request-target: escaped path + query
	Body   map[string]any
}

// areaiterationTestServer serves canned JSON responses keyed by request
// path+query prefix match against handler, recording every request it sees.
// Mirrors internal/devops/team_test.go's teamTestServer idiom.
func areaiterationTestServer(t *testing.T, handler func(r *http.Request) (int, string)) (*ado.Client, *[]areaiterationTestRequest) {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	var requests []areaiterationTestRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := areaiterationTestRequest{Method: r.Method, URL: r.URL.RequestURI()}
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			if len(b) > 0 {
				_ = json.Unmarshal(b, &req.Body)
			}
		}
		requests = append(requests, req)

		status, body := handler(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	client, err := ado.NewClient(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ado.NewClient: %v", err)
	}
	return client, &requests
}

// TestResolveClassificationNodePath covers the shared path-prefix-strip
// helper every project-scoped area/iteration command with --path routes
// through (boards_helper.py:12-21).
func TestResolveClassificationNodePath(t *testing.T) {
	tests := []struct {
		name           string
		structureGroup string
		path           string
		want           string
		wantErr        bool
	}{
		{
			name:           "iteration path strips iteration root",
			structureGroup: areaiterationStructureGroupIteration,
			path:           `\MyProject\Iteration\Sprint 1`,
			want:           `\Sprint 1`,
		},
		{
			name:           "area path strips area root case-insensitively",
			structureGroup: areaiterationStructureGroupArea,
			path:           `\MYPROJECT\Team A`,
			want:           `\Team A`,
		},
		{
			name:           "path not under the resolved root errors",
			structureGroup: areaiterationStructureGroupIteration,
			path:           `\SomeOtherProject\Iteration\Sprint 1`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootNodes := `{"count":2,"value":[
				{"structureType":"area","path":"\\MyProject"},
				{"structureType":"iteration","path":"\\MyProject\\Iteration"}
			]}`
			client, requests := areaiterationTestServer(t, func(r *http.Request) (int, string) {
				return http.StatusOK, rootNodes
			})

			got, err := areaiterationResolveClassificationNodePath(context.Background(), client, "MyProject", tt.structureGroup, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (resolved %q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolved path = %q, want %q", got, tt.want)
			}

			wantURL := "/MyProject/_apis/wit/classificationnodes?%24depth=0&api-version=5.0"
			if (*requests)[0].URL != wantURL {
				t.Errorf("root-nodes URL = %q, want %q", (*requests)[0].URL, wantURL)
			}
		})
	}
}

// TestProjectList_WithPath covers the two-call sequence (root-nodes fetch
// to resolve --path, then the depth-scoped GET) and that $depth is carried
// through as a query parameter.
func TestProjectList_WithPath(t *testing.T) {
	rootNodes := `{"count":1,"value":[{"structureType":"iteration","path":"\\Proj\\Iteration"}]}`
	node := `{"id":1,"name":"Sprint 1","path":"\\Proj\\Iteration\\Sprint 1","hasChildren":false,"children":[]}`

	call := 0
	client, requests := areaiterationTestServer(t, func(r *http.Request) (int, string) {
		call++
		if call == 1 {
			return http.StatusOK, rootNodes
		}
		return http.StatusOK, node
	})

	cmd := areaiterationNewProjectListCmd(areaiterationStructureGroupIteration)
	cmd.Flags().Set("path", `\Proj\Iteration\Sprint 1`)
	cmd.Flags().Set("depth", "2")
	cmd.Flags().Set("output", "json")

	if err := areaiterationProjectList(context.Background(), cmd, client, "Proj", areaiterationStructureGroupIteration); err != nil {
		t.Fatalf("areaiterationProjectList: %v", err)
	}

	if len(*requests) != 2 {
		t.Fatalf("got %d requests, want 2", len(*requests))
	}
	wantRoot := "/Proj/_apis/wit/classificationnodes?%24depth=0&api-version=5.0"
	if (*requests)[0].URL != wantRoot {
		t.Errorf("request 1 URL = %q, want %q", (*requests)[0].URL, wantRoot)
	}
	wantNode := "/Proj/_apis/wit/classificationnodes/iterations/%5CSprint%201?%24depth=2&api-version=5.0"
	if (*requests)[1].URL != wantNode {
		t.Errorf("request 2 URL = %q, want %q", (*requests)[1].URL, wantNode)
	}
}

// TestIterationTeamListWorkItems_FriendlyNames covers the two-call sequence
// (iteration work items, then relation types) and the client-side rel
// substitution (iteration.py:286-293).
func TestIterationTeamListWorkItems_FriendlyNames(t *testing.T) {
	workItems := `{"workItemRelations":[
		{"rel":"System.LinkTypes.Hierarchy-Forward","source":null,"target":{"id":7}},
		{"rel":"System.LinkTypes.Hierarchy-Reverse","source":{"id":7},"target":{"id":9}}
	]}`
	relationTypes := `{"count":2,"value":[
		{"referenceName":"System.LinkTypes.Hierarchy-Forward","name":"Child"},
		{"referenceName":"System.LinkTypes.Hierarchy-Reverse","name":"Parent"}
	]}`

	client, requests := areaiterationTestServer(t, func(r *http.Request) (int, string) {
		if r.URL.Path == "/_apis/wit/workitemrelationtypes" {
			return http.StatusOK, relationTypes
		}
		return http.StatusOK, workItems
	})

	result, err := areaiterationIterationTeamListWorkItems(context.Background(), client, "Proj", "MyTeam", "iter-1")
	if err != nil {
		t.Fatalf("areaiterationIterationTeamListWorkItems: %v", err)
	}

	if len(*requests) != 2 {
		t.Fatalf("got %d requests, want 2", len(*requests))
	}
	wantFirst := "/Proj/MyTeam/_apis/work/teamsettings/iterations/iter-1/workitems?api-version=5.0-preview.1"
	if (*requests)[0].URL != wantFirst {
		t.Errorf("request 1 URL = %q, want %q", (*requests)[0].URL, wantFirst)
	}
	wantSecond := "/_apis/wit/workitemrelationtypes?api-version=5.0"
	if (*requests)[1].URL != wantSecond {
		t.Errorf("request 2 URL = %q, want %q", (*requests)[1].URL, wantSecond)
	}

	relations, _ := result["workItemRelations"].([]any)
	if len(relations) != 2 {
		t.Fatalf("got %d relations, want 2", len(relations))
	}
	rel0 := relations[0].(map[string]any)
	if rel0["rel"] != "Child" {
		t.Errorf("relations[0].rel = %v, want %q", rel0["rel"], "Child")
	}
	rel1 := relations[1].(map[string]any)
	if rel1["rel"] != "Parent" {
		t.Errorf("relations[1].rel = %v, want %q", rel1["rel"], "Parent")
	}
}

// TestAreaTeamAdd_BodyConstruction covers the GET-then-PATCH sequence and
// area.py:141-151's body construction: values appended (not replaced),
// includeChildren defaulting to false when --include-sub-areas is unset,
// and defaultValue only changing when --set-as-default is passed.
func TestAreaTeamAdd_BodyConstruction(t *testing.T) {
	tests := []struct {
		name             string
		includeSubAreas  bool
		setAsDefault     bool
		wantDefaultValue string
	}{
		{name: "default false, keeps existing default", includeSubAreas: false, setAsDefault: false, wantDefaultValue: "\\Proj\\Existing"},
		{name: "set-as-default overrides default", includeSubAreas: true, setAsDefault: true, wantDefaultValue: "\\Proj\\New"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := `{"defaultValue":"\\Proj\\Existing","field":{"referenceName":"System.AreaPath"},"values":[{"value":"\\Proj\\Existing","includeChildren":false}]}`
			var patchBody map[string]any
			call := 0
			client, requests := areaiterationTestServer(t, func(r *http.Request) (int, string) {
				call++
				if call == 1 {
					return http.StatusOK, current
				}
				return http.StatusOK, `{}`
			})

			result, err := areaiterationAreaTeamAdd(context.Background(), client, "Proj", "MyTeam", "\\Proj\\New", tt.includeSubAreas, tt.setAsDefault)
			if err != nil {
				t.Fatalf("areaiterationAreaTeamAdd: %v", err)
			}
			_ = result

			if len(*requests) != 2 {
				t.Fatalf("got %d requests, want 2", len(*requests))
			}
			if (*requests)[0].Method != http.MethodGet || (*requests)[1].Method != http.MethodPatch {
				t.Fatalf("methods = %q, %q, want GET, PATCH", (*requests)[0].Method, (*requests)[1].Method)
			}
			patchBody = (*requests)[1].Body

			values, _ := patchBody["values"].([]any)
			if len(values) != 2 {
				t.Fatalf("patch values length = %d, want 2 (existing + appended)", len(values))
			}
			newEntry := values[1].(map[string]any)
			if newEntry["value"] != "\\Proj\\New" {
				t.Errorf("new entry value = %v, want %q", newEntry["value"], "\\Proj\\New")
			}
			if newEntry["includeChildren"] != tt.includeSubAreas {
				t.Errorf("new entry includeChildren = %v, want %v", newEntry["includeChildren"], tt.includeSubAreas)
			}
			if patchBody["defaultValue"] != tt.wantDefaultValue {
				t.Errorf("patch defaultValue = %v, want %q", patchBody["defaultValue"], tt.wantDefaultValue)
			}
		})
	}
}

// TestAreaTeamRemove_Behaviour covers area.py:159-187: the leading-backslash
// strip callers must apply before calling in, the default-area removal
// guard (no PATCH sent), and filtering the matched entry out of the PATCH
// body.
func TestAreaTeamRemove_Behaviour(t *testing.T) {
	t.Run("removing the default area is refused before any PATCH", func(t *testing.T) {
		current := `{"defaultValue":"\\Proj\\A","values":[{"value":"\\Proj\\A","includeChildren":false}]}`
		client, requests := areaiterationTestServer(t, func(r *http.Request) (int, string) {
			return http.StatusOK, current
		})

		_, err := areaiterationAreaTeamRemove(context.Background(), client, "Proj", "MyTeam", "\\Proj\\A")
		if err == nil {
			t.Fatal("want error, got nil")
		}
		if len(*requests) != 1 {
			t.Fatalf("got %d requests, want 1 (GET only, no PATCH)", len(*requests))
		}
	})

	t.Run("matching entry removed from PATCH body", func(t *testing.T) {
		current := `{"defaultValue":"\\Proj\\A","values":[
			{"value":"\\Proj\\A","includeChildren":false},
			{"value":"\\Proj\\B","includeChildren":true}
		]}`
		call := 0
		client, requests := areaiterationTestServer(t, func(r *http.Request) (int, string) {
			call++
			if call == 1 {
				return http.StatusOK, current
			}
			return http.StatusOK, `{}`
		})

		if _, err := areaiterationAreaTeamRemove(context.Background(), client, "Proj", "MyTeam", "\\Proj\\B"); err != nil {
			t.Fatalf("areaiterationAreaTeamRemove: %v", err)
		}

		patchBody := (*requests)[1].Body
		values, _ := patchBody["values"].([]any)
		if len(values) != 1 {
			t.Fatalf("patch values length = %d, want 1", len(values))
		}
		remaining := values[0].(map[string]any)
		if remaining["value"] != "\\Proj\\A" {
			t.Errorf("remaining entry = %v, want the \\Proj\\A entry kept", remaining["value"])
		}
	})

	t.Run("no match errors without a PATCH", func(t *testing.T) {
		current := `{"defaultValue":"\\Proj\\A","values":[{"value":"\\Proj\\A","includeChildren":false}]}`
		client, requests := areaiterationTestServer(t, func(r *http.Request) (int, string) {
			return http.StatusOK, current
		})

		_, err := areaiterationAreaTeamRemove(context.Background(), client, "Proj", "MyTeam", "\\Proj\\NotThere")
		if err == nil || err.Error() != "Path is not added to team area list." {
			t.Fatalf("err = %v, want the CLIError text", err)
		}
		if len(*requests) != 1 {
			t.Fatalf("got %d requests, want 1 (GET only, no PATCH)", len(*requests))
		}
	})
}

// TestAreaiterationLeafCmds_RejectStrayPositionalArgs is B-2: argparse
// errors on an unrecognized positional ("unrecognized arguments: junk"),
// but with no leaf command setting Args, cobra silently ignored one. This
// is also what let B-1's "--include-sub-areas false" silently invert the
// request instead of erroring.
func TestAreaiterationLeafCmds_RejectStrayPositionalArgs(t *testing.T) {
	cmd := areaiterationNewProjectListCmd(areaiterationStructureGroupArea)
	cmd.SetArgs([]string{"--project", "P", "--organization", "https://dev.azure.com/x", "junk"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute: want an error for the stray positional \"junk\", got nil")
	}
}

// TestAreaTeamAddCmd_IncludeSubAreasSpaceSeparatedErrors covers B-1: pflag
// can't disambiguate "--include-sub-areas false" (space separated) from a
// stray positional the way argparse's nargs='?' does (get_three_state_flag,
// azure/cli/core/commands/parameters.py:187-191) -- NoOptDefVal always wins
// over a following bare token (pflag flag.go:1017-1019), so "false" is left
// as a positional arg rather than the flag's value. Args: cobra.NoArgs (B-2)
// turns that into a hard, loud error instead of silently writing
// includeChildren=true -- the inverse of what was asked. Only the "=false"
// form (documented in the flag's help text) is supported.
func TestAreaTeamAddCmd_IncludeSubAreasSpaceSeparatedErrors(t *testing.T) {
	cmd := areaiterationNewAreaTeamAddCmd()
	cmd.SetArgs([]string{
		"--team", "T", "--path", `\P\A`, "--organization", "https://dev.azure.com/x",
		"--include-sub-areas", "false",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute: want an error for the unsupported space-separated form, got nil")
	}
}

// TestAreaiterationIterationCreateBody_AttributesAlwaysPresent is B-3:
// iteration.py:135-137 assigns attributes={} unconditionally on a fresh
// node, so create always POSTs "attributes" even with no dates given.
func TestAreaiterationIterationCreateBody_AttributesAlwaysPresent(t *testing.T) {
	t.Run("no dates: attributes is an empty object, not omitted", func(t *testing.T) {
		body, err := areaiterationIterationCreateBody("Sprint 1", "", "")
		if err != nil {
			t.Fatalf("areaiterationIterationCreateBody: %v", err)
		}
		attrs, ok := body["attributes"].(map[string]any)
		if !ok {
			t.Fatalf(`body["attributes"] = %#v, want an empty map`, body["attributes"])
		}
		if len(attrs) != 0 {
			t.Errorf("attributes = %v, want empty", attrs)
		}
	})

	t.Run("both dates: attributes carries startDate/finishDate", func(t *testing.T) {
		body, err := areaiterationIterationCreateBody("Sprint 1", "2019-06-03", "2019-06-21")
		if err != nil {
			t.Fatalf("areaiterationIterationCreateBody: %v", err)
		}
		attrs, ok := body["attributes"].(map[string]any)
		if !ok || attrs["startDate"] == nil || attrs["finishDate"] == nil {
			t.Errorf("attributes = %#v, want startDate/finishDate set", body["attributes"])
		}
	})
}

// TestAreaiterationParseDate_PreservesOffset is B-5: arguments.py:40 formats
// whatever dateutil.parser.parse returned -- an input's UTC offset survives
// into the stored value ("+00:00", not stripped), while a naive input stays
// naive. Losing the offset previously reformatted "...T10:00:00Z" down to
// "...T10:00:00", silently shifting what the stored instant means whenever
// the caller isn't in UTC.
func TestAreaiterationParseDate_PreservesOffset(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "2019-06-03", want: "2019-06-03T00:00:00"},
		{in: "2019-06-03T10:00:00Z", want: "2019-06-03T10:00:00+00:00"},
		{in: "2019-06-03T10:00:00+05:30", want: "2019-06-03T10:00:00+05:30"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := areaiterationParseDate(tt.in, "start_date")
			if err != nil {
				t.Fatalf("areaiterationParseDate(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("areaiterationParseDate(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestAreaiterationVisibleColumns covers the shared column-filtering helper
// behind B-7/T6/T7: a Field column is dropped only when every row's
// resolved value is nil/""/false, and a Value column is never dropped
// (mirrors Python's tabulate(headers='keys') over per-row OrderedDicts that
// omit a key entirely when the source is null/falsy, _format.py:138,162-164,
// vs. keys set unconditionally like _format.py:166,217).
func TestAreaiterationVisibleColumns(t *testing.T) {
	cols := []ado.Column{
		{Header: "ID", Field: "id"},
		{Header: "Start Date", Field: "attributes.startDate"},
	}

	t.Run("drops a column no row has data for", func(t *testing.T) {
		rows := []map[string]any{{"id": float64(1)}}
		got := areaiterationVisibleColumns(rows, cols)
		if len(got) != 1 || got[0].Header != "ID" {
			t.Errorf("got %v, want just ID", got)
		}
	})

	t.Run("keeps a column when any row has data", func(t *testing.T) {
		rows := []map[string]any{
			{"id": float64(1)},
			{"id": float64(2), "attributes": map[string]any{"startDate": "2019-06-03"}},
		}
		got := areaiterationVisibleColumns(rows, cols)
		if len(got) != 2 {
			t.Errorf("got %v, want both columns kept", got)
		}
	})

	t.Run("Value-func columns are never dropped", func(t *testing.T) {
		valCol := []ado.Column{{Header: "Has Children", Value: func(row map[string]any) string { return "False" }}}
		rows := []map[string]any{{"hasChildren": false}}
		got := areaiterationVisibleColumns(rows, valCol)
		if len(got) != 1 {
			t.Errorf("got %v, want the Value column kept", got)
		}
	})
}

// TestAreaiterationClassificationColumns_T6 is T6: an area node (no
// "attributes" key at all) drops Start Date/Finish Date, but Has Children
// survives even though Python sets it unconditionally (_format.py:217) and
// every row's value is the falsy `false`.
func TestAreaiterationClassificationColumns_T6(t *testing.T) {
	rows := []map[string]any{{"id": float64(1), "hasChildren": false}}
	got := areaiterationVisibleColumns(rows, areaiterationClassificationColumns)

	var haveStart, haveFinish, haveHasChildren bool
	for _, c := range got {
		switch c.Header {
		case "Start Date":
			haveStart = true
		case "Finish Date":
			haveFinish = true
		case "Has Children":
			haveHasChildren = true
		}
	}
	if haveStart || haveFinish {
		t.Errorf("columns = %v, want Start/Finish Date dropped (no row has attributes)", got)
	}
	if !haveHasChildren {
		t.Errorf("columns = %v, want Has Children kept (Python sets it unconditionally)", got)
	}
}

// TestAreaiterationTeamIterationWorkItemsColumns_B7 is B-7: a top-level
// iteration work item (source null) drops the Source column, but Relation
// Type survives -- Python sets it unconditionally (_format.py:166).
func TestAreaiterationTeamIterationWorkItemsColumns_B7(t *testing.T) {
	rows := []map[string]any{
		{"rel": "System.LinkTypes.Hierarchy-Forward", "source": nil, "target": map[string]any{"id": float64(7)}},
	}
	got := areaiterationVisibleColumns(rows, areaiterationTeamIterationWorkItemsColumns)

	var haveSource, haveTarget, haveRelType bool
	for _, c := range got {
		switch c.Header {
		case "Source":
			haveSource = true
		case "Target":
			haveTarget = true
		case "Relation Type":
			haveRelType = true
		}
	}
	if haveSource {
		t.Errorf("columns = %v, want Source dropped (every row's source is null)", got)
	}
	if !haveTarget || !haveRelType {
		t.Errorf("columns = %v, want Target and Relation Type kept", got)
	}
}

// TestAreaiterationTeamDefaultIterationColumns_T7 is T7: when
// defaultIteration is falsy, the printed row carries only
// defaultIterationMacro (areaiterationPrintDefaultIteration already builds
// it that way), so every ID/Name/... column should disappear and only
// Default Iteration Macro survive -- Python sets that key unconditionally
// (_format.py:175).
func TestAreaiterationTeamDefaultIterationColumns_T7(t *testing.T) {
	rows := []map[string]any{{"defaultIterationMacro": "@currentIteration"}}
	got := areaiterationVisibleColumns(rows, areaiterationTeamDefaultIterationColumns)
	if len(got) != 1 || got[0].Header != "Default Iteration Macro" {
		t.Errorf("columns = %v, want only Default Iteration Macro", got)
	}
}

// TestAreaiterationClassificationPath_RuneTruncation guards against
// byte-slicing a multi-byte Path: Python's code-point slice
// (_format.py:214-215, path[0:47] + '...') must survive non-ASCII input
// without splitting a rune into mojibake.
func TestAreaiterationClassificationPath_RuneTruncation(t *testing.T) {
	var col *ado.Column
	for i := range areaiterationClassificationColumns {
		if areaiterationClassificationColumns[i].Header == "Path" {
			col = &areaiterationClassificationColumns[i]
		}
	}
	if col == nil {
		t.Fatal("Path column not found")
	}

	path := strings.Repeat("路", 60) // 60 code points, 3 bytes each
	got := col.Value(map[string]any{"path": path})

	if !utf8.ValidString(got) {
		t.Fatalf("got %q, not valid UTF-8", got)
	}
	want := string([]rune(path)[:47]) + "..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if n := utf8.RuneCountInString(got); n != 50 {
		t.Errorf("rune count = %d, want 50 (Python: path[0:47] + '...')", n)
	}
}
