package ado

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestCellValue(t *testing.T) {
	row := map[string]any{
		"id":      float64(42),
		"enabled": true,
		"missing": nil,
		"project": map[string]any{"name": "MyProj"},
	}

	tests := []struct {
		name     string
		col      Column
		want     any
		wantOmit bool
	}{
		{name: "top-level field", col: Column{Field: "id"}, want: 42.0},
		{name: "bool passthrough", col: Column{Field: "enabled"}, want: true},
		{name: "absent field is omitted", col: Column{Field: "missing"}, wantOmit: true},
		{name: "dotted path", col: Column{Field: "project.name"}, want: "MyProj"},
		{
			name: "Value override wins over Field",
			col:  Column{Field: "id", Value: func(row map[string]any) string { return "computed" }},
			want: "computed",
		},
		{
			name:     "Value returning empty string omits the column",
			col:      Column{Value: func(row map[string]any) string { return "" }},
			wantOmit: true,
		},
		{
			name: "Value returning a single space keeps a blank cell",
			col:  Column{Value: func(row map[string]any) string { return " " }},
			want: " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := cellValue(row, tt.col)
			if ok == tt.wantOmit {
				t.Errorf("cellValue ok = %v, want omit=%v", ok, tt.wantOmit)
			}
			if ok && got != tt.want {
				t.Errorf("cellValue = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// tableCmd builds a cobra.Command carrying the inherited -o/--query flags
// Print reads, with output captured for assertion.
func tableCmd(format, queryStr string) (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	cmd.Flags().StringP("output", "o", format, "")
	cmd.Flags().String("query", queryStr, "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	return cmd, &buf
}

// TestPrintTable_ReposList pins az repos list -o table against knack ground
// truth (/tmp/devops-port/table-groundtruth.md §4a,
// transform_repos_table_output): declared column order (ID, Name, Default
// Branch, Project) survives pkg/output's table renderer, which alphabetizes
// a plain map with no --query active. "Default Branch" uses azure-cli's own
// " " (single space) placeholder for a missing branch, keeping the column
// open with a blank cell rather than deleting it.
func TestPrintTable_ReposList(t *testing.T) {
	cols := []Column{
		{Header: "ID", Field: "id"},
		{Header: "Name", Field: "name"},
		{Header: "Default Branch", Value: func(row map[string]any) string {
			if b, _ := row["defaultBranch"].(string); b != "" {
				return b
			}
			return " "
		}},
		{Header: "Project", Field: "project.name"},
	}

	rows := []map[string]any{
		{
			"id":      "aa11bb22-1111-4a1b-9f3d-1a2b3c4d5e6f",
			"name":    "api",
			"project": map[string]any{"name": "Contoso"},
		},
		{
			"id":            "6f8b1c2a-0000-4a1b-9f3d-1a2b3c4d5e6f",
			"name":          "contoso-web",
			"defaultBranch": "main",
			"project":       map[string]any{"name": "Contoso"},
		},
	}

	const want = "ID                                    Name         Default Branch    Project\n" +
		"------------------------------------  -----------  ----------------  ---------\n" +
		"aa11bb22-1111-4a1b-9f3d-1a2b3c4d5e6f  api                            Contoso\n" +
		"6f8b1c2a-0000-4a1b-9f3d-1a2b3c4d5e6f  contoso-web  main              Contoso\n"

	cmd, buf := tableCmd("table", "")
	if err := Print(cmd, rows, cols...); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("Print table output:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// refListColumns mirrors repos/_format.py's transform_refs_table_output:
// Success/Update Status are written from the row only when present, using
// knack's real None (omit), not a blank-cell placeholder.
func refListColumns() []Column {
	return []Column{
		{Header: "Object ID", Field: "objectId"},
		{Header: "Name", Field: "name"},
		{Header: "Success", Field: "success"},
		{Header: "Update Status", Field: "updateStatus"},
	}
}

// TestPrintTable_RefList_ColumnsVanish pins the load-bearing null-drop case
// from table-groundtruth.md §4c: when every row omits 'success'/
// 'updateStatus', both columns disappear from the rendered table entirely
// (not just render blank) -- a Column returning nil for every row must
// delete that column, matching knack's None-drop.
func TestPrintTable_RefList_ColumnsVanish(t *testing.T) {
	rows := []map[string]any{
		{"objectId": "1122334455667788990011223344556677889900", "name": "heads/dev"},
		{"objectId": "0f1e2d3c4b5a69788796a5b4c3d2e1f001122334", "name": "heads/main"},
	}

	const want = "Object ID                                 Name\n" +
		"----------------------------------------  ----------\n" +
		"1122334455667788990011223344556677889900  heads/dev\n" +
		"0f1e2d3c4b5a69788796a5b4c3d2e1f001122334  heads/main\n"

	cmd, buf := tableCmd("table", "")
	if err := Print(cmd, rows, refListColumns()...); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("Print table output:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestPrintTable_RefCreate_ColumnsSurvive is the same transformer's columns
// on a row that DOES supply success/updateStatus (table-groundtruth.md
// §4c, "ref create"): both columns render.
func TestPrintTable_RefCreate_ColumnsSurvive(t *testing.T) {
	row := map[string]any{
		"objectId":     "1122334455667788990011223344556677889900",
		"name":         "heads/main",
		"success":      true,
		"updateStatus": "succeeded",
	}

	const want = "Object ID                                 Name        Success    Update Status\n" +
		"----------------------------------------  ----------  ---------  ---------------\n" +
		"1122334455667788990011223344556677889900  heads/main  True       succeeded\n"

	cmd, buf := tableCmd("table", "")
	if err := Print(cmd, row, refListColumns()...); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("Print table output:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestPrintTable_QueryBypassesTransformer pins knack's rule that an active
// --query skips the table_transformer entirely (knack/output.py:66) --
// Print must fall through to output.PrintFormatted on the raw value, not
// apply cols.
func TestPrintTable_QueryBypassesTransformer(t *testing.T) {
	rows := []map[string]any{{"id": "x", "name": "n"}}
	cols := []Column{{Header: "ID", Field: "id"}, {Header: "Name", Field: "name"}}

	cmd, buf := tableCmd("table", "[].name")
	if err := Print(cmd, rows, cols...); err != nil {
		t.Fatalf("Print: %v", err)
	}
	const want = "Result\n--------\nn\n"
	if got := buf.String(); got != want {
		t.Errorf("Print with --query active = %q, want %q (cols must be bypassed)", got, want)
	}
}

// TestPrintTable_NoColumns falls back to pkg/output's generic table
// formatter unchanged -- the ~20 azure-cli commands with no
// table_transformer (table-groundtruth.md item 7).
func TestPrintTable_NoColumns(t *testing.T) {
	row := map[string]any{"zebra": "z", "apple": "a"}

	cmd, buf := tableCmd("table", "")
	if err := Print(cmd, row); err != nil {
		t.Fatalf("Print: %v", err)
	}
	const want = "Apple    Zebra\n" +
		"-------  -------\n" +
		"a        z\n"
	if got := buf.String(); got != want {
		t.Errorf("Print with no cols = %q, want %q", got, want)
	}
}

// TestPrintJSON_BypassesTransformer pins knack's rule that only -o table
// runs the table_transformer -- json/tsv/yaml always see the raw value.
func TestPrintJSON_BypassesTransformer(t *testing.T) {
	row := map[string]any{"id": "x", "name": "n"}
	cols := []Column{{Header: "ID", Field: "id"}}

	cmd, buf := tableCmd("json", "")
	if err := Print(cmd, row, cols...); err != nil {
		t.Fatalf("Print: %v", err)
	}
	const want = "{\n  \"id\": \"x\",\n  \"name\": \"n\"\n}\n"
	if got := buf.String(); got != want {
		t.Errorf("Print json = %q, want %q", got, want)
	}
}
