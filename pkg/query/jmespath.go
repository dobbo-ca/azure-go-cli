package query

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/jmespath/go-jmespath"
)

// ApplyJMESPath applies a JMESPath query to JSON data
func ApplyJMESPath(data interface{}, queryStr string) (interface{}, error) {
	if queryStr == "" {
		return data, nil
	}

	result, err := jmespath.Search(queryStr, data)
	if err != nil {
		return nil, fmt.Errorf("invalid JMESPath query: %w", err)
	}

	return result, nil
}

// ApplyJMESPathToJSON applies a JMESPath query to JSON bytes
func ApplyJMESPathToJSON(jsonData []byte, queryStr string) ([]byte, error) {
	if queryStr == "" {
		return jsonData, nil
	}

	// Parse JSON into interface{}
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Apply query
	result, err := ApplyJMESPath(data, queryStr)
	if err != nil {
		return nil, err
	}

	// Marshal result back to JSON
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return output, nil
}

// astCalibration holds the go-jmespath AST node-type integers this package
// introspects for. They are read via reflect rather than hardcoded, because
// jmespath.ASTNode's fields (nodeType, value, children) are unexported and
// astNodeType.String() cannot be called through reflect on a read-only
// field.
var (
	astOnce   sync.Once
	astUsable bool

	// multiSelectHashType/keyValPairType/comparatorType/filterProjectionType/
	// literalType get individual handling in collectHashKeys (see there for
	// why each is special-cased rather than a plain whitelist entry).
	multiSelectHashType  int64
	keyValPairType       int64
	comparatorType       int64
	filterProjectionType int64
	literalType          int64

	// pipeType/subexpressionType are the two "left then right" node types.
	// They are NOT generic flow types: their left child is only in output
	// position when the right child selects hash-built ELEMENTS (an index,
	// a slice, a projection) rather than reaching INSIDE a value. See
	// rhsSelectsIntoValue.
	pipeType          int64
	subexpressionType int64

	// projectionType/valueProjectionType are (source, applied-to-each-element)
	// pairs and get the same treatment as pipe/subexpression: the source is
	// only in output position when what is applied to each element does not
	// reach inside a value. valueProjection ("*") always reaches inside, so
	// its source is never in output position.
	projectionType int64

	// fieldType/valueProjectionType are the two node types that select a
	// value out of a map, i.e. arbitrary payload data rather than something
	// the hash itself built.
	fieldType           int64
	valueProjectionType int64

	// genericFlowTypes are node types that (a) can never themselves evaluate
	// to a map other than one forwarded unchanged from a child, and (b) pass
	// the "is this in output position" question straight through to every
	// child unchanged. Walking into any child of one of these with the same
	// collect-mode as the parent is therefore safe. See collectHashKeys.
	genericFlowTypes map[int64]bool
)

// calibrateAST determines the current go-jmespath nodeType values this
// package relies on by parsing known expressions and reading the node types
// they produce. If the AST no longer has the shape this function expects
// (fields renamed/removed, unexpected kinds), it leaves astUsable false and
// MultiSelectHashKeyOrders degrades to returning nil for every query.
func calibrateAST() {
	astOnce.Do(func() {
		types := map[string]int64{}
		get := func(name, expr string, path ...int) bool {
			node, err := jmespath.NewParser().Parse(expr)
			if err != nil {
				return false
			}
			v := reflect.ValueOf(node)
			for _, idx := range path {
				children := v.FieldByName("children")
				if !children.IsValid() || children.Kind() != reflect.Slice || idx >= children.Len() {
					return false
				}
				v = children.Index(idx)
			}
			t, ok := astNodeTypeInt(v)
			if !ok {
				return false
			}
			types[name] = t
			return true
		}

		// ASTMultiSelectHash (root) / ASTKeyValPair (its child).
		ok := get("hash", "{a:b}")
		ok = ok && get("keyval", "{a:b}", 0)
		// ASTField (root).
		ok = ok && get("field", "a")
		// ASTCurrentNode (root).
		ok = ok && get("current", "@")
		// ASTIdentity: left child of the ASTValueProjection produced by "*".
		ok = ok && get("identity", "*", 0)
		// ASTIndexExpression (root of "a[0]") / ASTIndex (its second child).
		ok = ok && get("indexExpr", "a[0]")
		ok = ok && get("index", "a[0]", 1)
		// ASTProjection (root of "a[0:1]") / ASTSlice (nested under its
		// ASTIndexExpression child).
		ok = ok && get("projection", "a[0:1]")
		ok = ok && get("slice", "a[0:1]", 0, 1)
		// ASTFilterProjection (root of "a[?b]").
		ok = ok && get("filterProjection", "a[?b]")
		// ASTComparator (root of "a==b").
		ok = ok && get("comparator", "a==b")
		// ASTPipe (root of "a|b").
		ok = ok && get("pipe", "a|b")
		// ASTSubexpression (root of "a.b").
		ok = ok && get("subexpr", "a.b")
		// ASTFlatten: first child of the ASTProjection produced by "a[]".
		ok = ok && get("flatten", "a[]", 0)
		// ASTValueProjection (root of "*").
		ok = ok && get("valueProjection", "*")
		// ASTLiteral (root of a backtick JSON literal).
		ok = ok && get("literal", "`1`")
		if !ok {
			return
		}

		multiSelectHashType = types["hash"]
		keyValPairType = types["keyval"]
		comparatorType = types["comparator"]
		filterProjectionType = types["filterProjection"]
		literalType = types["literal"]
		pipeType = types["pipe"]
		subexpressionType = types["subexpr"]
		projectionType = types["projection"]
		fieldType = types["field"]
		valueProjectionType = types["valueProjection"]
		genericFlowTypes = map[int64]bool{
			types["field"]:     true,
			types["current"]:   true,
			types["identity"]:  true,
			types["indexExpr"]: true,
			types["index"]:     true,
			types["slice"]:     true,
			types["flatten"]:   true,
		}
		astUsable = true
	})
}

