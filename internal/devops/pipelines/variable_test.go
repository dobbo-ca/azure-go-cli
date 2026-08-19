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
)

// variableCall is one HTTP request seen by a variableCapturingServer.
type variableCall struct {
	method string
	url    string
	body   string
}

// variableCapturingServer records every request and answers each in turn
// with the corresponding responder, matching the release_test.go precedent
// in this package (releaseCapturingServer) for driving multi-call sequences.
func variableCapturingServer(t *testing.T, responses ...func(w http.ResponseWriter, call variableCall)) (*httptest.Server, *[]variableCall) {
	t.Helper()
	calls := &[]variableCall{}
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		call := variableCall{method: r.Method, url: r.URL.String(), body: string(b)}
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

// variableTestClient builds an *ado.Client against srv directly, bypassing
// ado.Resolve* (whose org-URL validation rejects a plain httptest URL) with
// a hermetic, network-free auth path — same approach as
// internal/devops/pipelines/release_test.go and agentpool_test.go.
func variableTestClient(t *testing.T, srv *httptest.Server) *ado.Client {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	client, err := ado.NewClient(context.Background(), srv.URL+"/myorg")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

// TestVariableGroupFetchPut covers the GET/PUT pair every variable-group and
// variable-group-variable command builds on: route casing
// (distributedtask/variablegroups, all lowercase), api-version
// (5.0-preview.1), and that Put round-trips whatever body it is given
// (recording-verified: Python PUTs the full fetched object back unfiltered,
// not a narrowed VariableGroupParameters shape).
func TestVariableGroupFetchPut(t *testing.T) {
	srv, calls := variableCapturingServer(t,
		func(w http.ResponseWriter, call variableCall) {
			w.Write([]byte(`{"id":48,"name":"g1","type":"Vsts","description":"d","variables":{"v1":{"value":"a"}},"createdBy":{"id":"u1"}}`))
		},
		func(w http.ResponseWriter, call variableCall) {
			w.Write([]byte(`{"id":48,"name":"g1"}`))
		},
	)
	client := variableTestClient(t, srv)
	ctx := context.Background()

	group, err := variableGroupFetch(ctx, client, "MyProj", 48)
	if err != nil {
		t.Fatalf("variableGroupFetch: %v", err)
	}
	if group["name"] != "g1" || group["createdBy"] == nil {
		t.Fatalf("fetched group missing round-tripped fields: %v", group)
	}

	if _, err := variableGroupPut(ctx, client, "MyProj", 48, group); err != nil {
		t.Fatalf("variableGroupPut: %v", err)
	}

	if len(*calls) != 2 {
		t.Fatalf("got %d requests, want 2", len(*calls))
	}
	get, put := (*calls)[0], (*calls)[1]

	wantURL := "/myorg/MyProj/_apis/distributedtask/variablegroups/48?api-version=5.0-preview.1"
	if get.method != http.MethodGet || get.url != wantURL {
		t.Errorf("GET call = %s %s, want GET %s", get.method, get.url, wantURL)
	}
	if put.method != http.MethodPut || put.url != wantURL {
		t.Errorf("PUT call = %s %s, want PUT %s", put.method, put.url, wantURL)
	}

	var putBody map[string]any
	if err := json.Unmarshal([]byte(put.body), &putBody); err != nil {
		t.Fatalf("unmarshal PUT body: %v", err)
	}
	// The full fetched object -- including "createdBy", which a narrowed
	// VariableGroupParameters-shaped body would have dropped -- must be
	// present on the wire.
	if putBody["createdBy"] == nil {
		t.Errorf("PUT body dropped createdBy, want it round-tripped unfiltered: %v", putBody)
	}
	if putBody["description"] != "d" {
		t.Errorf("PUT body description = %v, want \"d\"", putBody["description"])
	}
}

// TestVariableGroupAuthorize covers the build-client authorizedresources
// GET/PATCH pair (pipeline_utils.get_authorize_resource /
// set_authorize_resource): query param names on GET, and that "id" travels
// as a STRING on the PATCH body wire (recording-verified against
// test_variable_group.yaml: `"id": "47"`, not a number) even though groupID
// is an int everywhere else.
func TestVariableGroupAuthorize(t *testing.T) {
	srv, calls := variableCapturingServer(t,
		func(w http.ResponseWriter, call variableCall) {
			w.Write([]byte(`{"count":0,"value":[]}`))
		},
		func(w http.ResponseWriter, call variableCall) {
			w.Write([]byte(`{"count":1,"value":[{"type":"variablegroup","id":"47","authorized":true}]}`))
		},
	)
	client := variableTestClient(t, srv)
	ctx := context.Background()

	// Empty value array -> nil, not false: variableGroupAuthorizedResult is
	// what coerces nil to false, not this helper.
	got, err := variableGroupGetAuthorized(ctx, client, "MyProj", 47)
	if err != nil {
		t.Fatalf("variableGroupGetAuthorized (empty): %v", err)
	}
	if got != nil {
		t.Errorf("authorized = %v, want nil for an empty authorizedresources list", got)
	}

	if err := variableGroupSetAuthorized(ctx, client, "MyProj", 47, "group1", true); err != nil {
		t.Fatalf("variableGroupSetAuthorized: %v", err)
	}

	if len(*calls) != 2 {
		t.Fatalf("got %d requests, want 2", len(*calls))
	}
	get, patch := (*calls)[0], (*calls)[1]

	wantGetURL := "/myorg/MyProj/_apis/build/authorizedresources?api-version=5.0-preview.1&id=47&type=variablegroup"
	if get.method != http.MethodGet || get.url != wantGetURL {
		t.Errorf("GET call = %s %s, want GET %s", get.method, get.url, wantGetURL)
	}

	wantPatchURL := "/myorg/MyProj/_apis/build/authorizedresources?api-version=5.0-preview.1"
	if patch.method != http.MethodPatch || patch.url != wantPatchURL {
		t.Errorf("PATCH call = %s %s, want PATCH %s", patch.method, patch.url, wantPatchURL)
	}

	var patchBody []map[string]any
	if err := json.Unmarshal([]byte(patch.body), &patchBody); err != nil {
		t.Fatalf("unmarshal PATCH body: %v", err)
	}
	if len(patchBody) != 1 {
		t.Fatalf("PATCH body = %v, want 1 entry", patchBody)
	}
	entry := patchBody[0]
	if id, ok := entry["id"].(string); !ok || id != "47" {
		t.Errorf("PATCH body id = %#v (%T), want string \"47\"", entry["id"], entry["id"])
	}
	if entry["type"] != "variablegroup" || entry["authorized"] != true || entry["name"] != "group1" {
		t.Errorf("PATCH body entry = %v", entry)
	}
}

// TestVariableResolvePipelineID covers the build/Definitions name-filtered
// lookup shared by every `az pipelines variable` command, including
// route-casing (capital D, unlike variablegroups' lowercase route) and the
// 0/1/many-match branching get_definition_id_from_name implements.
func TestVariableResolvePipelineID(t *testing.T) {
	tests := []struct {
		name    string
		matches []map[string]any
		wantID  int
		wantErr bool
	}{
		{"exact match", []map[string]any{{"id": float64(7), "name": "FabCI"}}, 7, false},
		{"no match", nil, 0, true},
		{"ambiguous", []map[string]any{{"id": float64(1)}, {"id": float64(2)}}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, calls := variableCapturingServer(t, func(w http.ResponseWriter, call variableCall) {
				_ = json.NewEncoder(w).Encode(map[string]any{"count": len(tt.matches), "value": tt.matches})
			})
			client := variableTestClient(t, srv)

			id, err := variableResolvePipelineID(context.Background(), client, "MyProj", "FabCI")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got id=%d", id)
				}
				return
			}
			if err != nil {
				t.Fatalf("variableResolvePipelineID: %v", err)
			}
			if id != tt.wantID {
				t.Errorf("id = %d, want %d", id, tt.wantID)
			}

			wantURL := "/myorg/MyProj/_apis/build/Definitions?api-version=5.0&name=FabCI"
			if len(*calls) != 1 || (*calls)[0].url != wantURL {
				t.Errorf("call = %+v, want GET %s", (*calls)[0], wantURL)
			}
		})
	}
}

