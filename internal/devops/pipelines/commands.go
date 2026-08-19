package pipelines

import "github.com/spf13/cobra"

// NewPipelinesCommand wires the `az pipelines` command group: the core
// create/update/list/show/delete/run/folder commands from
// newPipelineCommands, plus the runs, build, release, pool/agent/queue and
// variable-group/variable sibling groups.
func NewPipelinesCommand() *cobra.Command {
	cmd := newPipelineCommands()

	cmd.AddCommand(newRunsCommand())
	cmd.AddCommand(newBuildCommand())
	cmd.AddCommand(newReleaseCommand())
	cmd.AddCommand(newAgentPoolCommands()...)
	cmd.AddCommand(newVariableCommands()...)

	return cmd
}
