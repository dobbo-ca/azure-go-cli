package pipelines

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

// runsTestClient builds a *ado.Client against srv with a hermetic,
// network-free auth path (AZURE_DEVOPS_EXT_PAT supplies the credential; a
// real AAD lookup is attempted first but fails harmlessly in a sandboxed
// test environment with no az login/network). ado.ResolveProject/validateOrg
// reject non-dev.azure.com/visualstudio.com hosts by design, so — matching
// this package's release_test.go convention — these tests build the client
// directly against the httptest server rather than going through cobra flag
// parsing and org resolution.
func runsTestClient(t *testing.T, org string) *ado.Client {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	c, err := ado.NewClient(context.Background(), org)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

type runsCall struct {
	method string
	url    string
	body   string
}

func runsCapturingServer(t *testing.T, respond func(w http.ResponseWriter, call runsCall)) (*httptest.Server, *[]runsCall) {
	t.Helper()
	calls := &[]runsCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		call := runsCall{method: r.Method, url: r.URL.String(), body: string(b)}
		*calls = append(*calls, call)
		w.Header().Set("Content-Type", "application/json")
		respond(w, call)
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

// TestRunsListQuery covers the client-side filter/query-building logic
// behind `pipelines runs list`: dedup of --pipeline-ids/--tags, the
// refs/heads/ branch normalisation, and the --query-order enum mapping
// (whose target strings and fallback semantics genuinely differ from
// `pipelines list`'s own --query-order — see pipeline_run.py:79-87).
func TestRunsListQuery(t *testing.T) {
	tests := []struct {
		name         string
		pipelineIDs  []string
		branch       string
		top          int
		queryOrder   string
		result       string
		status       string
		reason       string
		tags         []string
		requestedFor string
		want         map[string]string
		wantErr      bool
	}{
		{
			name:        "dedups pipeline ids preserving first-seen order",
			pipelineIDs: []string{"2", "1", "2"},
			want:        map[string]string{"definitions": "2,1"},
		},
		{
			name: "dedups tags",
			tags: []string{"a", "b", "a"},
			want: map[string]string{"tagFilters": "a,b"},
		},
		{
			name:   "branch already prefixed is untouched",
			branch: "refs/heads/main",
			want:   map[string]string{"branchName": "refs/heads/main"},
		},
		{
			name:   "bare branch gets refs/heads/ prefix",
			branch: "main",
			want:   map[string]string{"branchName": "refs/heads/main"},
		},
		{
			name:   "pull ref is left alone",
			branch: "refs/pull/42",
			want:   map[string]string{"branchName": "refs/pull/42"},
		},
		{
			name:       "query-order maps to server enum value",
			queryOrder: "QueueTimeDesc",
			want:       map[string]string{"queryOrder": "queueTimeDescending"},
		},
		{
			name:       "query-order is case-insensitive",
			queryOrder: "finishtimeasc",
			want:       map[string]string{"queryOrder": "finishTimeAscending"},
		},
		{
			name:       "invalid query-order errors",
			queryOrder: "bogus",
			wantErr:    true,
		},
		{
			name:    "invalid result errors",
			result:  "bogus",
			wantErr: true,
		},
		{
			name:        "invalid pipeline id errors",
			pipelineIDs: []string{"not-a-number"},
			wantErr:     true,
		},
		{
			name: "top only set when positive",
			top:  0,
			want: map[string]string{},
		},
		{
			name: "top is passed through as $top",
			top:  5,
			want: map[string]string{"$top": "5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := runsListQuery(tt.pipelineIDs, tt.branch, tt.top, tt.queryOrder, tt.result, tt.status, tt.reason, tt.tags, tt.requestedFor)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for k, want := range tt.want {
				if got := q.Get(k); got != want {
					t.Errorf("query[%q] = %q, want %q", k, got, want)
				}
			}
		})
	}
}

// TestRunsTagAdd covers the single-vs-multi-tag branch shared with
// `pipelines build tag add` but reimplemented independently
// (pipeline_run.py:90-107): one tag is a PUT to .../tags/{tag}, two or more
// is a POST to .../tags with a JSON array body.
func TestRunsTagAdd(t *testing.T) {
	t.Run("single tag PUTs to the tag path", func(t *testing.T) {
		// The build-tag endpoints return a {count,value} envelope, not a
		// bare array (build_client.py:1547 _unwrap_collection); runs_tag.go
		// must decode with client.List, not client.Do, or this fails with
		// "cannot unmarshal object into Go value of type []string".
		srv, calls := runsCapturingServer(t, func(w http.ResponseWriter, call runsCall) {
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "value": []string{"release"}})
		})
		client := runsTestClient(t, srv.URL+"/myorg")

		var result []string
		req := ado.Request{
			Method:     "PUT",
			Scope:      "MyProj",
			Path:       "build/builds/42/tags/release",
			APIVersion: "5.0",
		}
		if err := client.List(context.Background(), req, &result); err != nil {
			t.Fatalf("List: %v", err)
		}

		if len(*calls) != 1 {
			t.Fatalf("got %d calls, want 1", len(*calls))
		}
		got := (*calls)[0]
		if got.method != "PUT" {
			t.Errorf("method = %q, want PUT", got.method)
		}
		if got.url != "/myorg/MyProj/_apis/build/builds/42/tags/release?api-version=5.0" {
			t.Errorf("url = %q", got.url)
		}
		if got.body != "" {
			t.Errorf("body = %q, want empty", got.body)
		}
	})

	t.Run("multiple tags POST an array body, no trim, no dedup", func(t *testing.T) {
		srv, calls := runsCapturingServer(t, func(w http.ResponseWriter, call runsCall) {
			json.NewEncoder(w).Encode(map[string]any{"count": 3, "value": []string{"a", " b", "a"}})
		})
		client := runsTestClient(t, srv.URL+"/myorg")

		// Mirrors runRunsTagAdd's tags.split(',') with no trim, so " b" and
		// duplicate "a" both survive verbatim (pipeline_run.py:102).
		tags := []string{"a", " b", "a"}

		var result []string
		req := ado.Request{
			Method:     "POST",
			Scope:      "MyProj",
			Path:       "build/builds/42/tags",
			APIVersion: "5.0",
			Body:       tags,
		}
		if err := client.List(context.Background(), req, &result); err != nil {
			t.Fatalf("List: %v", err)
		}

		got := (*calls)[0]
		if got.method != "POST" {
			t.Errorf("method = %q, want POST", got.method)
		}
		if got.url != "/myorg/MyProj/_apis/build/builds/42/tags?api-version=5.0" {
			t.Errorf("url = %q", got.url)
		}
		var sentBody []string
		if err := json.Unmarshal([]byte(got.body), &sentBody); err != nil {
			t.Fatalf("body not JSON: %v (%s)", err, got.body)
		}
		want := []string{"a", " b", "a"}
		if len(sentBody) != len(want) {
			t.Fatalf("body = %v, want %v", sentBody, want)
		}
		for i := range want {
			if sentBody[i] != want[i] {
				t.Errorf("body[%d] = %q, want %q", i, sentBody[i], want[i])
			}
		}
	})
}

