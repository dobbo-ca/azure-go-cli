package pipelines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// releaseColumns is the table shape for `release list`/`release create`/
// `release show` — _format.py:105-123 (_transform_release_row).
var releaseColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Definition Name", Field: "releaseDefinition.name"},
	{Header: "Created By", Field: "createdBy.displayName"},
	{Header: "Created On", Value: func(row map[string]any) string {
		s, _ := row["createdOn"].(string)
		return releaseLocalDateTime(s)
	}},
	{Header: "Status", Field: "status"},
	{Header: "Description", Field: "description"},
}

// releaseDefinitionColumns is the table shape for `release definition list`/
// `release definition show` — _format.py:130-146. Unlike releaseColumns,
// "Created On" here is the raw ISO string (not localised) — a faithful,
// deliberate inconsistency vs. release list/show (see py-common.md:145).
var releaseDefinitionColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "CreatedBy", Field: "createdBy.displayName"},
	{Header: "Created On", Field: "createdOn"},
}

// releaseHost is the resource-area subdomain ("vsrm", Release Management)
// every request in this file targets. It's a package-level var, not a
// literal, purely so tests can point it at an httptest server — real
// alternate hosts can't be exercised against httptest (see
// foundation-spec.md §10 / ado/client_test.go's TestHostFor note).
var releaseHost = "vsrm"

var releaseArtifactTypes = []string{"build", "jenkins", "github", "externaltfsbuild", "git", "tfvc"}

func releaseValidArtifactType(v string) bool {
	for _, t := range releaseArtifactTypes {
		if v == t {
			return true
		}
	}
	return false
}

// releaseListQuery builds the query parameters for `release list` —
// release_client.go:562-608 (get_releases), pulled out as a pure function so
// it's testable without an HTTP round trip.
func releaseListQuery(definitionID int, minCreatedTime, maxCreatedTime, sourceBranch, status string, top int) url.Values {
	q := url.Values{}
	if definitionID != 0 {
		q.Set("definitionId", strconv.Itoa(definitionID))
	}
	if minCreatedTime != "" {
		q.Set("minCreatedTime", minCreatedTime)
	}
	if maxCreatedTime != "" {
		q.Set("maxCreatedTime", maxCreatedTime)
	}
	if sourceBranch != "" {
		q.Set("sourceBranchFilter", sourceBranch)
	}
	if status != "" {
		q.Set("statusFilter", status)
	}
	if top != 0 {
		q.Set("$top", strconv.Itoa(top))
	}
	return q
}

// releaseDefinitionListQuery builds the query parameters for
// `release definition list` — release_definition.py:34 hardcodes
// queryOrder='nameAscending' client-side; it is not a flag.
func releaseDefinitionListQuery(name string, top int, artifactType, artifactSourceID string) url.Values {
	q := url.Values{"queryOrder": {"nameAscending"}}
	if name != "" {
		q.Set("searchText", name)
	}
	if top != 0 {
		q.Set("$top", strconv.Itoa(top))
	}
	if artifactType != "" {
		q.Set("artifactType", artifactType)
	}
	if artifactSourceID != "" {
		q.Set("artifactSourceId", artifactSourceID)
	}
	return q
}

// releaseParseArtifactMetadata parses --artifact-metadata-list entries into
// ArtifactMetadata bodies — release.py:42-53. Always returns a non-nil
// slice (possibly empty), matching Python's artifacts=[] default so the
// create body serialises "artifacts": [] rather than null.
func releaseParseArtifactMetadata(list []string) ([]map[string]any, error) {
	artifacts := make([]map[string]any, 0, len(list))
	for _, am := range list {
		pos := strings.Index(am, "=")
		if pos < 0 {
			// release.py:51-52: faithful port of Python's malformed error
			// text, including the missing space before "of" and the value
			// appended with no separator — a deliberate quirk, not fixed.
			return nil, fmt.Errorf("The --artifact_meta_data_list argument should consistof space separated \"alias=version_id\" pairs.%s", am)
		}
		artifacts = append(artifacts, map[string]any{
			"alias": am[:pos],
			"instanceReference": map[string]any{
				"id": am[pos+1:],
			},
		})
	}
	return artifacts, nil
}

