package repos

import "github.com/spf13/cobra"

// NewReposCommand wires the `az repos` command group: the core repository
// commands from newRepoCommands, plus the ref, policy and pr subgroups.
func NewReposCommand() *cobra.Command {
	cmd := newRepoCommands()

	cmd.AddCommand(newRefCommand())
	cmd.AddCommand(newPolicyCommand())
	cmd.AddCommand(newPRCommand())

	return cmd
}
