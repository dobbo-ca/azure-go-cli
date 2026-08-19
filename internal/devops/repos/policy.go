package repos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// Policy type GUIDs, verbatim from dev/repos/policy.py.
const (
	policyTypeApproverCount    = "fa4e907d-c16b-4a4c-9dfa-4906e5d171dd"
	policyTypeRequiredReviewer = "fd2167ab-b0be-447a-8ec8-39368250530e"
	policyTypeMergeStrategy    = "fa4e907d-c16b-4a4c-9dfa-4916e5d171ab"
	policyTypeBuild            = "0609b952-1397-4640-95ec-e00a01b2c241"
	policyTypeFileSize         = "2e26e725-8201-4edd-8bf5-978563c34a80"
	policyTypeCommentRequired  = "c6a1889d-b943-4856-b76f-9e46bb6b0df2"
	policyTypeWorkItemLinking  = "40e92b44-2fe1-4dd6-b3d8-74a9c21d0c6e"
	policyTypeCaseEnforcement  = "7ed39669-655c-494e-b4a0-a08b4da0fcce"
)

// policyMergeDeprecatedError is _MERGE_POLICY_DEPRECATED_OPTION_ERROR, policy.py:217-219.
const policyMergeDeprecatedError = "--use-squash-merge has been deprecated to align with the new merge " +
	"strategies in Azure Repos. Use --allow-squash instead. Refer https://aka.ms/merge-types for more information."

var policyColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Value: policyNameValue},
	{Header: "Is Blocking", Field: "isBlocking"},
	{Header: "Is Enabled", Field: "isEnabled"},
	{Header: "Repository Id", Field: "settings.scope[0].repositoryId"},
	{Header: "Branch", Value: policyBranchValue},
}

// policyNameValue ports _get_policy_display_name, _format.py:42-46: keys off
// presence of "displayName" in settings, not its type, so a JSON null there
// renders blank instead of falling back to type.displayName.
func policyNameValue(row map[string]any) string {
	if settings, ok := row["settings"].(map[string]any); ok {
		if dn, present := settings["displayName"]; present {
			s, _ := dn.(string)
			return s
		}
	}
	if t, ok := row["type"].(map[string]any); ok {
		if dn, ok := t["displayName"].(string); ok {
			return dn
		}
	}
	return ""
}

// policyBranchValue ports the refName/"All Branches" branch of
// _transform_repo_policy_request_row, _format.py:35-38: keys off presence of
// "refName" in scope[0], not its type, so a JSON null there renders blank
// instead of "All Branches".
func policyBranchValue(row map[string]any) string {
	settings, _ := row["settings"].(map[string]any)
	scope, _ := settings["scope"].([]any)
	if len(scope) > 0 {
		if s0, ok := scope[0].(map[string]any); ok {
			if rn, present := s0["refName"]; present {
				s, _ := rn.(string)
				return s
			}
		}
	}
	return "All Branches"
}

// newPolicyCommand wires the "repos policy" group and all typed sub-groups.
func newPolicyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage repository security policies",
		Long:  "Manage Azure Repos branch and repository security policies",
	}
	cmd.AddCommand(newPolicyListCmd())
	cmd.AddCommand(newPolicyShowCmd())
	cmd.AddCommand(newPolicyDeleteCmd())
	cmd.AddCommand(newPolicyCreateCmd())
	cmd.AddCommand(newPolicyUpdateCmd())
	cmd.AddCommand(newPolicyApproverCountCmd())
	cmd.AddCommand(newPolicyMergeStrategyCmd())
	cmd.AddCommand(newPolicyBuildCmd())
	cmd.AddCommand(newPolicyCommentRequiredCmd())
	cmd.AddCommand(newPolicyWorkItemLinkingCmd())
	cmd.AddCommand(newPolicyFileSizeCmd())
	cmd.AddCommand(newPolicyRequiredReviewerCmd())
	cmd.AddCommand(newPolicyCaseEnforcementCmd())
	return cmd
}

// policyAddIDFlags registers --policy-id/--id (policy.py arguments.py:24-25:
// options_list=('--policy-id', '--id')), following the same two-flag pattern
// AddOrgFlags uses for --organization/--org.
func policyAddIDFlags(cmd *cobra.Command) {
	cmd.Flags().String("policy-id", "", "ID of the policy.")
	cmd.Flags().String("id", "", "Alias for --policy-id.")
}

func policyIDValue(cmd *cobra.Command) (string, error) {
	v, _ := cmd.Flags().GetString("policy-id")
	if v == "" {
		v, _ = cmd.Flags().GetString("id")
	}
	if v == "" {
		return "", fmt.Errorf(`required flag(s) "policy-id" not set`)
	}
	return v, nil
}

