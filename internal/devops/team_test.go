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

// teamCaptureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything it wrote, same idiom as pkg/output/output_test.go's
// captureStdout.
func teamCaptureStdout(t *testing.T, fn func()) string {
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

	var b strings.Builder
	io.Copy(&b, r)
	return b.String()
}

// teamCapturedRequest is what each fake server handler records for a test to
// assert on.
type teamCapturedRequest struct {
	Method string
	URL    string // raw request-target, escaped path + query, as sent on the wire
	Body   map[string]any
}

func teamTestServer(t *testing.T, status int, respBody string) (*httptest.Server, *teamCapturedRequest) {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	captured := &teamCapturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Method = r.Method
		captured.URL = r.URL.RequestURI()
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			if len(b) > 0 {
				_ = json.Unmarshal(b, &captured.Body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func TestTeamCreate_BodyNullDescriptionWhenOmitted(t *testing.T) {
	srv, captured := teamTestServer(t, http.StatusOK, `{"id":"1","name":"T1","description":null}`)

	cmd := teamNewCreateCmd()
	cmd.Flags().Set("name", "T1")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := teamCreate(context.Background(), cmd, dctx, "T1", ""); err != nil {
		t.Fatalf("teamCreate: %v", err)
	}

	if captured.Method != http.MethodPost {
		t.Errorf("Method = %q, want POST", captured.Method)
	}
	wantURL := "/_apis/projects/MyProj/teams?api-version=5.0"
	if captured.URL != wantURL {
		t.Errorf("URL = %q, want %q", captured.URL, wantURL)
	}
	if captured.Body["name"] != "T1" {
		t.Errorf("body name = %v", captured.Body["name"])
	}
	if v, ok := captured.Body["description"]; !ok || v != nil {
		t.Errorf("body description = %v (ok=%v), want explicit null", v, ok)
	}
}

func TestTeamCreate_BodyWithDescription(t *testing.T) {
	srv, captured := teamTestServer(t, http.StatusOK, `{"id":"1","name":"T1"}`)

	cmd := teamNewCreateCmd()
	cmd.Flags().Set("name", "T1")
	cmd.Flags().Set("description", "d")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := teamCreate(context.Background(), cmd, dctx, "T1", "d"); err != nil {
		t.Fatalf("teamCreate: %v", err)
	}

	if captured.Body["description"] != "d" {
		t.Errorf("body description = %v, want %q", captured.Body["description"], "d")
	}
}

func TestTeamDelete_URLAndMethod(t *testing.T) {
	srv, captured := teamTestServer(t, http.StatusOK, "")

	cmd := teamNewDeleteCmd()
	cmd.Flags().Set("yes", "true")
	dctx := ado.Context{Org: srv.URL, Project: "My Proj"}

	if err := teamDelete(context.Background(), cmd, dctx, "team one"); err != nil {
		t.Fatalf("teamDelete: %v", err)
	}

	if captured.Method != http.MethodDelete {
		t.Errorf("Method = %q, want DELETE", captured.Method)
	}
	wantURL := "/_apis/projects/My%20Proj/teams/team%20one?api-version=5.0"
	if captured.URL != wantURL {
		t.Errorf("URL = %q, want %q", captured.URL, wantURL)
	}
}

func TestTeamUpdate_RequiresNameOrDescription(t *testing.T) {
	cmd := teamNewUpdateCmd()
	cmd.Flags().Set("team", "T1")

	err := teamRunUpdate(context.Background(), cmd, "T1", "", "")
	if err == nil || err.Error() != "Either name or description argument must be provided." {
		t.Fatalf("err = %v, want the CLIError text", err)
	}
}

func TestTeamUpdate_BodyNullsUnsetField(t *testing.T) {
	srv, captured := teamTestServer(t, http.StatusOK, `{"id":"1","name":"T1"}`)

	cmd := teamNewUpdateCmd()
	cmd.Flags().Set("team", "T1")
	cmd.Flags().Set("name", "NewName")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := teamUpdate(context.Background(), cmd, dctx, "T1", "NewName", ""); err != nil {
		t.Fatalf("teamUpdate: %v", err)
	}

	if captured.Method != http.MethodPatch {
		t.Errorf("Method = %q, want PATCH", captured.Method)
	}
	if captured.Body["name"] != "NewName" {
		t.Errorf("body name = %v", captured.Body["name"])
	}
	if v, ok := captured.Body["description"]; !ok || v != nil {
		t.Errorf("body description = %v (ok=%v), want explicit null", v, ok)
	}
}

func TestTeamList_TopSkipOmittedUnlessSet(t *testing.T) {
	srv, captured := teamTestServer(t, http.StatusOK, `{"count":0,"value":[]}`)

	cmd := teamNewListCmd()
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := teamList(context.Background(), cmd, dctx, 0, 0); err != nil {
		t.Fatalf("teamList: %v", err)
	}
	wantURL := "/_apis/projects/MyProj/teams?api-version=5.0"
	if captured.URL != wantURL {
		t.Errorf("URL = %q, want %q ($top/$skip omitted when unset)", captured.URL, wantURL)
	}
}

func TestTeamList_TopSkipSentWhenSet(t *testing.T) {
	srv, captured := teamTestServer(t, http.StatusOK, `{"count":0,"value":[]}`)

	cmd := teamNewListCmd()
	cmd.Flags().Set("top", "5")
	cmd.Flags().Set("skip", "2")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	if err := teamList(context.Background(), cmd, dctx, 5, 2); err != nil {
		t.Fatalf("teamList: %v", err)
	}
	wantURL := "/_apis/projects/MyProj/teams?%24skip=2&%24top=5&api-version=5.0"
	if captured.URL != wantURL {
		t.Errorf("URL = %q, want %q", captured.URL, wantURL)
	}
}

func TestTeamList_SortedByNameCaseInsensitive(t *testing.T) {
	srv, _ := teamTestServer(t, http.StatusOK,
		`{"count":3,"value":[{"id":"1","name":"beta"},{"id":"2","name":"Alpha"},{"id":"3","name":"gamma"}]}`)

	// _get_team_key sorting (_format.py:258-262) is wired only as this
	// command's table_transformer (commands.py:126), applied by knack for
	// -o table with no --query — so exercise that mode explicitly.
	cmd := teamNewListCmd()
	cmd.Flags().String("output", "table", "")
	cmd.Flags().String("query", "", "")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	out := teamCaptureStdout(t, func() {
		if err := teamList(context.Background(), cmd, dctx, 0, 0); err != nil {
			t.Fatalf("teamList: %v", err)
		}
	})

	// Table output, not JSON: header line + dash separator + one row per
	// team, in sorted order.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5 (header + separator + 3 rows)\noutput: %s", len(lines), out)
	}
	for i, want := range []string{"Alpha", "beta", "gamma"} {
		if !strings.Contains(lines[i+2], want) {
			t.Errorf("row %d = %q, want it to contain %q", i, lines[i+2], want)
		}
	}
}

// TestTeamList_JSONKeepsServerOrder guards m1/T10: _get_team_key sorting is
// applied by knack only for the table transformer (commands.py:126); -o
// json/tsv must keep the server's order.
func TestTeamList_JSONKeepsServerOrder(t *testing.T) {
	srv, _ := teamTestServer(t, http.StatusOK,
		`{"count":3,"value":[{"id":"1","name":"beta"},{"id":"2","name":"Alpha"},{"id":"3","name":"gamma"}]}`)

	cmd := teamNewListCmd()
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	out := teamCaptureStdout(t, func() {
		if err := teamList(context.Background(), cmd, dctx, 0, 0); err != nil {
			t.Fatalf("teamList: %v", err)
		}
	})

	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	names := []string{got[0]["name"].(string), got[1]["name"].(string), got[2]["name"].(string)}
	want := []string{"beta", "Alpha", "gamma"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("row %d name = %q, want %q (server order: %v)", i, names[i], want[i], names)
		}
	}
}

func TestTeamListMember_URLAndNestedFieldSort(t *testing.T) {
	srv, captured := teamTestServer(t, http.StatusOK,
		`{"count":2,"value":[`+
			`{"identity":{"id":"1","displayName":"Bob","uniqueName":"zed@example.com"},"isTeamAdmin":false},`+
			`{"identity":{"id":"2","displayName":"Amy","uniqueName":"amy@example.com"},"isTeamAdmin":true}`+
			`]}`)

	cmd := teamNewListMemberCmd()
	cmd.Flags().Set("team", "T1")
	// _get_member_key sorting (_format.py:314-318) is wired only as this
	// command's table_transformer (commands.py:127).
	cmd.Flags().String("output", "table", "")
	cmd.Flags().String("query", "", "")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	out := teamCaptureStdout(t, func() {
		if err := teamListMember(context.Background(), cmd, dctx, "T1", 0, 0); err != nil {
			t.Fatalf("teamListMember: %v", err)
		}
	})
	wantURL := "/_apis/projects/MyProj/teams/T1/members?api-version=5.0"
	if captured.URL != wantURL {
		t.Errorf("URL = %q, want %q", captured.URL, wantURL)
	}

	// Table output, not JSON: header line + dash separator + one row per
	// member, in sorted order.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (header + separator + 2 rows)\noutput: %s", len(lines), out)
	}
	if !strings.Contains(lines[2], "amy@example.com") {
		t.Errorf("first row = %q, want amy@example.com (sorted before zed@)", lines[2])
	}
}

// TestTeamListMember_JSONKeepsServerOrder guards m1/T10 for list-member: -o
// json/tsv must keep the server's order, unlike -o table.
func TestTeamListMember_JSONKeepsServerOrder(t *testing.T) {
	srv, _ := teamTestServer(t, http.StatusOK,
		`{"count":2,"value":[`+
			`{"identity":{"id":"1","displayName":"Bob","uniqueName":"zed@example.com"},"isTeamAdmin":false},`+
			`{"identity":{"id":"2","displayName":"Amy","uniqueName":"amy@example.com"},"isTeamAdmin":true}`+
			`]}`)

	cmd := teamNewListMemberCmd()
	cmd.Flags().Set("team", "T1")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	dctx := ado.Context{Org: srv.URL, Project: "MyProj"}

	out := teamCaptureStdout(t, func() {
		if err := teamListMember(context.Background(), cmd, dctx, "T1", 0, 0); err != nil {
			t.Fatalf("teamListMember: %v", err)
		}
	})

	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	firstIdentity := got[0]["identity"].(map[string]any)
	if firstIdentity["uniqueName"] != "zed@example.com" {
		t.Errorf("first row uniqueName = %v, want zed@example.com (server order preserved)", firstIdentity["uniqueName"])
	}
}
