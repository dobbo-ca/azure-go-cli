package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cdobbyn/azure-go-cli/pkg/query"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// PrintJSON prints data honoring the global --query flag and, when the
// caller's command tree defines it, the global --output/-o flag. When the
// format is json (or unset) this is byte-for-byte the output it has always
// produced: the value is marshalled directly, so struct field order is
// preserved. Any other format is delegated to PrintFormatted, which
// normalizes through a generic JSON tree.
func PrintJSON(cmd *cobra.Command, data interface{}) error {
	format, _ := cmd.Flags().GetString("output")
	switch strings.ToLower(format) {
	case "", "json":
		return printJSONIndented(cmd, data)
	default:
		return PrintFormatted(cmd, data, format)
	}
}

// marshalIndentNoEscape behaves like json.MarshalIndent(v, "", "  ") except
// it does not HTML-escape &, < and > to \u0026, \u003c and
// \u003e, matching azure-cli's json.dumps(..., ensure_ascii=False)
// output for those three characters.
// It does NOT match json.dumps for U+2028/U+2029: Go's encoder still
// escapes them to \u2028/\u2029 even with SetEscapeHTML(false), while
// ensure_ascii=False leaves them as raw UTF-8. That divergence remains.
// json.Encoder.Encode appends a trailing "\n" that MarshalIndent does not,
// so it is trimmed to keep callers (which already do fmt.Fprintln)
// byte-identical apart from the escaping.
func marshalIndentNoEscape(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// printJSONIndented is the historical PrintJSON body: it marshals data
// directly (preserving struct field declaration order) rather than routing
// through the generic map[string]interface{} tree PrintFormatted uses.
func printJSONIndented(cmd *cobra.Command, data interface{}) error {
	queryStr, _ := cmd.Flags().GetString("query")

	// Marshal to JSON first
	jsonData, err := marshalIndentNoEscape(data)
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

	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
	return nil
}

// PrintFormatted renders data honoring the global --query flag and the given
// output format ("json", "table", "tsv", "yaml", or "none"). It matches
// azure-cli's behavior closely enough for scripting: --query is applied
// first, then the result is rendered.
func PrintFormatted(cmd *cobra.Command, data interface{}, format string) error {
	queryStr, _ := cmd.Flags().GetString("query")

	// Normalize through JSON so query and rendering operate on the same
	// generic shape regardless of the concrete Go type passed in.
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	var result interface{}
	if queryStr == "" {
		// No --query: decode numbers as json.Number so large integers and
		// wire-literal decimals (e.g. int64 values above 2^53, "3.0") render
		// exactly as received instead of round-tripping through float64.
		dec := json.NewDecoder(bytes.NewReader(jsonData))
		dec.UseNumber()
		if err := dec.Decode(&result); err != nil {
			return fmt.Errorf("failed to parse output: %w", err)
		}
	} else {
		// go-jmespath v0.4.0 is hardcoded to float64: its numeric builtins
		// (avg, sum, max, ceil, ...) error on a json.Number argument and its
		// comparators silently evaluate to false. So the UseNumber decode is
		// used only for the queries query.PreservesNumberLiterals proves are
		// free of both (no function call, no comparator, anywhere in the AST)
		// - which is most real --query usage: field paths, projections and
		// multiselect hashes. Everything else decodes to float64 exactly as
		// before. See PreservesNumberLiterals for the full argument.
		dec := json.NewDecoder(bytes.NewReader(jsonData))
		if query.PreservesNumberLiterals(queryStr) {
			dec.UseNumber()
		}
		if err := dec.Decode(&result); err != nil {
			return fmt.Errorf("failed to parse output: %w", err)
		}
		result, err = query.ApplyJMESPath(result, queryStr)
		if err != nil {
			return err
		}
		// Re-encode the query's result and decode it again with UseNumber so
		// an integer-shaped value stays an integer literal for rendering
		// (matches azure-cli; a bare float64 would otherwise render "8080.0"
		// for an ARM integer field selected by --query). json.Marshal writes a
		// json.Number verbatim, so this is a no-op for values that survived
		// the UseNumber path above.
		//
		// REMAINING, DOCUMENTED DIVERGENCE from knack, for the queries
		// PreservesNumberLiterals refuses (those containing a function call or
		// a comparator): the float64 decode has already lost precision before
		// this re-encode runs, and re-decoding with UseNumber cannot recover
		// it. Three classes:
		//   - an integer literal outside float64's exact range, i.e. above
		//     2^53 (or below -2^53), rounds to the nearest representable
		//     float64 (e.g. 9007199254740993 -> 9007199254740992,
		//     9223372036854775807 -> 9223372036854776000); knack, with
		//     arbitrary-precision Python ints, renders it exactly.
		//   - a wire literal that is a genuine float but happens to be
		//     integral (e.g. 3.0, or an ARM value like 25000000000.0) comes
		//     back from float64 with no way to tell it apart from an integer,
		//     so it renders without its decimal point (e.g. "3", not knack's
		//     "3.0"); the same loss applies to -0.0, which renders "0".
		//   - a wire literal in exponent form from 1e+16 (inclusive) up to
		//     1e+21 (exclusive) reprints in positional form (1e+16 ->
		//     10000000000000000), because Go's float64 encoder switches to
		//     exponent form at 1e+21 while Python's repr switches at 1e+16.
		//     At or above 1e+21 the two agree again (both print "1e+21").
		// A separate divergence this cannot address at all: Python jmespath's
		// sum/max/min/ceil/floor return an int for integral input (knack
		// prints sum([1,2,3]) as "6", avg([1,2,3]) as "2.0"), while
		// go-jmespath returns float64 for every one of them, so avg renders
		// "2" here.
		if reencoded, err := json.Marshal(result); err == nil {
			dec := json.NewDecoder(bytes.NewReader(reencoded))
			dec.UseNumber()
			var renum interface{}
			if dec.Decode(&renum) == nil {
				result = renum
			}
		}
	}

	// ko recovers a --query multiselect-hash's declared column order (see
	// keyOrder's doc comment). It is only built on the query path: with no
	// --query, ko stays nil and every map renders in sorted-key order,
	// matching knack's should_sort_keys = not is_query_active.
	var ko *keyOrder
	if queryStr != "" {
		ko = newKeyOrder(query.MultiSelectHashKeyOrders(queryStr))
	}

	w := cmd.OutOrStdout()
	switch strings.ToLower(format) {
	case "tsv":
		fmt.Fprint(w, renderTSV(result, queryStr != "", ko))
		return nil
	case "table":
		fmt.Fprint(w, renderTable(result, ko))
		return nil
	case "yaml":
		out, err := renderYAML(result)
		if err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}
		fmt.Fprint(w, out)
		return nil
	case "none":
		return nil
	case "json", "":
		out, err := marshalIndentNoEscape(result)
		if err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}
		fmt.Fprintln(w, string(out))
		return nil
	default:
		return fmt.Errorf("argument --output/-o: invalid choice: %q (choose from 'json', 'table', 'tsv', 'yaml', 'none')", format)
	}
}