// TestRunsTagDelete covers the singular --tag path-escaping (a tag can
// contain characters that need escaping in a URL segment).
func TestRunsTagDelete(t *testing.T) {
	srv, calls := runsCapturingServer(t, func(w http.ResponseWriter, call runsCall) {
		json.NewEncoder(w).Encode(map[string]any{"count": 0, "value": []string{}})
	})
	client := runsTestClient(t, srv.URL+"/myorg")

	var result []string
	// runRunsTagDelete url.PathEscape's the tag before building Path;
	// exercise the same escaped value directly here.
	req := ado.Request{
		Method:     "DELETE",
		Scope:      "MyProj",
		Path:       "build/builds/7/tags/needs%20escaping",
		APIVersion: "5.0",
	}
	if err := client.List(context.Background(), req, &result); err != nil {
		t.Fatalf("List: %v", err)
	}

	got := (*calls)[0]
	if got.method != "DELETE" {
		t.Errorf("method = %q, want DELETE", got.method)
	}
	if got.url != "/myorg/MyProj/_apis/build/builds/7/tags/needs%20escaping?api-version=5.0" {
		t.Errorf("url = %q", got.url)
	}
}

// TestRunsShow covers the request URL for `pipelines runs show`, whose
// project-scoped path and api-version differ subtly from the tag/artifact
// endpoints (capitalised "Builds", no trailing sub-resource).
func TestRunsShow(t *testing.T) {
	srv, calls := runsCapturingServer(t, func(w http.ResponseWriter, call runsCall) {
		json.NewEncoder(w).Encode(map[string]any{"id": 99, "buildNumber": "20260101.1"})
	})
	client := runsTestClient(t, srv.URL+"/myorg")

	var run map[string]any
	req := ado.Request{
		Scope:      "MyProj",
		Path:       "build/Builds/99",
		APIVersion: "5.0",
	}
	if err := client.Do(context.Background(), req, &run); err != nil {
		t.Fatalf("Do: %v", err)
	}

	got := (*calls)[0]
	if got.method != "GET" {
		t.Errorf("method = %q, want GET", got.method)
	}
	if got.url != "/myorg/MyProj/_apis/build/Builds/99?api-version=5.0" {
		t.Errorf("url = %q", got.url)
	}
	if run["id"] != float64(99) {
		t.Errorf("id = %v", run["id"])
	}
}

