package repos

import (
	"testing"

	"github.com/spf13/cobra"
)

func prSortTestCmd(output, query string) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("output", output, "")
	cmd.Flags().String("query", query, "")
	return cmd
}

// TestPRPolicyNameCellNoMultiReviewerSuffix: _build_policy_name
// (_format.py:239-251) only appends a "(N)"/"(name)" suffix when
// identity_display_name is not None, which transform_policies_table_output
// (_format.py:175-192) only ever supplies for exactly one
// requiredReviewerIds entry — so the len>1 branch (_format.py:248-249) is
// dead code in Python and must never fire here either.
func TestPRPolicyNameCellNoMultiReviewerSuffix(t *testing.T) {
	row := map[string]any{
		"configuration": map[string]any{
			"type":     map[string]any{"displayName": "Required reviewers"},
			"settings": map[string]any{"requiredReviewerIds": []any{"a", "b", "c"}},
		},
	}
	got := prPolicyNameCell(row)
	if got != "Required reviewers" {
		t.Errorf("prPolicyNameCell = %q, want no reviewer-count suffix", got)
	}
}

// TestPRSortReviewersForTable covers _get_reviewer_table_key
// (_format.py:88-94): required reviewers first, then displayName lowercased
// — but only for actual table rendering (-o table --query keeps server
// order, matching query.py:49 applying the query to the raw result).
func TestPRSortReviewersForTable(t *testing.T) {
	unsorted := []map[string]any{
		{"displayName": "Zeta", "isRequired": false},
		{"displayName": "Bob", "isRequired": true},
		{"displayName": "alice", "isRequired": false},
	}

	t.Run("table sorts required-first then name", func(t *testing.T) {
		rows := append([]map[string]any(nil), unsorted...)
		prSortReviewersForTable(prSortTestCmd("table", ""), rows)
		want := []string{"Bob", "alice", "Zeta"}
		for i, w := range want {
			if rows[i]["displayName"] != w {
				t.Errorf("rows[%d].displayName = %v, want %v (order: %v)", i, rows[i]["displayName"], w, rows)
			}
		}
	})

	t.Run("table with query keeps server order", func(t *testing.T) {
		rows := append([]map[string]any(nil), unsorted...)
		prSortReviewersForTable(prSortTestCmd("table", "[0]"), rows)
		if rows[0]["displayName"] != "Zeta" {
			t.Errorf("query must bypass the sort: rows[0].displayName = %v, want Zeta", rows[0]["displayName"])
		}
	})
}

// TestPRSortPolicyEvalsForTable covers _get_policy_table_key
// (_format.py:207-213): blocking policies first, then the Policy name
// lowercased.
func TestPRSortPolicyEvalsForTable(t *testing.T) {
	unsorted := []map[string]any{
		{"configuration": map[string]any{"isBlocking": false, "type": map[string]any{"displayName": "Zeta policy"}, "settings": map[string]any{}}},
		{"configuration": map[string]any{"isBlocking": true, "type": map[string]any{"displayName": "Bob policy"}, "settings": map[string]any{}}},
		{"configuration": map[string]any{"isBlocking": false, "type": map[string]any{"displayName": "alpha policy"}, "settings": map[string]any{}}},
	}

	t.Run("table sorts blocking-first then name", func(t *testing.T) {
		evals := append([]map[string]any(nil), unsorted...)
		prSortPolicyEvalsForTable(prSortTestCmd("table", ""), evals)
		want := []string{"Bob policy", "alpha policy", "Zeta policy"}
		for i, w := range want {
			if prPolicyNameCell(evals[i]) != w {
				t.Errorf("evals[%d] Policy = %q, want %q", i, prPolicyNameCell(evals[i]), w)
			}
		}
	})

	t.Run("table with query keeps server order", func(t *testing.T) {
		evals := append([]map[string]any(nil), unsorted...)
		prSortPolicyEvalsForTable(prSortTestCmd("table", "[0]"), evals)
		if prPolicyNameCell(evals[0]) != "Zeta policy" {
			t.Errorf("query must bypass the sort: evals[0] Policy = %q, want Zeta policy", prPolicyNameCell(evals[0]))
		}
	})
}
