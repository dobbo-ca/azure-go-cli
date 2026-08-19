package repos

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

// Reuses repoTestServer/repoCapturedRequest/repoTestClient from
// repo_test.go (same package, same httptest-recording idiom) rather than
// building a second harness. Every pr* command is split into a thin
// ado.Resolve*-driven RunE wrapper and an *Exec function taking an
// already-built client (+ ado.Context where needed) — same seam
// repoTestClient/refTestClient exist for — so tests call the *Exec half
// directly and never hit ado.Resolve*'s org-URL validation, which rejects
// a plain httptest URL.

func TestPRCreate(t *testing.T) {
	// Exercises: branch ref-resolution (short names -> refs/heads/...), the
	// title/description "explicitly given" short-circuit (so no commit-fetch
	// calls happen), and the reviewer-dedupe quirk: a reviewer present in
	// both --reviewers and --required-reviewers ends up as TWO entries in
	// the body, both isRequired=true (pull_request.py:637-660).
	srv, got := repoTestServer(t,
		func(r *repoCapturedRequest) (int, any) {
			return 200, r.Body // echo the create body back as the "created" PR
		},
	)
	client := repoTestClient(t, srv)
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj", Repo: "myrepo"}

	cmd := newPRCreateCmd()
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	cmd.Flags().Set("source-branch", "feature")
	cmd.Flags().Set("target-branch", "main")
	cmd.Flags().Set("title", "mytitle")
	cmd.Flags().Set("description", "mydesc")
	cmd.Flags().Set("reviewers", "11111111-1111-1111-1111-111111111111")
	cmd.Flags().Set("required-reviewers", "11111111-1111-1111-1111-111111111111")

	if err := prCreateExec(context.Background(), cmd, client, dctx); err != nil {
		t.Fatalf("prCreateExec: %v", err)
	}

	reqs := *got
	if len(reqs) != 1 {
		t.Fatalf("want 1 request (title+description both given, no completion options), got %d", len(reqs))
	}
	req := reqs[0]
	if req.Method != http.MethodPost || req.Path != "/myorg/myproj/_apis/git/repositories/myrepo/pullRequests" {
		t.Fatalf("create request = %+v", req)
	}
	if req.Body["sourceRefName"] != "refs/heads/feature" {
		t.Errorf("sourceRefName = %v", req.Body["sourceRefName"])
	}
	if req.Body["targetRefName"] != "refs/heads/main" {
		t.Errorf("targetRefName = %v", req.Body["targetRefName"])
	}
	if req.Body["title"] != "mytitle" {
		t.Errorf("title = %v", req.Body["title"])
	}
	if req.Body["description"] != "mydesc" {
		t.Errorf("description = %v", req.Body["description"])
	}
	reviewers, ok := req.Body["reviewers"].([]any)
	if !ok || len(reviewers) != 2 {
		t.Fatalf("reviewers = %#v, want 2 entries", req.Body["reviewers"])
	}
	for i, rv := range reviewers {
		m := rv.(map[string]any)
		if m["id"] != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("reviewer[%d].id = %v", i, m["id"])
		}
		if m["isRequired"] != true {
			t.Errorf("reviewer[%d].isRequired = %v, want true (both entries flagged required)", i, m["isRequired"])
		}
	}
}