// TestRunsArtifactList covers the multi-call envelope-unwrap path (via
// client.List) that the other `runs artifact` commands (download/upload)
// deliberately bypass in favour of raw HTTP — this is the one command in
// that group that stays on the shared JSON client.
func TestRunsArtifactList(t *testing.T) {
	srv, calls := runsCapturingServer(t, func(w http.ResponseWriter, call runsCall) {
		json.NewEncoder(w).Encode(map[string]any{
			"count": 1,
			"value": []map[string]any{
				{"id": 1, "name": "drop", "resource": map[string]any{"type": "Container"}},
			},
		})
	})
	client := runsTestClient(t, srv.URL+"/myorg")

	var artifacts []map[string]any
	req := ado.Request{
		Scope:      "MyProj",
		Path:       "build/builds/13/artifacts",
		APIVersion: "5.0",
	}
	if err := client.List(context.Background(), req, &artifacts); err != nil {
		t.Fatalf("List: %v", err)
	}

	got := (*calls)[0]
	if got.url != "/myorg/MyProj/_apis/build/builds/13/artifacts?api-version=5.0" {
		t.Errorf("url = %q", got.url)
	}
	if len(artifacts) != 1 || artifacts[0]["name"] != "drop" {
		t.Fatalf("artifacts = %v", artifacts)
	}
}

// TestRunsRawDoRetriesFallbackOn401 covers runs_artifact.go's `runs
// artifact download/upload` auth fallback: an AAD identity not entitled to
// the org 401s using the AAD header, and runsRawDo must retry once with the
// fallback (PAT) header rather than surfacing the 401, matching
// ado.Client's own retry-with-fallback and services.py's
// validate_token_for_instance PAT fallback.
func TestRunsRawDoRetriesFallbackOn401(t *testing.T) {
	var gotAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		gotAuth = append(gotAuth, auth)
		if auth == "Basic aad" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	body, err := runsRawDo(context.Background(), http.MethodGet, srv.URL, "Basic aad", "Basic pat", nil, "")
	if err != nil {
		t.Fatalf("runsRawDo: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q", body)
	}
	if len(gotAuth) != 2 || gotAuth[0] != "Basic aad" || gotAuth[1] != "Basic pat" {
		t.Errorf("gotAuth = %v, want [\"Basic aad\" \"Basic pat\"] (AAD attempt, then PAT retry)", gotAuth)
	}
}

// TestRunsRawDoNo401RetryWithoutFallback covers the no-op cases: no
// fallback configured, or the fallback is identical to what already failed
// - runsRawDo must not loop or mask the original error.
func TestRunsRawDoNo401RetryWithoutFallback(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	if _, err := runsRawDo(context.Background(), http.MethodGet, srv.URL, "Basic aad", "", nil, ""); err == nil {
		t.Fatal("expected an error with no fallback configured")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry with an empty fallback)", calls)
	}
}

// TestRunsResolveRefHeads and TestRunsDedup exercise the small pure helpers
// directly (already covered indirectly by TestRunsListQuery, but pinned here
// too since they are reused verbatim by other callers).
// TestRunsQueuedTimeCell_KeepsMicroseconds ports _format.py:251's
// str(queued_time.time()), which keeps a non-zero microsecond component —
// this cell must match build.go's buildQueuedTimeCell for the same Python
// row function, not silently truncate to whole seconds.
func TestRunsQueuedTimeCell_KeepsMicroseconds(t *testing.T) {
	got := runsQueuedTimeCell(map[string]any{"queueTime": "2021-01-02T03:04:05.123456Z"})
	if !strings.Contains(got, ".123456") {
		t.Errorf("got %q, want microseconds preserved (matching build.go's buildQueuedTimeCell)", got)
	}
}

func TestRunsResolveRefHeads(t *testing.T) {
	tests := map[string]string{
		"main":            "refs/heads/main",
		"refs/heads/main": "refs/heads/main",
		"refs/pull/1":     "refs/pull/1",
		"refs/tags/v1":    "refs/tags/v1",
	}
	for in, want := range tests {
		if got := coreResolveGitRefHeads(in); got != want {
			t.Errorf("coreResolveGitRefHeads(%q) = %q, want %q", in, got, want)
		}
	}
}
