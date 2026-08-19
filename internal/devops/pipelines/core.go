// Package pipelines implements `az pipelines ...`. This file (core.go) plus every
// core_*.go file in this package covers: create, update, list, show, delete, run,
// and the folder group. Other command groups under `az pipelines` (build, runs,
// release, pool, agent, queue, variable-group, variable) are implemented by
// sibling files outside this ownership.
package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newPipelineCommands wires the "pipelines" command tree for the groups this
// file owns: create, update, list, show, delete, run, folder.
func newPipelineCommands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipelines",
		Short: "Manage Azure Pipelines",
		Long:  "Manage Azure Pipelines build pipelines, runs, and folders",
	}

	cmd.AddCommand(coreNewCreateCmd())
	cmd.AddCommand(coreNewUpdateCmd())
	cmd.AddCommand(coreNewListCmd())
	cmd.AddCommand(coreNewShowCmd())
	cmd.AddCommand(coreNewDeleteCmd())
	cmd.AddCommand(coreNewRunCmd())
	cmd.AddCommand(coreNewFolderCmd())

	return cmd
}

// coreTruncate mirrors Python's `s[:n] + suffix if len(s) > n else s`
// (_format.py truncation idiom, used with ".." at n=50 for pipeline/run rows
// and "..." at n=80 for folder/variable descriptions).
// coreTruncate slices by rune (code point), matching Python's str slicing
// (common/format.py:16-17 and friends) — a byte-slice would cut a
// multi-byte rune in half and render U+FFFD.
func coreTruncate(s string, n int, suffix string) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + suffix
	}
	return s
}

// coreStr renders a decoded-JSON value (string/float64/bool/nil) as a plain
// string, for building URLs and other id/name interpolation.
func coreStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprintf("%v", t)
	}
}

