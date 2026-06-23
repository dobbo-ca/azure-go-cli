package output

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/pkg/query"
	"github.com/spf13/cobra"
)

// PrintJSON prints data as JSON, optionally applying a JMESPath query
func PrintJSON(cmd *cobra.Command, data interface{}) error {
	queryStr, _ := cmd.Flags().GetString("query")

	// Marshal to JSON first
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Apply query if specified
	if queryStr != "" {
		jsonData, err = query.ApplyJMESPathToJSON(jsonData, queryStr)
		if err != nil {
			return err
		}
	}

	fmt.Println(string(jsonData))
	return nil
}

// PrintFormatted renders data honoring the global --query flag and the given
// output format ("json" or "tsv"). It matches azure-cli's behavior closely
// enough for scripting: --query is applied first, then the result is rendered.
func PrintFormatted(cmd *cobra.Command, data interface{}, format string) error {
	queryStr, _ := cmd.Flags().GetString("query")

	// Normalize through JSON so query and rendering operate on the same
	// generic shape regardless of the concrete Go type passed in.
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	var result interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return fmt.Errorf("failed to parse output: %w", err)
	}

	if queryStr != "" {
		result, err = query.ApplyJMESPath(result, queryStr)
		if err != nil {
			return err
		}
	}

	switch strings.ToLower(format) {
	case "tsv":
		fmt.Print(renderTSV(result))
		return nil
	case "json", "":
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}
		fmt.Println(string(out))
		return nil
	default:
		return fmt.Errorf("unsupported output format %q (use json, table, or tsv)", format)
	}
}

// renderTSV renders a JMESPath/JSON result as tab-separated values, one record
// per line. Lists produce one line per element; objects emit their values
// (sorted by key) joined by tabs; scalars emit themselves.
func renderTSV(v interface{}) string {
	var b strings.Builder
	switch val := v.(type) {
	case nil:
		return ""
	case []interface{}:
		for _, el := range val {
			b.WriteString(tsvRow(el))
			b.WriteByte('\n')
		}
	default:
		b.WriteString(tsvRow(val))
		b.WriteByte('\n')
	}
	return b.String()
}

// tsvRow renders a single record into one tab-separated line.
//
// For object rows (a JMESPath multiselect-hash like `[].{a:x, b:y}`), columns
// are emitted in sorted-key order. azure-cli preserves the query's written
// order, but a multiselect-hash decodes into a Go map, whose order is not
// recoverable — so sorted order is the deterministic choice. Scripts that need
// a guaranteed column order should use a multiselect-list (`[].[x, y]`), which
// arrives as an ordered slice and is emitted in query order below.
func tsvRow(v interface{}) string {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		cells := make([]string, 0, len(keys))
		for _, k := range keys {
			cells = append(cells, tsvScalar(val[k]))
		}
		return strings.Join(cells, "\t")
	case []interface{}:
		cells := make([]string, 0, len(val))
		for _, el := range val {
			cells = append(cells, tsvScalar(el))
		}
		return strings.Join(cells, "\t")
	default:
		return tsvScalar(val)
	}
}

// tsvScalar renders a leaf value the way azure-cli's tsv formatter does.
func tsvScalar(v interface{}) string {
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
