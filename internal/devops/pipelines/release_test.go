package pipelines

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// releaseHermeticEnv makes ado.NewClient resolve auth without touching a
// real ~/.azure profile or the network: AZ_SESSION points profile.Load() at
// a per-test file that doesn't exist (so AAD resolution fails fast), and
// AZURE_DEVOPS_EXT_PAT supplies the fallback credential. Mirrors
// internal/devops/ado's own newTestClient helper, using only the env-var
// seams available outside that package. ado.ResolveProject/validateOrg
// reject non-dev.azure.com/visualstudio.com hosts (by design — Azure DevOps
// Server is out of scope), so these tests build a *ado.Client directly
// against the httptest server rather than going through cobra flag parsing.
func releaseHermeticEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AZ_SESSION", "release-test-"+t.Name())
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")
}

func releaseTestClient(t *testing.T, org string) *ado.Client {
	t.Helper()
	releaseHermeticEnv(t)
	c, err := ado.NewClient(context.Background(), org)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// releaseNoHostRewrite disables the "vsrm." resource-area subdomain rewrite
// for the duration of the test, so calls into helpers that hardcode
// Host: releaseHost (releaseResolveDefinitionID, releaseCreateRelease) reach
// the plain-host httptest server instead of a nonexistent "vsrm.127.0.0.1".
func releaseNoHostRewrite(t *testing.T) {
	t.Helper()
	orig := releaseHost
	releaseHost = ""
	t.Cleanup(func() { releaseHost = orig })
}

type releaseCall struct {
	method string
	url    string
	body   string
}

func releaseCapturingServer(t *testing.T, responses ...func(w http.ResponseWriter, call releaseCall)) (*httptest.Server, *[]releaseCall) {
	t.Helper()
	calls := &[]releaseCall{}
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		call := releaseCall{method: r.Method, url: r.URL.String(), body: string(b)}
		*calls = append(*calls, call)
		w.Header().Set("Content-Type", "application/json")
		if i >= len(responses) {
			t.Fatalf("unexpected extra request #%d: %s %s", i+1, call.method, call.url)
		}
		responses[i](w, call)
		i++
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

func releaseWriteDefs(w http.ResponseWriter, defs ...map[string]any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"count": len(defs), "value": defs})
}