// keyOrder maps a map's key SET to the order a --query multiselect-hash wrote
// those keys (azure-go-cli-c41). A nil *keyOrder means "no --query ordering
// information": every map is rendered in sorted-key order, which is what
// knack does when is_query_active is false (knack/output.py:74).
type keyOrder struct {
	byKeySet map[string][]string // sorted-keys fingerprint -> declared order
}

// newKeyOrder builds a keyOrder from query.MultiSelectHashKeyOrders. It
// returns nil when there is nothing to order by. A key set claimed by two
// hashes with DIFFERENT declared orders is dropped (ambiguous -> sorted), as
// is a hash with a duplicate key (it cannot correspond to a Go map's key
// set).
func newKeyOrder(orders [][]string) *keyOrder {
	if len(orders) == 0 {
		return nil
	}
	byKeySet := make(map[string][]string, len(orders))
	dropped := map[string]bool{}
	for _, order := range orders {
		sorted := append([]string(nil), order...)
		sort.Strings(sorted)
		fp := keySetFingerprint(sorted)
		if hasDuplicate(sorted) {
			dropped[fp] = true
			continue
		}
		if dropped[fp] {
			continue
		}
		if existing, ok := byKeySet[fp]; ok {
			if !slices.Equal(existing, order) {
				delete(byKeySet, fp)
				dropped[fp] = true
			}
			continue
		}
		byKeySet[fp] = order
	}
	if len(byKeySet) == 0 {
		return nil
	}
	return &keyOrder{byKeySet: byKeySet}
}

// hasDuplicate reports whether sorted (already sorted) contains adjacent
// equal elements.
func hasDuplicate(sorted []string) bool {
	for i := 1; i < len(sorted); i++ {
		if sorted[i] == sorted[i-1] {
			return true
		}
	}
	return false
}

// keys returns the order in which m's keys should be rendered: the declared
// order if m's key set exactly matches a multiselect-hash in the query,
// otherwise sorted. Safe on a nil receiver.
func (ko *keyOrder) keys(m map[string]interface{}) []string {
	sorted := make([]string, 0, len(m))
	for k := range m {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	if ko == nil {
		return sorted
	}
	fp := keySetFingerprint(sorted)
	if order, ok := ko.byKeySet[fp]; ok {
		return order
	}
	return sorted
}

// keySetFingerprint builds a fingerprint for a sorted key list that cannot
// collide between two different key sets. A plain "\x00"-join is not
// injective: a single key containing an actual NUL byte can fingerprint the
// same as a different, shorter key set. JSON object keys and JMESPath quoted
// identifiers can both contain a NUL byte, so each key is length-prefixed
// here instead.
func keySetFingerprint(sorted []string) string {
	var b strings.Builder
	for _, k := range sorted {
		fmt.Fprintf(&b, "%d:%s", len(k), k)
	}
	return b.String()
}

// renderTSV renders a JMESPath/JSON result as tab-separated values, one record
// per line. Lists produce one line per element; objects emit their values
// (sorted by key, or in --query multiselect-hash order — see keyOrder)
// joined by tabs; scalars emit themselves.
func renderTSV(v interface{}, queryActive bool, ko *keyOrder) string {
	// D2a/D2b: a top-level null renders nothing UNLESS it came from --query.
	// The common case reaching this with v == nil and no query is an
	// unpopulated `var xs []T` that marshalled to JSON "null" — an empty
	// *list*, not a genuine null scalar — and azure-cli prints nothing for
	// an empty list. A null a query actually produced (a missing field, a
	// non-matching filter) is a genuine null, so it falls through to the
	// default case below and renders one "None" row, matching knack.
	if v == nil && !queryActive {
		return ""
	}
	var b strings.Builder
	switch val := v.(type) {
	case []interface{}:
		for _, el := range val {
			b.WriteString(tsvRow(el, ko))
			b.WriteByte('\n')
		}
	default:
		b.WriteString(tsvRow(val, ko))
		b.WriteByte('\n')
	}
	return b.String()
}

// tsvRow renders a single record into one tab-separated line.
//
// For object rows, columns are emitted in the order ko.keys reports: a
// JMESPath multiselect-hash's declared order (`[].{a:x, b:y}`) when ko
// recovers one for this row's exact key set, sorted-key order otherwise —
// matching knack's _TsvOutput._dump_row, which special-cases only an
// OrderedDict (the hash-derived case) and falls back to sorted() for a plain
// dict. Scripts that need a guaranteed column order regardless can also use a
// multiselect-list (`[].[x, y]`), which arrives as an ordered slice and is
// emitted in query order below.
func tsvRow(v interface{}, ko *keyOrder) string {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := ko.keys(val)
		cells := make([]string, 0, len(keys))
		for _, k := range keys {
			cells = append(cells, tsvCell(val[k]))
		}
		return strings.Join(cells, "\t")
	case []interface{}:
		cells := make([]string, 0, len(val))
		for _, el := range val {
			cells = append(cells, tsvCell(el))
		}
		return strings.Join(cells, "\t")
	case bool:
		// D5: a ROW-level bool (a scalar result, a top-level list element,
		// or a --query selection) goes through knack's _dump_row, which
		// lowercases it. A bool used as a CELL inside a dict/list row goes
		// through _dump_obj instead (tsvCell below) and stays "True"/"False".
		if val {
			return "true"
		}
		return "false"
	default:
		return tsvCell(val)
	}
}

