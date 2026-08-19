// This file and the other pr_*.go files implement the `az repos pr` command
// group: create/update/show/list/checkout/set-vote plus the reviewer,
// work-item and policy subgroups (dev/repos/pull_request.py). Other repos
// subgroups (core repo commands, policy, ref, import) are implemented in
// sibling files by other contributors.
package repos

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// prColumns is the table shape shared by pr create/show/update/list
// (_format.py:53-73, _transform_pull_request_row via
// transform_pull_request(s)_table_output).
var prColumns = []ado.Column{
	{Header: "ID", Field: "pullRequestId"},
	{Header: "Created", Value: prCreatedCell},
	{Header: "Creator", Field: "createdBy.uniqueName"},
	{Header: "Title", Value: prTitleCell},
	{Header: "Status", Value: prStatusCell},
	{Header: "IsDraft", Value: prIsDraftCell},
	{Header: "Repository", Field: "repository.name"},
}

func prCreatedCell(row map[string]any) string {
	s, _ := row["creationDate"].(string)
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02")
}

// prTitleTruncationLength is _PR_TITLE_TRUNCATION_LENGTH, _format.py:11.
const prTitleTruncationLength = 50

func prTitleCell(row map[string]any) string {
	title, _ := row["title"].(string)
	r := []rune(title)
	if len(r) > prTitleTruncationLength {
		title = string(r[:prTitleTruncationLength-3]) + "..."
	}
	return title
}

func prStatusCell(row map[string]any) string {
	s, _ := row["status"].(string)
	return prCapitalize(s)
}

func prIsDraftCell(row map[string]any) string {
	b, _ := row["isDraft"].(bool)
	if b {
		return "True"
	}
	return "False"
}

// prCapitalize matches Python's str.capitalize(): first rune upper, rest
// lower. Statuses are always plain ASCII words ("active", "queued", ...).
func prCapitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// prReviewerColumns ports _transform_reviewer_row, _format.py:76-107.
var prReviewerColumns = []ado.Column{
	{Header: "Name", Field: "displayName"},
	{Header: "Email", Value: prReviewerEmailCell},
	{Header: "ID", Field: "id"},
	{Header: "Vote", Value: prReviewerVoteCell},
	{Header: "Required", Value: prReviewerRequiredCell},
}

// prReviewerGroupPrefix is _UNIQUE_NAME_GROUP_PREFIX, _format.py:93.
const prReviewerGroupPrefix = "vstfs:///"

// prSortReviewersForTable sorts reviewers required-first, then by
// displayName lowercased (_get_reviewer_table_key, _format.py:88-94), but
// only for actual table rendering: same guard ado.Print uses internally, so
// -o table --query keeps server order (query.py:49 applies the query to the
// raw, un-sorted result).
func prSortReviewersForTable(cmd *cobra.Command, reviewers []map[string]any) {
	if !ado.TableMode(cmd) {
		return
	}
	sort.SliceStable(reviewers, func(i, j int) bool {
		ri, _ := reviewers[i]["isRequired"].(bool)
		rj, _ := reviewers[j]["isRequired"].(bool)
		if ri != rj {
			return ri // required (true) sorts first
		}
		ni, _ := reviewers[i]["displayName"].(string)
		nj, _ := reviewers[j]["displayName"].(string)
		return strings.ToLower(ni) < strings.ToLower(nj)
	})
}

func prReviewerEmailCell(row map[string]any) string {
	u, _ := row["uniqueName"].(string)
	if strings.HasPrefix(u, prReviewerGroupPrefix) {
		return " "
	}
	return u
}

// prReviewerVoteCell ports _get_vote_from_vote_number, _format.py:154-162.
func prReviewerVoteCell(row map[string]any) string {
	v, _ := row["vote"].(float64)
	switch int(v) {
	case 10:
		return "Approved"
	case 5:
		return "Approved with suggestions"
	case -5:
		return "Waiting for author"
	case -10:
		return "Rejected"
	default:
		return " "
	}
}

func prReviewerRequiredCell(row map[string]any) string {
	b, _ := row["isRequired"].(bool)
	if b {
		return "True"
	}
	return "False"
}