// astNodeTypeInt reads the unexported nodeType field of a reflect.Value
// wrapping a jmespath.ASTNode, returning false if the field is missing or
// not an integer kind.
func astNodeTypeInt(node reflect.Value) (int64, bool) {
	if !node.IsValid() || node.Kind() != reflect.Struct {
		return 0, false
	}
	nt := node.FieldByName("nodeType")
	if !nt.IsValid() {
		return 0, false
	}
	switch nt.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return nt.Int(), true
	default:
		return 0, false
	}
}

// MultiSelectHashKeyOrders returns the declared key order of the query's
// multiselect-hash ({...}), if and only if the query is provably safe to
// recover an order for. It returns nil for: an empty or unparseable query; a
// query with no multiselect-hash; a query with more than one
// multiselect-hash anywhere in its tree (nested included); a query the
// go-jmespath AST no longer has the shape this function introspects for; and
// — the bulk of the guarantee — a query containing any AST node type this
// function has not individually verified safe, in any position, or a
// multiselect-hash reachable only through a position this function has
// verified UNSAFE (see below). On success it returns a single-element
// [][]string (kept as a slice of slices for compatibility with callers that
// pre-date the one-hash-only rule).
//
// # Why this must be a whitelist
//
// The recovered order is applied downstream by matching a *result* map's key
// SET against the hash's declared key list (pkg/output.keyOrder), with no
// static way to tell a hash-constructed map from an unrelated map the query
// merely passed through that happens to share the same keys. Getting this
// wrong silently reorders the wrong map's columns — a real, previously
// shipped bug (azure-go-cli-c41): `not_null(p, one.{b:x, a:y})` leaked the
// hash's [b,a] order onto `p`, an unrelated pass-through map with the same
// {a,b} key set. A blacklist (reject known-bad shapes, allow everything
// else) can never close this class of bug, because the unsafe shape is "some
// node this function has not reasoned about", which by definition keeps
// growing. So instead: every node type below is individually permitted only
// after checking it cannot produce a map other than one the hash itself
// constructs (or forwards unchanged). Anything else is refused, including
// every ASTFunctionExpression (`not_null`, `merge`, `sort_by`, `to_array`,
// ...) — this deliberately gives up recovering order for a query like
// `sort_by(@,&n)[].{...}`, which is structurally recoverable but not proven
// safe without reasoning about every function's semantics individually.
// Correctness over coverage: failing safe costs nothing but a sorted column
// order, which is exactly pre-c41 behaviour.
//
// # Permitted node types and why each is safe
//
//   - ASTField, ASTCurrentNode, ASTIdentity, ASTIndex, ASTIndexExpression,
//     ASTSlice: pure navigation into existing data (field/index/slice
//     access). None of these can construct a new map; they only select an
//     existing value or produce a scalar/nil.
//   - ASTFlatten (`[]`): maps a child expression over a list; the shape of
//     any resulting map comes entirely from that child expression, which is
//     walked with the same scrutiny.
//   - ASTPipe, ASTSubexpression, ASTProjection (`|`, `.`, `[*]`): a
//     (left, right) pair where the right-hand child is evaluated against the
//     left-hand child's result — see the output-position rule below.
//   - ASTValueProjection (`.*`): extracts the VALUES of its left child, so
//     that child's own shape never survives; its left child is always walked
//     in non-collecting mode.
//   - ASTLiteral: permitted ONLY when its decoded value is a scalar (string,
//     number, bool, or null) — never an object or array. A backtick JSON
//     literal can embed an arbitrary object (“ `{"b":1,"a":2}` “), which
//     would be exactly the same "map with a spuriously borrowed order" bug
//     the whitelist exists to prevent; a scalar literal cannot.
//   - ASTMultiSelectHash, ASTKeyValPair: the hash construct itself and its
//     key/value children — this is the thing being measured.
//   - ASTComparator (`==`, `!=`, `<`, ...) and the condition child of
//     ASTFilterProjection (`[?...]`): permitted as node types, but their
//     operands/condition are walked in "non-collecting" mode (see below) —
//     a comparator's result is always a boolean, never the map it compared,
//     and a filter condition's result is always discarded after filtering.
//     A multiselect-hash reachable ONLY through one of these positions is
//     therefore never the query's actual output, so its order is never
//     collected; if the walk finds one there anyway, the whole query is
//     rejected (nil) rather than silently ignoring it, since a hash placed
//     there is unusual enough that erring conservative costs nothing.
//
// Every other node type — ASTFunctionExpression, ASTOrExpression,
// ASTAndExpression, ASTNotExpression, ASTMultiSelectList, ASTExpRef,
// ASTEmpty, and any type this function has not been taught about — is
// refused unconditionally, anywhere in the tree.
//
// # The output-position rule
//
// For the (left, right) node types, the left child is only in output
// position when the right child selects something the left child BUILT — an
// element, by index, slice or sub-projection. The moment the right child
// reaches INSIDE a value (any ASTField or ASTValueProjection anywhere in its
// subtree — see rhsSelectsIntoValue) what comes out is arbitrary payload
// data, so a hash on the left is non-terminal and is walked in
// non-collecting mode.
//
// That distinction is what keeps `objs[].{b:x,a:y} | [0]` working (the right
// child indexes the hash's own results, so the declared order still applies,
// matching knack) while rejecting `{b:p,a:one}.b`, `objs[].{b:p,a:x}[].b`
// and `{b:p,a:one}.*`, where the result is a map the query merely forwarded
// out of the payload. Getting that wrong is the azure-go-cli-c41 bug itself:
// stamping a hash's declared column order onto an unrelated map whose key
// set happens to match. Verified against knack 0.14.0 + jmespath 1.1.0 over
// the leak shapes above; see TestMultiSelectHashKeyOrders and
// TestHashOrderNotBorrowedByPassthroughMap.
func MultiSelectHashKeyOrders(queryStr string) [][]string {
	if queryStr == "" {
		return nil
	}
	calibrateAST()
	if !astUsable {
		return nil
	}
	node, err := jmespath.NewParser().Parse(queryStr)
	if err != nil {
		return nil
	}
	var out [][]string
	hashCount := 0
	unsafe := false
	collectHashKeys(reflect.ValueOf(node), true, &out, &hashCount, &unsafe)
	if unsafe || hashCount != 1 || len(out) == 0 {
		return nil
	}
	return out
}

