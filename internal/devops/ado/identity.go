package ado

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// uuidRe ports dev/common/uuid.py's is_uuid exactly (an 8-4-4-4-12 hex
// regex, case-insensitive) rather than a general-purpose UUID parser, so the
// "already a GUID, skip resolution" short-circuits fire on exactly the same
// inputs Python's do.
var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-([0-9a-f]{4}-){3}[0-9a-f]{12}$`)

// IsUUID ports is_uuid, dev/common/uuid.py:9-17.
func IsUUID(s string) bool { return uuidRe.MatchString(s) }

// CurrentIdentity is get_current_identity (dev/common/identities.py:90-91):
// the connection's authenticatedUser, via the well-known connectionData
// endpoint (org-scoped, no host override).
func CurrentIdentity(ctx context.Context, client *Client) (map[string]any, error) {
	var data struct {
		AuthenticatedUser map[string]any `json:"authenticatedUser"`
	}
	if err := client.Do(ctx, Request{
		Path:       "connectionData",
		APIVersion: "5.0-preview.1",
	}, &data); err != nil {
		return nil, err
	}
	return data.AuthenticatedUser, nil
}

// ReadIdentities is identity_client.read_identities against a single search
// filter (GET _apis/Identities on the vssps host).
func ReadIdentities(ctx context.Context, client *Client, searchFilter, filterValue string) ([]map[string]any, error) {
	var page struct {
		Value []map[string]any `json:"value"`
	}
	if err := client.Do(ctx, Request{
		Host:       "vssps",
		Path:       "Identities",
		APIVersion: "5.0",
		Query: url.Values{
			"searchFilter": {searchFilter},
			"filterValue":  {filterValue},
		},
	}, &page); err != nil {
		return nil, err
	}
	return page.Value, nil
}

// IdentityDomain reads identity.properties.Domain.$value, the field
// ResolveIdentity uses to disambiguate multiple matches by tenant.
func IdentityDomain(identity map[string]any) (string, bool) {
	props, _ := identity["properties"].(map[string]any)
	domain, _ := props["Domain"].(map[string]any)
	v, ok := domain["$value"].(string)
	return v, ok
}

// ResolveIdentity ports resolve_identity (dev/common/identities.py:49-87):
// "me" resolves to the caller; otherwise it tries one search filter, falls
// back to the other, and disambiguates multiple hits by matching the
// caller's tenant domain.
func ResolveIdentity(ctx context.Context, client *Client, filter string) (map[string]any, error) {
	if strings.EqualFold(filter, "me") {
		return CurrentIdentity(ctx, client)
	}

	// identities.py:60: `find(' ') > 0 or find('@') > 0` — General first when
	// the filter looks like an email/display name, DirectoryAlias first
	// otherwise. A leading space or '@' (index 0) does NOT flip the order.
	first, second := "DirectoryAlias", "General"
	if strings.Index(filter, " ") > 0 || strings.Index(filter, "@") > 0 {
		first, second = "General", "DirectoryAlias"
	}

	identities, err := ReadIdentities(ctx, client, first, filter)
	if err != nil {
		return nil, err
	}
	if len(identities) == 0 {
		identities, err = ReadIdentities(ctx, client, second, filter)
		if err != nil {
			return nil, err
		}
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("Could not resolve identity: %s", filter)
	}
	if len(identities) > 1 {
		current, err := CurrentIdentity(ctx, client)
		if err != nil {
			return nil, err
		}
		currentDomain, _ := IdentityDomain(current)
		var withTenant []map[string]any
		if currentDomain != "" {
			for _, ident := range identities {
				if d, ok := IdentityDomain(ident); ok && d == currentDomain {
					withTenant = append(withTenant, ident)
				}
			}
		}
		if len(withTenant) == 1 {
			return withTenant[0], nil
		}
		return nil, fmt.Errorf("There are multiple identities found for %q Please provide a more specific identifier for this identity.", filter)
	}
	return identities[0], nil
}