// prWorkItemTitleTruncationLength is _WORK_ITEM_TITLE_TRUNCATION_LENGTH, _format.py:12.
const prWorkItemTitleTruncationLength = 70

// prWorkItemColumns ports _transform_work_items_row, _format.py:125-151.
//
// ponytail: Python omits the "Type" key entirely (not just blanks it) when
// the row has no "fields" block at all, giving rows an inconsistent column
// set (_format.py:143-147). ado.Print always renders a fixed header set, so
// that row would show "Type" as blank instead of the column being absent —
// an unavoidable, cosmetic-only divergence given the shared table renderer.
var prWorkItemColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Type", Value: prWorkItemFieldCell("System.WorkItemType")},
	{Header: "Assigned To", Value: prWorkItemFieldCell("System.AssignedTo")},
	{Header: "State", Value: prWorkItemFieldCell("System.State")},
	{Header: "Title", Value: prWorkItemTitleCell},
}

func prWorkItemFieldCell(field string) func(map[string]any) string {
	return func(row map[string]any) string {
		fields, _ := row["fields"].(map[string]any)
		v, ok := fields[field]
		if !ok {
			return " "
		}
		switch t := v.(type) {
		case string:
			return t
		case map[string]any:
			if dn, ok := t["displayName"].(string); ok {
				return dn
			}
		}
		return " "
	}
}

func prWorkItemTitleCell(row map[string]any) string {
	fields, _ := row["fields"].(map[string]any)
	title, ok := fields["System.Title"].(string)
	if !ok {
		return " "
	}
	r := []rune(title)
	if len(r) > prWorkItemTitleTruncationLength {
		title = string(r[:prWorkItemTitleTruncationLength-3]) + "..."
	}
	return title
}

// prPolicyColumns ports _transform_policy_row, _format.py:216-236.
//
// ponytail: skips the single-required-reviewer display-name suffix in the
// "Policy" cell. Python resolves it via an extra vssps identities lookup
// keyed off a process-global "first org this process ever talked to" cache
// (transform_policies_table_output, _format.py:175-192) — a latent bug the
// foundation spec itself calls out as not worth reproducing. Add a batched
// identity lookup here if the decoration is needed. The len>1 "(N)" count
// suffix in _build_policy_name (_format.py:248-249) is gated on that same
// resolved name, which is only ever supplied for len==1 (_format.py:195-199)
// — so Python never actually emits the len>1 suffix; prPolicyNameCell
// doesn't either.
var prPolicyColumns = []ado.Column{
	{Header: "Evaluation ID", Field: "evaluationId"},
	{Header: "Policy", Value: prPolicyNameCell},
	{Header: "Blocking", Value: prPolicyBlockingCell},
	{Header: "Status", Value: prPolicyStatusCell},
	{Header: "Expired", Value: prPolicyExpiredCell},
	{Header: "Build ID", Value: prPolicyBuildIDCell},
}

func prPolicyNameCell(row map[string]any) string {
	cfg, _ := row["configuration"].(map[string]any)
	typ, _ := cfg["type"].(map[string]any)
	name, _ := typ["displayName"].(string)
	settings, _ := cfg["settings"].(map[string]any)
	if dn, ok := settings["displayName"].(string); ok && dn != "" {
		name += " (" + dn + ")"
	}
	if mac, ok := settings["minimumApproverCount"]; ok && mac != nil {
		name += fmt.Sprintf(" (%v)", mac)
	}
	return name
}

// prSortPolicyEvalsForTable sorts policy evaluations blocking-first, then by
// the rendered "Policy" cell lowercased (_get_policy_table_key,
// _format.py:207-213), only for actual table rendering — same guard
// prSortReviewersForTable uses, so -o table --query keeps server order.
func prSortPolicyEvalsForTable(cmd *cobra.Command, evals []map[string]any) {
	if !ado.TableMode(cmd) {
		return
	}
	sort.SliceStable(evals, func(i, j int) bool {
		ci, _ := evals[i]["configuration"].(map[string]any)
		cj, _ := evals[j]["configuration"].(map[string]any)
		bi, _ := ci["isBlocking"].(bool)
		bj, _ := cj["isBlocking"].(bool)
		if bi != bj {
			return bi // blocking sorts first
		}
		return strings.ToLower(prPolicyNameCell(evals[i])) < strings.ToLower(prPolicyNameCell(evals[j]))
	})
}

