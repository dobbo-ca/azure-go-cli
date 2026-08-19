package devops

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// securityPermissionBitsColumns is transform_resolve_permission_bits
// (dev/team/_format.py:242-257).
var securityPermissionBitsColumns = []ado.Column{
	{Header: "Name", Field: "name"},
	{Header: "Bit", Field: "bit"},
	{Header: "Permission Description", Field: "displayName"},
	{Header: "Permission Value", Field: "effectivePermission"},
}

// securityAddNamespaceIDFlag registers --namespace-id/--id, matching
// context.go's --organization/--org idiom: two independent flags, read with
// a manual fallback (arguments.py:136-137, shared across the whole
// `devops security permission` prefix).
func securityAddNamespaceIDFlag(cmd *cobra.Command) {
	cmd.Flags().String("namespace-id", "", "ID of security namespace.")
	cmd.Flags().String("id", "", "Alias for --namespace-id.")
}

func securityNamespaceID(cmd *cobra.Command) (string, error) {
	id, _ := cmd.Flags().GetString("namespace-id")
	if id == "" {
		id, _ = cmd.Flags().GetString("id")
	}
	if id == "" {
		return "", fmt.Errorf("--namespace-id/--id is required")
	}
	return id, nil
}

// securityQueryACL is security_permission.py's _query_permissions /
// security_client.query_access_control_lists.
func securityQueryACL(ctx context.Context, client *ado.Client, namespaceID, subject, token string, recurse bool) ([]map[string]any, error) {
	q := url.Values{}
	if token != "" {
		q.Set("token", token)
	}
	if subject != "" {
		q.Set("descriptors", subject)
	}
	q.Set("includeExtendedInfo", "true")
	q.Set("recurse", strconv.FormatBool(recurse))

	var acls []map[string]any
	if err := client.List(ctx, ado.Request{
		Path:       "AccessControlLists/" + url.PathEscape(namespaceID),
		APIVersion: "5.0",
		Query:      q,
	}, &acls); err != nil {
		return nil, err
	}
	return acls, nil
}

// securityQuerySecurityNamespaceList is _get_permission_types /
// security_client.query_security_namespaces(security_namespace_id=...): the
// full [SecurityNamespaceDescription] list (namespaceId, name, displayName,
// dataspaceCategory, read/writePermission, ..., actions), before any
// narrowing — what `security permission namespace show` prints (B3).
func securityQuerySecurityNamespaceList(ctx context.Context, client *ado.Client, namespaceID string) ([]map[string]any, error) {
	var namespaces []map[string]any
	if err := client.List(ctx, ado.Request{
		Path:       "SecurityNamespaces/" + url.PathEscape(namespaceID),
		APIVersion: "5.0",
	}, &namespaces); err != nil {
		return nil, err
	}
	return namespaces, nil
}

