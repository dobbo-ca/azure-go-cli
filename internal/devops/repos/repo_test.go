package repos

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
	"github.com/spf13/cobra"
)

// repoCapturedRequest is one HTTP request seen by a test server, decoded
// enough to assert routing and body shape.
type repoCapturedRequest struct {
	Method string
	Path   string
	Query  string
	Body   map[string]any
}

// repoTestServer builds an httptest server that records every request and
// dispatches JSON responses from handlers, in call order.
func repoTestServer(t *testing.T, handlers ...func(r *repoCapturedRequest) (int, any)) (*httptest.Server, *[]repoCapturedRequest) {
	t.Helper()
	var got []repoCapturedRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &body)
		}
		rec := repoCapturedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body}
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

// repoTestClient builds an *ado.Client against srv directly (bypassing
// ado.Resolve*, whose org-URL check rejects a plain httptest URL), with a
// hermetic, network-free auth path: a fake PAT stands in for AAD, and the
// config dir is isolated per test. Same approach as internal/devops/ado's
// own client_test.go.
func repoTestClient(t *testing.T, srv *httptest.Server) *ado.Client {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	client, err := ado.NewClient(context.Background(), srv.URL+"/myorg")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

// repoTestCmd carries the persistent --output/--query flags every leaf
// command inherits from the root in production (cmd/az/main.go); the
// split-out repo* functions under test read those directly off cmd.
func repoTestCmd(output string) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("output", output, "")
	cmd.Flags().String("query", "", "")
	return cmd
}

func TestRepoUpdate(t *testing.T) {
	srv, got := repoTestServer(t,
		func(r *repoCapturedRequest) (int, any) {
			return 200, map[string]any{
				"id": "repo1", "name": "old-name", "defaultBranch": "refs/heads/old",
				"project": map[string]any{"name": "myproj"},
			}
		},
		func(r *repoCapturedRequest) (int, any) {
			return 200, r.Body // echo the PATCH body back
		},
	)
	client := repoTestClient(t, srv)
	cmd := repoTestCmd("json")
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj", Repo: "myrepo"}

	if err := repoUpdate(context.Background(), cmd, client, dctx, "live", "new-name"); err != nil {
		t.Fatalf("repoUpdate: %v", err)
	}

	reqs := *got
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests, got %d", len(reqs))
	}
	if reqs[0].Method != http.MethodGet || reqs[0].Path != "/myorg/myproj/_apis/git/repositories/myrepo" {
		t.Errorf("GET request = %+v", reqs[0])
	}
	if reqs[1].Method != http.MethodPatch || reqs[1].Path != "/myorg/myproj/_apis/git/repositories/myrepo" {
		t.Errorf("PATCH request = %+v", reqs[1])
	}

	// Whole fetched object is resent, not a delta: id/project survive
	// untouched alongside the two mutated fields.
	body := reqs[1].Body
	if body["id"] != "repo1" {
		t.Errorf("PATCH body dropped id: %+v", body)
	}
	if body["defaultBranch"] != "refs/heads/live" {
		t.Errorf("defaultBranch = %v, want refs/heads/live", body["defaultBranch"])
	}
	if body["name"] != "new-name" {
		t.Errorf("name = %v, want new-name", body["name"])
	}
}

func TestRepoDelete(t *testing.T) {
	srv, got := repoTestServer(t, func(r *repoCapturedRequest) (int, any) {
		return 200, nil
	})
	client := repoTestClient(t, srv)
	cmd := repoTestCmd("json")
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj"}

	out := repoCaptureStdout(t, func() {
		if err := repoDelete(context.Background(), cmd, client, dctx, "repo with spaces"); err != nil {
			t.Fatalf("repoDelete: %v", err)
		}
	})

	reqs := *got
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	if reqs[0].Method != http.MethodDelete || reqs[0].Path != "/myorg/myproj/_apis/git/repositories/repo with spaces" {
		t.Errorf("DELETE request = %+v", reqs[0])
	}
	if !strings.Contains(out, "Deleted repository repo with spaces") {
		t.Errorf("stdout = %q, want the redundant Python-parity message", out)
	}
	// delete_repository returns None (repository.py:52-53); knack skips
	// output entirely for a None result (cli.py:237), so nothing beyond
	// the "Deleted repository" line should print — not even a stray
	// "null" from -o json's default marshalling.
	if strings.Contains(out, "null") {
		t.Errorf("stdout = %q, must not print a stray null", out)
	}
}

func TestRepoList(t *testing.T) {
	unsorted := []map[string]any{
		{"id": "2", "name": "zeta", "defaultBranch": "", "project": map[string]any{"name": "p"}},
		{"id": "1", "name": "alpha", "defaultBranch": "", "project": map[string]any{"name": "p"}},
	}

	t.Run("table sorts by name", func(t *testing.T) {
		srv, _ := repoTestServer(t, func(r *repoCapturedRequest) (int, any) {
			return 200, map[string]any{"count": 2, "value": unsorted}
		})
		client := repoTestClient(t, srv)
		cmd := repoTestCmd("table")
		dctx := ado.Context{Org: srv.URL + "/myorg", Project: "p"}

		out := repoCaptureStdout(t, func() {
			if err := repoList(context.Background(), cmd, client, dctx); err != nil {
				t.Fatalf("repoList: %v", err)
			}
		})

		if idx := strings.Index(out, "alpha"); idx == -1 || strings.Index(out, "zeta") < idx {
			t.Errorf("table output not sorted by name:\n%s", out)
		}
	})

	t.Run("json keeps server order", func(t *testing.T) {
		srv, _ := repoTestServer(t, func(r *repoCapturedRequest) (int, any) {
			return 200, map[string]any{"count": 2, "value": unsorted}
		})
		client := repoTestClient(t, srv)
		cmd := repoTestCmd("json")
		dctx := ado.Context{Org: srv.URL + "/myorg", Project: "p"}

		out := repoCaptureStdout(t, func() {
			if err := repoList(context.Background(), cmd, client, dctx); err != nil {
				t.Fatalf("repoList: %v", err)
			}
		})

		if idx := strings.Index(out, "zeta"); idx == -1 || strings.Index(out, "alpha") < idx {
			t.Errorf("json output should preserve server order (zeta first):\n%s", out)
		}
	})
}