func prPolicyBlockingCell(row map[string]any) string {
	cfg, _ := row["configuration"].(map[string]any)
	b, _ := cfg["isBlocking"].(bool)
	if b {
		return "True"
	}
	return "False"
}

// prPolicyStatusCell ports _convert_policy_status, _format.py:255-258.
func prPolicyStatusCell(row map[string]any) string {
	s, _ := row["status"].(string)
	if s == "queued" {
		return " "
	}
	return prCapitalize(s)
}

func prPolicyExpiredCell(row map[string]any) string {
	ctxm, _ := row["context"].(map[string]any)
	if ctxm == nil {
		return " "
	}
	if b, ok := ctxm["isExpired"].(bool); ok {
		if b {
			return "True"
		}
		return "False"
	}
	return " "
}

func prPolicyBuildIDCell(row map[string]any) string {
	ctxm, _ := row["context"].(map[string]any)
	if ctxm == nil {
		return " "
	}
	if v, ok := ctxm["buildId"]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return " "
}

// newPRCommand wires the `az repos pr` command group: create/update/show/
// list/checkout/set-vote plus the reviewer, work-item and policy subgroups.
func newPRCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Manage pull requests",
		Long:  "Manage Azure Repos pull requests",
	}

	cmd.AddCommand(newPRCreateCmd())
	cmd.AddCommand(newPRUpdateCmd())
	cmd.AddCommand(newPRShowCmd())
	cmd.AddCommand(newPRListCmd())
	cmd.AddCommand(newPRCheckoutCmd())
	cmd.AddCommand(newPRSetVoteCmd())
	cmd.AddCommand(newPRReviewerCommand())
	cmd.AddCommand(newPRWorkItemCommand())
	cmd.AddCommand(newPRPolicyCommand())

	return cmd
}

// prClientOrg resolves only the organization (ado.Resolve) and builds a
// client — used by every pr subcommand whose Python function declares no
// --project/--repository (show/update/checkout/set-vote/reviewer/work-item/
// policy all take the PR id and discover project+repo from the by-id GET).
// Split out from each RunE so tests can exercise the *Exec half against an
// httptest server without going through ado.Resolve's org-URL validation —
// same seam as refClient (ref_common.go) and repoTestClient (repo_test.go).
func prClientOrg(ctx context.Context, cmd *cobra.Command) (*ado.Client, ado.Context, error) {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return nil, ado.Context{}, err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return nil, ado.Context{}, err
	}
	return client, dctx, nil
}

// prClientProject resolves organization + project (ado.ResolveProject) —
// used by `pr list`.
func prClientProject(ctx context.Context, cmd *cobra.Command) (*ado.Client, ado.Context, error) {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return nil, ado.Context{}, err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return nil, ado.Context{}, err
	}
	return client, dctx, nil
}

// prClientRepo resolves organization + project + repository
// (ado.ResolveRepo) — used by `pr create` (see prRunCreate's deviation
// comment on why create needs a concrete repo, unlike Python).
func prClientRepo(ctx context.Context, cmd *cobra.Command) (*ado.Client, ado.Context, error) {
	dctx, err := ado.ResolveRepo(cmd)
	if err != nil {
		return nil, ado.Context{}, err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return nil, ado.Context{}, err
	}
	return client, dctx, nil
}

// prGetByID fetches a pull request by id, cross-project
// (get_pull_request_by_id, git_client_base.py:1886-1902) — the only lookup
// available before the caller knows which project/repo the PR lives in.
func prGetByID(ctx context.Context, client *ado.Client, id string) (map[string]any, error) {
	var pr map[string]any
	if err := client.Do(ctx, ado.Request{
		Path:       "git/pullRequests/" + url.PathEscape(id),
		APIVersion: "5.0",
	}, &pr); err != nil {
		return nil, fmt.Errorf("failed to get pull request %s: %w", id, err)
	}
	return pr, nil
}