// policyAddConfigFlags registers --policy-configuration/--config.
func policyAddConfigFlags(cmd *cobra.Command) {
	cmd.Flags().String("policy-configuration", "", `Local file path for configuration file. Please use \backslash when typing in directory path.`)
	cmd.Flags().String("config", "", "Alias for --policy-configuration.")
}

func policyConfigValue(cmd *cobra.Command) (string, error) {
	v, _ := cmd.Flags().GetString("policy-configuration")
	if v == "" {
		v, _ = cmd.Flags().GetString("config")
	}
	if v == "" {
		return "", fmt.Errorf(`required flag(s) "policy-configuration" not set`)
	}
	return v, nil
}

// policyAddTriStateFlag registers name as a tri-state flag mirroring Python's
// get_three_state_flag(): unset means "not passed" (nil, so update commands
// fall back to server state), a bare --flag means true, and --flag=false is
// explicit false. Same mechanism as ado.AddOrgFlags' --detect.
func policyAddTriStateFlag(cmd *cobra.Command, name, help string) {
	cmd.Flags().String(name, "", help)
	cmd.Flags().Lookup(name).NoOptDefVal = "true"
}

// policyTriState reads a flag registered by policyAddTriStateFlag.
func policyTriState(cmd *cobra.Command, name string) (*bool, error) {
	raw, _ := cmd.Flags().GetString(name)
	switch {
	case raw == "" && cmd.Flags().Changed(name):
		// arguments.py:36-39: get_three_state_flag() restricts the value to
		// true/false via argparse choices, so an explicitly empty
		// --flag= is rejected there too — it must not silently become nil
		// here and panic a `*blocking` deref in a create command that
		// otherwise treats Changed+required as "safe to dereference".
		return nil, fmt.Errorf("invalid value %q for --%s; must be true or false", raw, name)
	case raw == "":
		return nil, nil
	case strings.EqualFold(raw, "true"):
		b := true
		return &b, nil
	case strings.EqualFold(raw, "false"):
		b := false
		return &b, nil
	default:
		return nil, fmt.Errorf("invalid value %q for --%s; must be true or false", raw, name)
	}
}

func policyBoolIface(p *bool) any {
	if p == nil {
		return nil
	}
	return *p
}

func policyFalseIfNil(v any) bool {
	b, _ := v.(bool)
	return b
}

// policyCoalesce returns the first non-nil value, matching Python's
// `a if a is not None else b` / dict.get(..., None) chains.
func policyCoalesce(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

var policyBranchPrefixes = []string{"refs/heads/", "refs/pull/", "refs/tags/"}

// policyResolveRefHeads ports resolve_git_ref_heads, dev/common/git.py:143-152.
// Shared with `repos pr create/list` and `repos update --default-branch`;
// every caller guards against "" itself, so the empty short-circuit below is
// only belt-and-braces (Python would prefix "" to "refs/heads/").
func policyResolveRefHeads(ref string) string {
	if ref == "" {
		return ref
	}
	for _, p := range policyBranchPrefixes {
		if strings.HasPrefix(ref, p) {
			return ref
		}
	}
	return "refs/heads/" + ref
}

// policyValidateMatchType validates --branch-match-type against
// _BRANCH_MATCH_KIND_VALUES = ['prefix', 'exact'] (dev/repos/arguments.py:10).
func policyValidateMatchType(v string) (string, error) {
	switch strings.ToLower(v) {
	case "prefix":
		return "prefix", nil
	case "exact":
		return "exact", nil
	default:
		return "", fmt.Errorf("--branch-match-type must be one of: prefix, exact")
	}
}

// policyConfig is the PolicyConfiguration wire body, policy.py:617-640.
type policyConfig struct {
	IsBlocking bool           `json:"isBlocking"`
	IsEnabled  bool           `json:"isEnabled"`
	Type       policyTypeRef  `json:"type"`
	Settings   map[string]any `json:"settings"`
}

type policyTypeRef struct {
	ID string `json:"id"`
}

// policyBuildScoped builds a configuration whose scope carries a branch,
// ports create_configuration_object + createScope for branch != nil,
// policy.py:617-666.
func policyBuildScoped(repositoryID, branch, matchType string, blocking, enabled bool, typeID string, settings map[string]any) policyConfig {
	branch = policyResolveRefHeads(branch)
	if settings == nil {
		settings = map[string]any{}
	}
	settings["scope"] = []any{map[string]any{
		"repositoryId": repositoryID,
		"refName":      branch,
		"matchKind":    matchType,
	}}
	return policyConfig{IsBlocking: blocking, IsEnabled: enabled, Type: policyTypeRef{ID: typeID}, Settings: settings}
}

// policyBuildRepoWide builds a configuration with no branch in scope (used
// only by file-size and case-enforcement, policy.py:437-438,580-581).
func policyBuildRepoWide(repositoryID string, blocking, enabled bool, typeID string, settings map[string]any) policyConfig {
	if settings == nil {
		settings = map[string]any{}
	}
	settings["scope"] = []any{map[string]any{"repositoryId": repositoryID}}
	return policyConfig{IsBlocking: blocking, IsEnabled: enabled, Type: policyTypeRef{ID: typeID}, Settings: settings}
}

// policyCurrent is the subset of a fetched PolicyConfiguration update
// commands read from before mutating and re-sending it whole.
type policyCurrent struct {
	IsBlocking bool           `json:"isBlocking"`
	IsEnabled  bool           `json:"isEnabled"`
	Settings   map[string]any `json:"settings"`
}

func (c policyCurrent) scope() map[string]any {
	scope, _ := c.Settings["scope"].([]any)
	if len(scope) == 0 {
		return map[string]any{}
	}
	m, _ := scope[0].(map[string]any)
	return m
}

func (c policyCurrent) scopeStr(key string) string {
	v, _ := c.scope()[key].(string)
	return v
}

// policyGetCurrent fetches the policy configuration to be updated.
func policyGetCurrent(ctx context.Context, client *ado.Client, dctx ado.Context, policyID string) (policyCurrent, error) {
	var cur policyCurrent
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "policy/configurations/" + url.PathEscape(policyID),
		APIVersion: "5.0",
	}, &cur); err != nil {
		return policyCurrent{}, fmt.Errorf("failed to get policy: %w", err)
	}
	return cur, nil
}

