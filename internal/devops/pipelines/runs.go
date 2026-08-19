// Package pipelines implements `az pipelines` (this file's slice: `pipelines
// runs`, `pipelines runs artifact`, `pipelines runs tag`).
package pipelines

import (
	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newRunsCommand returns the `pipelines runs` command tree: list/show plus
// the artifact and tag subgroups.
func newRunsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Manage runs for a pipeline.",
	}
	cmd.AddCommand(newRunsListCmd())
	cmd.AddCommand(newRunsShowCmd())
	cmd.AddCommand(newRunsArtifactCommand())
	cmd.AddCommand(newRunsTagCommand())
	return cmd
}

// runsColumns is the shared "Run ID, Number, Status, Result, Pipeline ID,
// Pipeline Name, Source Branch, Queued Time, Reason" row shape used by both
// `pipelines runs list` and `pipelines runs show` — Python has one row
// function (_transform_pipeline_run_row, _format.py:206-226) behind two
// thin table_transformer wrappers.
var runsColumns = []ado.Column{
	{Header: "Run ID", Field: "id"},
	{Header: "Number", Field: "buildNumber"},
	{Header: "Status", Field: "status"},
	{Header: "Result", Value: runsResultCell},
	{Header: "Pipeline ID", Field: "definition.id"},
	{Header: "Pipeline Name", Field: "definition.name"},
	{Header: "Source Branch", Value: buildSourceBranchCell},
	{Header: "Queued Time", Value: runsQueuedTimeCell},
	{Header: "Reason", Field: "reason"},
}

// runsResultCell mirrors _format.py:229-232: a falsy result renders as a
// single space, not an empty cell.
func runsResultCell(row map[string]any) string {
	if s, ok := row["result"].(string); ok && s != "" {
		return s
	}
	return " "
}

// runsQueuedTimeCell mirrors _format.py:243-244/250-251: parse queueTime,
// convert to local time, render as "date time" — including microseconds
// when non-zero (str(time()) keeps them), same as build.go's
// buildQueuedTimeCell for the identical Python row function.
func runsQueuedTimeCell(row map[string]any) string {
	s, _ := row["queueTime"].(string)
	if s == "" {
		return " "
	}
	return buildQueuedTimeCell(row)
}