// securityQuerySecurityNamespace narrows securityQuerySecurityNamespaceList
// to the single namespace's action catalog, for the show/update/reset
// permission flows that only ever consume actions (security_permission.py's
// _resolve_bits, security_permission.py:126-127).
func securityQuerySecurityNamespace(ctx context.Context, client *ado.Client, namespaceID string) ([]map[string]any, error) {
	namespaces, err := securityQuerySecurityNamespaceList(ctx, client, namespaceID)
	if err != nil {
		return nil, err
	}
	// _transform_namespace_details_row/transform_namespace_table_output
	// unconditionally index result[0] with no length guard (not a bug to
	// "fix" per the port's Python-parity policy for observable behaviour),
	// but an out-of-range index is a Go crash where Python would raise
	// IndexError, so return a clear error instead of panicking.
	if len(namespaces) == 0 {
		return nil, fmt.Errorf("security namespace not found: %s", namespaceID)
	}
	actions, _ := namespaces[0]["actions"].([]any)
	out := make([]map[string]any, 0, len(actions))
	for _, a := range actions {
		if m, ok := a.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func securityIntField(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

// securityResolveBits ports security_permission.py's _resolve_bits: cross
// references the (single) ACL's raw allow/deny bitmasks against the
// namespace's action catalog to produce human-readable Allow/Deny/Not set
// rows. changedBits==0 means "display all permissions defined by the
// namespace" (show); a non-zero value narrows to just those bits
// (update/reset).
func securityResolveBits(acls []map[string]any, namespaceActions []map[string]any, changedBits int) ([]map[string]any, error) {
	acesDict, _ := acls[0]["acesDictionary"].(map[string]any)
	if len(acls) > 1 || len(acesDict) > 1 {
		return nil, fmt.Errorf("Multiple entries found in acesDictionary. Please filter the response by token.")
	}

	var ace map[string]any
	for _, v := range acesDict {
		if m, ok := v.(map[string]any); ok {
			ace = m
		}
	}

	allowBit := securityIntField(ace, "allow")
	denyBit := securityIntField(ace, "deny")

	includeExtended, _ := acls[0]["includeExtendedInfo"].(bool)
	effectiveAllow, effectiveDeny := 0, 0
	inheritedAllow, inheritedDeny := 0, 0
	if includeExtended {
		ext, _ := ace["extendedInfo"].(map[string]any)
		effectiveAllow = securityIntField(ext, "effectiveAllow")
		effectiveDeny = securityIntField(ext, "effectiveDeny")
		inheritedAllow = allowBit ^ effectiveAllow
		inheritedDeny = denyBit ^ effectiveDeny
	}

	if changedBits == 0 {
		if len(namespaceActions) == 0 {
			return nil, fmt.Errorf("security namespace has no actions")
		}
		last := securityIntField(namespaceActions[len(namespaceActions)-1], "bit")
		changedBits = 2*last - 1
	}

	var out []map[string]any
	for _, action := range namespaceActions {
		bit := securityIntField(action, "bit")
		if changedBits&bit == 0 {
			continue
		}
		var value string
		switch {
		case effectiveDeny != 0 && bit&effectiveDeny != 0:
			value = "Deny"
			if inheritedDeny&bit != 0 {
				value = "Deny (inherited)"
			}
		case effectiveAllow != 0 && bit&effectiveAllow != 0:
			value = "Allow"
			if inheritedAllow&bit != 0 {
				value = "Allow (inherited)"
			}
		default:
			value = "Not set"
		}
		out = append(out, map[string]any{
			"bit":                 bit,
			"name":                action["name"],
			"displayName":         action["displayName"],
			"effectivePermission": value,
		})
	}
	return out, nil
}

// securityResolvedPermissions is the show/update/reset-shared sequence:
// query the ACL, query the namespace action catalog, then locally resolve
// bits (security_permission.py's show_permissions/update_permissions/
// reset_permissions), returning the value ado.Print should render.
//
// _update_json (security_permission.py:171-178) splices the resolved bits
// into every ace of every ACL's acesDictionary and returns that whole ACL
// list — not the bits array alone — so token/allow/deny/extendedInfo/
// acesDictionary survive in -o json/tsv and --query. The table transformer
// (transform_resolve_permission_bits, _format.py:242-248) digs back into
// result[0].acesDictionary[*].resolvedPermissions at render time, so table
// mode still renders the flat bits rows via securityPermissionBitsColumns.
func securityResolvedPermissions(ctx context.Context, cmd *cobra.Command, client *ado.Client, namespaceID, subject, token string, changedBits int) (any, error) {
	acls, err := securityQueryACL(ctx, client, namespaceID, subject, token, false)
	if err != nil {
		return nil, err
	}
	if len(acls) == 0 {
		return nil, fmt.Errorf("no access control list found for token %q", token)
	}
	namespaceActions, err := securityQuerySecurityNamespace(ctx, client, namespaceID)
	if err != nil {
		return nil, err
	}
	resolved, err := securityResolveBits(acls, namespaceActions, changedBits)
	if err != nil {
		return nil, err
	}

	if ado.TableMode(cmd) {
		return resolved, nil
	}

	for _, acl := range acls {
		acesDict, _ := acl["acesDictionary"].(map[string]any)
		for _, v := range acesDict {
			if ace, ok := v.(map[string]any); ok {
				ace["resolvedPermissions"] = resolved
			}
		}
	}
	return acls, nil
}
