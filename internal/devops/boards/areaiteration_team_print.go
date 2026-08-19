package boards

import (
	"sort"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationPrintDefaultIteration renders a TeamSetting object per
// transform_work_item_team_default_iteration_table_output (_format.py:170-177):
// table view extracts defaultIteration (if any) plus defaultIterationMacro;
// every other format renders the full TeamSetting object Python returns
// unmodified.
func areaiterationPrintDefaultIteration(cmd *cobra.Command, result map[string]any) error {
	if !ado.TableMode(cmd) {
		return ado.Print(cmd, result)
	}

	row := map[string]any{}
	if di, ok := result["defaultIteration"].(map[string]any); ok {
		for k, v := range di {
			row[k] = v
		}
	}
	row["defaultIterationMacro"] = result["defaultIterationMacro"]
	rows := []map[string]any{row}
	return ado.Print(cmd, rows, areaiterationVisibleColumns(rows, areaiterationTeamDefaultIterationColumns)...)
}

// areaiterationPrintBacklogIteration renders a TeamSetting object per
// transform_work_item_team_backlog_iteration_table_output (_format.py:180-183).
// Python has no null guard here and crashes (TypeError) if backlogIteration
// is None; this port renders zero rows instead, per the crash-fix policy.
func areaiterationPrintBacklogIteration(cmd *cobra.Command, result map[string]any) error {
	if !ado.TableMode(cmd) {
		return ado.Print(cmd, result)
	}

	var rows []map[string]any
	if bi, ok := result["backlogIteration"].(map[string]any); ok {
		rows = append(rows, bi)
	}
	return ado.Print(cmd, rows, areaiterationVisibleColumns(rows, areaiterationTeamIterationColumns)...)
}

// areaiterationPrintIterationWorkItems renders an IterationWorkItems object
// per transform_work_item_team_iteration_work_items (_format.py:153-158):
// table view is one row per workItemRelations entry; every other format
// renders the full object.
func areaiterationPrintIterationWorkItems(cmd *cobra.Command, result map[string]any) error {
	if !ado.TableMode(cmd) {
		return ado.Print(cmd, result)
	}

	items, _ := result["workItemRelations"].([]any)
	rows := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			rows = append(rows, m)
		}
	}
	return ado.Print(cmd, rows, areaiterationVisibleColumns(rows, areaiterationTeamIterationWorkItemsColumns)...)
}

// areaiterationPrintTeamAreas renders a TeamFieldValues object per
// transform_work_item_team_areas_table_output (_format.py:221-236): table
// view is one row per values[] entry, sorted by value.lower()
// (_get_team_area_key), with an "Is Default" flag computed against
// defaultValue; every other format renders the full object.
func areaiterationPrintTeamAreas(cmd *cobra.Command, result map[string]any) error {
	if !ado.TableMode(cmd) {
		return ado.Print(cmd, result)
	}

	values, _ := result["values"].([]any)
	defaultValue, _ := result["defaultValue"].(string)

	rows := make([]map[string]any, 0, len(values))
	for _, v := range values {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		row := map[string]any{}
		for k, vv := range entry {
			row[k] = vv
		}
		val, _ := entry["value"].(string)
		row["_isDefault"] = val == defaultValue
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		vi, _ := rows[i]["value"].(string)
		vj, _ := rows[j]["value"].(string)
		return strings.ToLower(vi) < strings.ToLower(vj)
	})

	return ado.Print(cmd, rows, areaiterationTeamAreasColumns...)
}
