package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// rootWithTestCmd returns the real root command plus a "boom" subcommand
// whose RunE fails, so the tests exercise the production wiring.
func rootWithTestCmd() *cobra.Command {
	root := newRootCmd()
	boom := &cobra.Command{
		Use:  "boom",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("it broke")
		},
	}
	boom.Flags().String("need", "", "a required flag")
	boom.MarkFlagRequired("need")
	root.AddCommand(boom)
	return root
}

func TestRunErrorPrintedOnceWithoutUsage(t *testing.T) {
	var stderr bytes.Buffer
	code := run(rootWithTestCmd(), []string{"boom", "--need", "x"}, &stderr)

	out := stderr.String()
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if n := strings.Count(out, "Error:"); n != 1 {
		t.Errorf("got %d Error: lines, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "it broke") {
		t.Errorf("missing error text:\n%s", out)
	}
	if strings.Contains(out, "Usage:") {
		t.Errorf("runtime error printed usage:\n%s", out)
	}
}

func TestUsageErrorsPrintUsage(t *testing.T) {
	cases := map[string][]string{
		"unknown flag":          {"boom", "--need", "x", "--nope"},
		"missing required flag": {"boom"},
		"unexpected argument":   {"boom", "--need", "x", "extra"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			code := run(rootWithTestCmd(), args, &stderr)

			out := stderr.String()
			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if n := strings.Count(out, "Error:"); n != 1 {
				t.Errorf("got %d Error: lines, want 1:\n%s", n, out)
			}
			if !strings.Contains(out, "Usage:") {
				t.Errorf("usage error did not print usage:\n%s", out)
			}
		})
	}
}

func TestRunSuccess(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(rootWithTestCmd(), []string{"version"}, &stderr); code != 0 {
		t.Errorf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr:\n%s", stderr.String())
	}
}
