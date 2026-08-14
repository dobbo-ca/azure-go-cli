package output

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"math"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/cdobbyn/azure-go-cli/pkg/query"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestRenderTSV(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{"nil", nil, ""}, // D2a: top-level null with no --query renders nothing
		{"scalar string", "abc", "abc\n"},
		{"list of strings", []interface{}{"id1", "id2"}, "id1\nid2\n"},
		{"empty list", []interface{}{}, ""},
		{"object sorted keys", []interface{}{map[string]interface{}{"b": "2", "a": "1"}}, "1\t2\n"},
		{"integral float keeps .0 (Python str(float) semantics)", float64(5), "5.0\n"},
		// D5: a ROW-level bool (knack's _dump_row) lowercases; a bool used as
		// a CELL inside a dict/list row (knack's _dump_obj) does not — see
		// the "cell bool stays True/False" case below.
		{"bool", true, "true\n"},
		// Below: knack-verified rows (knack 0.14.0 format_tsv), D1/D3/D4/D5.
		{"null cell in dict row", []interface{}{map[string]interface{}{"a": nil, "b": "x"}}, "None\tx\n"},
		{"null cell in list row", []interface{}{[]interface{}{nil, "x"}}, "None\tx\n"},
		{"null value in object", map[string]interface{}{"a": nil}, "None\n"},
		{"list of nulls", []interface{}{nil, nil}, "None\nNone\n"},
		{"list cell becomes element count", []interface{}{map[string]interface{}{"a": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")}, "b": "x"}}, "3\tx\n"},
		{"empty list cell becomes 0", []interface{}{map[string]interface{}{"a": []interface{}{}, "b": "x"}}, "0\tx\n"},
		{"nested list in list row", []interface{}{[]interface{}{json.Number("1"), []interface{}{json.Number("2"), json.Number("3")}, "x"}}, "1\t2\tx\n"},
		{"dict cell becomes empty", []interface{}{map[string]interface{}{"a": map[string]interface{}{"k": "v"}, "b": "x"}}, "\tx\n"},
		{"cell bool stays True/False", []interface{}{map[string]interface{}{"a": true, "b": false}}, "True\tFalse\n"},
		{"empty object", map[string]interface{}{}, "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderTSV(tt.in, false, nil); got != tt.want {
				t.Errorf("renderTSV(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestPrintFormatted_QueryTSV exercises the idempotency-check shape:
// a JMESPath projection to ids rendered as tsv (one id per line).
func TestPrintFormatted_QueryTSV(t *testing.T) {
	data := []map[string]interface{}{
		{"id": "ra-1", "principalId": "p1", "roleDefinitionName": "Reader", "scope": "/subscriptions/s"},
		{"id": "ra-2", "principalId": "p2", "roleDefinitionName": "Owner", "scope": "/subscriptions/s"},
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("query", "[?roleDefinitionName=='Reader'].id", "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := PrintFormatted(cmd, data, "tsv"); err != nil {
		t.Fatalf("PrintFormatted: %v", err)
	}

	if got := buf.String(); got != "ra-1\n" {
		t.Errorf("got %q, want %q", got, "ra-1\n")
	}
}

// TestPrintFormatted_QueryNoMatch confirms an empty match yields no output,
// which the idempotency check relies on to decide "not yet assigned".
func TestPrintFormatted_QueryNoMatch(t *testing.T) {
	data := []map[string]interface{}{{"id": "ra-1", "roleDefinitionName": "Reader"}}

	cmd := &cobra.Command{}
	cmd.Flags().String("query", "[?roleDefinitionName=='Nope'].id", "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := PrintFormatted(cmd, data, "tsv"); err != nil {
		t.Fatalf("PrintFormatted: %v", err)
	}

	if got := buf.String(); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// TestPrintFormatted_QueryFromPersistentFlag proves a --query defined as a
// PERSISTENT flag on the root command is reachable from a leaf subcommand via
// cmd.Flags().GetString("query") — the mechanism the list commands rely on.
func TestPrintFormatted_QueryFromPersistentFlag(t *testing.T) {
	var captured string

	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("query", "", "")
	var buf bytes.Buffer
	root.SetOut(&buf)

	child := &cobra.Command{
		Use: "child",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]interface{}{{"id": "x"}, {"id": "y"}}
			if err := PrintFormatted(cmd, data, "tsv"); err != nil {
				t.Errorf("PrintFormatted: %v", err)
			}
			captured = buf.String()
			return nil
		},
	}
	child.Flags().StringP("output", "o", "table", "")
	root.AddCommand(child)
	root.SetArgs([]string{"child", "--query", "[].id", "-o", "tsv"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if captured != "x\ny\n" {
		t.Errorf("got %q, want %q", captured, "x\ny\n")
	}
}

// TestPrintFormatted_SupportedFormats confirms every documented -o value
// (case-insensitively) is accepted.
func TestPrintFormatted_SupportedFormats(t *testing.T) {
	for _, format := range []string{"json", "", "JSON", "table", "Table", "tsv", "yaml", "YAML", "none", "NONE"} {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := PrintFormatted(cmd, []string{"x"}, format); err != nil {
			t.Errorf("format %q: unexpected error: %v", format, err)
		}
	}
}

// TestPrintFormatted_InvalidFormat confirms an unknown -o value errors
// instead of silently rendering JSON.
func TestPrintFormatted_InvalidFormat(t *testing.T) {
	for _, format := range []string{"tsvv", "jsonc", "yamlc", "xml"} {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		err := PrintFormatted(cmd, []string{"x"}, format)
		if err == nil {
			t.Errorf("format %q: expected error, got nil", format)
			continue
		}
		if !strings.Contains(err.Error(), "invalid choice") {
			t.Errorf("format %q: error %q does not contain %q", format, err.Error(), "invalid choice")
		}
	}
}

// TestPrintFormatted_None confirms -o none writes nothing.
func TestPrintFormatted_None(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("query", "", "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := PrintFormatted(cmd, []string{"x"}, "none"); err != nil {
		t.Fatalf("PrintFormatted: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("got %q, want empty", buf.String())
	}
}

// TestPrintFormatted_NoneSurfacesQueryError confirms -o none still evaluates
// --query, so a broken query still errors rather than being silently
// swallowed.
func TestPrintFormatted_NoneSurfacesQueryError(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("query", "[?", "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := PrintFormatted(cmd, []string{"x"}, "none")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if buf.Len() != 0 {
		t.Errorf("got %q, want empty", buf.String())
	}
}

// TestPrintFormatted_QueryYAML confirms --query is applied before yaml
// rendering.
func TestPrintFormatted_QueryYAML(t *testing.T) {
	data := []map[string]interface{}{
		{"name": "rg-alpha"},
		{"name": "rg-beta"},
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("query", "[].name", "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := PrintFormatted(cmd, data, "yaml"); err != nil {
		t.Fatalf("PrintFormatted: %v", err)
	}
	want := "- rg-alpha\n- rg-beta\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestPrintFormatted_QueryTable confirms --query is applied before table
// rendering, and that a multiselect-hash's columns come out in the query's
// written order (Name, Location) rather than sorted — azure-go-cli-c41
// recovers that order from the parsed query AST (see keyOrder).
func TestPrintFormatted_QueryTable(t *testing.T) {
	data := []map[string]interface{}{
		{"name": "rg-alpha", "location": "eastus"},
		{"name": "rg-beta", "location": "westeurope"},
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("query", "[].{Name:name,Location:location}", "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := PrintFormatted(cmd, data, "table"); err != nil {
		t.Fatalf("PrintFormatted: %v", err)
	}
	want := "Name      Location\n" +
		"--------  ----------\n" +
		"rg-alpha  eastus\n" +
		"rg-beta   westeurope\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestPrintJSON_HonorsOutputFlag confirms PrintJSON reads --output/-o and
// delegates non-json formats to PrintFormatted.
func TestPrintJSON_HonorsOutputFlag(t *testing.T) {
	data := []map[string]interface{}{
		{"name": "rg-alpha"},
		{"name": "rg-beta"},
	}

	tests := []struct {
		format string
		want   string
	}{
		{"table", "Name\n--------\nrg-alpha\nrg-beta\n"},
		{"yaml", "- name: rg-alpha\n- name: rg-beta\n"},
		{"none", ""},
		{"tsv", "rg-alpha\nrg-beta\n"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			root := &cobra.Command{Use: "root"}
			root.PersistentFlags().String("query", "", "")
			root.PersistentFlags().StringP("output", "o", "json", "")
			var buf bytes.Buffer
			root.SetOut(&buf)

			child := &cobra.Command{
				Use: "child",
				RunE: func(cmd *cobra.Command, args []string) error {
					return PrintJSON(cmd, data)
				},
			}
			root.AddCommand(child)
			root.SetArgs([]string{"child", "-o", tt.format})

			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("default", func(t *testing.T) {
		root := &cobra.Command{Use: "root"}
		root.PersistentFlags().String("query", "", "")
		root.PersistentFlags().StringP("output", "o", "json", "")
		var buf bytes.Buffer
		root.SetOut(&buf)

		child := &cobra.Command{
			Use: "child",
			RunE: func(cmd *cobra.Command, args []string) error {
				return PrintJSON(cmd, data)
			},
		}
		root.AddCommand(child)
		root.SetArgs([]string{"child"})

		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		want, _ := json.MarshalIndent(data, "", "  ")
		if got := buf.String(); got != string(want)+"\n" {
			t.Errorf("got %q, want %q", got, string(want)+"\n")
		}
	})
}

// TestPrintJSON_JSONPathUnchanged is a regression guard: PrintJSON's json
// path must marshal the struct directly (preserving declaration order), not
// route through PrintFormatted's generic map[string]interface{} tree, which
// would alphabetize the fields.
func TestPrintJSON_JSONPathUnchanged(t *testing.T) {
	type out struct {
		Name     string `json:"name"`
		Location string `json:"location"`
		ID       string `json:"id"`
	}
	data := out{Name: "rg-alpha", Location: "eastus", ID: "/subscriptions/s/rg-alpha"}

	cmd := &cobra.Command{}
	cmd.Flags().String("query", "", "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := PrintJSON(cmd, data); err != nil {
		t.Fatalf("PrintJSON: %v", err)
	}

	want, _ := json.MarshalIndent(data, "", "  ")
	if got := buf.String(); got != string(want)+"\n" {
		t.Errorf("got %q, want %q", got, string(want)+"\n")
	}
	// Guard against a future change that routes json through PrintFormatted:
	// that would alphabetize keys (id, location, name).
	if strings.Index(buf.String(), "\"name\"") > strings.Index(buf.String(), "\"id\"") {
		t.Errorf("fields appear alphabetized, want declaration order: %s", buf.String())
	}
}

func TestRenderTable(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{
			name: "drops id/type, sorts keys, capitalizes header",
			in: []interface{}{map[string]interface{}{
				"id":    "/subscriptions/s1/providers/Microsoft.Authorization/locks/l1",
				"type":  "Microsoft.Authorization/locks",
				"level": "CanNotDelete",
				"name":  "mylock",
				"notes": "do not delete",
			}},
			want: "Level         Name    Notes\n" +
				"------------  ------  -------------\n" +
				"CanNotDelete  mylock  do not delete\n",
		},
		{
			name: "null in every row drops the column entirely",
			in: []interface{}{map[string]interface{}{
				"level":  "ReadOnly",
				"name":   "l1",
				"owners": nil,
			}},
			want: "Level     Name\n" +
				"--------  ------\n" +
				"ReadOnly  l1\n",
		},
		{
			name: "null in only some rows leaves a blank cell",
			in: []interface{}{
				map[string]interface{}{"level": "ReadOnly", "name": "a", "notes": "keep"},
				map[string]interface{}{"level": "ReadOnly", "name": "b", "notes": nil},
			},
			want: "Level     Name    Notes\n" +
				"--------  ------  -------\n" +
				"ReadOnly  a       keep\n" +
				"ReadOnly  b\n",
		},
		{
			name: "single object renders like a one-row list",
			in:   map[string]interface{}{"level": "ReadOnly", "name": "l1"},
			want: "Level     Name\n" +
				"--------  ------\n" +
				"ReadOnly  l1\n",
		},
		{
			name: "nested values are dropped",
			in: []interface{}{map[string]interface{}{
				"name":       "l1",
				"systemData": map[string]interface{}{"createdBy": "x"},
			}},
			want: "Name\n" +
				"------\n" +
				"l1\n",
		},
		{name: "empty list", in: []interface{}{}, want: "\n"},
		{
			// The only \n in the whole table sits at a cell's edge, so
			// pyStrip removes it — but tabulate's multiline decision looks
			// at the RAW header/cell text (tabulate/__init__.py:2367-2388),
			// before that stripping happens, so it still enters multiline
			// mode. In multiline mode a wholly-empty row contributes zero
			// lines and is dropped entirely, rather than printing as a
			// blank line the way it would outside multiline mode. Verified
			// against knack 0.14.0's format_table with this exact payload:
			// the second (all-empty) row does not appear in the output at
			// all.
			name: "edge newline still triggers multiline mode, dropping an all-empty row",
			in: []interface{}{
				map[string]interface{}{"name": "\na", "note": "x"},
				map[string]interface{}{"name": "", "note": ""},
			},
			want: "Name    Note\n------  ------\na       x\n",
		},
		{
			// \x1c (FS) is not \r or \n, but Python's str.splitlines() (and
			// thus tabulate) still splits on it once multiline mode is
			// already active. Verified against knack 0.14.0's format_table
			// with this exact payload.
			name: "splitlines() separator beyond \\r/\\n splits a cell in multiline mode",
			in:   []interface{}{"a\nb", "q\x1cr"},
			want: "Result\n--------\na\nb\nq\nr\n",
		},
		{
			// The case above cannot detect how the column is SIZED: "q\x1cr"
			// is 3 runes, under the MIN_PADDING floor of len("Result")+2 = 8.
			// Here the cell is wider than the floor, so the rule line proves
			// the width came from a \r\n-only split (9 = len("xxxx\x1cyyyy"))
			// and not from splitLines' output (4). Expected value produced by
			// running knack 0.14.0's format_table on this exact payload.
			name: "exotic separator splits for rendering but not for width",
			in:   []interface{}{"a\nb", "xxxx\x1cyyyy"},
			want: "Result\n---------\na\nb\nxxxx\nyyyy\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderTable(tt.in, nil); got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// Expected values verified against `python3 -c "print(str(float(x)))"` for
// each literal below.
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{1, "1.0"},
		{1.5, "1.5"},
		{0.0001, "0.0001"},
		{1e-7, "1e-07"},
		{1000000, "1000000.0"},
		{1e20, "1e+20"},
		{1e21, "1e+21"},
		{-0.5, "-0.5"},
		{0, "0.0"},
		{3, "3.0"},
	}
	for _, tt := range tests {
		if got := formatNumber(tt.in); got != tt.want {
			t.Errorf("formatNumber(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if got := formatNumber(math.Copysign(0, -1)); got != "-0.0" {
		t.Errorf("formatNumber(-0.0) = %q, want %q", got, "-0.0")
	}
}

// TestFormatJSONNumber pins the fix for json.Number exponent/decimal
// literals: integer-shaped literals are echoed verbatim (arbitrary-
// precision), but anything with a '.' or exponent is parsed and re-rendered
// through formatNumber, matching Python's str(float) exactly (verified
// against `python3 -c "print(str(float(x)))"` for each literal below) —
// including keeping a trailing ".0" for integral decimals and the sign of
// -0.0, rather than collapsing them to int-looking strings.
func TestFormatJSONNumber(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"1e-7", "1e-07"},
		{"2.5e-8", "2.5e-08"},
		{"1e20", "1e+20"},
		{"1e21", "1e+21"},
		{"1e-6", "1e-06"},
		{"0.1", "0.1"},
		{"-0.0", "-0.0"},
		{"1.0", "1.0"},
		{"3.0", "3.0"},
		{"9007199254740993", "9007199254740993"},
		{"1234567890123456789", "1234567890123456789"},
	}
	for _, tt := range tests {
		if got := formatJSONNumber(json.Number(tt.in)); got != tt.want {
			t.Errorf("formatJSONNumber(%s) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestValueToYAMLNode_JSONNumber mirrors TestFormatJSONNumber for the YAML
// path: an integer-shaped literal is emitted verbatim, anything else is
// rendered through formatNumber and then, if the result is an exponent form
// with no '.' in the mantissa, yamlFloatLiteral injects one so the scalar
// stays a resolvable YAML float under PyYAML's YAML-1.1 resolver (an
// un-padded "1e-7" has no '.', so it would read back as a string rather than
// a float).
//
// wantTag pins intYAMLNode's tag choice (beads azure-go-cli-6y9): a literal
// yaml.v3's own resolver can read back as an int (up to math.MaxUint64, down
// to math.MinInt64) keeps the "!!int" tag; anything outside that range drops
// the tag so the digits are emitted bare, matching knack.
func TestValueToYAMLNode_JSONNumber(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantTag string
	}{
		{"1e-7", "1.0e-07", "!!float"},
		{"2.5e-8", "2.5e-08", "!!float"},
		{"1e21", "1.0e+21", "!!float"},
		{"9007199254740993", "9007199254740993", "!!int"},
		{"1234567890123456789", "1234567890123456789", "!!int"},
		{"9223372036854775807", "9223372036854775807", "!!int"},   // math.MaxInt64
		{"-9223372036854775808", "-9223372036854775808", "!!int"}, // math.MinInt64
		{"18446744073709551615", "18446744073709551615", "!!int"}, // math.MaxUint64
		{"18446744073709551616", "18446744073709551616", ""},      // MaxUint64 + 1
		{"-9223372036854775809", "-9223372036854775809", ""},      // MinInt64 - 1
		{"123456789012345678901234567890", "123456789012345678901234567890", ""},
	}
	for _, tt := range tests {
		node := valueToYAMLNode(json.Number(tt.in))
		if node.Value != tt.want {
			t.Errorf("valueToYAMLNode(%s).Value = %q, want %q", tt.in, node.Value, tt.want)
		}
		if node.Tag != tt.wantTag {
			t.Errorf("valueToYAMLNode(%s).Tag = %q, want %q", tt.in, node.Tag, tt.wantTag)
		}
	}
}

// TestPrintFormatted_ExponentAndBigIntNumbers is an end-to-end regression
// test for blocker 2: with no --query, PrintFormatted decodes numbers as
// json.Number (see the UseNumber comment), so this exercises the exact path
// that regressed exponent floats while fixing big-integer precision.
func TestPrintFormatted_ExponentAndBigIntNumbers(t *testing.T) {
	data := json.RawMessage(`{"avgLatency":1e-7,"tinyAvg":2.5e-8,"huge":1e21,"count":9007199254740993,"big":1234567890123456789}`)

	t.Run("tsv", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := PrintFormatted(cmd, data, "tsv"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
		// tsvRow sorts object keys: avgLatency, big, count, huge, tinyAvg.
		want := "1e-07\t1234567890123456789\t9007199254740993\t1e+21\t2.5e-08\n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("yaml", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := PrintFormatted(cmd, data, "yaml"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
		want := "avgLatency: 1.0e-07\nbig: 1234567890123456789\ncount: 9007199254740993\nhuge: 1.0e+21\ntinyAvg: 2.5e-08\n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		// The rendered YAML must remain resolvable to floats where the JSON
		// literal was a float, and specifically under PyYAML's stricter
		// YAML-1.1 resolver (verified separately with `python3 -c "import
		// yaml; ..."`, since this package has no PyYAML to shell out to):
		// PyYAML requires a '.' in the mantissa, so a bare "1e-07"/"1e+21"
		// would decode as a *string* there even though yaml.v3's own decoder
		// (used below as a secondary, less strict check) accepts it as a
		// float.
		var decoded map[string]interface{}
		if err := yaml.Unmarshal(buf.Bytes(), &decoded); err != nil {
			t.Fatalf("decode rendered yaml: %v", err)
		}
		if _, ok := decoded["avgLatency"].(float64); !ok {
			t.Errorf("avgLatency decoded as %T, want float64", decoded["avgLatency"])
		}
	})
}

// TestPrintFormatted_IntegerBeyondYAMLIntRange is the azure-go-cli-6y9
// regression test: wire integers outside yaml.v3's own !!int resolver range
// (below math.MinInt64, or above math.MaxUint64) must render byte-identical
// to knack's format_yaml/format_tsv, and the rendered YAML must still parse
// under yaml.Unmarshal (via intYAMLNode dropping the tag it can't back up).
// Expectations captured by running knack 0.14.0 (format_yaml/format_tsv) on
// this exact payload, not hand-written.
func TestPrintFormatted_IntegerBeyondYAMLIntRange(t *testing.T) {
	data := json.RawMessage(`{"big":12345678901234567890,"huge":123456789012345678901234567890,"neg":-123456789012345678901234567890,"small":9223372036854775807,"umax":18446744073709551615,"umax1":18446744073709551616}`)

	t.Run("yaml", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := PrintFormatted(cmd, data, "yaml"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
		want := "big: 12345678901234567890\nhuge: 123456789012345678901234567890\nneg: -123456789012345678901234567890\nsmall: 9223372036854775807\numax: 18446744073709551615\numax1: 18446744073709551616\n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		var decoded map[string]interface{}
		if err := yaml.Unmarshal(buf.Bytes(), &decoded); err != nil {
			t.Errorf("rendered yaml did not parse: %v\nyaml:\n%s", err, buf.String())
		}
	})

	t.Run("tsv", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := PrintFormatted(cmd, data, "tsv"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
		// tsvRow sorts object keys: big, huge, neg, small, umax, umax1.
		want := "12345678901234567890\t123456789012345678901234567890\t-123456789012345678901234567890\t9223372036854775807\t18446744073709551615\t18446744073709551616\n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestPrintFormatted_QueryIntegerNotFloat is the D7 regression test: an
// integer selected or computed by --query must render as an integer, not a
// Python-str(float)-style "N.0". Before this fix, --query results were
// decoded as plain float64 (go-jmespath's numeric builtins require it), so
// PrintFormatted rendered "8080.0" where knack (and this package's own
// no-query path) renders "8080".
func TestPrintFormatted_QueryIntegerNotFloat(t *testing.T) {
	data := json.RawMessage(`[{"port":8080,"n":1},{"port":9090,"n":1}]`)

	t.Run("tsv selection", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "[].port", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := PrintFormatted(cmd, data, "tsv"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
		if got, want := buf.String(), "8080\n9090\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("tsv computed via length", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "length(@)", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := PrintFormatted(cmd, data, "tsv"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
		if got, want := buf.String(), "2\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("tsv negative int", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "[0].a", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := PrintFormatted(cmd, json.RawMessage(`[{"a":-7}]`), "tsv"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
		if got, want := buf.String(), "-7\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("table selection", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("query", "[].port", "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := PrintFormatted(cmd, data, "table"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
		if got, want := buf.String(), "Result\n--------\n8080\n9090\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestPrintFormatted_QueryTSVNull is the D2b test: a null a --query actually
// produces (as opposed to a top-level null with no query, D2a) renders as
// "None\n", matching knack.
func TestPrintFormatted_QueryTSVNull(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("query", "a.b", "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := PrintFormatted(cmd, json.RawMessage(`{"a":{"b":null}}`), "tsv"); err != nil {
		t.Fatalf("PrintFormatted: %v", err)
	}
	if got, want := buf.String(), "None\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestRenderYAML_StructuralShapes pins renderYAML's output for the shapes
// that let blocker 1 (dedentBlockSequences) through: a sequence nested three
// deep, a map value that is a list inside separate list items, and a
// multi-line string whose first line starts with "- " (which
// dedentBlockSequences mistook for a block sequence and corrupted). Every
// case is also asserted to round-trip to the original data.
func TestRenderYAML_StructuralShapes(t *testing.T) {
	tests := []struct {
		name string
		in   string // JSON literal
		want string // exact renderYAML output (Go-observed, not knack-verified: these shapes exercise YAML correctness, not table formatting)
	}{
		{
			name: "sequence nested three deep",
			in:   `[[[["a"]]]]`,
			want: "- - - - a\n",
		},
		{
			name: "list-in-list-in-item (former corruption case)",
			in:   `[[["a"]],[["b","c"]]]`,
			want: "- - - a\n- - - b\n    - c\n",
		},
		{
			name: "map value that is a list, across multiple list items",
			in:   `[{"name":"a","zones":["1","2"]},{"name":"b","zones":["3"]}]`,
			want: "- name: a\n  zones:\n    - \"1\"\n    - \"2\"\n- name: b\n  zones:\n    - \"3\"\n",
		},
		{
			name: "multi-line string whose first line starts with \"- \"",
			in:   `[{"description":"- read\n- write","name":"Reader"}]`,
			want: "- description: \"- read\\n- write\"\n  name: Reader\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v interface{}
			if err := json.Unmarshal([]byte(tt.in), &v); err != nil {
				t.Fatalf("decode input: %v", err)
			}
			got, err := renderYAML(v)
			if err != nil {
				t.Fatalf("renderYAML: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}

			var roundTripped interface{}
			if err := yaml.Unmarshal([]byte(got), &roundTripped); err != nil {
				t.Fatalf("rendered yaml did not parse: %v\nyaml:\n%s", err, got)
			}
			if !reflect.DeepEqual(normalizeYAMLRoundTrip(roundTripped), normalizeYAMLRoundTrip(v)) {
				t.Errorf("round-trip mismatch:\ngot:  %#v\nwant: %#v",
					normalizeYAMLRoundTrip(roundTripped), normalizeYAMLRoundTrip(v))
			}
		})
	}
}

// randStringPool is the corpus of adversarial key/value strings the property
// test draws from: newline-only, leading-newline, trailing-newline, and
// multi-line strings (the shapes that lost data to yaml.v3's block-scalar
// selection — see stringYAMLNode); empty; YAML-1.1 words ("y"/"n"/"yes"/
// "no"/"on"/"off"/"true"/"false"); numeric-looking ("1.5", "007", "1e5");
// leading/trailing whitespace; "#", ":", and "- " prefixes (comment/mapping/
// sequence indicators if left unquoted); embedded quotes; tabs; and Unicode
// including a non-BMP emoji.
var randStringPool = []string{
	"plain", "- dash prefixed", "multi\nline\nvalue", "- a\n- b",
	"", "\n", "\na", "a\n", "a\nb", "\n\n",
	"y", "n", "yes", "no", "on", "off", "true", "false",
	"1.5", "007", "1e5", "-3", "0x1F",
	" lead", "trail ", "  both  ",
	"#hash", "a:b", ": colon-lead", "- dash", "- ",
	"'single'", "\"double\"", "it's", `has "both" and 'quote'`,
	"tab\ttab", "unicode: café 日本", "emoji😀",
}

// randJSONValue generates a deterministic pseudo-random JSON-shaped value:
// nested maps and lists (up to depth 3) of strings (from randStringPool),
// numbers, bools, and nulls. Map keys are also drawn from randStringPool,
// not a fixed "k0".."k3" set, so key-side quoting/escaping is exercised the
// same as value-side.
func randJSONValue(rng *rand.Rand, depth int) interface{} {
	choices := 5
	if depth >= 3 {
		choices = 3 // force a leaf once nesting gets deep enough
	}
	switch rng.Intn(choices) {
	case 0:
		return nil
	case 1:
		return randNumber(rng)
	case 2:
		return randStringPool[rng.Intn(len(randStringPool))]
	case 3:
		n := rng.Intn(4)
		out := make([]interface{}, n)
		for i := range out {
			out[i] = randJSONValue(rng, depth+1)
		}
		return out
	default:
		n := rng.Intn(4)
		out := make(map[string]interface{}, n)
		for i := 0; i < n; i++ {
			out[randStringPool[rng.Intn(len(randStringPool))]] = randJSONValue(rng, depth+1)
		}
		return out
	}
}

// randNumber generates a float64 leaf, occasionally a small-magnitude value
// (to exercise formatNumber's exponent-notation switch), matching the
// numeric leaves a jmespath computation would produce.
//
// The last arm covers the magnitude band this generator used to skip. A
// whole-number float in [1e18, 1e21) is written by encoding/json as a
// full-digit literal (it only switches to exponent form at 1e21), so on the
// UseNumber path it reaches renderYAML as an integer-shaped json.Number
// indistinguishable from a wire integer. Literals outside
// [math.MinInt64, math.MaxUint64] are exactly the ones yaml.v3's own resolver
// cannot read back as an int, which made the encoder write an explicit
// "!!int" tag its own decoder then rejected (beads azure-go-cli-6y9; see
// intYAMLNode). The arm spans both sides of that limit, and both signs.
func randNumber(rng *rand.Rand) float64 {
	switch rng.Intn(4) {
	case 0:
		return rng.Float64()*200 - 100
	case 1:
		return math.Trunc(rng.Float64() * 1e6) // integral
	case 2:
		return rng.Float64() * 1e-8 // forces exponent notation
	default:
		f := math.Trunc(rng.Float64()*999) * 1e18 // [0, 1e21): full-digit in JSON
		if rng.Intn(2) == 0 {
			f = -f
		}
		return f
	}
}

// TestRenderYAMLRoundTripProperty is the property test that would have
// caught blocker 1: it generates a deterministic corpus of nested
// JSON-shaped values (fixed seed 42), renders each with renderYAML, and
// asserts the result parses back (yaml.Unmarshal) to the same value.
//
// Every third case is additionally round-tripped through
// encoding/json.Decoder with UseNumber() before rendering — mirroring
// PrintFormatted's own no-query decode path — so numeric leaves reach
// renderYAML as json.Number (exercising its integer/exponent-literal
// handling) rather than only as the float64 a jmespath computation would
// produce.
func TestRenderYAMLRoundTripProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	const n = 600
	numberViaJSONNumber := 0
	outOfRangeInts := 0
	for i := 0; i < n; i++ {
		v := randJSONValue(rng, 0)

		if i%3 == 0 {
			raw, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("case %d: marshal input: %v", i, err)
			}
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.UseNumber()
			if err := dec.Decode(&v); err != nil {
				t.Fatalf("case %d: UseNumber decode: %v", i, err)
			}
			numberViaJSONNumber++
			outOfRangeInts += countOutOfRangeInts(v)
		}

		got, err := renderYAML(v)
		if err != nil {
			t.Fatalf("case %d: renderYAML(%#v): %v", i, v, err)
		}
		var roundTripped interface{}
		if err := yaml.Unmarshal([]byte(got), &roundTripped); err != nil {
			t.Fatalf("case %d: rendered yaml did not parse: %v\ninput: %#v\nyaml:\n%s", i, v, err, got)
		}
		want := normalizeYAMLRoundTrip(v)
		gotNorm := normalizeYAMLRoundTrip(roundTripped)
		if !reflect.DeepEqual(gotNorm, want) {
			t.Fatalf("case %d: round-trip mismatch\ninput: %#v\nyaml:\n%s\ngot:  %#v\nwant: %#v", i, v, got, gotNorm, want)
		}
	}
	if numberViaJSONNumber == 0 {
		t.Fatal("no cases exercised the json.Number path")
	}
	if outOfRangeInts == 0 {
		t.Fatal("no integer literal outside yaml.v3's !!int resolver range was generated: randNumber's large arm no longer covers the band")
	}
	t.Logf("%d/%d cases decoded via json.Number (UseNumber); %d out-of-!!int-range integer literals", numberViaJSONNumber, n, outOfRangeInts)
}

// yamlIntResolvable reports whether yaml.v3's own !!int resolver would read
// this integer-shaped literal back as an int, mirroring intYAMLNode's check.
func yamlIntResolvable(lit string) bool {
	if _, err := strconv.ParseInt(lit, 10, 64); err == nil {
		return true
	}
	_, err := strconv.ParseUint(lit, 10, 64)
	return err == nil
}

// countOutOfRangeInts walks a decoded value (as produced by the UseNumber
// path) and counts integer-shaped json.Number leaves outside yaml.v3's !!int
// resolver range.
func countOutOfRangeInts(v interface{}) int {
	switch t := v.(type) {
	case map[string]interface{}:
		n := 0
		for _, e := range t {
			n += countOutOfRangeInts(e)
		}
		return n
	case []interface{}:
		n := 0
		for _, e := range t {
			n += countOutOfRangeInts(e)
		}
		return n
	case json.Number:
		s := string(t)
		if !strings.ContainsAny(s, ".eE") && !yamlIntResolvable(s) {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// golden mirrors one entry of testdata/goldens.json: the "table" expectations
// were produced by running the real azure-cli output layer — knack 0.14.0
// (_TableOutput) on tabulate 0.10.0, i.e. exactly what `az ... -o table`
// invokes:
//
//	from knack.output import _TableOutput
//	_TableOutput(should_sort_keys=True).dump(payload if isinstance(payload, list) else [payload])
//
// with one documented exception: the "null result" entry's "table" value is
// NOT knack's literal output for that input — it encodes this package's
// deliberate divergence instead (see its tableDivergenceNote field in
// goldens.json, and renderTable's doc comment).
//
// The "yaml" expectations were diffed against knack's format_yaml
// (yaml.safe_dump(default_flow_style=False, allow_unicode=True), PyYAML 6.0.3).
// They are NOT byte-identical to PyYAML's output in general; known,
// semantically-equivalent divergences are:
//   - PyYAML's trailing "..." document-end marker after a top-level scalar
//   - yaml.v3 double-quotes scalars that need quoting where PyYAML uses
//     single quotes
//   - yaml.v3 indents nested block sequences one level deeper than PyYAML's
//     default "indentless" sequences (see the "nested non-empty lists"
//     golden below, and its yamlPyYAMLDivergenceNote in goldens.json)
//   - yaml.v3 escapes non-BMP runes (emoji) where PyYAML's
//     allow_unicode=True emits them raw
//
// All of the above divergences parse to the same data, asserted by
// TestGoldenYAMLRoundTrip.
//
// The "tsv" expectations were produced the same way, via knack's format_tsv
// (_TsvOutput), with one documented exception: the "null result" entry's
// "tsv" value is NOT knack's literal output for that input (knack renders
// "None\n") — it encodes this package's D2a divergence instead (see its
// tsvDivergenceNote field in goldens.json, and renderTSV's doc comment).
type golden struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Table string          `json:"table"`
	YAML  string          `json:"yaml"`
	TSV   string          `json:"tsv"`
}

//go:embed testdata/goldens.json
var goldensJSON []byte

func loadGoldens(t *testing.T) []golden {
	t.Helper()
	var goldens []golden
	if err := json.Unmarshal(goldensJSON, &goldens); err != nil {
		t.Fatalf("decode goldens.json: %v", err)
	}
	return goldens
}

func TestGoldens(t *testing.T) {
	for _, g := range loadGoldens(t) {
		t.Run(g.Name, func(t *testing.T) {
			// Decode with UseNumber, mirroring PrintFormatted's own decode
			// when no --query is given (see its UseNumber comment): these
			// goldens simulate ordinary command output, not jmespath-query
			// results, so integer-shaped literals must reach renderTable/
			// renderYAML as json.Number rather than collapsing to float64.
			var v interface{}
			dec := json.NewDecoder(bytes.NewReader(g.Input))
			dec.UseNumber()
			if err := dec.Decode(&v); err != nil {
				t.Fatalf("decode input: %v", err)
			}
			if got := renderTable(v, nil); got != g.Table {
				t.Errorf("renderTable: got %q, want %q", got, g.Table)
			}
			if got := renderTSV(v, false, nil); got != g.TSV {
				t.Errorf("renderTSV: got %q, want %q", got, g.TSV)
			}
			got, err := renderYAML(v)
			if err != nil {
				t.Fatalf("renderYAML: %v", err)
			}
			if got != g.YAML {
				t.Errorf("renderYAML: got %q, want %q", got, g.YAML)
			}
		})
	}
}

// TestGoldenYAMLRoundTrip proves the accepted PyYAML divergences (Y3, Y4) are
// cosmetic: our rendered YAML round-trips to the same data as the source
// JSON.
func TestGoldenYAMLRoundTrip(t *testing.T) {
	for _, g := range loadGoldens(t) {
		t.Run(g.Name, func(t *testing.T) {
			var want interface{}
			if err := json.Unmarshal(g.Input, &want); err != nil {
				t.Fatalf("decode input: %v", err)
			}
			var got interface{}
			if err := yaml.Unmarshal([]byte(g.YAML), &got); err != nil {
				t.Fatalf("decode rendered yaml: %v", err)
			}
			wantNorm := normalizeYAMLRoundTrip(want)
			gotNorm := normalizeYAMLRoundTrip(got)
			if !reflect.DeepEqual(wantNorm, gotNorm) {
				t.Errorf("round-trip mismatch:\ngot:  %#v\nwant: %#v", gotNorm, wantNorm)
			}
		})
	}
}

// normalizeYAMLRoundTrip flattens the two decoders' differing numeric and map
// types (json gives float64/map[string]interface{}; yaml.v3 gives
// int/map[string]interface{} with some numeric types as int) down to a
// common shape so DeepEqual compares values, not decoder-specific types.
func normalizeYAMLRoundTrip(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, e := range t {
			out[k] = normalizeYAMLRoundTrip(e)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, e := range t {
			out[i] = normalizeYAMLRoundTrip(e)
		}
		return out
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case uint64:
		// yaml.v3 decodes an integer scalar in (math.MaxInt64, math.MaxUint64]
		// as uint64.
		return float64(t)
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return v
		}
		return f
	default:
		return v
	}
}

// TestKeyOrderFallbacks (azure-go-cli-c41) pins keyOrder.keys' fallback-to-
// sorted behaviour: a nil receiver, a key set that matches no recovered
// hash, two hashes sharing a key set but declaring different orders
// (ambiguous), and a hash with a duplicate key (cannot correspond to any Go
// map's key set) must all sort rather than pick an arbitrary order.
func TestKeyOrderFallbacks(t *testing.T) {
	m := map[string]interface{}{"b": 2, "a": 1}
	wantSorted := []string{"a", "b"}

	t.Run("nil receiver sorts", func(t *testing.T) {
		var ko *keyOrder
		if got := ko.keys(m); !reflect.DeepEqual(got, wantSorted) {
			t.Errorf("got %v, want %v", got, wantSorted)
		}
	})

	t.Run("no matching key set sorts", func(t *testing.T) {
		ko := newKeyOrder([][]string{{"x", "y"}})
		if got := ko.keys(m); !reflect.DeepEqual(got, wantSorted) {
			t.Errorf("got %v, want %v", got, wantSorted)
		}
	})

	t.Run("matching key set uses declared order", func(t *testing.T) {
		ko := newKeyOrder([][]string{{"b", "a"}})
		want := []string{"b", "a"}
		if got := ko.keys(m); !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("same key set, conflicting orders sorts", func(t *testing.T) {
		ko := newKeyOrder([][]string{{"b", "a"}, {"a", "b"}})
		if got := ko.keys(m); !reflect.DeepEqual(got, wantSorted) {
			t.Errorf("got %v, want %v", got, wantSorted)
		}
	})

	t.Run("duplicate key in hash ignored, falls back to sorted", func(t *testing.T) {
		ko := newKeyOrder([][]string{{"a", "a"}})
		if ko != nil {
			t.Fatalf("newKeyOrder with only a duplicate-key hash = %v, want nil", ko)
		}
		if got := ko.keys(m); !reflect.DeepEqual(got, wantSorted) {
			t.Errorf("got %v, want %v", got, wantSorted)
		}
	})

	t.Run("empty orders returns nil", func(t *testing.T) {
		if ko := newKeyOrder(nil); ko != nil {
			t.Errorf("newKeyOrder(nil) = %v, want nil", ko)
		}
	})
}

// queryGolden is one entry in query_goldens.json: a --query
// multiselect-hash case whose table/tsv rendering must preserve the query's
// declared column order (azure-go-cli-c41), rather than the sorted order
// used when no --query is active.
//
// Payload constraint (see the file's generator, documented in the c41
// design doc): string-valued scalar fields only, no nulls, no numbers, and
// no nested containers in selected columns. Those axes hit divergences that
// are already tracked elsewhere (tsv null-row rendering, tsv nested-
// container rendering, and a pre-existing --query "1.0" vs "1" integer bug)
// and would bake a known-wrong byte string into a golden here.
//
// One entry, "sort_by-hash-order-not-recovered", is a DELIBERATE exception:
// its query wraps the hash in sort_by(), a function expression, which the
// azure-go-cli-c41 follow-up whitelist (see query.MultiSelectHashKeyOrders)
// refuses to reason about at all — real knack still recovers the hash's
// order here (its jmespath evaluator preserves OrderedDict insertion order
// at runtime, no static AST analysis needed), so that entry's table/tsv are
// this package's safe, sorted-fallback output, NOT knack's actual output;
// accepted as a known, documented coverage regression in exchange for
// closing the class of bug the whitelist exists to prevent.
type queryGolden struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Query string          `json:"query"`
	Table string          `json:"table"`
	TSV   string          `json:"tsv"`
}

//go:embed testdata/query_goldens.json
var queryGoldensJSON []byte

func TestQueryGoldens(t *testing.T) {
	var goldens []queryGolden
	if err := json.Unmarshal(queryGoldensJSON, &goldens); err != nil {
		t.Fatalf("decode query_goldens.json: %v", err)
	}
	for _, g := range goldens {
		t.Run(g.Name, func(t *testing.T) {
			// Mirrors PrintFormatted's --query decode path: plain
			// json.Unmarshal (not UseNumber), then ApplyJMESPath.
			var v interface{}
			if err := json.Unmarshal(g.Input, &v); err != nil {
				t.Fatalf("decode input: %v", err)
			}
			result, err := query.ApplyJMESPath(v, g.Query)
			if err != nil {
				t.Fatalf("ApplyJMESPath: %v", err)
			}
			ko := newKeyOrder(query.MultiSelectHashKeyOrders(g.Query))
			if got := renderTable(result, ko); got != g.Table {
				t.Errorf("renderTable: got %q, want %q", got, g.Table)
			}
			if got := renderTSV(result, true, ko); got != g.TSV {
				t.Errorf("renderTSV: got %q, want %q", got, g.TSV)
			}
		})
	}
}

// TestNotNullPassthroughDoesNotBorrowHashOrder pins the exact leak found in
// the azure-go-cli-c41 follow-up gate: `not_null(p, one.{b:x, a:y})` returns
// `p` unchanged (an unrelated map that happens to share the hash's {a,b} key
// set), and `p`'s OWN key order must render, never the hash's declared
// [b,a]. query.MultiSelectHashKeyOrders must therefore return nil for this
// query (see its "unsafe: hash as not_null argument" test case) so ko falls
// back to nil and renderTable sorts p's keys instead of guessing.
//
// The expected table string was generated by running knack 0.14.0 +
// jmespath 1.1.0 directly (collections.OrderedDict input, is_query_active =
// True, table_transformer = None -> should_sort_keys = False,
// knack.output._TableOutput(False).dump): "A    B\n---  ---\naa   bb\n" —
// knack renders p's own insertion order (a, b), not the hash's declared
// (b, a).
func TestNotNullPassthroughDoesNotBorrowHashOrder(t *testing.T) {
	const q = "not_null(p, one.{b:x, a:y})"
	const input = `{"p":{"a":"aa","b":"bb"}, "one":{"x":"X","y":"Y"}}`
	const wantTable = "A    B\n---  ---\naa   bb\n"

	if orders := query.MultiSelectHashKeyOrders(q); orders != nil {
		t.Fatalf("MultiSelectHashKeyOrders(%q) = %v, want nil (hash used as a not_null argument)", q, orders)
	}

	var v interface{}
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("decode input: %v", err)
	}
	result, err := query.ApplyJMESPath(v, q)
	if err != nil {
		t.Fatalf("ApplyJMESPath: %v", err)
	}
	ko := newKeyOrder(query.MultiSelectHashKeyOrders(q))
	if got := renderTable(result, ko); got != wantTable {
		t.Errorf("renderTable = %q, want %q (knack: p's own key order, not the hash's)", got, wantTable)
	}
}

// TestHashOrderNotBorrowedByPassthroughMap pins azure-go-cli-c41's core
// hazard: a --query multiselect-hash's declared column order must reach only
// the maps the hash itself built, never a map the query merely forwarded out
// of the payload whose key SET happens to match.
//
// `p` below is such a map — its keys are {a,b}, the same set every hash here
// declares, but in the opposite order. Every want value was generated by
// running knack 0.14.0 + jmespath 1.1.0 (Options(OrderedDict),
// is_query_active=True) on this exact payload.
func TestHashOrderNotBorrowedByPassthroughMap(t *testing.T) {
	const payload = `{"p":{"a":"aa","b":"bb"},"one":{"x":"X","y":"Y"},` +
		`"objs":[{"p":{"a":"a1","b":"b1"},"x":"X1","y":"Y1"},` +
		`{"p":{"a":"a2","b":"b2"},"x":"X2","y":"Y2"}]}`

	tests := []struct {
		query string
		want  string
	}{
		// Hash is NON-terminal: the result is `p`, so `p`'s own order wins.
		{"{b: p, a: one}.b", "A    B\n---  ---\naa   bb\n"},
		{"objs[].{b: p, a: x}[].b", "A    B\n---  ---\na1   b1\na2   b2\n"},
		// `{b: p, a: one}.*` is the third leak shape, but go-jmespath's value
		// projection iterates a Go map, so its ROW order is nondeterministic
		// and the rendered table cannot be pinned here. Its guard is asserted
		// deterministically in pkg/query's TestMultiSelectHashKeyOrders.
		// Hash IS terminal: the declared order must still be honoured.
		{"objs[].{b: x, a: y} | [0]", "B    A\n---  ---\nX1   Y1\n"},
		{"objs[].{Name: x, Location: y}", "Name    Location\n------  ----------\nX1      Y1\nX2      Y2\n"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			var data interface{}
			if err := json.Unmarshal([]byte(payload), &data); err != nil {
				t.Fatalf("payload: %v", err)
			}
			cmd := &cobra.Command{}
			cmd.Flags().String("query", tt.query, "")
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			if err := PrintFormatted(cmd, data, "table"); err != nil {
				t.Fatalf("PrintFormatted: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
