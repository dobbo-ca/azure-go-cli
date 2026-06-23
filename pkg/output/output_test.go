package output

import (
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestRenderTSV(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{"nil", nil, ""},
		{"scalar string", "abc", "abc\n"},
		{"list of strings", []interface{}{"id1", "id2"}, "id1\nid2\n"},
		{"empty list", []interface{}{}, ""},
		{"object sorted keys", []interface{}{map[string]interface{}{"b": "2", "a": "1"}}, "1\t2\n"},
		{"int without decimal", float64(5), "5\n"},
		{"bool", true, "True\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderTSV(tt.in); got != tt.want {
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

	got := captureStdout(t, func() {
		if err := PrintFormatted(cmd, data, "tsv"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
	})

	if got != "ra-1\n" {
		t.Errorf("got %q, want %q", got, "ra-1\n")
	}
}

// TestPrintFormatted_QueryNoMatch confirms an empty match yields no output,
// which the idempotency check relies on to decide "not yet assigned".
func TestPrintFormatted_QueryNoMatch(t *testing.T) {
	data := []map[string]interface{}{{"id": "ra-1", "roleDefinitionName": "Reader"}}

	cmd := &cobra.Command{}
	cmd.Flags().String("query", "[?roleDefinitionName=='Nope'].id", "")

	got := captureStdout(t, func() {
		if err := PrintFormatted(cmd, data, "tsv"); err != nil {
			t.Fatalf("PrintFormatted: %v", err)
		}
	})

	if got != "" {
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

	child := &cobra.Command{
		Use: "child",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]interface{}{{"id": "x"}, {"id": "y"}}
			captured = captureStdout(t, func() {
				if err := PrintFormatted(cmd, data, "tsv"); err != nil {
					t.Errorf("PrintFormatted: %v", err)
				}
			})
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

// TestPrintFormatted_UnsupportedFormat confirms an unknown -o value errors
// instead of silently rendering JSON.
func TestPrintFormatted_UnsupportedFormat(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("query", "", "")

	for _, format := range []string{"yaml", "none", "tsvv"} {
		err := PrintFormatted(cmd, []string{"x"}, format)
		if err == nil {
			t.Errorf("format %q: expected error, got nil", format)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}
