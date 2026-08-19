// Package pipelines implements `az pipelines` — Azure DevOps Pipelines
// commands. This file wires the `pipelines build` group (list/queue/show,
// `pipelines build tag`, `pipelines build definition`) and holds the small
// bits shared by more than one build_*.go file.
package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// newBuildCommand wires `az pipelines build` (list/queue/show), plus its
// `tag` and `definition` subgroups.
func newBuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Manage builds",
		Long:  "Manage Azure Pipelines builds",
	}
	cmd.AddCommand(newBuildListCmd())
	cmd.AddCommand(newBuildQueueCmd())
	cmd.AddCommand(newBuildShowCmd())
	cmd.AddCommand(newBuildCancelCmd())
	cmd.AddCommand(newBuildTagCommand())
	cmd.AddCommand(newBuildDefinitionCommand())
	return cmd
}

// buildColumns are the table columns for `build list`/`build queue`/`build
// show` (transform_builds_table_output / transform_build_table_output,
// _format.py:12-21): ID, Number, Status, Result, Definition ID, Definition
// Name, Source Branch, Queued Time, Reason.
var buildColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Number", Field: "buildNumber"},
	{Header: "Status", Field: "status"},
	{Header: "Result", Value: buildResultCell},
	{Header: "Definition ID", Field: "definition.id"},
	{Header: "Definition Name", Field: "definition.name"},
	{Header: "Source Branch", Value: buildSourceBranchCell},
	{Header: "Queued Time", Value: buildQueuedTimeCell},
	{Header: "Reason", Field: "reason"},
}

func buildResultCell(row map[string]any) string {
	if v, ok := row["result"]; ok && v != nil && v != "" {
		return ado.TSVScalar(v)
	}
	return " "
}

const buildRefHeadsPrefix = "refs/heads/"

func buildSourceBranchCell(row map[string]any) string {
	v, _ := row["sourceBranch"].(string)
	if v == "" {
		return " "
	}
	return strings.TrimPrefix(v, buildRefHeadsPrefix)
}

// buildQueuedTimeCell reproduces
// `str(queued_time.date()) + ' ' + str(queued_time.time())` after converting
// to local time (_transform_build_row, _format.py:44-45) -- the same
// rendering release_common.go already does for _format.py's release rows.
func buildQueuedTimeCell(row map[string]any) string {
	v, _ := row["queueTime"].(string)
	if v == "" {
		return ""
	}
	return releaseLocalDateTime(v)
}

// buildProjectName reads row["project"]["name"], used to build --open web
// URLs from a server response rather than the input --project value
// (_open_build/_open_definition always read build.project.name).
func buildProjectName(row map[string]any) string {
	if p, ok := row["project"].(map[string]any); ok {
		if n, ok := p["name"].(string); ok {
			return n
		}
	}
	return ""
}

// buildOpenInBrowser opens webURL, warning (never failing the command) on
// error — foundation-spec.md §7.1: --open never suppresses or changes the
// printed output.
func buildOpenInBrowser(webURL string) {
	if err := ado.OpenBrowser(webURL); err != nil {
		logger.Warning("failed to open browser: %v", err)
	}
}

// buildBuildURL reproduces _open_build (build.py:196-203): used by both
// `build queue` and `build show`.
func buildBuildURL(org string, build map[string]any) string {
	return strings.TrimRight(org, "/") + "/" + url.PathEscape(buildProjectName(build)) +
		"/_build/index?buildid=" + url.PathEscape(ado.TSVScalar(build["id"]))
}

// buildDefinitionURL reproduces _open_definition (build_definition.py:80-87).
func buildDefinitionURL(org string, def map[string]any) string {
	return strings.TrimRight(org, "/") + "/" + url.PathEscape(buildProjectName(def)) +
		"/_build/index?definitionId=" + url.PathEscape(ado.TSVScalar(def["id"]))
}

// buildDefinitionIDFromName ports get_definition_id_from_name
// (build_definition.py:101-113, called with path=None by both `build queue`
// and `build definition show` — the folder-`path` disambiguation parameter
// is dropped here since no caller in this group ever supplies one).
func buildDefinitionIDFromName(ctx context.Context, client *ado.Client, project, name string) (int, error) {
	var defs []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      project,
		Path:       "build/Definitions",
		APIVersion: "5.0",
		Query:      url.Values{"name": {name}},
	}, &defs); err != nil {
		return 0, fmt.Errorf("failed to look up build definition %q: %w", name, err)
	}
	switch len(defs) {
	case 1:
		id, _ := defs[0]["id"].(float64)
		return int(id), nil
	case 0:
		return 0, fmt.Errorf("There were no build definitions matching name %q in project %q.", name, project)
	default:
		proj := project
		if ado.IsUUID(project) {
			if p := buildProjectName(defs[0]); p != "" {
				proj = p
			}
		}
		return 0, fmt.Errorf("Multiple definitions were found matching name %q in project %q. Try supplying the definition ID or folder path to differentiate.", name, proj)
	}
}
