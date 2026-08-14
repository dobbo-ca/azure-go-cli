package query

import (
	"reflect"
	"testing"
)

// TestASTIntrospectionCalibration is a canary: it pins the reflect-based
// key-order recovery to go-jmespath's current AST shape. If go-jmespath is
// ever upgraded and ASTNode's fields (or the ordering guarantees of
// ASTMultiSelectHash's children) change shape, this test fails loudly rather
// than MultiSelectHashKeyOrders silently degrading to "always nil" in
// production.
func TestASTIntrospectionCalibration(t *testing.T) {
	got := MultiSelectHashKeyOrders("{b:x, a:y}")
	want := [][]string{{"b", "a"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MultiSelectHashKeyOrders(%q) = %v, want %v (go-jmespath AST shape may have changed)", "{b:x, a:y}", got, want)
	}
}

func TestMultiSelectHashKeyOrders(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  [][]string
	}{
		{"empty query", "", nil},
		{"simple projection hash", "[].{Name:name, Location:location}", [][]string{{"Name", "Location"}}},
		{"reversed order", "[].{Location:location, Name:name}", [][]string{{"Location", "Name"}}},
		{"index hash", "[0].{Name:name, Location:location}", [][]string{{"Name", "Location"}}},
		{"top-level hash", "{Name:name, Location:location}", [][]string{{"Name", "Location"}}},
		{"current-node hash", "@.{Name:name, Location:location}", [][]string{{"Name", "Location"}}},
		{"wildcard hash", "[*].{Name:name, Location:location}", [][]string{{"Name", "Location"}}},
		{"field-prefixed projection hash", "foo[].{Name:name, Location:location}", [][]string{{"Name", "Location"}}},
		{"dotted hash", "a.b.{Name:name, Location:location}", [][]string{{"Name", "Location"}}},
		{"filter projection hash", "[?name=='a'].{Name:name, Location:location}", [][]string{{"Name", "Location"}}},
		{"pipe then hash", "x | [].{Name:name, Location:location}", [][]string{{"Name", "Location"}}},
		{"hash then pipe", "[].{Name:name, Location:location} | [0]", [][]string{{"Name", "Location"}}},

		// azure-go-cli-c41: a hash in NON-terminal position must yield nil.
		// The query's result is then a map the payload supplied, not one the
		// hash built, so borrowing the hash's declared order would reorder
		// unrelated data whose key set merely matches. Each of these was
		// confirmed to leak before the output-position rule landed, and each
		// matches knack 0.14.0 once it returns nil (sorted fallback).
		{"hash then field", "{b: p, a: n}.b", nil},
		{"hash then piped field", "{b: p, a: n} | b", nil},
		{"projected hash then field", "objs[].{b: p, a: x}[].b", nil},
		{"projected hash then indexed field", "objs[].{b: p, a: x} | [0].b", nil},
		{"hash then value projection", "{b: p, a: n}.*", nil},
		{"hash then field then current", "{b: p, a: n}.b | @", nil},
		{"quoted alias", `[].{"My Col":name, l:location}`, [][]string{{"My Col", "l"}}},

		// azure-go-cli-c41 follow-up: the guard is now a whitelist, so any
		// AST node type it has not individually verified safe makes the
		// whole query nil, even when the shape "looks" recoverable.

		// A function call anywhere in the tree is unconditionally unsafe: no
		// individual function's semantics are reasoned about, so none is
		// trusted not to smuggle a foreign map into the output next to the
		// hash's own result. Accepted coverage regression vs. the old
		// blacklist, which allowed this.
		{"no hash: sort_by wrapping a hash (function expression, unsafe)", "sort_by(@,&n)[].{Name:name, Location:location}", nil},
		{"no hash: reverse/sort_by/slice wrapping a hash (function expression, unsafe)", "reverse(sort_by([],&n))[:2].{Name:name, Location:location}", nil},

		// More than one multiselect-hash anywhere in the tree — including
		// two independent top-level hashes joined by a pipe, and a hash
		// nested inside another hash's value — is unconditionally unsafe
		// per the "exactly one hash" rule, even though neither of these
		// particular shapes is actually ambiguous. Accepted coverage
		// regression: correctness over coverage.
		{"no hash: two hashes piped (more than one hash, unsafe)", "{a:name} | {b:a}", nil},
		{"no hash: hash nested inside another hash's value (more than one hash, unsafe)", "[].{Name:name, Nested:{a:x,b:y}}", nil},

		{"no hash: raw projection", "[]", nil},
		{"no hash: index", "[0]", nil},
		{"no hash: field", "[].name", nil},
		{"no hash: multiselect list", "[].[a,b]", nil},
		{"no hash: function", "length(@)", nil},
		{"unparseable query", "{{{", nil},

		// --- pinned safety-property regression tests (azure-go-cli-c41
		// follow-up gate: previously untested, so the guard could have been
		// deleted or inverted with nothing failing) ---

		{"unsafe: hash alongside ||", "[?f].{a:x,b:y} || [].tags", nil},
		{"unsafe: hash alongside &&", "[?f].{a:x,b:y} && [].tags", nil},
		{"unsafe: hash inside multiselect-list", "[foo, {b:x, a:y}]", nil},
		{"unsafe: hash as not_null argument", "not_null(p, one.{b:x, a:y})", nil},
		{"unsafe: hash as merge argument", "merge(p, one.{b:x, a:y})", nil},
		{"unsafe: hash as sort_by argument", "sort_by([{b:x, a:y}], &b)", nil},
		{"unsafe: two hashes with different key sets", "{a:x} | {b:y,c:z}", nil},
		{"unsafe: two hashes with same key set, different declared orders", "{a:x,b:y} | {b:p,a:q}", nil},
		{"unsafe: hash inside a filter condition", "a[?{x:val}==`1`]", nil},
		{"unsafe: hash inside a comparator", "{a:x,b:y} == `1`", nil},
		{"unsafe: object literal alongside a hash via pipe", "{a:x,b:y} | `{\"b\":1,\"a\":2}`", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MultiSelectHashKeyOrders(tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MultiSelectHashKeyOrders(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

// TestPreservesNumberLiterals pins the whitelist. A query answers true only
// when its whole AST is free of function expressions and comparators, which
// are the two places go-jmespath v0.4.0 type-asserts a number to float64
// (functions.go:353 / functions.go:405 and interpreter.go:48-55).
func TestPreservesNumberLiterals(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		// Safe: pure navigation / construction.
		{"field", "name", true},
		{"subexpression", "properties.provisioningState", true},
		{"index", "[0].id", true},
		{"slice", "[1:3]", true},
		{"flatten projection", "[].id", true},
		{"list projection", "[*].id", true},
		{"value projection", "a.*", true},
		{"multiselect hash", "[].{Name: name, Port: port}", true},
		{"multiselect list", "[].[name, port]", true},
		{"pipe", "[].{Name: name} | [0]", true},
		{"current node", "@", true},
		{"scalar literal", "`1`", true},
		{"truthiness filter", "[?enabled].name", true},
		{"or", "a || b", true},
		{"and", "a && b", true},
		{"not", "!a", true},

		// Unsafe: comparator anywhere. go-jmespath's <,>,<=,>= return nil for
		// a json.Number operand and ==/!= compare it with reflect.DeepEqual
		// against a float64 literal, so these silently mis-filter.
		{"comparator at root", "a == `1`", false},
		{"comparator in filter condition", "[?port > `80`].name", false},
		{"string comparator in filter condition", "[?name == 'x'].id", false},
		{"comparator under a hash", "{A: a == `1`}", false},
		{"comparator behind a pipe", "[].{Name: name} | [?Name == 'x']", false},

		// Unsafe: any function. These error outright on a json.Number.
		{"avg", "avg(a)", false},
		{"sum", "sum([].port)", false},
		{"to_number", "to_number(a)", false},
		{"max", "max(a)", false},
		{"sort_by", "sort_by(items, &v)", false},
		{"length even though it is number-safe", "length(@)", false},
		{"function nested under a hash", "{N: length(a)}", false},
		{"function nested in a filter condition", "[?contains(name, 'x')].id", false},

		// Degenerate.
		{"empty query", "", false},
		{"unparseable query", "{{{", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PreservesNumberLiterals(tt.query); got != tt.want {
				t.Errorf("PreservesNumberLiterals(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}