// collectHashKeys recursively walks the AST rooted at node. collect reports
// whether node is in a position whose evaluated value can become (part of)
// the query's final output ("flow position"); it starts true at the root and
// is forced false for an ASTComparator's operands and an ASTFilterProjection
// condition child (see MultiSelectHashKeyOrders), since neither ever
// contributes to the rendered result.
//
// Every multiselect-hash found anywhere increments *hashCount, so the
// "exactly one hash total" rule in MultiSelectHashKeyOrders also rejects a
// hash that exists only in a non-flow position (its order is never
// collected there, but its mere presence still makes the query one this
// function declines to reason further about). A hash found while
// collect is false additionally sets *unsafe directly, short-circuiting to
// nil even in the (impossible today, since hashCount>1 already nils out)
// case where it was the query's only hash.
//
// Any node type not individually recognised sets *unsafe. Every reflect
// access is guarded so a hostile or unexpected AST shape is a no-op rather
// than a panic. A malformed hash (an unexpected child kind) is simply
// skipped, not treated as a reason to abandon the rest of the walk.
func collectHashKeys(node reflect.Value, collect bool, out *[][]string, hashCount *int, unsafe *bool) {
	nodeType, ok := astNodeTypeInt(node)
	if !ok {
		return
	}

	if nodeType == literalType {
		val := node.FieldByName("value")
		if val.IsValid() && val.Kind() == reflect.Interface && val.Elem().IsValid() {
			switch val.Elem().Kind() {
			case reflect.Map, reflect.Slice:
				*unsafe = true
			}
		}
		return
	}

	if nodeType == multiSelectHashType {
		*hashCount++
		if !collect {
			*unsafe = true
		}
		children := node.FieldByName("children")
		if children.IsValid() && children.Kind() == reflect.Slice {
			keys := make([]string, 0, children.Len())
			malformed := false
			for i := 0; i < children.Len(); i++ {
				kv := children.Index(i)
				kvType, ok := astNodeTypeInt(kv)
				if !ok || kvType != keyValPairType {
					malformed = true
					break
				}
				val := kv.FieldByName("value")
				if !val.IsValid() || val.Kind() != reflect.Interface || val.Elem().Kind() != reflect.String {
					malformed = true
					break
				}
				keys = append(keys, val.Elem().String())
			}
			if !malformed && collect {
				*out = append(*out, keys)
			}
			for i := 0; i < children.Len(); i++ {
				collectHashKeys(children.Index(i), collect, out, hashCount, unsafe)
			}
		}
		return
	}

	if nodeType == keyValPairType {
		children := node.FieldByName("children")
		if children.IsValid() && children.Kind() == reflect.Slice {
			for i := 0; i < children.Len(); i++ {
				collectHashKeys(children.Index(i), collect, out, hashCount, unsafe)
			}
		}
		return
	}

	if nodeType == comparatorType {
		children := node.FieldByName("children")
		if children.IsValid() && children.Kind() == reflect.Slice {
			for i := 0; i < children.Len(); i++ {
				collectHashKeys(children.Index(i), false, out, hashCount, unsafe)
			}
		}
		return
	}

	if nodeType == filterProjectionType {
		children := node.FieldByName("children")
		if !children.IsValid() || children.Kind() != reflect.Slice || children.Len() != 3 {
			*unsafe = true
			return
		}
		collectHashKeys(children.Index(0), collect, out, hashCount, unsafe) // source
		collectHashKeys(children.Index(1), collect, out, hashCount, unsafe) // projection RHS
		collectHashKeys(children.Index(2), false, out, hashCount, unsafe)   // condition: discarded
		return
	}

	if nodeType == valueProjectionType {
		// "X.*" extracts the VALUES of X, so whatever X built is never what
		// comes out. A hash on the left is therefore always non-terminal.
		children := node.FieldByName("children")
		if !children.IsValid() || children.Kind() != reflect.Slice || children.Len() != 2 {
			*unsafe = true
			return
		}
		collectHashKeys(children.Index(0), false, out, hashCount, unsafe)
		collectHashKeys(children.Index(1), collect, out, hashCount, unsafe)
		return
	}

	if nodeType == pipeType || nodeType == subexpressionType || nodeType == projectionType {
		children := node.FieldByName("children")
		if !children.IsValid() || children.Kind() != reflect.Slice || children.Len() != 2 {
			*unsafe = true
			return
		}
		// The right child is always in the parent's output position. The left
		// child is only in output position when the right child selects
		// something the hash itself BUILT (an element, by index/slice/
		// projection). The moment the right child reaches INSIDE a value —
		// any field access or value projection — what comes out is arbitrary
		// payload data, so a hash on the left is non-terminal and its declared
		// key order must not be attributed to the result (azure-go-cli-c41).
		leftCollect := collect && !rhsSelectsIntoValue(children.Index(1))
		collectHashKeys(children.Index(0), leftCollect, out, hashCount, unsafe)
		collectHashKeys(children.Index(1), collect, out, hashCount, unsafe)
		return
	}

	if genericFlowTypes[nodeType] {
		children := node.FieldByName("children")
		if !children.IsValid() {
			return
		}
		if children.Kind() != reflect.Slice {
			*unsafe = true
			return
		}
		for i := 0; i < children.Len(); i++ {
			collectHashKeys(children.Index(i), collect, out, hashCount, unsafe)
		}
		return
	}

	*unsafe = true
}

// rhsSelectsIntoValue reports whether the right-hand side of a pipe or
// subexpression reaches inside a value anywhere — a field access (a.b) or a
// value projection (a.*). Both pull out arbitrary payload data, so a
// multiselect-hash to their left cannot be the query's output.
//
// It answers for the whole subtree, not just the spine, which errs toward
// "yes". That bias is deliberate: a false "yes" only demotes a hash to sorted
// column order (the behaviour before azure-go-cli-c41), while a false "no"
// stamps a hash's declared order onto an unrelated map.
func rhsSelectsIntoValue(node reflect.Value) bool {
	nodeType, ok := astNodeTypeInt(node)
	if !ok {
		return true
	}
	if nodeType == fieldType || nodeType == valueProjectionType {
		return true
	}
	children := node.FieldByName("children")
	if !children.IsValid() || children.Kind() != reflect.Slice {
		return false
	}
	for i := 0; i < children.Len(); i++ {
		if rhsSelectsIntoValue(children.Index(i)) {
			return true
		}
	}
	return false
}