// tsvCell renders a value nested inside a dict or list row, matching knack's
// _TsvOutput._dump_obj (knack/output.py:218-229):
//   - a nested list becomes its element COUNT, not its contents (D3).
//   - a nested map becomes "" (D4), keeping column counts stable when a
//     field is sometimes a dict, sometimes null.
//   - null becomes the string "None" (D1).
//   - anything else is a scalar, rendered by tsvScalar (shared with table).
func tsvCell(v interface{}) string {
	switch val := v.(type) {
	case []interface{}:
		return strconv.Itoa(len(val))
	case map[string]interface{}:
		return ""
	case nil:
		return "None"
	default:
		return tsvScalar(val)
	}
}

// tsvScalar is the scalar formatter shared by both cell paths that reach it:
// TSV (via tsvCell, for a value nested in a dict/list row, and via tsvRow's
// bool-free default case for a row that is itself a bare scalar) and table
// (tableEntry/tableScalar). knack's own _dump_obj (knack/output.py:218-229,
// the TSV-only container handling — count a nested list, blank a nested map,
// "None" for null) lives in tsvCell, NOT here, so it cannot leak into table
// rendering, which has its own container handling (isTableScalar/tableScalar
// via pyRepr). Do not merge tsvCell into tsvScalar.
//
// Here: strings pass through as-is, bools become "True"/"False" (knack's
// str(bool) — the row-level lowercasing in tsvRow's bool case is the one
// place that differs), numbers go through formatNumber/formatJSONNumber, nil
// renders "" (knack's table renders None as "" too, via tabulate's
// missingval; a TSV null goes through tsvCell instead, which knows to render
// "None"), and a nested list/map falls back to compact JSON — reachable only
// from the table path today (tsvCell intercepts both container types before
// they get here).
//
// Verified against knack's own float/int rendering (knack/output.py:84,
// class _TsvOutput at :216): [{"a":3.0,"b":"x"}] -> "3.0\tx\n",
// [{"a":3,"b":"x"}] -> "3\tx\n", [{"a":1.5}] -> "1.5\n", [{"a":1e-7}] ->
// "1e-07\n". The remaining tsv-specific divergences from knack are D2a
// (a top-level null with no --query renders nothing, not "None\n" — see
// renderTSV) and the --query numeric fidelity loss documented on
// PrintFormatted (integers above 2^53 and integral floats), tracked as
// azure-go-cli-c41. A multiselect-hash's column order is now recovered from
// the query in the common case — see tsvRow and keyOrder.
//
// NOTE on the float change vs HEAD: PrintFormatted now decodes JSON with
// json.Decoder.UseNumber() and routes numbers through formatJSONNumber
// instead of float64. A wire literal 3.0 now renders "3.0" where HEAD
// rendered "3" (HEAD collapsed integral floats via
// `if val == float64(int64(val))`). This is an intentional fidelity fix, not
// a regression: knack renders 3.0 as "3.0" and 3 as "3" (verified above), so
// HEAD's collapsing was itself the divergence from azure-cli.
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
		return formatNumber(val)
	case json.Number:
		return formatJSONNumber(val)
	default:
		// Nested arrays/objects: fall back to compact JSON.
		b, err := json.Marshal(val)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// formatNumber renders a Go float64 the way Python's str()/repr() renders a
