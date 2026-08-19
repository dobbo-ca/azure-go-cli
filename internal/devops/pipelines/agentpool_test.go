package pipelines

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// agentpoolCapturedRequest is one HTTP request seen by a test server.
type agentpoolCapturedRequest struct {
	Method string
	Path   string
	Query  string
}

// agentpoolTestServer builds an httptest server that records every request
// and always answers 200 with body.
func agentpoolTestServer(t *testing.T, body string) (*httptest.Server, *[]agentpoolCapturedRequest) {
	t.Helper()
	var got []agentpoolCapturedRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, agentpoolCapturedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

// agentpoolTestClient builds an *ado.Client against srv directly (bypassing
// ado.Resolve*, whose org-URL check rejects a plain httptest URL), with a
// hermetic, network-free auth path: a fake PAT stands in for AAD, and the
// config dir is isolated per test. Same approach as internal/devops/ado's
// own client_test.go and internal/devops/repos/repo_test.go.
func agentpoolTestClient(t *testing.T, srv *httptest.Server) *ado.Client {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	client, err := ado.NewClient(context.Background(), srv.URL+"/myorg")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

// agentpoolTestCmd carries the persistent --output/--query flags every leaf
// command inherits from the root in production (cmd/az/main.go).
func agentpoolTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")
	return cmd
}

// TestAgentPoolValidateChoiceCaseInsensitive ports knack's enum_choice_list
// (CaseInsensitiveList + normalising type, arguments.py:92-93,103,108,110):
// --action/--pool-type accept any case and get normalised to the canonical
// (lowercase) value that goes on the wire.
func TestAgentPoolValidateChoiceCaseInsensitive(t *testing.T) {
	got, err := agentpoolValidateChoice("Use", "action", agentpoolActionChoices)
	if err != nil {
		t.Fatalf("agentpoolValidateChoice: %v", err)
	}
	if got != "use" {
		t.Errorf("got %q, want the canonical \"use\"", got)
	}

	if _, err := agentpoolValidateChoice("bogus", "action", agentpoolActionChoices); err == nil {
		t.Fatal("expected an error for an unrecognised value")
	}

	got, err = agentpoolValidateChoice("", "action", agentpoolActionChoices)
	if err != nil || got != "" {
		t.Errorf("got=%q err=%v, want empty/nil (unset is always allowed)", got, err)
	}
}

func TestAgentPoolList(t *testing.T) {
	srv, got := agentpoolTestServer(t, `{"count":0,"value":[]}`)
	client := agentpoolTestClient(t, srv)
	cmd := agentpoolTestCmd()

	if err := agentpoolPoolList(context.Background(), cmd, client, "myPool", "automation", "use"); err != nil {
		t.Fatalf("agentpoolPoolList: %v", err)
	}

	reqs := *got
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	r := reqs[0]
	if r.Method != http.MethodGet || r.Path != "/myorg/_apis/distributedtask/pools" {
		t.Errorf("request = %+v", r)
	}
	q := r.Query
	for _, want := range []string{"poolName=myPool", "poolType=automation", "actionFilter=use", "api-version=5.1"} {
		if !agentpoolContainsQueryParam(q, want) {
			t.Errorf("query = %q, want to contain %q", q, want)
		}
	}
}

func TestAgentPoolPoolShow_IDAlias(t *testing.T) {
	srv, got := agentpoolTestServer(t, `{"id":42}`)
	client := agentpoolTestClient(t, srv)
	cmd := agentpoolTestCmd()

	// The --id alias resolves to the same poolID an explicit --pool-id
	// would (agentpoolRequiredIntFlag), so the caller here just passes the
	// already-resolved int, same as production RunE does after alias
	// fallback.
	if err := agentpoolPoolShow(context.Background(), cmd, client, 42, "manage"); err != nil {
		t.Fatalf("agentpoolPoolShow: %v", err)
	}

	reqs := *got
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	r := reqs[0]
	if r.Method != http.MethodGet || r.Path != "/myorg/_apis/distributedtask/pools/42" {
		t.Errorf("request = %+v", r)
	}
	if !agentpoolContainsQueryParam(r.Query, "actionFilter=manage") || !agentpoolContainsQueryParam(r.Query, "api-version=5.1") {
		t.Errorf("query = %q", r.Query)
	}
}

func TestAgentPoolAgentList_TriStateAndDemands(t *testing.T) {
	srv, got := agentpoolTestServer(t, `{"count":0,"value":[]}`)
	client := agentpoolTestClient(t, srv)
	cmd := agentpoolTestCmd()

	trueVal := true
	falseVal := false
	if err := agentpoolAgentList(context.Background(), cmd, client, 7, "build-agent", "a,b",
		&trueVal, &falseVal, nil); err != nil {
		t.Fatalf("agentpoolAgentList: %v", err)
	}

	reqs := *got
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	r := reqs[0]
	if r.Method != http.MethodGet || r.Path != "/myorg/_apis/distributedtask/pools/7/agents" {
		t.Errorf("request = %+v", r)
	}
	for _, want := range []string{
		"agentName=build-agent",
		"includeCapabilities=true",
		"includeAssignedRequest=false",
		"demands=a%2Cb",
		"api-version=5.1",
	} {
		if !agentpoolContainsQueryParam(r.Query, want) {
			t.Errorf("query = %q, want to contain %q", r.Query, want)
		}
	}
	// includeLastCompletedRequest was passed nil (unset): must be omitted
	// entirely, not sent as "includeLastCompletedRequest=false".
	if agentpoolContainsQueryParam(r.Query, "includeLastCompletedRequest") {
		t.Errorf("query = %q, want no includeLastCompletedRequest param", r.Query)
	}
}

func TestAgentPoolQueueShow_ProjectScopedPreviewAPI(t *testing.T) {
	srv, got := agentpoolTestServer(t, `{"id":9}`)
	client := agentpoolTestClient(t, srv)
	cmd := agentpoolTestCmd()

	if err := agentpoolQueueShow(context.Background(), cmd, client, "my project", "9", "none"); err != nil {
		t.Fatalf("agentpoolQueueShow: %v", err)
	}

	reqs := *got
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	r := reqs[0]
	if r.Method != http.MethodGet || r.Path != "/myorg/my project/_apis/distributedtask/queues/9" {
		t.Errorf("request = %+v", r)
	}
	if !agentpoolContainsQueryParam(r.Query, "actionFilter=none") || !agentpoolContainsQueryParam(r.Query, "api-version=5.1-preview.1") {
		t.Errorf("query = %q, want actionFilter=none and api-version=5.1-preview.1", r.Query)
	}
}

// agentpoolContainsQueryParam is a small substring check over an already-encoded
// query string; good enough since these tests only assert individual
// key=value pairs are present, not full ordering.
func agentpoolContainsQueryParam(query, kv string) bool {
	for _, part := range agentpoolSplitAmp(query) {
		if part == kv {
			return true
		}
	}
	return false
}

func agentpoolSplitAmp(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '&' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
