package pipelines

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
)

// TestTriStateBoolFlags_SpaceSeparatedFalseErrors covers a cluster of
// findings: --authorize/--secret/--prompt-value/--allow-override/
// --skip-first-run were plain cobra Bool flags, so the Python CLI's own
// three-state idiom `--flag false` (space separated) parsed as `--flag`
// (true) plus a silently-discarded positional "false" — inverting intent
// instead of rejecting it. Converting the flags to the tri-state
// String+NoOptDefVal idiom, combined with Args: cobra.NoArgs on every leaf
// command, turns that silent inversion into a loud parse error: cobra
// consumes "--authorize" bare as true (NoOptDefVal) and then rejects the
// leftover "false" positional before RunE (and any network call) ever runs.
func TestTriStateBoolFlags_SpaceSeparatedFalseErrors(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"variable-group create --authorize", variableNewGroupCreateCmd(),
			[]string{"--name", "g", "--variables", "k=v", "--authorize", "false"}},
		{"variable-group update --authorize", variableNewGroupUpdateCmd(),
			[]string{"--group-id", "1", "--authorize", "false"}},
		{"variable-group variable create --secret", variableNewGroupVariableCreateCmd(),
			[]string{"--group-id", "1", "--name", "v", "--secret", "false"}},
		{"variable-group variable update --secret", variableNewGroupVariableUpdateCmd(),
			[]string{"--group-id", "1", "--name", "v", "--secret", "false"}},
		{"variable-group variable update --prompt-value", variableNewGroupVariableUpdateCmd(),
			[]string{"--group-id", "1", "--name", "v", "--prompt-value", "false"}},
		{"variable create --secret", variableNewPipelineCreateCmd(),
			[]string{"--name", "v", "--pipeline-id", "1", "--secret", "false"}},
		{"variable create --allow-override", variableNewPipelineCreateCmd(),
			[]string{"--name", "v", "--pipeline-id", "1", "--allow-override", "false"}},
		{"variable update --prompt-value", variableNewPipelineUpdateCmd(),
			[]string{"--name", "v", "--pipeline-id", "1", "--prompt-value", "false"}},
		{"create --skip-first-run", coreNewCreateCmd(),
			[]string{"--name", "p", "--yml-path", "azure-pipelines.yml", "--skip-first-run", "false"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				t.Fatal("RunE ran; Args: cobra.NoArgs should have rejected the leftover \"false\" first")
				return nil
			}
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tc.args)

			if err := cmd.Execute(); err == nil {
				t.Fatal("expected an error for the space-separated \"... false\" form, got nil")
			}
		})
	}
}