// float. Python picks fixed-point notation when the value's decimal exponent
// (decpt, the position of the decimal point relative to the first significant
// digit) satisfies -4 < decpt <= 16, and switches to scientific notation
// otherwise; fixed-point integral values keep a trailing ".0" and scientific
// notation always carries a signed, at-least-2-digit exponent. Go's shortest
// round-tripping digit generation (strconv's 'e'/'f' verbs with prec -1)
// produces the same digits Python's dtoa-based repr does, so decpt is derived
// from Go's own scientific-notation exponent and reused to choose the same
// notation and digits Python would.
func formatNumber(f float64) string {
	if f == 0 {
		return strconv.FormatFloat(f, 'f', -1, 64) + ".0"
	}
	es := strconv.FormatFloat(f, 'e', -1, 64)
	eIdx := strings.IndexByte(es, 'e')
	expVal, err := strconv.Atoi(es[eIdx+1:])
	if err != nil {
		return es
	}
	decpt := expVal + 1
	if decpt <= -4 || decpt > 16 {
		return es
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// jsonNegativeZeroRe matches an integer-shaped literal that is negative zero
// (e.g. "-0"), which json.Marshal can emit for math.Copysign(0, -1).
var jsonNegativeZeroRe = regexp.MustCompile(`^-0+$`)

// formatJSONNumber renders a decoded json.Number the way azure-cli does. For
// an integer-shaped literal (no '.', 'e' or 'E') the wire literal is echoed
// verbatim, preserving arbitrary-precision integers that would lose
// precision through float64 — except "-0", which Python's json.loads/str()
// round-trips to plain "0". For anything else (any literal containing a
// decimal point or an exponent, including "3.0") it is parsed as a float64
// and rendered through formatNumber, matching Python's str(float) exactly
// (e.g. "1.0" stays "1.0", "1e-7" becomes "1e-07").
func formatJSONNumber(n json.Number) string {
	s := string(n)
	if !strings.ContainsAny(s, ".eE") {
		if jsonNegativeZeroRe.MatchString(s) {
			return "0"
		}
		return s
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	return formatNumber(f)
}

// tableSkipKeys mirrors knack's _TableOutput.SKIP_KEYS.
var tableSkipKeys = map[string]bool{"id": true, "type": true, "etag": true}

// renderTable renders a result the way azure-cli's generic table formatter
// does (knack's _TableOutput, tabulate "simple" format). azure-cli commands
// only get a bespoke table layout when they register a table_transformer;
// without one, knack derives columns from the data itself. For an object row:
// skip id/type/etag, drop values that are null or non-scalar, sort the
// remaining keys alphabetically, and uppercase the first character of each
// for the header. For a list-of-lists row (a JMESPath multiselect-list),
// columns become Column1..ColumnN with no filtering. Anything else becomes a
// single "Result" column. A non-list result renders as a single row.
//
// Column order is a first-seen union across rows (keys are sorted, or ordered
// per ko, *within* a row, but the row-to-row union is not re-sorted) — this
// matches knack, which builds each row from an OrderedDict and lets tabulate
// union them in the order it encounters them.
//
// A --query multiselect-hash's declared column order is recovered from the
// parsed query AST (see keyOrder) and used for any row whose key set exactly
// matches that hash — but only for a query that query.MultiSelectHashKeyOrders
// can PROVE safe: exactly one multiselect-hash anywhere in the query, reachable
// only through node types individually verified not to smuggle an unrelated
// map into the output next to the hash's own result (see that function's doc
// comment for the exact whitelist and why it must be one). Any other query
// shape — including a hash alongside `[a, b]`, `||` or `&&`, a hash passed to
// a function like `not_null`/`merge`/`sort_by`, or more than one hash — gets
// no recovered order at all, because those shapes can put a hash-produced
// map and an unrelated plain map with the same key set into the same result,
// and only the hash-produced one may be reordered. A row from a plain map (a
// raw dict passed through unchanged by the query, e.g. `[]` or `[0]`, or one
// the whitelist declined to reason about) therefore always sorts its keys
// even under an active query, where knack preserves the payload's original
// JSON key order — that gap is deliberately out of scope (azure-go-cli-c41
// tracks only the multiselect-hash case). Scripts that need a fixed column
// order regardless can also use a multiselect-list (`[].[name, location]`),
// which arrives as an ordered slice and renders as Column1..N in query
// order.
func renderTable(v interface{}, ko *keyOrder) string {
	// DELIBERATE, DOCUMENTED DIVERGENCE from knack: a nil result here renders
	// as "\n", the same as an empty list. knack does NOT do this for a
	// literal JSON null — it wraps a non-list result as [None], so
	// _auto_table_item's AttributeError branch fires and it renders a
	// one-row "Result" table with a blank cell ("Result\n--------\n\n"),
	// verified directly against knack 0.14.0's _TableOutput. But the common
	// case reaching this function with v == nil is an unpopulated `var xs
	// []T` that marshalled to JSON "null" — i.e. an empty *list*, not a
	// genuine null scalar — and knack prints "\n" (nothing) for an empty
	// list, matching the empty-columns branch a few lines below. We choose
	// the friendlier "\n" for that common case rather than knack's literal
	// null handling.
	if v == nil {
		return "\n"
	}

	var items []interface{}
	if list, ok := v.([]interface{}); ok {
		items = list
	} else {
		items = []interface{}{v}
	}

	rows := make([]tableRowData, len(items))
	for i, item := range items {
		rows[i] = tableEntry(item, ko)
	}

	// Column set: first-seen union of headers across rows.
	var columns []string
	seen := map[string]bool{}
	for _, row := range rows {
		for _, h := range row.headers {
			if !seen[h] {
				seen[h] = true
				columns = append(columns, h)
			}
		}
	}
	if len(columns) == 0 {
		return "\n"
	}

	// Cell matrix, indexed [row][column]; missing values are "".
	cells := make([][]string, len(rows))
	for i, row := range rows {
		byHeader := make(map[string]string, len(row.headers))
		for j, h := range row.headers {
			byHeader[h] = row.cells[j]
		}
		cells[i] = make([]string, len(columns))
		for j, h := range columns {
			cells[i][j] = byHeader[h]
		}
	}

	// Raw (pre-pyStrip) cell matrix, aligned the same way as cells, used
	// only for the isMultiline decision below.
	rawCells := make([][]string, len(rows))
	for i, row := range rows {
		byHeader := make(map[string]string, len(row.headers))
		for j, h := range row.headers {
			byHeader[h] = row.rawCells[j]
		}
		rawCells[i] = make([]string, len(columns))
		for j, h := range columns {
			rawCells[i][j] = byHeader[h]
		}
	}

	// tabulate decides multiline mode ONCE for the whole table, by searching
	// the flattened text of every RAW header and cell for \r or \n — BEFORE
	// any stripping happens (tabulate/__init__.py:2367-2388, which runs
	// ahead of the per-column strip in _align_column_choose_padfn at
	// 1189-1190). Only in multiline mode does it split cells with
	// splitlines() semantics, where an empty cell contributes zero lines (so
	// an all-empty row vanishes); otherwise every row — including an
	// all-empty one — emits exactly one line. See splitLines's doc comment.
	// Using the RAW text matters: a cell whose only \n sits at an edge (e.g.
	// "\nfoo") is stripped to "foo" by pyStrip, but tabulate still saw the
	// newline and entered multiline mode, so we must decide on the same raw
	// text tabulate did rather than on the already-stripped cells.
	isMultiline := false
	for _, h := range columns {
		if strings.ContainsAny(h, "\r\n") {
			isMultiline = true
			break
		}
	}
	if !isMultiline {
	outer:
		for _, row := range rawCells {
			for _, c := range row {
				if strings.ContainsAny(c, "\r\n") {
					isMultiline = true
					break outer
				}
			}
		}
	}

	// Split every cell into sub-lines (multi-line cells produce continuation
	// rows) and compute each row's height.
	lineRows := make([][][]string, len(cells))
	heights := make([]int, len(cells))
	for i, row := range cells {
		lineRows[i] = make([][]string, len(row))
		for j, c := range row {
			var lines []string
			if isMultiline {
				lines = splitLines(c)
			} else {
				lines = []string{c}
			}
			lineRows[i][j] = lines
			if len(lines) > heights[i] {
				heights[i] = len(lines)
			}
		}
	}

	// tabulate applies MIN_PADDING (2) as a floor on each header's width, so a
	// column is at least runeLen(header)+2 wide even when its data is
	// narrower. Width is measured in runes (code points), matching tabulate
	// without wcwidth installed.
	widths := make([]int, len(columns))
	for i, h := range columns {
		widths[i] = utf8.RuneCountInString(h) + 2
	}
	// Width is measured on a \r\n-only split, NOT on splitLines' output:
	// tabulate emits continuation lines with str.splitlines() but sizes
	// columns with re.split("[\r\n]", ...) in _multiline_width and
	// _align_column_multiline_width. A cell whose only break is an exotic
	// separator (\v, \f, \x1c-\x1e, \x85, U+2028, U+2029) therefore renders
	// as several lines while still being measured as one long line.
	for _, row := range cells {
		for j, c := range row {
			segs := []string{c}
			if isMultiline {
				segs = widthLines(c)
			}
			for _, s := range segs {
				if n := utf8.RuneCountInString(s); n > widths[j] {
					widths[j] = n
				}
			}
		}
	}

	var b strings.Builder
	// Headers containing a newline are written as-is rather than split into
	// continuation lines the way tabulate splits multi-line headers; out of
	// scope for this fix (very unlikely with real Azure keys, since a
	// header comes from a JSON object key or a --query alias).
	headers := make([]string, len(columns))
	copy(headers, columns)
	writeTableRow(&b, headers, widths)
	rules := make([]string, len(columns))
	for i, w := range widths {
		rules[i] = strings.Repeat("-", w)
	}
	writeTableRow(&b, rules, widths)
	for i, row := range lineRows {
		for line := 0; line < heights[i]; line++ {
			lineCells := make([]string, len(columns))
			for j, sublines := range row {
				if line < len(sublines) {
					lineCells[j] = sublines[line]
				}
			}
			writeTableRow(&b, lineCells, widths)
		}
	}
	return b.String()
}

// tableRowData is one row's ordered (header, cell) pairs, as knack's
// _auto_table_item would build it.
type tableRowData struct {
	headers []string
	cells   []string
	// rawCells holds each cell's value before pyStrip, matching what
	// tabulate joins into plain_text (headers + RAW row values) to decide
	// multiline mode, at tabulate/__init__.py:2367-2388 — before stripping
	// happens in _align_column_choose_padfn. See renderTable's isMultiline
	// computation.
	rawCells []string
}

// tableEntry converts a single item into a table row per knack's
// _auto_table_item. Cell values are whitespace-stripped (Python str.strip()
// semantics via pyStrip), matching tabulate's default cell alignment, which
// strips every data cell before measuring width and splitting it into lines.
func tableEntry(item interface{}, ko *keyOrder) tableRowData {
	switch val := item.(type) {
	case map[string]interface{}:
		keys := ko.keys(val)
		var row tableRowData
		for _, k := range keys {
			if tableSkipKeys[k] || !isTableScalar(val[k]) {
				continue
			}
			raw := tsvScalar(val[k])
			row.headers = append(row.headers, capitalizeFirst(k))
			row.cells = append(row.cells, pyStrip(raw))
			row.rawCells = append(row.rawCells, raw)
		}
		return row
	case []interface{}:
		row := tableRowData{
			headers:  make([]string, len(val)),
			cells:    make([]string, len(val)),
			rawCells: make([]string, len(val)),
		}
		for i, el := range val {
			raw := tableScalar(el)
			row.headers[i] = fmt.Sprintf("Column%d", i+1)
			row.cells[i] = pyStrip(raw)
			row.rawCells[i] = raw
		}
		return row
	default:
		raw := tableScalar(val)
		return tableRowData{headers: []string{"Result"}, cells: []string{pyStrip(raw)}, rawCells: []string{raw}}
	}
}

// pyStrip trims whitespace the way Python's str.strip() does: unicode.IsSpace
// plus the ASCII separator controls U+001C-U+001F (FS/GS/RS/US), which
// Python's str.strip() also strips but Go's unicode.IsSpace does not.
func pyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
	})
}