// coreFixPathForAPI ports build_definition.py:93-100 fix_path_for_api: a
// folder path with no leading backslash is not interpreted correctly by the
// server, so one is injected.
func coreFixPathForAPI(path string) string {
	if path == "" {
		return path
	}
	path = strings.TrimPrefix(path, "/")
	if !strings.HasPrefix(path, `\`) {
		path = `\` + path
	}
	return path
}

// coreDefinitionColumns builds the `ID, Path, Name, [Draft], Status, Default
// Queue` table columns (_format.py:183-203 _transform_pipeline_row). The
// Draft column is only included when at least one row has quality=="draft"
// (_format.py:166-175 transform_pipelines_table_output) — computed from the
// actual rows being rendered, since ado.Column has no notion of conditional
// columns.
func coreDefinitionColumns(rows []map[string]any) []ado.Column {
	includeDraft := false
	for _, r := range rows {
		if q, _ := r["quality"].(string); q == "draft" {
			includeDraft = true
			break
		}
	}

	cols := []ado.Column{
		{Header: "ID", Value: func(row map[string]any) string { return coreStr(row["id"]) }},
		{Header: "Path", Value: func(row map[string]any) string {
			p, _ := row["path"].(string)
			return coreTruncate(p, 50, "..")
		}},
		{Header: "Name", Value: func(row map[string]any) string {
			n, _ := row["name"].(string)
			return coreTruncate(n, 50, "..")
		}},
	}
	if includeDraft {
		cols = append(cols, ado.Column{Header: "Draft", Value: func(row map[string]any) string {
			if q, _ := row["quality"].(string); q == "draft" {
				return "True"
			}
			return " "
		}})
	}
	cols = append(cols,
		ado.Column{Header: "Status", Value: func(row map[string]any) string {
			if s, _ := row["queueStatus"].(string); s != "" {
				return s
			}
			return " "
		}},
		ado.Column{Header: "Default Queue", Value: func(row map[string]any) string {
			if q, ok := row["queue"].(map[string]any); ok {
				if n, _ := q["name"].(string); n != "" {
					return n
				}
			}
			return " "
		}},
	)
	return cols
}

// coreRunColumns builds the `Run ID, Number, Status, Result, Pipeline ID,
// Pipeline Name, Source Branch, Queued Time, Reason` table columns
// (_format.py:229-253 _transform_pipeline_run_row).
func coreRunColumns() []ado.Column {
	return []ado.Column{
		{Header: "Run ID", Value: func(row map[string]any) string { return coreStr(row["id"]) }},
		{Header: "Number", Value: func(row map[string]any) string { return coreStr(row["buildNumber"]) }},
		{Header: "Status", Value: func(row map[string]any) string { return coreStr(row["status"]) }},
		{Header: "Result", Value: func(row map[string]any) string {
			if r, _ := row["result"].(string); r != "" {
				return r
			}
			return " "
		}},
		{Header: "Pipeline ID", Value: func(row map[string]any) string {
			if d, ok := row["definition"].(map[string]any); ok {
				return coreStr(d["id"])
			}
			return ""
		}},
		{Header: "Pipeline Name", Value: func(row map[string]any) string {
			if d, ok := row["definition"].(map[string]any); ok {
				return coreStr(d["name"])
			}
			return ""
		}},
		{Header: "Source Branch", Value: func(row map[string]any) string {
			sb, _ := row["sourceBranch"].(string)
			if sb == "" {
				return " "
			}
			return strings.TrimPrefix(sb, "refs/heads/")
		}},
		{Header: "Queued Time", Value: buildQueuedTimeCell},
		{Header: "Reason", Value: func(row map[string]any) string { return coreStr(row["reason"]) }},
	}
}

// coreRunOrDefinitionColumns ports _format.py:218-221
// _transform_pipeline_or_row: dispatches on presence of a non-empty
// "buildNumber" key. `pipelines create`/`update` share this with `pipelines
// run` because create can return either a pipeline (--skip-first-run) or a
// run.
func coreRunOrDefinitionColumns(row map[string]any) []ado.Column {
	if bn, _ := row["buildNumber"].(string); bn != "" {
		return coreRunColumns()
	}
	return coreDefinitionColumns([]map[string]any{row})
}

// coreResolveQueryOrder ports pipeline.py:64-72 _resolve_query_order: a
// case-insensitive substring match against the real API enum, falling back
// to "none" (with a warning) if nothing matches. Only ever called with an
// already-validated value (coreQueryOrderChoices, checked by
// coreValidateChoice at the command layer), so the "none" fallback here is
// unreachable in practice, not a silent swallow of a bad --query-order.
func coreResolveQueryOrder(queryOrder string) string {
	if queryOrder == "" {
		return "none"
	}
	values := []string{"definitionNameAscending", "definitionNameDescending", "lastModifiedAscending", "lastModifiedDescending", "none"}
	lower := strings.ToLower(queryOrder)
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), lower) {
			return v
		}
	}
	return "none"
}

// coreQueryOrderChoices is _PIPELINES_QUERY_ORDER (arguments.py:17), the
// choices for `pipelines list --query-order`.
var coreQueryOrderChoices = []string{"NameAsc", "NameDesc", "ModifiedAsc", "ModifiedDesc", "None"}

// coreFolderQueryOrderChoices is _FOLDERS_QUERY_ORDER (arguments.py:28), the
// choices for `pipelines folder list --query-order`.
var coreFolderQueryOrderChoices = []string{"Asc", "Desc", "None"}

// coreCreateRepositoryTypeChoices is `pipelines create --repository-type`'s
// choices (arguments.py:81-82) — narrower than `pipelines list`'s.
var coreCreateRepositoryTypeChoices = []string{"tfsgit", "github"}