// TestPRCreateThreeStateFlags covers arguments.py:117-121: --auto-complete,
// --squash, --delete-source-branch, --bypass-policy and
// --transition-work-items on `pr create` must be declared and parsed the
// same way pr_update.go already declares them (policyAddTriStateFlag /
// policyTriState), not as plain pflag Bool flags. A Bool flag can't even
// represent an invalid value the way the shared three-state helper does:
// Set() fails immediately with pflag's own strconv.ParseBool error instead
// of the friendly "invalid value ... must be true or false" text
// policyTriState produces, and GetString on a bool-typed flag errors out
// rather than reading the raw value at all.
func TestPRCreateThreeStateFlags(t *testing.T) {
	for _, name := range []string{"auto-complete", "squash", "delete-source-branch", "bypass-policy", "transition-work-items"} {
		cmd := newPRCreateCmd()

		if typ := cmd.Flags().Lookup(name).Value.Type(); typ != "string" {
			t.Errorf("%s: flag type = %q, want \"string\" (same type as pr_update.go's policyAddTriStateFlag)", name, typ)
		}

		if err := cmd.Flags().Set(name, "false"); err != nil {
			t.Fatalf("%s: Set(false): %v", name, err)
		}
		v, err := policyTriState(cmd, name)
		if err != nil {
			t.Fatalf("%s: policyTriState: %v", name, err)
		}
		if v == nil || *v != false {
			t.Errorf("%s = %v, want false", name, v)
		}

		cmd2 := newPRCreateCmd()
		if err := cmd2.Flags().Set(name, "banana"); err != nil {
			t.Fatalf("%s: Set(banana): %v (want no error at Set time; validation happens in policyTriState)", name, err)
		}
		_, err = policyTriState(cmd2, name)
		wantErr := `invalid value "banana" for --` + name + `; must be true or false`
		if err == nil || err.Error() != wantErr {
			t.Errorf("%s: policyTriState(banana) err = %v, want %q", name, err, wantErr)
		}
	}
}

// TestPRCreateDescriptionWithComma covers pull_request.py:165
// ('\n'.join(description)): a single --description value containing a
// comma must survive intact, not get split into multiple lines the way
// pflag's StringSlice would (arguments.py:109 is nargs='*', not a
// comma-list).
func TestPRCreateDescriptionWithComma(t *testing.T) {
	srv, got := repoTestServer(t,
		func(r *repoCapturedRequest) (int, any) {
			return 200, r.Body
		},
	)
	client := repoTestClient(t, srv)
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj", Repo: "myrepo"}

	cmd := newPRCreateCmd()
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	cmd.Flags().Set("source-branch", "feature")
	cmd.Flags().Set("target-branch", "main")
	cmd.Flags().Set("title", "mytitle")
	cmd.Flags().Set("description", "Fixes A, B")

	if err := prCreateExec(context.Background(), cmd, client, dctx); err != nil {
		t.Fatalf("prCreateExec: %v", err)
	}

	req := (*got)[0]
	if req.Body["description"] != "Fixes A, B" {
		t.Errorf("description = %q, want %q (comma must not split the value)", req.Body["description"], "Fixes A, B")
	}
}

// TestPRUpdateInvalidStatusNoHTTP covers arguments.py:144's
// enum_choice_list(_PR_TARGET_STATUS_VALUES), which validates --status at
// parse time, before any HTTP call: an invalid --status must be rejected
// before the GET, not surface as a 404 from the (never-reached) PATCH.
func TestPRUpdateInvalidStatusNoHTTP(t *testing.T) {
	srv, got := repoTestServer(t, func(r *repoCapturedRequest) (int, any) {
		t.Fatalf("no HTTP request should be made for an invalid --status, got %s %s", r.Method, r.Path)
		return 200, nil
	})
	client := repoTestClient(t, srv)

	cmd := newPRUpdateCmd()
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	cmd.Flags().Set("id", "42")
	cmd.Flags().Set("status", "bogus")

	err := prUpdateExec(context.Background(), cmd, client)
	if err == nil || !strings.Contains(err.Error(), "--status must be one of") {
		t.Fatalf("want --status validation error, got %v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("want 0 requests, got %d", len(*got))
	}
}

func TestPRUpdate(t *testing.T) {
	// Exercises: the GET-then-merge completionOptions behaviour (existing
	// server fields survive alongside the one flag actually passed) and the
	// name-based (not GUID-based) PATCH route, pull_request.py:355-356.
	srv, got := repoTestServer(t,
		func(r *repoCapturedRequest) (int, any) {
			return 200, map[string]any{
				"repository": map[string]any{
					"name":    "myrepo",
					"project": map[string]any{"name": "myproj"},
				},
				"completionOptions": map[string]any{"mergeCommitMessage": "old-msg"},
			}
		},
		func(r *repoCapturedRequest) (int, any) {
			return 200, r.Body
		},
	)
	client := repoTestClient(t, srv)

	cmd := newPRUpdateCmd()
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	cmd.Flags().Set("id", "42")
	cmd.Flags().Set("squash", "true")
	// arguments.py:109 is nargs='*' (joined with '\n', pull_request.py:314),
	// not a comma-list: a comma in one --description value must survive
	// intact.
	cmd.Flags().Set("description", "Fixes A, B")

	if err := prUpdateExec(context.Background(), cmd, client); err != nil {
		t.Fatalf("prUpdateExec: %v", err)
	}

	reqs := *got
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests, got %d", len(reqs))
	}
	if reqs[0].Method != http.MethodGet || reqs[0].Path != "/myorg/_apis/git/pullRequests/42" {
		t.Errorf("GET by id = %+v", reqs[0])
	}
	patch := reqs[1]
	if patch.Method != http.MethodPatch || patch.Path != "/myorg/myproj/_apis/git/repositories/myrepo/pullRequests/42" {
		t.Fatalf("PATCH request = %+v", patch)
	}
	co, ok := patch.Body["completionOptions"].(map[string]any)
	if !ok {
		t.Fatalf("completionOptions missing: %#v", patch.Body)
	}
	if co["mergeCommitMessage"] != "old-msg" {
		t.Errorf("mergeCommitMessage = %v, want existing value preserved", co["mergeCommitMessage"])
	}
	if co["squashMerge"] != true {
		t.Errorf("squashMerge = %v, want true", co["squashMerge"])
	}
	if patch.Body["description"] != "Fixes A, B" {
		t.Errorf("description = %q, want %q (comma must not split the value)", patch.Body["description"], "Fixes A, B")
	}
}

