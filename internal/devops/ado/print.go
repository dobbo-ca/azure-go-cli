package ado

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/cdobbyn/azure-go-cli/pkg/query"
	"github.com/spf13/cobra"
)

// Column is one column of a knack table_transformer's projection. Header is
// the exact heading text azure-cli's Python transformer produces (already
// capitalized as knack would render it -- pkg/output only upper-cases a
// header's first rune, so a header needing more than that, e.g. a literal
// trailing space, must be spelled out exactly here). Field is a JMESPath
// expression evaluated against the row ("id", "project.name"); Value
// overrides it for a computed cell.
//
// A row that has nothing for this column omits it from that row entirely,
// matching knack's None-drop -- which can delete the column from the whole
// table if every row omits it (see pkg/output's renderTable). Field
// expresses that directly: a JMESPath lookup that is missing or explicitly
// null is omitted. Value's signature has no nil to return, so it uses
// azure-cli's own convention for the same thing (several _format.py
// row-builders do exactly this): return "" to omit the column from this
// row, or " " (a single space) for a blank cell that keeps the column open.
type Column struct {
	Header string
	Field  string
	Value  func(row map[string]any) string
}

// TableMode reports whether cmd will actually run a table_transformer's
// projection: -o table with no --query, the gate Print applies internally
// (knack/output.py:64-74). Callers that must pre-shape v for the table path
// alone -- a Python transformer that also sorts, flattens or unwraps its
// input -- ask this rather than re-deriving the condition.
func TableMode(cmd *cobra.Command) bool {
	format, _ := cmd.Flags().GetString("output")
	queryStr, _ := cmd.Flags().GetString("query")
	return format == "table" && queryStr == ""
}

// Print renders v honouring the inherited -o/--output and --query flags.
//
// knack only ever runs a command's table_transformer for `-o table` with no
// --query active (knack/output.py:64-74); every other format, and a table
// request with --query set, sees the raw, untransformed v. Print reproduces
// that: cols is applied only on the table/no-query path. Everything else --
// including every non-table format and any --query -- is
// output.PrintFormatted(cmd, v, format), unchanged.
//
// On the table/no-query path, Print projects each row through cols and
// renders the result via output.PrintFormatted's generic table formatter --
// but in the *declared* column order, not the alphabetical order that
// formatter falls back to for a plain map. It does this by handing
// PrintFormatted a one-off *cobra.Command whose --query is a
// multiselect-hash matching cols' headers 1:1 (`[].{"ID": ID, ...}`):
// PrintFormatted already recovers a multiselect-hash's declared column order
// for exactly this shape (azure-go-cli-c41), so this reuses that existing,
// tested mechanism instead of adding a new one to pkg/output.
func Print(cmd *cobra.Command, v any, cols ...Column) error {
	if !TableMode(cmd) || len(cols) == 0 {
		format, _ := cmd.Flags().GetString("output")
		return output.PrintFormatted(cmd, v, format)
	}

	rows, err := tableRows(v)
	if err != nil {
		return err
	}

	projected := make([]map[string]any, len(rows))
	for i, row := range rows {
		m := make(map[string]any, len(cols))
		for _, c := range cols {
			if val, ok := cellValue(row, c); ok {
				m[c.Header] = val
			}
		}
		projected[i] = m
	}

	shadow := &cobra.Command{}
	shadow.Flags().String("query", multiSelectHashQuery(cols), "")
	shadow.SetOut(cmd.OutOrStdout())

	return output.PrintFormatted(shadow, projected, "table")
}

// tableRows normalises v (a struct, slice, or already-generic value) into
// []map[string]any via the same JSON round-trip pkg/output uses, so Column
// evaluation doesn't care about the concrete Go type passed in.
func tableRows(v any) ([]map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to format output: %w", err)
	}

	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	switch val := generic.(type) {
	case nil:
		return nil, nil
	case []any:
		rows := make([]map[string]any, 0, len(val))
		for _, el := range val {
			if m, ok := el.(map[string]any); ok {
				rows = append(rows, m)
			}
		}
		return rows, nil
	case map[string]any:
		return []map[string]any{val}, nil
	default:
		return nil, fmt.Errorf("cannot render %T as a table", v)
	}
}

// cellValue evaluates a single column against a row, returning the value to
// project and whether the column should appear in this row at all -- false
// means omit (see Column's doc comment on knack's None-drop).
//
// Value wins when set: its return already IS knack's rendered cell text, so
// it is kept as a string and read for the "" (omit) / " " (blank, keep)
// convention documented on Column. Field is looked up as a JMESPath
// expression (also valid for a plain dotted path like "project.name") and
// kept at its native JSON type -- nil is omitted directly, everything else
// (bool, number, nested container, ...) is left for pkg/output's table
// renderer to format exactly as it would any other JSON value.
func cellValue(row map[string]any, c Column) (any, bool) {
	if c.Value != nil {
		s := c.Value(row)
		if s == "" {
			return nil, false
		}
		return s, true
	}
	v, _ := query.ApplyJMESPath(row, c.Field)
	if v == nil {
		return nil, false
	}
	return v, true
}

// multiSelectHashQuery builds a JMESPath multiselect-hash that selects each
// column's header onto itself, e.g. `[].{"ID": ID, "Name": Name}`. Handed to
// PrintFormatted as --query, this is an identity projection over the already
// -projected rows -- its only purpose is to make
// query.MultiSelectHashKeyOrders recover cols' declared order for
// pkg/output's table renderer. Headers are JMESPath quoted-string
// identifiers (JSON string-literal syntax) on both sides so a header
// containing a space, or deliberately trailing one, still round-trips.
func multiSelectHashQuery(cols []Column) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		b, _ := json.Marshal(c.Header)
		parts[i] = string(b) + ": " + string(b)
	}
	return "[].{" + strings.Join(parts, ", ") + "}"
}

// TSVScalar formats a scalar value for a Column.Value function's cell text.
// Exported so callers building a computed cell from a raw JSON value (an
// ado.Column with Value set) can format it the way pkg/output's table cells
// are formatted, without duplicating that logic.
//
// ponytail: this IS a small duplicate of pkg/output's unexported tsvScalar,
// kept because the two are not interchangeable here. tsvScalar routes
// float64 through formatNumber, which renders an integral float as "42.0";
// tableRows decodes without UseNumber, so every JSON integer arrives here as
// float64 and would grow a spurious ".0". Upgrade path: decode tableRows
// with json.Decoder.UseNumber so wire literals survive, then export
// pkg/output's tsvScalar and delete this.
func TSVScalar(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case bool:
		if val {
			return "True"
		}
		return "False"
	case float64:
		// Render integers without a trailing ".0".
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'g', -1, 64)
	default:
		// Nested arrays/objects: fall back to compact JSON.
		b, err := json.Marshal(val)
		if err != nil {
			return ""
		}
		return string(b)
	}
}