func TestRepoImportCreate(t *testing.T) {
	t.Setenv(repoGitSourcePasswordEnvVar, "hunter2")

	srv, got := repoTestServer(t,
		// 1: create the temp service endpoint
		func(r *repoCapturedRequest) (int, any) {
			return 200, map[string]any{"id": "se-123"}
		},
		// 2: create the import request
		func(r *repoCapturedRequest) (int, any) {
			return 200, map[string]any{"importRequestId": 7, "status": "queued"}
		},
		// 3: poll — already terminal, so no sleep is exercised
		func(r *repoCapturedRequest) (int, any) {
			return 200, map[string]any{
				"status": "Completed",
				"repository": map[string]any{
					"name":    "fabrikam-open-source",
					"project": map[string]any{"name": "myproj"},
				},
			}
		},
	)
	client := repoTestClient(t, srv)
	cmd := repoTestCmd("json")
	dctx := ado.Context{Org: srv.URL + "/myorg", Project: "myproj", Repo: "fabrikam-open-source"}

	if err := repoImportCreate(context.Background(), cmd, client, dctx,
		"https://github.com/fabrikamprime/fabrikam-open-source", true, "alice", ""); err != nil {
		t.Fatalf("repoImportCreate: %v", err)
	}

	reqs := *got
	if len(reqs) != 3 {
		t.Fatalf("want 3 requests (service endpoint, import create, poll), got %d", len(reqs))
	}

	se := reqs[0]
	if se.Method != http.MethodPost || se.Path != "/myorg/myproj/_apis/serviceendpoint/endpoints" || se.Query != "api-version=5.0-preview.2" {
		t.Errorf("service endpoint request = %+v", se)
	}
	auth, _ := se.Body["authorization"].(map[string]any)
	params, _ := auth["parameters"].(map[string]any)
	if params["password"] != "hunter2" || params["username"] != "alice" || auth["scheme"] != "UsernamePassword" {
		t.Errorf("service endpoint authorization = %+v", auth)
	}
	if name, _ := se.Body["name"].(string); len(name) != 10 {
		t.Errorf("service endpoint name = %q, want 10 chars", name)
	}

	imp := reqs[1]
	if imp.Method != http.MethodPost || imp.Path != "/myorg/myproj/_apis/git/repositories/fabrikam-open-source/importRequests" || imp.Query != "api-version=5.0-preview.1" {
		t.Errorf("import create request = %+v", imp)
	}
	impParams, _ := imp.Body["parameters"].(map[string]any)
	if impParams["serviceEndpointId"] != "se-123" {
		t.Errorf("serviceEndpointId = %v, want se-123", impParams["serviceEndpointId"])
	}
	if impParams["deleteServiceEndpointAfterImportIsDone"] != true {
		t.Errorf("deleteServiceEndpointAfterImportIsDone = %v, want true", impParams["deleteServiceEndpointAfterImportIsDone"])
	}
	gitSource, _ := impParams["gitSource"].(map[string]any)
	if gitSource["overwrite"] != false || gitSource["url"] != "https://github.com/fabrikamprime/fabrikam-open-source" {
		t.Errorf("gitSource = %+v", gitSource)
	}
	if _, ok := impParams["tfvcSource"]; !ok {
		t.Errorf("tfvcSource key missing, want explicit null")
	}

	poll := reqs[2]
	if poll.Method != http.MethodGet || poll.Path != "/myorg/myproj/_apis/git/repositories/fabrikam-open-source/importRequests/7" {
		t.Errorf("poll request = %+v", poll)
	}
}

// TestRepoImportCreateNoRepositoryRequired ensures --repository is not
// cobra-required: import_request.py:20 defaults repository=None, and
// ado.ResolveRepo (not flag parsing) is the thing that enforces it once git
// remote detection has had a chance to run.
func TestRepoImportCreateNoRepositoryRequired(t *testing.T) {
	cmd := newRepoImportCreateCmd()
	args := []string{"--git-source-url", "https://example.com/x", "--organization", "https://dev.azure.com/o", "--project", "p", "--detect", "false"}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if err := cmd.ValidateRequiredFlags(); err != nil {
		t.Errorf("--repository must not be cobra-required: %v", err)
	}
}

// TestRepoImportCreateMissingURLBeforeOrg ensures the missing
// --git-source-url error is reported before org/project/repo resolution
// (import_request.py:20 has no default, so Python fails on it before
// resolve_instance_project_and_repo runs at :37-42).
func TestRepoImportCreateMissingURLBeforeOrg(t *testing.T) {
	cmd := newRepoImportCreateCmd()
	err := repoRunImportCreate(context.Background(), cmd)
	if err == nil || !strings.Contains(err.Error(), "--git-source-url must be specified") {
		t.Fatalf("want --git-source-url error, got %v", err)
	}
}

// repoCaptureStdout redirects os.Stdout for the duration of fn, mirroring
// pkg/output/output_test.go's captureStdout helper (unexported there, so not
// reusable directly).
func repoCaptureStdout(t *testing.T, fn func()) string {
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