// tableScalar renders a leaf or container value for a table cell. It behaves
// like tsvScalar for scalars (knack's _auto_table_item drops non-scalar
// values from *dict* rows before this is ever reached, via isTableScalar
// above, but a *list* row, i.e. a JMESPath multiselect-list, assigns every
// element unfiltered), and for a nested list/dict it renders Python's repr()
// instead of tsvScalar's compact-JSON fallback, matching what tabulate
// actually stringifies knack's containers with.
func tableScalar(v interface{}) string {
	switch v.(type) {
	case map[string]interface{}, []interface{}:
		return pyRepr(v)
	default:
		return tsvScalar(v)
	}
}

// pyRepr renders a value the way Python's repr() would, for the container
// shapes that reach it as elements of a table's Column1..N cells: None/True
// /False, single-quoted strings, numbers via formatNumber/formatJSONNumber,
// and lists/dicts joined recursively with ", " and ": " separators. Nested
// map keys are sorted for determinism — Go's map has no ordering to recover,
// where Python's dict preserves JSON literal order. That is the one
// remaining divergence from knack here: a 1200-payload differential fuzz
// against knack 0.14.0 matched 1143, and 49 of the 57 mismatches were this
// key order alone (the other 8 were the documented literal-null case).
// azure-go-cli-c41 recovers --query multiselect-hash column order for
// renderTSV/renderTable (see keyOrder), but deliberately not here: a hash
// nested inside a table cell would need Python's OrderedDict repr to match,
// and that repr is itself Python-version dependent (verified to differ
// between CPython 3.9 and 3.14) — not byte-identical to any single knack
// build regardless of what this function does.
func pyRepr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return pyReprString(t)
	case float64:
		return formatNumber(t)
	case json.Number:
		return formatJSONNumber(t)
	case []interface{}:
		parts := make([]string, len(t))
		for i, el := range t {
			parts[i] = pyRepr(el)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = pyReprString(k) + ": " + pyRepr(t[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", t)
	}
}

// pyReprString renders a string the way Python's repr() would: single-quoted
// unless the string contains a single quote and no double quote (in which
// case it switches to double quotes, leaving the single quotes unescaped);
// backslashes, the chosen quote character, and \n/\r/\t are backslash-escaped;
// other ASCII control characters (and \x7f) become \xXX; and non-printable
// runes above the ASCII range are escaped as \xXX/\uXXXX/\UXXXXXXXX (CPython's
// own tiering by code point width), while ordinary printable Unicode
// (accented letters, CJK, emoji, ...) is emitted literally.
func pyReprString(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteByte(quote)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r > 0x7f && !unicode.IsPrint(r):
			switch {
			case r <= 0xff:
				fmt.Fprintf(&b, `\x%02x`, r)
			case r <= 0xffff:
				fmt.Fprintf(&b, `\u%04x`, r)
			default:
				fmt.Fprintf(&b, `\U%08x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// splitLines splits a cell's value on line breaks, matching Python's
// str.splitlines(), which breaks on \r\n, \n, \r, \v, \f, \x1c, \x1d, \x1e,
// \x85 (NEL), U+2028 (LINE SEPARATOR), and U+2029 (PARAGRAPH SEPARATOR) —
// see tabulate/__init__.py:1249, :1262, :2605, which relies on this full
// set. tabulate uses the full set only for EMITTING continuation lines; it
// measures column width with re.split("[\r\n]", ...) in _multiline_width and
// _align_column_multiline_width, so renderTable must not size columns from
// this function's output — see widthLines.
// It is only called when renderTable has determined the table is in
// multiline mode (some cell or header contains \r or \n) — mirroring
// tabulate, which decides multiline mode once for the WHOLE table (on \r/\n
// only) and only then splits cells with splitlines() semantics. Python's
// "".splitlines() returns [] rather than [""], so an empty string here
// returns nil, and an all-empty row in a multiline table is correctly
// omitted. Outside multiline mode, renderTable does not call this function
// at all — every row gets exactly one line regardless of whether its cells
// are empty, matching tabulate's non-multiline row emission.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.NewReplacer(
		"\r", "\n",
		"\v", "\n",
		"\f", "\n",
		"\x1c", "\n",
		"\x1d", "\n",
		"\x1e", "\n",
		"\u0085", "\n",
		"\u2028", "\n",
		"\u2029", "\n",
	).Replace(s)
	return strings.Split(s, "\n")
}

// widthLines splits a cell the way tabulate MEASURES it, on \r and \n only,
// mirroring re.split("[\r\n]", ...) in _multiline_width and
// _align_column_multiline_width. Splitting on each of \r and \n separately
// means "\r\n" yields an empty segment between them, exactly as re.split
// does; that is harmless here because only the widest segment is used.
func widthLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r", "\n"), "\n")
}

// writeTableRow joins cells with two spaces, padding all but the last
// (measured in runes). Trailing whitespace is trimmed, matching tabulate's
// _build_simple_row, which rstrips the joined line with Python's str.rstrip()
// — hence pyStrip's predicate rather than unicode.IsSpace alone, so the
// ASCII separator controls U+001C-U+001F are trimmed here too. Data cells
// are already pyStrip'd, but headers are not, so a JSON object key ending in
// one of those controls reaches this function untrimmed.
func writeTableRow(b *strings.Builder, cells []string, widths []int) {
	line := make([]string, len(cells))
	for i, c := range cells {
		if i == len(cells)-1 {
			line[i] = c
			continue
		}
		line[i] = c + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c))
	}
	b.WriteString(strings.TrimRightFunc(strings.Join(line, "  "), func(r rune) bool {
		return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
	}))
	b.WriteByte('\n')
}

// isTableScalar reports whether knack would keep this value as a column. It
// drops nulls and anything non-scalar.
func isTableScalar(v interface{}) bool {
	switch v.(type) {
	case nil:
		return false
	case map[string]interface{}, []interface{}:
		return false
	}
	return true
}

// capitalizeFirst uppercases a header's first rune, approximating knack's
// x[0].upper() + x[1:]. KNOWN DIVERGENCE: knack runs on Python's FULL
// Unicode case mapping, where a single rune can expand to several (e.g.
// "ß".upper() == "SS"); unicode.ToUpper here is Go's SIMPLE case mapping,
// which is always one rune in, one rune out, so such runes are left
// unchanged and the resulting header (and the column width derived from it)
// can differ from knack's. Not worth a special-case table for the rare
// characters affected.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// renderYAML approximates knack's format_yaml: yaml.safe_dump(result,
// default_flow_style=False, allow_unicode=True) with PyYAML's default
// sort_keys=True. It builds an explicit *yaml.Node tree (via valueToYAMLNode)
// rather than handing yaml.v3 a Go map/slice to marshal through reflection,
// so every scalar's quoting style is controlled directly instead of left to
// yaml.v3's block-scalar heuristics — see stringYAMLNode and needsYAMLQuote
// for why that matters. It is not byte-identical to PyYAML, but the
// remaining divergences below are cosmetic only — quoting is chosen so that
// PyYAML's safe_load (what every `az ... -o yaml` consumer actually decodes
// with) recovers the same data, not just yaml.v3's own decoder:
//   - yaml.v3 double-quotes a resolver-ambiguous scalar (one that would
//     otherwise parse as a different type, e.g. "true", "007", "1.5", or
//     "") where PyYAML single-quotes it instead; both decode identically.
//     (C0 control characters are NOT part of this divergence: both
//     libraries double-quote a tab as "a\tb" and U+0001 as "a\x01b".
//     U+0085 (NEL) IS: yaml.v3 emits "a\Nb" where PyYAML single-quotes and
//     folds it as a line break. U+2028 is not — both single-quote and fold
//     it identically. All three still safe_load to the same string.);
//   - this package sorts map keys explicitly via sort.Strings, matching
//     PyYAML's sort_keys=True default (yaml.v3's own map-marshalling
//     default is a digit-aware "natural" sort, which building the node
//     ourselves bypasses);
//   - yaml.v3 indents nested block sequences one level deeper than their
//     parent container, where PyYAML's default emits "indentless"
//     sequences flush with the parent key;
//   - yaml.v3 escapes non-BMP runes (e.g. emoji) where PyYAML's
//     allow_unicode=True emits them raw;
//   - PyYAML appends a "...\n" document-end marker after a top-level plain
//     scalar that yaml.v3 does not emit;
//   - a string containing a newline is always rendered here as a
//     double-quoted scalar; PyYAML represents the same string with a
//     single-quoted "blank continuation line" style. Both are lossless —
//     yaml.v3's own literal/folded block-scalar styles are not (see
//     stringYAMLNode), so this package never selects them.
//
// All of these are semantically-equivalent YAML (same data on decode under
// both yaml.v3 and PyYAML) rather than byte-identical output; correctness of
// the round-trip is preferred over cosmetic parity with PyYAML.
func renderYAML(v interface{}) (string, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(valueToYAMLNode(v)); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return b.String(), nil
}

// valueToYAMLNode converts a generic value, as PrintFormatted produces it
// (map[string]interface{}, []interface{}, string, bool, nil, float64 for a
// jmespath-computed number, or json.Number for a value decoded straight off
// the wire), into an explicit *yaml.Node tree for the encoder.
func valueToYAMLNode(v interface{}) *yaml.Node {
	switch t := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	case bool:
		val := "false"
		if t {
			val = "true"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: val}
	case string:
		return stringYAMLNode(t)
	case map[string]interface{}:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		// Plain lexicographic byte order, matching PyYAML's sort_keys=True
		// default (yaml.v3's own map-marshalling default is a digit-aware
		// "natural" sort, which this bypasses by building the node
		// ourselves).
		sort.Strings(keys)
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, k := range keys {
			n.Content = append(n.Content, stringYAMLNode(k), valueToYAMLNode(t[k]))
		}
		return n
	case []interface{}:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, e := range t {
			n.Content = append(n.Content, valueToYAMLNode(e))
		}
		return n
	case float64:
		// Only reached for a value a jmespath query computed (e.g. avg());
		// json.Number below is the path for a value decoded straight off
		// the wire (see PrintFormatted's UseNumber comment). Integral
		// values print as a plain integer rather than e.g. 1.073741824e+09.
		if t == math.Trunc(t) && math.Abs(t) <= 1<<62 {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(int64(t), 10)}
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: yamlFloatLiteral(formatNumber(t))}
	case json.Number:
		s := string(t)
		if !strings.ContainsAny(s, ".eE") {
			// Integer-shaped literal: emit verbatim so arbitrary-precision
			// integers render exactly as received instead of round-tripping
			// through float64/int64.
			return intYAMLNode(s)
		}
		// Anything with a '.' or exponent is rendered through formatNumber
		// and then, if that produced an exponent form whose mantissa has no
		// '.' (e.g. "1e-07"), yamlFloatLiteral injects one (-> "1.0e-07").
		// Without that, PyYAML's YAML-1.1 resolver would read the scalar
		// back as a string, not a float; knack itself emits "1.0e-07" for
		// this value.
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return stringYAMLNode(s)
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: yamlFloatLiteral(formatNumber(f))}
	default:
		return stringYAMLNode(fmt.Sprintf("%v", t))
	}
}

// yamlBoolWordRe matches PyYAML's YAML-1.1 boolean resolver word list
// (yaml.resolver.Resolver's bool pattern). Notably "y"/"n"/"Y"/"N" are NOT
// in PyYAML's own list and stay bare under both encoders.
var yamlBoolWordRe = regexp.MustCompile(`^(?:yes|Yes|YES|no|No|NO|true|True|TRUE|false|False|FALSE|on|On|ON|off|Off|OFF)$`)

// yamlSexagesimalIntRe and yamlSexagesimalFloatRe match PyYAML's YAML-1.1 int
// and float resolver patterns for base-60 (colon-separated) literals, e.g.
// "22:00" or "1:30:00.5". A leading zero disqualifies the int form (PyYAML's
// pattern requires [1-9] first), which is why plain zero-padded times like
// "01:00" are unaffected.
var (
	yamlSexagesimalIntRe   = regexp.MustCompile(`^[-+]?[1-9][0-9_]*(?::[0-5]?[0-9])+$`)
	yamlSexagesimalFloatRe = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+\.[0-9_]*$`)
)

// yamlTimestampRe matches PyYAML's YAML-1.1 timestamp resolver pattern
// (yaml.resolver.Resolver's timestamp regex): either a plain "YYYY-MM-DD"
// date, or a date+time joined by "T"/"t" or plain whitespace, with a
// mandatory HH:MM:SS, an optional fractional second, and an optional "Z" or
// "+HH[:MM]"/"-HH[:MM]" zone offset. yaml.v3's own YAML-1.2 resolver only
// recognizes a narrower set of these (e.g. it wants "T" and a zone), so a
// "T"-joined timestamp with no zone, or a space-joined one WITH a zone, is
// left bare by yaml.v3 and PyYAML's safe_load recovers a datetime.datetime
// instead of the original string — this closes that gap.
var yamlTimestampRe = regexp.MustCompile(
	`^(?:[0-9]{4}-[0-9]{2}-[0-9]{2}` +
		`|[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}(?:[Tt]|[ \t]+)[0-9]{1,2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]*)?(?:[ \t]*(?:Z|[-+][0-9]{1,2}(?::[0-9]{2})?))?)$`)

// needsYAMLQuote reports whether PyYAML's YAML-1.1-aware implicit resolver
// would read this string back as something other than tag:yaml.org,2002:str
// if it were emitted bare — i.e. whether a lossless round-trip through
// PyYAML (what every `az ... -o yaml` consumer actually decodes with)
// requires quoting it. yaml.v3's own YAML-1.2 resolver already forces
// quoting for its own set of ambiguous scalars (true/false/null/1/1.5/007/
// ""); this closes the gap for the YAML-1.1 forms PyYAML additionally
// resolves: the "<<" merge key and "=" value key tags, the yes/no/on/off
// boolean words, colon-separated sexagesimal ints/floats (e.g. "22:00"), and
// timestamp-shaped strings PyYAML's resolver recognizes but yaml.v3's does
// not (e.g. "2024-01-15T10:30:00" with no zone).
func needsYAMLQuote(s string) bool {
	if s == "<<" || s == "=" {
		return true
	}
	if yamlBoolWordRe.MatchString(s) {
		return true
	}
	if yamlSexagesimalIntRe.MatchString(s) || yamlSexagesimalFloatRe.MatchString(s) {
		return true
	}
	if yamlTimestampRe.MatchString(s) {
		return true
	}
	return false
}

// stringYAMLNode builds a scalar string node. Tagging it "!!str" is what
// makes yaml.v3's encoder quote the value whenever leaving it bare would
// change its resolved type on decode under yaml.v3's own YAML-1.2 resolver
// (e.g. "true", "1.5", "007", ""); an ordinary string is left with Style 0
// so the encoder still picks its usual bare/quoted form for it. needsYAMLQuote
// forces quoting for the further set of scalars PyYAML's YAML-1.1 resolver
// treats specially that yaml.v3 would otherwise leave bare (see its doc
// comment).
//
// A string containing a newline, or one needsYAMLQuote flags, is always
// forced to DoubleQuotedStyle. Left alone, yaml.v3 selects a literal block
// scalar for a string with embedded newlines, and block scalars cannot
// represent a leading newline: yaml.v3 renders "\nb" as a block scalar that
// its own decoder reads back as "b", silently dropping data — and a string
// of only newlines becomes an EMPTY block scalar, corrupting a map key
// entirely (an empty key decodes as ""). Double-quoting sidesteps
// block-scalar selection altogether and is always lossless.
func stringYAMLNode(s string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	if strings.Contains(s, "\n") || needsYAMLQuote(s) {
		n.Style = yaml.DoubleQuotedStyle
	}
	return n
}

// intYAMLNode builds a scalar node for an integer-shaped JSON literal. The
// value is always emitted verbatim (digits only, so no quoting question
// arises); the only choice is whether to carry an explicit "!!int" tag.
//
// yaml.v3's encoder writes the tag out as a literal "!!int " prefix whenever
// its own resolver would NOT read the plain scalar back as an integer — which
// is any literal outside [math.MinInt64, math.MaxUint64], since that resolver
// tries ParseInt then ParseUint and falls through to float. Its decoder then
// refuses the document it just wrote ("cannot decode !!float `N` as a !!int").
// PyYAML has no such limit: Python ints are arbitrary precision, and knack
// emits (and safe_loads) such a value as a bare integer scalar.
//
// So for a literal yaml.v3 cannot resolve as an int, the tag is dropped and
// the digits are emitted bare — byte-identical to knack's own output. PyYAML
// then recovers the exact integer, and yaml.v3 recovers the nearest float64
// (lossy, but exactly what it would do with real azure-cli YAML) instead of
// failing outright.
func intYAMLNode(s string) *yaml.Node {
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: s}
	}
	if _, err := strconv.ParseUint(s, 10, 64); err == nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: s}
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Value: s}
}

// yamlFloatLiteral adjusts a Python-str(float)-style number literal (as
// produced by formatNumber) so it round-trips as a float under PyYAML's
// YAML-1.1 resolver: an exponent form with no '.' in the mantissa (e.g.
// "1e-07") gets one injected before the "e" (-> "1.0e-07"), matching
// PyYAML's own float representer. Fixed-point literals already carry a '.'
// (formatNumber always appends ".0" to integral fixed-point values) and are
// returned unchanged.
func yamlFloatLiteral(s string) string {
	i := strings.IndexByte(s, 'e')
	if i < 0 || strings.Contains(s[:i], ".") {
		return s
	}
	return s[:i] + ".0" + s[i:]
}