func TestReleaseListQueryBuilding(t *testing.T) {
	srv, calls := releaseCapturingServer(t, func(w http.ResponseWriter, call releaseCall) {
		w.Write([]byte(`{"count":0,"value":[]}`))
	})
	client := releaseTestClient(t, srv.URL+"/myorg")

	_, err := releaseListPage(context.Background(), client, ado.Request{
		Scope:      "MyProj",
		Path:       "release/releases",
		APIVersion: "5.0",
		Query:      releaseListQuery(5, "", "", "refs/heads/main", "succeeded", 10),
	})
	if err != nil {
		t.Fatalf("releaseListPage: %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("got %d requests, want 1", len(*calls))
	}
	got := (*calls)[0]
	if got.method != http.MethodGet {
		t.Errorf("method = %q, want GET", got.method)
	}
	wantURL := "/myorg/MyProj/_apis/release/releases?%24top=10&api-version=5.0&definitionId=5&sourceBranchFilter=refs%2Fheads%2Fmain&statusFilter=succeeded"
	if got.url != wantURL {
		t.Errorf("url = %q, want %q", got.url, wantURL)
	}
}

func TestReleaseCreateResolvesNameThenPosts(t *testing.T) {
	releaseNoHostRewrite(t)
	srv, calls := releaseCapturingServer(t,
		func(w http.ResponseWriter, call releaseCall) {
			releaseWriteDefs(w, map[string]any{"id": float64(7), "name": "FabCI"})
		},
		func(w http.ResponseWriter, call releaseCall) {
			w.Write([]byte(`{"id":100,"name":"Release-100"}`))
		},
	)
	client := releaseTestClient(t, srv.URL+"/myorg")
	ctx := context.Background()

	id, err := releaseResolveDefinitionID(ctx, client, "MyProj", "FabCI")
	if err != nil {
		t.Fatalf("releaseResolveDefinitionID: %v", err)
	}
	if id != 7 {
		t.Fatalf("resolved id = %d, want 7", id)
	}

	artifacts, err := releaseParseArtifactMetadata([]string{"alias1=42", "alias2=99"})
	if err != nil {
		t.Fatalf("releaseParseArtifactMetadata: %v", err)
	}

	if _, err := releaseCreateRelease(ctx, client, "MyProj", id, artifacts, ""); err != nil {
		t.Fatalf("releaseCreateRelease: %v", err)
	}

	if len(*calls) != 2 {
		t.Fatalf("got %d requests, want 2", len(*calls))
	}

	resolve := (*calls)[0]
	wantResolveURL := "/myorg/MyProj/_apis/release/definitions?api-version=5.0&isExactNameMatch=true&searchText=FabCI"
	if resolve.method != http.MethodGet || resolve.url != wantResolveURL {
		t.Errorf("resolve call = %s %s, want GET %s", resolve.method, resolve.url, wantResolveURL)
	}

	create := (*calls)[1]
	if create.method != http.MethodPost {
		t.Errorf("create method = %q, want POST", create.method)
	}
	wantCreateURL := "/myorg/MyProj/_apis/release/releases?api-version=5.0"
	if create.url != wantCreateURL {
		t.Errorf("create url = %q, want %q", create.url, wantCreateURL)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(create.body), &body); err != nil {
		t.Fatalf("unmarshal create body: %v", err)
	}
	if body["definitionId"] != float64(7) {
		t.Errorf("definitionId = %v, want 7 (resolved from name)", body["definitionId"])
	}
	// msrest drops a None description entirely rather than sending a JSON
	// null (release.py:55); reading a Go map's zero value can't tell "key
	// absent" from "key present with value null" apart, so check presence
	// via the comma-ok form, not just the value.
	if _, ok := body["description"]; ok {
		t.Errorf("body = %+v, want no \"description\" key (not supplied)", body)
	}
	artifactsOut, ok := body["artifacts"].([]any)
	if !ok || len(artifactsOut) != 2 {
		t.Fatalf("artifacts = %v, want 2 entries", body["artifacts"])
	}
	first := artifactsOut[0].(map[string]any)
	if first["alias"] != "alias1" {
		t.Errorf("artifacts[0].alias = %v, want alias1", first["alias"])
	}
	ref := first["instanceReference"].(map[string]any)
	if ref["id"] != "42" {
		t.Errorf("artifacts[0].instanceReference.id = %v, want 42", ref["id"])
	}
}

// TestReleaseCreateNoDefinitionErrorText and
// TestReleaseDefinitionShowNoIDErrorText pin the exact Python error text
// (release.py:35-36, release_definition.py:61) — this port had re-worded it
// (lowercased leading word, dropped the trailing period).
func TestReleaseCreateNoDefinitionErrorText(t *testing.T) {
	releaseHermeticEnv(t)
	cmd := releaseNewCreateCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("project", "MyProj")
	cmd.Flags().Set("detect", "false")

	err := releaseRunCreate(context.Background(), cmd, 0, "", nil, "", false)
	const want = "Either the --definition-id argument or the --definition-name argument must be supplied for this command."
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestReleaseDefinitionShowNoIDErrorText(t *testing.T) {
	releaseHermeticEnv(t)
	cmd := releaseDefinitionNewShowCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("project", "MyProj")
	cmd.Flags().Set("detect", "false")

	err := releaseDefinitionRunShow(context.Background(), cmd, 0, "", false)
	const want = "Either the --id argument or the --name argument must be supplied for this command."
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestReleaseParseArtifactMetadataRejectsMissingEquals(t *testing.T) {
	if _, err := releaseParseArtifactMetadata([]string{"no-equals-sign"}); err == nil {
		t.Fatal("expected an error for a malformed --artifact-metadata-list entry")
	}

	artifacts, err := releaseParseArtifactMetadata(nil)
	if err != nil {
		t.Fatalf("releaseParseArtifactMetadata(nil): %v", err)
	}
	if artifacts == nil || len(artifacts) != 0 {
		t.Errorf("artifacts = %v, want a non-nil empty slice", artifacts)
	}
}

func TestReleaseDefinitionListHardcodesQueryOrderAndLowersArtifactType(t *testing.T) {
	srv, calls := releaseCapturingServer(t, func(w http.ResponseWriter, call releaseCall) {
		w.Write([]byte(`{"count":0,"value":[]}`))
	})
	client := releaseTestClient(t, srv.URL+"/myorg")

	_, err := releaseListPage(context.Background(), client, ado.Request{
		Scope:      "MyProj",
		Path:       "release/definitions",
		APIVersion: "5.0",
		Query:      releaseDefinitionListQuery("", 0, "build", ""),
	})
	if err != nil {
		t.Fatalf("releaseListPage: %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("got %d requests, want 1", len(*calls))
	}
	wantURL := "/myorg/MyProj/_apis/release/definitions?api-version=5.0&artifactType=build&queryOrder=nameAscending"
	if (*calls)[0].url != wantURL {
		t.Errorf("url = %q, want %q", (*calls)[0].url, wantURL)
	}
}

func TestReleaseResolveDefinitionIDAmbiguousNameUsesProjectNameForUUID(t *testing.T) {
	releaseNoHostRewrite(t)
	projectGUID := "11111111-2222-3333-4444-555555555555"
	srv, _ := releaseCapturingServer(t, func(w http.ResponseWriter, call releaseCall) {
		releaseWriteDefs(w,
			map[string]any{"id": float64(1), "name": "FabCI", "project": map[string]any{"name": "Contoso"}},
			map[string]any{"id": float64(2), "name": "FabCI", "project": map[string]any{"name": "Contoso"}},
		)
	})
	client := releaseTestClient(t, srv.URL+"/myorg")

	_, err := releaseResolveDefinitionID(context.Background(), client, projectGUID, "FabCI")
	if err == nil {
		t.Fatal("expected an ambiguous-match error")
	}
	want := fmt.Sprintf("Multiple definitions were found matching name %q in project %q.  Try supplying the definition ID.", "FabCI", "Contoso")
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