func TestPRSetVote(t *testing.T) {
	// Exercises: "me" identity resolution via ConnectionData and the
	// vote-string-to-int mapping (approve -> 10).
	srv, got := repoTestServer(t,
		func(r *repoCapturedRequest) (int, any) {
			return 200, map[string]any{
				"repository": map[string]any{"id": "repo1", "project": map[string]any{"id": "proj1"}},
			}
		},
		func(r *repoCapturedRequest) (int, any) {
			return 200, map[string]any{"authenticatedUser": map[string]any{"id": "me-id"}}
		},
		func(r *repoCapturedRequest) (int, any) {
			return 200, r.Body
		},
	)
	client := repoTestClient(t, srv)

	cmd := newPRSetVoteCmd()
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	cmd.Flags().Set("id", "9")
	cmd.Flags().Set("vote", "approve")

	if err := prSetVoteExec(context.Background(), cmd, client); err != nil {
		t.Fatalf("prSetVoteExec: %v", err)
	}

	reqs := *got
	if len(reqs) != 3 {
		t.Fatalf("want 3 requests, got %d", len(reqs))
	}
	if reqs[1].Path != "/myorg/_apis/ConnectionData" {
		t.Errorf("ConnectionData path = %s", reqs[1].Path)
	}
	// location_client.py:44 registers GetConnectionData at version
	// '5.0-preview.1', not '5.1-preview.1'.
	if reqs[1].Query != "api-version=5.0-preview.1" {
		t.Errorf("ConnectionData query = %s, want api-version=5.0-preview.1", reqs[1].Query)
	}
	vote := reqs[2]
	wantPath := "/myorg/proj1/_apis/git/repositories/repo1/pullRequests/9/reviewers/me-id"
	if vote.Method != http.MethodPut || vote.Path != wantPath {
		t.Fatalf("PUT vote = %+v, want path %s", vote, wantPath)
	}
	if vote.Body["vote"] != float64(10) {
		t.Errorf("vote = %v, want 10 (approve)", vote.Body["vote"])
	}
}

