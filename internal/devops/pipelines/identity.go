package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// pipelinesResolveIdentityID ports resolve_identity_as_id,
// dev/common/identities.py:13-22: "" (None) passes through, a UUID passes
// through, "me" resolves the caller's own identity, anything else goes
// through resolve_identity's General/DirectoryAlias vssps lookup
// (identities.py:49-68). Used by `pipelines build list --requested-for`
// (build.py:122) and `pipelines runs list --requested-for`
// (pipeline_run.py:75). Mirrors repos/pr.go's prResolveIdentity /
// prCurrentIdentityID / policy.go's policyResolveIdentityID, duplicated here
// rather than exported cross-package since those helpers are unexported.
func pipelinesResolveIdentityID(ctx context.Context, client *ado.Client, filter string) (string, error) {
	if filter == "" {
		return "", nil
	}
	if ado.IsUUID(filter) {
		return filter, nil
	}
	if strings.EqualFold(filter, "me") {
		return pipelinesCurrentIdentityID(ctx, client)
	}
	return pipelinesLookupIdentityID(ctx, client, filter)
}

// pipelinesCurrentIdentityID resolves the caller's own identity id via
// GET _apis/ConnectionData (get_current_identity/get_connection_data,
// dev/common/identities.py:94-95; identical to repos/pr.go's
// prCurrentIdentityID).
func pipelinesCurrentIdentityID(ctx context.Context, client *ado.Client) (string, error) {
	var conn struct {
		AuthenticatedUser struct {
			ID string `json:"id"`
		} `json:"authenticatedUser"`
	}
	if err := client.Do(ctx, ado.Request{
		Path:       "ConnectionData",
		APIVersion: "5.1-preview.1",
	}, &conn); err != nil {
		return "", fmt.Errorf("failed to resolve current identity: %w", err)
	}
	return conn.AuthenticatedUser.ID, nil
}

// pipelinesLookupIdentityID ports resolve_identity, dev/common/identities.py:49-68
// (minus the multi-match same-tenant-domain narrowing at identities.py:74-87,
// same simplification as repos/policy.go's policyResolveIdentityID: a >1
// match always errors asking for a more specific identifier).
func pipelinesLookupIdentityID(ctx context.Context, client *ado.Client, filter string) (string, error) {
	order := []string{"DirectoryAlias", "General"}
	// identities.py:60: `identity_filter.find(' ') > 0 or identity_filter.find('@') > 0`
	// — a leading '@' or space (index 0) does NOT trigger the General-first
	// order, unlike a plain Contains check.
	if strings.Index(filter, " ") > 0 || strings.Index(filter, "@") > 0 {
		order = []string{"General", "DirectoryAlias"}
	}

	var identities []map[string]any
	for _, sf := range order {
		var page []map[string]any
		if err := client.List(ctx, ado.Request{
			Host:       "vssps",
			Path:       "Identities",
			APIVersion: "5.0",
			Query:      url.Values{"searchFilter": {sf}, "filterValue": {filter}},
		}, &page); err != nil {
			return "", fmt.Errorf("failed to resolve identity %q: %w", filter, err)
		}
		if len(page) > 0 {
			identities = page
			break
		}
	}

	if len(identities) == 0 {
		return "", fmt.Errorf("Could not resolve identity: %s", filter)
	}
	if len(identities) > 1 {
		return "", fmt.Errorf("There are multiple identities found for %q. Please provide a more specific identifier for this identity.", filter)
	}
	id, _ := identities[0]["id"].(string)
	return id, nil
}