// coreValidateChoice checks value against allowed when value is non-empty
// and returns the canonical (allowed-list) casing, matching knack's
// enum_choice_list (CaseInsensitiveList + normalising type) used for
// --query-order (arguments.py:75,121), and type=str.lower used for
// --repository-type (arguments.py:77-79,82) — str.lower differs from
// enum_choice_list in that it lowercases the input rather than restoring
// canonical casing, but since every repository-type choices list is already
// all-lowercase the two are equivalent here. Same shape as
// agentpoolValidateChoice; duplicated rather than shared across files owned
// by different phases of this port.
func coreValidateChoice(value, flag string, allowed []string) (string, error) {
	if value == "" {
		return "", nil
	}
	for _, a := range allowed {
		if strings.EqualFold(value, a) {
			return a, nil
		}
	}
	return "", fmt.Errorf("--%s must be one of %s", flag, strings.Join(allowed, ", "))
}

// coreDefinitionIDByName ports build_definition.py:103-116
// get_definition_id_from_name: resolve a pipeline id from its name (and
// optional folder path), erroring on 0 or >1 matches.
func coreDefinitionIDByName(ctx context.Context, client *ado.Client, project, name, folderPath, apiVersion string) (int, error) {
	q := url.Values{}
	q.Set("name", name)
	if p := coreFixPathForAPI(folderPath); p != "" {
		q.Set("path", p)
	}

	var defs []map[string]any
	if err := client.List(ctx, ado.Request{Scope: project, Path: "build/Definitions", APIVersion: apiVersion, Query: q}, &defs); err != nil {
		return 0, fmt.Errorf("failed to look up pipeline %q: %w", name, err)
	}

	switch len(defs) {
	case 1:
		id, _ := defs[0]["id"].(float64)
		return int(id), nil
	case 0:
		return 0, fmt.Errorf("there were no build definitions matching name %q in project %q", name, project)
	default:
		return 0, fmt.Errorf("multiple definitions were found matching name %q in project %q. Try supplying the definition ID or folder path to differentiate", name, project)
	}
}

// coreResolveRepositoryID ports pipeline.py:220-227 _resolve_repository_as_id:
// a client-side, case-insensitive linear scan of the project's repositories.
// Returns "" (not an error) when nothing matches, matching Python's None.
func coreResolveRepositoryID(ctx context.Context, client *ado.Client, project, name string) (string, error) {
	var repos []map[string]any
	if err := client.List(ctx, ado.Request{Scope: project, Path: "git/repositories", APIVersion: "5.0"}, &repos); err != nil {
		return "", fmt.Errorf("failed to list repositories: %w", err)
	}
	for _, r := range repos {
		if rn, _ := r["name"].(string); strings.EqualFold(rn, name) {
			return coreStr(r["id"]), nil
		}
	}
	return "", nil
}

// coreParseNameValuePairs ports pipeline.py:104-117 set_param_variable: each
// item is split on the first "=". asDict wraps each value as {"value": v},
// used for --variables on the pipelines-run (v6.0, --parameters) path.
func coreParseNameValuePairs(items []string, argName string, asDict bool) (map[string]any, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := map[string]any{}
	for _, item := range items {
		i := strings.Index(item, "=")
		if i < 0 {
			return nil, fmt.Errorf("the --%s argument should consist of space separated \"name=value\" pairs", argName)
		}
		key, val := item[:i], item[i+1:]
		if asDict {
			out[key] = map[string]string{"value": val}
		} else {
			out[key] = val
		}
	}
	return out, nil
}

// coreResolveGitRefHeads ports git.py:143-152 resolve_git_ref_heads: prefixes
// "refs/heads/" unless ref already looks like a full ref. Shared by
// `pipelines create/run`, `pipelines build list/queue` and `pipelines runs
// list` -- every caller guards against "" itself, so the empty-input
// short-circuit below is only belt-and-braces.
func coreResolveGitRefHeads(ref string) string {
	if ref == "" {
		return ref
	}
	if strings.HasPrefix(ref, "refs/heads/") || strings.HasPrefix(ref, "refs/pull/") || strings.HasPrefix(ref, "refs/tags/") {
		return ref
	}
	return "refs/heads/" + ref
}
