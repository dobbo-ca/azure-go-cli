package boards

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newWorkItemCommand wires the `az boards work-item` command group: show,
// create, update, delete, and the nested `relation` subgroup (list-type,
// add, remove, show). Mirrors azext_devops/dev/boards/work_item.py and
// relations.py, registered in dev/boards/commands.py:44-63.
func newWorkItemCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work-item",
		Short: "Manage work items.",
	}

	cmd.AddCommand(workitemShowCmd())
	cmd.AddCommand(workitemCreateCmd())
	cmd.AddCommand(workitemUpdateCmd())
	cmd.AddCommand(workitemDeleteCmd())
	cmd.AddCommand(workitemRelationCommand())

	return cmd
}

// workitemColumns is transform_work_item_table_output's row shape
// (dev/boards/_format.py:48-82), shared by show/create/update.
var workitemColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Type", Value: func(row map[string]any) string { return workitemFieldStr(row, "System.WorkItemType") }},
	{Header: "Title", Value: workitemTitleCell},
	{Header: "Assigned To", Value: workitemAssignedToCell},
	{Header: "State", Value: func(row map[string]any) string { return workitemFieldStr(row, "System.State") }},
}

// workitemRelationColumns is transform_work_item_relations' row shape
// (_format.py:26-38), one row per relation, shared by relation
// add/remove/show.
var workitemRelationColumns = []ado.Column{
	{Header: "Relation Type", Field: "rel"},
	{Header: "Url", Field: "url"},
}

const workitemTitleTruncateLen = 70

func workitemFields(row map[string]any) map[string]any {
	f, _ := row["fields"].(map[string]any)
	return f
}

// workitemFieldStr replicates _transform_work_items_row's " " fallback for a
// missing field (_format.py:53-77) -- knack hides table cells that are
// falsy/empty, so a literal single space is used as the "no value" marker.
func workitemFieldStr(row map[string]any, field string) string {
	f := workitemFields(row)
	if f == nil {
		return " "
	}
	if s, ok := f[field].(string); ok {
		return s
	}
	return " "
}

func workitemTitleCell(row map[string]any) string {
	f := workitemFields(row)
	if f == nil {
		return " "
	}
	title, ok := f["System.Title"].(string)
	if !ok {
		return " "
	}
	runes := []rune(title)
	if len(runes) > workitemTitleTruncateLen {
		title = string(runes[:workitemTitleTruncateLen-3]) + "..."
	}
	return title
}

func workitemAssignedToCell(row map[string]any) string {
	f := workitemFields(row)
	if f == nil {
		return " "
	}
	assigned, ok := f["System.AssignedTo"].(map[string]any)
	if !ok {
		return " "
	}
	name, _ := assigned["uniqueName"].(string)
	if name == "" {
		return " "
	}
	return name
}