// releaseCreateRelease POSTs a ReleaseStartMetadata body and returns the
// created Release — release.py:55-59 (client.create_release).
func releaseCreateRelease(ctx context.Context, client *ado.Client, project string, definitionID int, artifacts []map[string]any, description string) (map[string]any, error) {
	body := map[string]any{
		"definitionId": definitionID,
		"artifacts":    artifacts,
	}
	// release.py:55 passes description=None when not supplied, and msrest
	// drops None attributes from the serialized body entirely (recording
	// test_pipeline_create_and_variables_test.yaml:1487-1488) — omit the
	// key rather than sending an explicit JSON null.
	if description != "" {
		body["description"] = description
	}

	var release map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Host:       releaseHost,
		Scope:      project,
		Path:       "release/releases",
		APIVersion: "5.0",
		Body:       body,
	}, &release); err != nil {
		return nil, err
	}
	return release, nil
}

// releaseListPage does a single GET/unwrap of the Azure DevOps
// {"count":N,"value":[...]} envelope, with NO continuation-token following.
// This intentionally does not use ado.Client.List: the Python SDK methods
// backing every `release`/`release definition` list-shaped call
// (get_releases, get_release_definitions) return exactly one page — there is
// no client-side paging loop to port (see surface/pipelines-b.md's "single
// HTTP call, no paging" notes for release list/release definition list).
func releaseListPage(ctx context.Context, client *ado.Client, r ado.Request) ([]map[string]any, error) {
	var page struct {
		Value []map[string]any `json:"value"`
	}
	if err := client.Do(ctx, r, &page); err != nil {
		return nil, err
	}
	return page.Value, nil
}

// releaseWebURL extracts _links.web.href, tolerating missing links —
// release.py:118-122 (_get_release_web_url) / release_definition.py:86-92.
func releaseWebURL(row map[string]any) string {
	links, _ := row["_links"].(map[string]any)
	web, _ := links["web"].(map[string]any)
	href, _ := web["href"].(string)
	return href
}

// releaseResolveDefinitionID looks up a release definition id by exact name,
// shared by `release create --definition-name` and
// `release definition show --name` — release_definition.py:78-91
// (get_definition_id_from_name).
func releaseResolveDefinitionID(ctx context.Context, client *ado.Client, project, name string) (int, error) {
	defs, err := releaseListPage(ctx, client, ado.Request{
		Host:       releaseHost,
		Scope:      project,
		Path:       "release/definitions",
		APIVersion: "5.0",
		Query: url.Values{
			"searchText":       {name},
			"isExactNameMatch": {"true"},
		},
	})
	if err != nil {
		return 0, err
	}

	switch len(defs) {
	case 1:
		id, _ := defs[0]["id"].(float64)
		return int(id), nil
	case 0:
		return 0, fmt.Errorf("There were no release definitions matching name %q in project %q.", name, project)
	default:
		proj := project
		if ado.IsUUID(project) {
			if p, ok := defs[0]["project"].(map[string]any); ok {
				if n, ok := p["name"].(string); ok {
					proj = n
				}
			}
		}
		return 0, fmt.Errorf("Multiple definitions were found matching name %q in project %q.  Try supplying the definition ID.", name, proj)
	}
}

// releaseLocalDateTime ports _format.py's
// `str(created_on.date()) + ' ' + str(created_on.time())` after converting
// to local time: date as YYYY-MM-DD, time as HH:MM:SS, with a 6-digit
// fractional suffix only when the fraction is non-zero (matching Python's
// datetime.time.__str__).
func releaseLocalDateTime(iso string) string {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		return iso
	}
	t = t.Local()

	s := t.Format("2006-01-02 15:04:05")
	if micro := t.Nanosecond() / 1000; micro != 0 {
		s += fmt.Sprintf(".%06d", micro)
	}
	return s
}