// TestVariableResolvePipelineIDAmbiguousNameUsesProjectNameForUUID ports
// build_definition.py:106-111: when --project is a GUID, the ambiguous-match
// error substitutes the resolved project *name* off the first match rather
// than echoing the raw GUID back.
func TestVariableResolvePipelineIDAmbiguousNameUsesProjectNameForUUID(t *testing.T) {
	projectGUID := "11111111-2222-3333-4444-555555555555"
	srv, _ := variableCapturingServer(t, func(w http.ResponseWriter, call variableCall) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"count": 2,
			"value": []map[string]any{
				{"id": float64(1), "name": "FabCI", "project": map[string]any{"name": "Contoso"}},
				{"id": float64(2), "name": "FabCI", "project": map[string]any{"name": "Contoso"}},
			},
		})
	})
	client := variableTestClient(t, srv)

	_, err := variableResolvePipelineID(context.Background(), client, projectGUID, "FabCI")
	if err == nil {
		t.Fatal("expected an ambiguous-match error")
	}
	want := `Multiple definitions were found matching name "FabCI" in project "Contoso". ` +
		`Try supplying the definition ID or folder path to differentiate.`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestVariableCaseInsensitiveGet covers the case-insensitive-lookup-but-
// casing-preserving-on-storage semantics shared by every
// create/update/delete command in this file set (both variable-group
// variables and pipeline variables key off the same helper).
// TestVariableGroupListColumns_NoIsAuthorized ports _format.py:337-341:
// list's rows never carry an `authorized` field (variable_group.py:81-103
// does no per-item authorization lookup), so "Is Authorized" must not be a
// column on `variable-group list`, unlike create/show/update.
func TestVariableGroupListColumns_NoIsAuthorized(t *testing.T) {
	for _, c := range variableGroupListColumns {
		if c.Header == "Is Authorized" {
			t.Fatalf("variableGroupListColumns = %+v, want no \"Is Authorized\" column", variableGroupListColumns)
		}
	}

	found := false
	for _, c := range variableGroupColumns {
		if c.Header == "Is Authorized" {
			found = true
		}
	}
	if !found {
		t.Errorf("variableGroupColumns = %+v, want an \"Is Authorized\" column (create/show/update)", variableGroupColumns)
	}
}

