package boards

import (
	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/query"
)

// areaiterationVisibleColumns drops any column whose Field resolves to a
// falsy value (nil, "", false) for every row in rows, mirroring Python's
// tabulate(headers='keys') over rows built as per-row OrderedDicts that
// omit a key entirely when the source value is absent/null/falsy (e.g.
// _format.py:138 `if row['attributes']:`, :162-164 `if row['source']:`/
// `if row['target']:`). Columns with a Value func are always kept, since
// they have no single source field to test and Python's corresponding keys
// (e.g. Default Iteration Macro) are set unconditionally.
func areaiterationVisibleColumns(rows []map[string]any, cols []ado.Column) []ado.Column {
	kept := make([]ado.Column, 0, len(cols))
	for _, c := range cols {
		if c.Value != nil {
			kept = append(kept, c)
			continue
		}
		visible := false
		for _, row := range rows {
			v, _ := query.ApplyJMESPath(row, c.Field)
			switch vv := v.(type) {
			case nil:
			case string:
				visible = vv != ""
			case bool:
				visible = vv
			default:
				visible = true
			}
			if visible {
				break
			}
		}
		if visible {
			kept = append(kept, c)
		}
	}
	return kept
}

// areaiterationTeamIterationColumns is _transform_team_iteration_row
// (_format.py:134-150): ID, Name, Start Date, Finish Date, [Time Frame if
// present], Path. Start Date/Finish Date/Time Frame are dropped by
// areaiterationVisibleColumns when no row has an attributes object.
var areaiterationTeamIterationColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Start Date", Field: "attributes.startDate"},
	{Header: "Finish Date", Field: "attributes.finishDate"},
	{Header: "Time Frame", Field: "attributes.timeFrame"},
	{Header: "Path", Field: "path"},
}

// areaiterationTeamDefaultIterationColumns is areaiterationTeamIterationColumns
// plus the always-present "Default Iteration Macro" column
// (transform_work_item_team_default_iteration_table_output, _format.py:170-177).
// Default Iteration Macro uses a Value func so areaiterationVisibleColumns
// never drops it: Python sets it unconditionally, even when falsy, and when
// defaultIteration itself is falsy the ID/Name/... columns correctly
// disappear because the row has none of those keys at all.
var areaiterationTeamDefaultIterationColumns = append(
	append([]ado.Column{}, areaiterationTeamIterationColumns...),
	ado.Column{Header: "Default Iteration Macro", Value: func(row map[string]any) string {
		return ado.TSVScalar(row["defaultIterationMacro"])
	}},
)

// areaiterationTeamIterationWorkItemsColumns is
// _transform_team_iteration_work_item_row (_format.py:153-167): Source
// (id), Target (id), Relation Type. Source/Target are dropped by
// areaiterationVisibleColumns when no row has that side of the relation
// (the common shape for top-level items, whose "source" is null); Relation
// Type uses a Value func so it's never dropped, matching Python setting it
// unconditionally (_format.py:166).
var areaiterationTeamIterationWorkItemsColumns = []ado.Column{
	{Header: "Source", Field: "source.id"},
	{Header: "Target", Field: "target.id"},
	{Header: "Relation Type", Value: func(row map[string]any) string {
		return ado.TSVScalar(row["rel"])
	}},
}

// areaiterationTeamAreasColumns is _transform_work_item_team_area_row
// (_format.py:228-236): Area, Include sub areas, Is Default.
var areaiterationTeamAreasColumns = []ado.Column{
	{Header: "Area", Field: "value"},
	{Header: "Include sub areas", Field: "includeChildren"},
	{Header: "Is Default", Value: func(row map[string]any) string {
		v, _ := row["_isDefault"].(bool)
		if v {
			return "True"
		}
		return "False"
	}},
}
