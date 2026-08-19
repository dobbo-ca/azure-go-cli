package devops

import (
	"context"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// securityResolveIdentityAsID is resolve_identity_as_id: a bare UUID passes
// through unchanged, "me" resolves to the caller's id, otherwise a full
// identity lookup.
func securityResolveIdentityAsID(ctx context.Context, client *ado.Client, filter string) (string, error) {
	if filter == "" || ado.IsUUID(filter) {
		return filter, nil
	}
	if strings.EqualFold(filter, "me") {
		current, err := ado.CurrentIdentity(ctx, client)
		if err != nil {
			return "", err
		}
		id, _ := current["id"].(string)
		return id, nil
	}
	identity, err := ado.ResolveIdentity(ctx, client, filter)
	if err != nil {
		return "", err
	}
	id, _ := identity["id"].(string)
	return id, nil
}

// securityResolveIdentityAsIdentityDescriptor is
// resolve_identity_as_identity_descriptor. Unlike securityResolveIdentityAsID
// it has no is_uuid short-circuit in Python, so neither does this.
func securityResolveIdentityAsIdentityDescriptor(ctx context.Context, client *ado.Client, filter string) (string, error) {
	if filter == "" {
		return filter, nil
	}
	if strings.EqualFold(filter, "me") {
		current, err := ado.CurrentIdentity(ctx, client)
		if err != nil {
			return "", err
		}
		d, _ := current["descriptor"].(string)
		return d, nil
	}
	identity, err := ado.ResolveIdentity(ctx, client, filter)
	if err != nil {
		return "", err
	}
	d, _ := identity["descriptor"].(string)
	return d, nil
}

// securityIdentityDescriptorFromSubjectDescriptor is
// get_identity_descriptor_from_subject_descriptor: resolve a graph subject
// descriptor to its identity descriptor, falling back to the input
// unchanged if the lookup returns nothing.
func securityIdentityDescriptorFromSubjectDescriptor(ctx context.Context, client *ado.Client, subjectDescriptor string) (string, error) {
	var page struct {
		Value []map[string]any `json:"value"`
	}
	if err := client.Do(ctx, ado.Request{
		Host:       "vssps",
		Path:       "Identities",
		APIVersion: "5.0",
		Query:      url.Values{"subjectDescriptors": {subjectDescriptor}},
	}, &page); err != nil {
		return "", err
	}
	if len(page.Value) > 0 {
		if d, ok := page.Value[0]["descriptor"].(string); ok {
			return d, nil
		}
	}
	return subjectDescriptor, nil
}

// securityResolveSubjectAsIdentityDescriptor is security_permission.py's
// _resolve_subject_as_identity_descriptor: an '@' means an email to resolve
// via identity search, a '.' (and no '@') means a graph subject descriptor
// to resolve to its identity descriptor, anything else passes through.
func securityResolveSubjectAsIdentityDescriptor(ctx context.Context, client *ado.Client, subject string) (string, error) {
	if strings.Contains(subject, "@") {
		return securityResolveIdentityAsIdentityDescriptor(ctx, client, subject)
	}
	if strings.Contains(subject, ".") {
		return securityIdentityDescriptorFromSubjectDescriptor(ctx, client, subject)
	}
	return subject, nil
}

// securityResolveMemberDescriptor is the resolution heuristic shared by
// security_group.py's list_memberships/add_membership/remove_membership: an
// '@' or the absence of a '.' means id is an identity filter (email/alias)
// that must be resolved to an id and then a descriptor; a bare descriptor
// (which always contains a '.') passes through unchanged.
func securityResolveMemberDescriptor(ctx context.Context, client *ado.Client, id string) (string, error) {
	if strings.Contains(id, "@") || !strings.Contains(id, ".") {
		identID, err := securityResolveIdentityAsID(ctx, client, id)
		if err != nil {
			return "", err
		}
		return securityDescriptorFromStorageKey(ctx, client, identID)
	}
	return id, nil
}