// TestVariableGroupVariableUpdate_EmptyValueNotCountedAsSupplied and
// TestVariablePipelineUpdate_EmptyValueNotCountedAsSupplied port
// variable_group.py:222/pipeline_variables.py:101's `not value` (falsy)
// check on the "at least one field" guard: --value "" alone must still
// error, not be treated as a supplied value (unlike Changed("value"), which
// is true even for an explicitly empty string).
func TestVariableGroupVariableUpdate_EmptyValueNotCountedAsSupplied(t *testing.T) {
	cmd := variableNewGroupVariableUpdateCmd()
	cmd.Flags().Set("group-id", "1")
	cmd.Flags().Set("name", "v")
	cmd.Flags().Set("value", "")

	err := variableRunGroupVariableUpdate(context.Background(), cmd)
	const want = "Atleast one of --new-name, --value or --is-secret, --prompt-value must be specified for update."
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestVariablePipelineUpdate_EmptyValueNotCountedAsSupplied(t *testing.T) {
	cmd := variableNewPipelineUpdateCmd()
	cmd.Flags().Set("name", "v")
	cmd.Flags().Set("value", "")

	err := variableRunPipelineUpdate(context.Background(), cmd)
	const want = "Atleast one of --new-name, --value, --is-secret, --prompt-value or --allow-override must be specified for update."
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// TestVariableTruncate_RuneSafe covers variableTruncate slicing by rune, not
// byte, matching Python's str-slicing truncation semantics.
func TestVariableTruncate_RuneSafe(t *testing.T) {
	s := strings.Repeat("é", 100) // each 'é' is 2 bytes in UTF-8
	got := variableTruncate(s)
	want := strings.Repeat("é", variableValueTruncationLength) + "..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !utf8.ValidString(got) {
		t.Errorf("got %q, not valid UTF-8", got)
	}
}

// TestVariableTruncate_EmptyStaysBlankCell covers the ado.Column Value
// convention: Python's row builder always assigns table_row['Value'] (a nil
// value falls back to an empty string), so a nil/empty variable value must
// render as " " (a kept, blank cell), not "" -- which ado.Print's cellValue
// would instead treat as "omit this column from this row", and would drop
// the whole Value column from a table where every listed variable happens
// to be empty.
func TestVariableTruncate_EmptyStaysBlankCell(t *testing.T) {
	for _, v := range []any{nil, "", 42} {
		if got := variableTruncate(v); got != " " {
			t.Errorf("variableTruncate(%#v) = %q, want %q", v, got, " ")
		}
	}
}

// TestVariableSortedRows_AlphabeticalNotWireOrder pins variableSortedRows'
// chosen row order (alphabetical by name) against a payload whose keys are
// deliberately not in alphabetical order, per the ponytail note on
// variablePrintMap: azure-cli's Python table transformer preserves wire
// order instead, but reproducing that here was judged to cost more than the
// task's ~30-line ceiling, so this test pins the fallback (sorted) behaviour
// actually shipped.
func TestVariableSortedRows_AlphabeticalNotWireOrder(t *testing.T) {
	m := map[string]any{
		"zeta":  map[string]any{"value": "1"},
		"alpha": map[string]any{"value": "2"},
		"mid":   map[string]any{"value": "3"},
	}

	rows := variableSortedRows(m)

	var got []string
	for _, r := range rows {
		got = append(got, r["name"].(string))
	}
	want := []string{"alpha", "mid", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("variableSortedRows(%v) names = %v, want %v", m, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("variableSortedRows(%v) names = %v, want %v", m, got, want)
			break
		}
	}
}

// TestVariableInheritedBool ports variable_group.py:237 (`secret = old_
// value.is_secret if secret is None else secret`): the flag wins when set;
// otherwise the old entry's value; otherwise nil (both unset) so the PUT
// omits the key instead of sending an incorrect explicit false.
func TestVariableInheritedBool(t *testing.T) {
	// flag explicitly set wins, even over a differing old value.
	if got := variableInheritedBool(true, true, map[string]any{"isSecret": false}, "isSecret"); got == nil || *got != true {
		t.Errorf("flag set: got %v, want true", got)
	}

	// flag unset, old value present: inherit it.
	if got := variableInheritedBool(false, false, map[string]any{"isSecret": true}, "isSecret"); got == nil || *got != true {
		t.Errorf("inherit: got %v, want true", got)
	}

	// flag unset, old value absent: nil, not false.
	if got := variableInheritedBool(false, false, map[string]any{}, "isSecret"); got != nil {
		t.Errorf("neither set: got %v, want nil", got)
	}
}

func TestVariableCaseInsensitiveGet(t *testing.T) {
	m := map[string]any{"MyVar": map[string]any{"value": "x"}}

	key, val, found := variableCaseInsensitiveGet(m, "myvar")
	if !found || key != "MyVar" || val["value"] != "x" {
		t.Errorf("got key=%q val=%v found=%v, want MyVar/x/true", key, val, found)
	}

	if _, _, found := variableCaseInsensitiveGet(m, "nope"); found {
		t.Errorf("expected not found for a missing key")
	}
}

// TestVariableParseKeyValues covers --variables key=value parsing
// (variable_group.py:52, split on the FIRST '=' only) and that a malformed
// entry is a controlled error rather than the unpacking crash Python has.
func TestVariableParseKeyValues(t *testing.T) {
	got, err := variableParseKeyValues([]string{"a=1", "b=c=d"})
	if err != nil {
		t.Fatalf("variableParseKeyValues: %v", err)
	}
	if got["a"].(map[string]any)["value"] != "1" {
		t.Errorf("a.value = %v, want 1", got["a"])
	}
	if got["b"].(map[string]any)["value"] != "c=d" {
		t.Errorf("b.value = %v, want \"c=d\" (split on first '=' only)", got["b"])
	}
	if got["a"].(map[string]any)["isSecret"] != false {
		t.Errorf("a.isSecret = %v, want false", got["a"].(map[string]any)["isSecret"])
	}

	if _, err := variableParseKeyValues([]string{"no-equals-sign"}); err == nil {
		t.Fatal("expected an error for a malformed --variables entry")
	}
}

// TestVariableValueOrPromptNonTTYErrors ports pipeline_variables.py:197-208
// _get_value_from_env_or_stdin's verify_is_a_tty_or_raise_error: with no env
// var set and stdin not a TTY (always true under `go test`),
// variableValueOrPrompt must error instead of silently reading an unrelated
// line off stdin as the secret value (that non-TTY-reads-a-line behaviour
// belongs to ado.PromptSecret's own caller, `az devops login`, not here).
func TestVariableValueOrPromptNonTTYErrors(t *testing.T) {
	_, err := variableValueOrPrompt("does-not-exist-var")
	if err == nil {
		t.Fatal("expected an error for a non-TTY secret prompt with no env var set")
	}
}

// TestVariableValueOrPromptUsesEnvVar covers the env-var fast path, which
// must still work and not require a TTY.
func TestVariableValueOrPromptUsesEnvVar(t *testing.T) {
	t.Setenv("AZURE_DEVOPS_EXT_PIPELINE_VAR_myvar", "from-env")
	got, err := variableValueOrPrompt("myvar")
	if err != nil {
		t.Fatalf("variableValueOrPrompt: %v", err)
	}
	if got != "from-env" {
		t.Errorf("got %q, want from-env", got)
	}
}