func TestPRList(t *testing.T) {
	// Exercises the repository-scoped vs project-wide path branch
	// (list_pull_requests, pull_request.py:85-89).
	tests := []struct {
		name     string
		repo     string
		wantPath string
	}{
		{"repository given", "myrepo", "/myorg/myproj/_apis/git/repositories/myrepo/pullRequests"},
		{"repository omitted", "", "/myorg/myproj/_apis/git/pullRequests"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, got := repoTestServer(t, func(r *repoCapturedRequest) (int, any) {
				return 200, map[string]any{"count": 0, "value": []any{}}
			})
			client := repoTestClient(t, srv)
			dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj", Repo: tt.repo}

			cmd := newPRListCmd()
			cmd.Flags().String("output", "json", "")
			cmd.Flags().String("query", "", "")

			if err := prListExec(context.Background(), cmd, client, dctx); err != nil {
				t.Fatalf("prListExec: %v", err)
			}

			reqs := *got
			if len(reqs) != 1 {
				t.Fatalf("want 1 request, got %d", len(reqs))
			}
			if reqs[0].Path != tt.wantPath {
				t.Errorf("path = %s, want %s", reqs[0].Path, tt.wantPath)
			}
		})
	}
}

// prWorkItemCall is one raw HTTP request captured by TestPRWorkItemAdd's
// server. A dedicated capture is needed here (rather than reusing
// repoTestServer) because the work-item PATCH body is a JSON *array*
// ([]JsonPatchOperation), and repoCapturedRequest only decodes object bodies.
type prWorkItemCall struct {
	Method string
	Path   string
	Query  string
	Raw    []byte
}

func TestPRWorkItemAdd(t *testing.T) {
	// Exercises the multi-call sequence (GET pr, PATCH work item, GET
	// workitemrefs, GET batch work items) and the literal-int "op": 0 body
	// quirk (pull_request.py:442) rather than the string "add" the sibling
	// `boards work-item relation add` command sends for the same endpoint.
	var calls []prWorkItemCall

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, prWorkItemCall{r.Method, r.URL.Path, r.URL.RawQuery, raw})

		w.Header().Set("Content-Type", "application/json")
		switch len(calls) {
		case 1: // GET pr by id
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"repository": map[string]any{"id": "repo1", "project": map[string]any{"id": "proj1"}},
			})
		case 2: // PATCH wit/workItems/100
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{})
		case 3: // GET workitemrefs
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{"count": 1, "value": []map[string]any{{"id": "100"}}})
		case 4: // GET wit/workItems?ids=100
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{"count": 1, "value": []map[string]any{{"id": "100"}}})
		default:
			t.Fatalf("unexpected call #%d: %s %s", len(calls), r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	client := repoTestClient(t, srv)

	cmd := newPRWorkItemAddCmd()
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	cmd.Flags().Set("id", "7")
	cmd.Flags().Set("work-items", "100")

	if err := prWorkItemAddExec(context.Background(), cmd, client); err != nil {
		t.Fatalf("prWorkItemAddExec: %v", err)
	}

	if len(calls) != 4 {
		t.Fatalf("want 4 requests, got %d", len(calls))
	}
	if calls[0].Method != http.MethodGet || calls[0].Path != "/myorg/_apis/git/pullRequests/7" {
		t.Errorf("GET pr by id = %+v", calls[0])
	}
	if calls[1].Method != http.MethodPatch || calls[1].Path != "/myorg/_apis/wit/workItems/100" {
		t.Errorf("PATCH work item = %+v", calls[1])
	}
	var patch []map[string]any
	if err := json.Unmarshal(calls[1].Raw, &patch); err != nil {
		t.Fatalf("unmarshal patch body: %v", err)
	}
	if len(patch) != 1 {
		t.Fatalf("want 1 patch op, got %d", len(patch))
	}
	opVal, ok := patch[0]["op"].(float64)
	if !ok || opVal != 0 {
		t.Errorf(`op = %#v, want literal 0 (not the string "add")`, patch[0]["op"])
	}
	wantRefsPath := "/myorg/proj1/_apis/git/repositories/repo1/pullRequests/7/workitemrefs"
	if calls[2].Path != wantRefsPath {
		t.Errorf("GET workitemrefs path = %s, want %s", calls[2].Path, wantRefsPath)
	}
	if calls[3].Path != "/myorg/_apis/wit/workItems" || calls[3].Query != "api-version=5.0&ids=100" {
		t.Errorf("GET work items = path %s query %s", calls[3].Path, calls[3].Query)
	}
}