// policyCreate POSTs a new policy configuration.
func policyCreate(ctx context.Context, dctx ado.Context, cfg policyConfig) (map[string]any, error) {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      dctx.Project,
		Path:       "policy/configurations",
		APIVersion: "5.0",
		Body:       cfg,
	}, &result); err != nil {
		return nil, fmt.Errorf("failed to create policy: %w", err)
	}
	return result, nil
}

// policyDoUpdate runs the shared GET-then-PUT update sequence: fetch the
// current policy, let build assemble the merged configuration, PUT it back.
func policyDoUpdate(ctx context.Context, dctx ado.Context, policyID string, build func(cur policyCurrent) (policyConfig, error)) (map[string]any, error) {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return nil, err
	}
	cur, err := policyGetCurrent(ctx, client, dctx, policyID)
	if err != nil {
		return nil, err
	}
	cfg, err := build(cur)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPut,
		Scope:      dctx.Project,
		Path:       "policy/configurations/" + url.PathEscape(policyID),
		APIVersion: "5.0",
		Body:       cfg,
	}, &result); err != nil {
		return nil, fmt.Errorf("failed to update policy: %w", err)
	}
	return result, nil
}

// policyFileNamePatterns ports createFileNamePatterns, policy.py:643-647.
func policyFileNamePatterns(pathFilter string) []string {
	if pathFilter == "" {
		return []string{}
	}
	return strings.Split(pathFilter, ";")
}

// policyResolveReviewerIDs ports resolveIdentityMailsToIds, policy.py:669-685.
// mailList == "" (flag omitted) matches Python's `required_reviewer_ids is
// None` and returns nil, distinct from a whitespace-only string, which also
// resolves to nil (Python: None) per policy.py:674-675 - both cases mean
// "caller should fall back to the current server value".
func policyResolveReviewerIDs(ctx context.Context, org, mailList string) ([]string, error) {
	if strings.TrimSpace(mailList) == "" {
		return nil, nil
	}
	client, err := ado.NewClient(ctx, org)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	for _, m := range strings.Split(mailList, ";") {
		email := strings.TrimSpace(m)
		id, err := policyResolveIdentityID(ctx, client, email)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// policyResolveIdentityID ports resolve_identity/resolve_identity_as_id,
// dev/common/identities.py:13-89, minus the multi-identity
// same-tenant-domain preference branch (identities.py:74-87) - a Go port
// that hits >1 match always errors asking for a more specific identifier,
// same as Python's fallback when the domain-preference narrowing doesn't
// reduce the set to one.
func policyResolveIdentityID(ctx context.Context, client *ado.Client, filter string) (string, error) {
	if ado.IsUUID(filter) {
		return filter, nil
	}
	// identities.py:17-19: ME='me' (identities.py:139) resolves to the
	// caller, checked before any vssps identities search.
	if strings.EqualFold(filter, "me") {
		return prCurrentIdentityID(ctx, client)
	}

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