// workitemIDFromRow reads the numeric "id" field back out of a decoded
// WorkItem response, tolerating the float64 JSON decodes to.
func workitemIDFromRow(row map[string]any) string {
	switch v := row["id"].(type) {
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// workitemOpenBrowserURL ports _open_work_item (work_item.py:345-354): the
// project segment comes from the *response*'s System.TeamProject field, not
// any --project the caller may have passed (most of these commands don't
// even have one).
func workitemOpenBrowserURL(org string, wi map[string]any) string {
	fields := workitemFields(wi)
	project, _ := fields["System.TeamProject"].(string)
	id := workitemIDFromRow(wi)
	return strings.TrimRight(org, "/") + "/" + url.PathEscape(project) + "/_workitems?id=" + url.PathEscape(id)
}

// workitemFieldOp builds one JSON Patch "add" operation for a System field,
// mirroring _create_work_item_field_patch_operation (work_item.py:362-364).
func workitemFieldOp(field string, value any) map[string]any {
	return map[string]any{"op": "add", "path": "/fields/" + field, "value": value}
}

// workitemParseFieldPairs ports the --fields "field=value" parsing shared by
// create_work_item (work_item.py:83-87) and update_work_item (work_item.py:145-149).
func workitemParseFieldPairs(fields []string) ([]map[string]any, error) {
	ops := []map[string]any{}
	for _, f := range fields {
		kvp := strings.SplitN(f, "=", 2)
		if len(kvp) != 2 {
			return nil, fmt.Errorf(`The --fields argument should consist of space separated "field=value" pairs.`)
		}
		ops = append(ops, workitemFieldOp(kvp[0], kvp[1]))
	}
	return ops, nil
}

var workitemExpandChoices = []string{"none", "relations", "fields", "links", "all"}

func workitemValidateExpand(expand string) error {
	for _, c := range workitemExpandChoices {
		if expand == c {
			return nil
		}
	}
	return fmt.Errorf("--expand must be one of %s", strings.Join(workitemExpandChoices, ", "))
}

// workitemGetByID is a plain GET .../_apis/wit/workitems/{id}?$expand=...,
// org-scoped (no {project} route segment -- work item ids are unique per
// organization). Shared by the relation commands, which always expand All.
func workitemGetByID(ctx context.Context, client *ado.Client, id, expand string) (map[string]any, error) {
	q := url.Values{}
	if expand != "" {
		q.Set("$expand", expand)
	}
	var wi map[string]any
	if err := client.Do(ctx, ado.Request{
		Path:       "wit/workitems/" + url.PathEscape(id),
		APIVersion: "5.0",
		Query:      q,
	}, &wi); err != nil {
		return nil, err
	}
	return wi, nil
}

// workitemExtractRelations pulls wi["relations"] out as a slice of maps,
// defaulting to an empty (not nil) slice so it marshals to "[]" rather than
// "null" -- matches transform_work_item_relations returning [] when
// result['relations'] is None (_format.py:27-28).
func workitemExtractRelations(wi map[string]any) []map[string]any {
	relations := []map[string]any{}
	if rs, ok := wi["relations"].([]any); ok {
		for _, r := range rs {
			if m, ok := r.(map[string]any); ok {
				relations = append(relations, m)
			}
		}
	}
	return relations
}

// workitemFillRelationNames ports fill_friendly_name_for_relations_in_work_item
// (relations.py:121-129): swap each relation's system reference name back to
// its friendly display name.
func workitemFillRelationNames(relationTypes, relations []map[string]any) []map[string]any {
	for _, rel := range relations {
		refName, _ := rel["rel"].(string)
		for _, rt := range relationTypes {
			if rtRef, _ := rt["referenceName"].(string); rtRef == refName {
				rel["rel"] = rt["name"]
				break
			}
		}
	}
	return relations
}

// workitemPrintRelationResult prints a relation add/remove/show result the
// way Python does: relations.py:70,106,118 all `return work_item` (the full
// object), while commands.py:60-63 attach transform_work_item_relations as a
// table_transformer only -- so -o table sees just the relations list, but
// every other format (and any --query) sees the whole work item. ado.Print
// renders one shared value per call, so this replicates its table/--query
// predicate here to choose which value it gets. wi's relations are mutated
// in place by workitemFillRelationNames (relations is a view over the same
// maps), so wi already carries the friendly names by the time this runs.
func workitemPrintRelationResult(cmd *cobra.Command, wi map[string]any, relations []map[string]any) error {
	if ado.TableMode(cmd) {
		return ado.Print(cmd, relations, workitemRelationColumns...)
	}
	return ado.Print(cmd, wi)
}

// --- identity resolution: a self-contained port of dev/common/identities.py,
// used only for --assigned-to on create/update (work_item.py:370-382). ---

// workitemAccountFromIdentity is get_account_from_identity (identities.py:126-129).
func workitemAccountFromIdentity(identity map[string]any) string {
	props, _ := identity["properties"].(map[string]any)
	if account, ok := props["Account"].(map[string]any); ok {
		if v, ok := account["$value"].(string); ok {
			return v
		}
	}
	name, _ := identity["providerDisplayName"].(string)
	return name
}

// workitemResolveIdentityAsUniqueUserID is _resolve_identity_as_unique_user_id
// (work_item.py:370-382): a value containing a space or '@' (past position 0)
// is used verbatim, no identity-service call.
func workitemResolveIdentityAsUniqueUserID(ctx context.Context, client *ado.Client, filter string) (string, error) {
	if strings.Index(filter, " ") > 0 || strings.Index(filter, "@") > 0 {
		return filter, nil
	}
	identity, err := ado.ResolveIdentity(ctx, client, filter)
	if err != nil {
		return "", err
	}
	if identity == nil {
		return "", nil
	}
	return workitemAccountFromIdentity(identity), nil
}

// workitemResolveAssignedTo is the --assigned-to handling shared by create
// and update (work_item.py:61-67, 130-136): trim whitespace, an
// empty-after-trim value clears assignment, otherwise resolve to the unique
// user id.
func workitemResolveAssignedTo(ctx context.Context, client *ado.Client, assignedTo string) (string, error) {
	trimmed := strings.TrimSpace(assignedTo)
	if trimmed == "" {
		return "", nil
	}
	return workitemResolveIdentityAsUniqueUserID(ctx, client, trimmed)
}