// prRepoProjectID extracts the repository and project GUIDs off a fetched
// pull request, as used to scope every follow-up call.
func prRepoProjectID(pr map[string]any) (repoID, projectID string) {
	repo, _ := pr["repository"].(map[string]any)
	repoID, _ = repo["id"].(string)
	project, _ := repo["project"].(map[string]any)
	projectID, _ = project["id"].(string)
	return
}

// prRepoNames extracts the repository and project names off a fetched pull
// request (update_pull_request uses names, not GUIDs — pull_request.py:355-356).
func prRepoNames(pr map[string]any) (repoName, projectName string) {
	repo, _ := pr["repository"].(map[string]any)
	repoName, _ = repo["name"].(string)
	project, _ := repo["project"].(map[string]any)
	projectName, _ = project["name"].(string)
	return
}

// prIDString normalises pullRequestId (a JSON number once decoded) back to
// the string form needed for path segments.
func prIDString(pr map[string]any) string {
	switch v := pr["pullRequestId"].(type) {
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// prOpenInBrowser opens pr's web page: {org}/{project}/_git/{repo}/pullrequest/{id}
// (_open_pull_request, pull_request.py:685-692). Errors are warned, never
// fatal, matching foundation-spec.md §7.
func prOpenInBrowser(org string, pr map[string]any) {
	repoName, projectName := prRepoNames(pr)
	webURL := strings.TrimRight(org, "/") + "/" + url.PathEscape(projectName) +
		"/_git/" + url.PathEscape(repoName) + "/pullrequest/" + prIDString(pr)
	if err := ado.OpenBrowser(webURL); err != nil {
		logger.Warning("failed to open browser: %v", err)
	}
}

// prCurrentIdentityID resolves the caller's own identity id via
// GET _apis/ConnectionData (get_current_identity/get_connection_data,
// dev/common/identities.py:94-95, dev/common/services.py:420-425;
// LocationClient.get_connection_data, location_client.py:28-47, location
// 00d9565f-ed9c-4a06-9a50-00e7896ccab4, verified path casing + api-version
// from recording test_pull_request_...yaml:1972 "_apis/ConnectionData").
// Ports resolve_identity_as_id(ME, ...).
func prCurrentIdentityID(ctx context.Context, client *ado.Client) (string, error) {
	var conn struct {
		AuthenticatedUser struct {
			ID string `json:"id"`
		} `json:"authenticatedUser"`
	}
	if err := client.Do(ctx, ado.Request{
		Path:       "ConnectionData",
		APIVersion: "5.0-preview.1",
	}, &conn); err != nil {
		return "", fmt.Errorf("failed to resolve current identity: %w", err)
	}
	return conn.AuthenticatedUser.ID, nil
}

// prResolveIdentity ports resolve_identity_as_id, dev/common/identities.py:13-22:
// "" (None) passes through, a UUID passes through, "me" resolves the caller,
// anything else is resolved via the shared vssps identities lookup already
// implemented for `repos policy required-reviewer` (policyResolveIdentityID,
// policy.go — same General/DirectoryAlias search-order logic as
// dev/common/identities.py:57-68, reused rather than duplicated).
func prResolveIdentity(ctx context.Context, client *ado.Client, filter string) (string, error) {
	if filter == "" {
		return "", nil
	}
	if ado.IsUUID(filter) {
		return filter, nil
	}
	if strings.EqualFold(filter, "me") {
		return prCurrentIdentityID(ctx, client)
	}
	return policyResolveIdentityID(ctx, client, filter)
}

// prDedupe ports Python's `list(set(x))` distinct-preserving dedupe (order
// is insertion order here; CPython's set order is hash-seed dependent, so no
// particular Python order exists to match).
func prDedupe(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, i := range items {
		if !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	return out
}

// prLowerDedupe ports `list(set(x.lower() for x in items))`
// (create_pull_request, pull_request.py:181-186).
func prLowerDedupe(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, i := range items {
		l := strings.ToLower(i)
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}

// prRefIDs extracts work item ids off a []ResourceRef response
// (id/url pairs, ResourceRef.id is str — models.py:3023-3038).
func prRefIDs(refs []map[string]any) []string {
	ids := make([]string, 0, len(refs))
	for _, r := range refs {
		if v, ok := r["id"].(string); ok && v != "" {
			ids = append(ids, v)
		}
	}
	return ids
}
