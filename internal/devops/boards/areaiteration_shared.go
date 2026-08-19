package boards

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// Structure groups for the classification-node tree, matching
// iteration.py:22 / area.py:16.
const (
	areaiterationStructureGroupArea      = "areas"
	areaiterationStructureGroupIteration = "iterations"
)

// areaiterationAddTeamFlag registers the required --team flag shared by
// every `boards area team`/`boards iteration team` command. No Python
// argument_context declares a short alias for it (checked against
// dev/boards/arguments.py and dev/team/arguments.py) — plain --team only.
func areaiterationAddTeamFlag(cmd *cobra.Command) {
	cmd.Flags().String("team", "", "The name or id of the team.")
	_ = cmd.MarkFlagRequired("team")
}

// areaiterationTeamScope builds the project/team Request.Scope segment.
func areaiterationTeamScope(project, team string) string {
	return project + "/" + team
}

// areaiterationClassificationColumns is _transform_project_classification_node_row
// (_format.py:205-218): ID, Identifier, Name, [Start Date, Finish Date if
// attributes present], Path (truncated to 50 chars), Has Children. Start
// Date/Finish Date are dropped by areaiterationVisibleColumns when no row
// has an attributes object (area nodes never have one) — Has Children uses
// a Value func rather than Field so areaiterationVisibleColumns' falsy-value
// check never drops it (Python sets it unconditionally, _format.py:217,
// even when every row's hasChildren is false).
var areaiterationClassificationColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Identifier", Field: "identifier"},
	{Header: "Name", Field: "name"},
	{Header: "Start Date", Field: "attributes.startDate"},
	{Header: "Finish Date", Field: "attributes.finishDate"},
	{Header: "Path", Value: func(row map[string]any) string {
		p, _ := row["path"].(string)
		r := []rune(p)
		if len(r) > 50 {
			p = string(r[:47]) + "..."
		}
		return p
	}},
	{Header: "Has Children", Value: func(row map[string]any) string {
		return ado.TSVScalar(row["hasChildren"])
	}},
}

// areaiterationFlattenNodes ports
// transform_work_item_project_classification_nodes_table_output_recursive
// (_format.py:190-197): depth-first flatten of node.children into one row
// per node, table view only.
func areaiterationFlattenNodes(node map[string]any) []map[string]any {
	rows := []map[string]any{node}
	if children, ok := node["children"].([]any); ok {
		for _, c := range children {
			if child, ok := c.(map[string]any); ok {
				rows = append(rows, areaiterationFlattenNodes(child)...)
			}
		}
	}
	return rows
}

// areaiterationPrintClassificationTree renders a single classification node
// (with possible nested .children). -o table renders the recursive flatten
// (matching the registered table_transformer); every other format, and
// --query, render the node exactly as returned (nested tree), matching
// Python's un-transformed JSON output. Mirrors ado.Print's own table-vs-not
// condition (internal/devops/repos/ref_list.go uses the same
// pre-shape-by-format pattern for its table-only sort).
func areaiterationPrintClassificationTree(cmd *cobra.Command, node map[string]any) error {
	if ado.TableMode(cmd) {
		rows := areaiterationFlattenNodes(node)
		return ado.Print(cmd, rows, areaiterationVisibleColumns(rows, areaiterationClassificationColumns)...)
	}
	return ado.Print(cmd, node)
}

// areaiterationNodePath builds the "wit/classificationnodes/{group}[/path]"
// request path. path is the already root-stripped relative path; empty
// means the group root, matching Python dropping the {path} route segment
// entirely when path is None (iteration.py:145-148, area.py:63-66).
func areaiterationNodePath(structureGroup, path string) string {
	p := "wit/classificationnodes/" + structureGroup
	if path != "" {
		p += "/" + url.PathEscape(path)
	}
	return p
}

// areaiterationResolveClassificationNodePath ports
// boards_helper.py:12-21 resolve_classification_node_path: fetches the
// project's root nodes ($depth=0) and strips the matching structure group's
// root-node path prefix off the caller-supplied absolute path.
func areaiterationResolveClassificationNodePath(ctx context.Context, client *ado.Client, project, structureGroup, path string) (string, error) {
	var roots []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      project,
		Path:       "wit/classificationnodes",
		APIVersion: "5.0",
		Query:      url.Values{"$depth": {"0"}},
	}, &roots); err != nil {
		return "", fmt.Errorf("failed to resolve --path: %w", err)
	}

	singular := strings.TrimSuffix(structureGroup, "s") // "iterations"/"areas" -> "iteration"/"area"
	var rootPath string
	for _, entry := range roots {
		if st, _ := entry["structureType"].(string); st == singular {
			rootPath, _ = entry["path"].(string)
		}
		if rootPath != "" && strings.HasPrefix(strings.ToLower(path), strings.ToLower(rootPath)) {
			return path[len(rootPath):], nil
		}
	}
	return "", errors.New("--path parameter is expected to be absolute path.")
}

// areaiterationParseDate ports convert_date_only_string_to_iso8601
// (dev/common/arguments.py:31-41). Python parses with dateutil.parser
// (very permissive) and calls .isoformat(); this covers the documented
// date-only input shape ("2019-06-03") plus a couple of common variants —
// not a full dateutil-equivalent parser, which stdlib has no match for.
// Unlike convert_date_string_to_iso8601, this variant never attaches a
// local zone to a naive input (arguments.py:40 formats whatever
// dateutil.parser.parse returned, tzinfo or not) — so an offset in the
// input is preserved verbatim, and a bare date/datetime stays offset-less.
func areaiterationParseDate(value, argument string) (string, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.Format("2006-01-02T15:04:05-07:00"), nil
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Format("2006-01-02T15:04:05"), nil
		}
	}
	return "", fmt.Errorf("The --%s argument must be a valid ISO 8601 string.", argument)
}

// areaiterationHandleBoardsError ports handle_common_boards_errors
// (boards_helper.py:24-29): appends the troubleshooting-docs link to a
// failed API call's message. Only wraps *ado.APIError (an
// AzureDevOpsServiceError in Python); any other error (e.g. a network
// failure) passes through unchanged, matching handle_common_boards_errors
// only ever being reached from an `except AzureDevOpsServiceError` block.
func areaiterationHandleBoardsError(err error) error {
	var apiErr *ado.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	return fmt.Errorf("%s\nPlease see https://aka.ms/azure-devops-cli-troubleshooting for more information on troubleshooting common errors.", apiErr.Message)
}
