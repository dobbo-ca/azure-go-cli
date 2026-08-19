package pipelines

import (
	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newBuildDefinitionCommand wires `az pipelines build definition`
// (list/show).
func newBuildDefinitionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "definition",
		Short: "Manage build definitions",
	}
	cmd.AddCommand(newBuildDefinitionListCmd())
	cmd.AddCommand(newBuildDefinitionShowCmd())
	return cmd
}

// buildDefinitionColumns is transform_definitions_table_output /
// transform_definition_table_output (_format.py:64-98): ID, Name, [Draft],
// Status, Default Queue. The Draft column is only included when at least
// one row being rendered has quality=="draft" (_format.py:66-71 scans the
// whole list for `list`; _format.py:76-78 checks just the one row for
// `show`) — computed from the actual rows passed in, mirroring core.go's
// coreDefinitionColumns for the sibling `pipelines list`/`show` table.
func buildDefinitionColumns(rows []map[string]any) []ado.Column {
	includeDraft := false
	for _, r := range rows {
		if q, _ := r["quality"].(string); q == "draft" {
			includeDraft = true
			break
		}
	}

	cols := []ado.Column{
		{Header: "ID", Field: "id"},
		{Header: "Name", Field: "name"},
	}
	if includeDraft {
		cols = append(cols, ado.Column{Header: "Draft", Value: buildDraftCell})
	}
	return append(cols,
		ado.Column{Header: "Status", Value: buildQueueStatusCell},
		ado.Column{Header: "Default Queue", Value: buildDefaultQueueCell},
	)
}

func buildDraftCell(row map[string]any) string {
	if q, _ := row["quality"].(string); q == "draft" {
		return "True"
	}
	return " "
}

func buildQueueStatusCell(row map[string]any) string {
	if v, ok := row["queueStatus"]; ok && v != nil && v != "" {
		return ado.TSVScalar(v)
	}
	return " "
}

func buildDefaultQueueCell(row map[string]any) string {
	if q, ok := row["queue"].(map[string]any); ok {
		if n, ok := q["name"].(string); ok {
			return n
		}
	}
	return " "
}
