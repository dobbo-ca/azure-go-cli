package boards

import "github.com/spf13/cobra"

// NewBoardsCommand wires the `az boards` command group: work-item (plus its
// nested relation subgroup), query, and the area/iteration sibling groups.
// Short/Long mirror azext_devops's own group help
// (dev/boards/_help.py:10-14, helps['boards']).
func NewBoardsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "boards",
		Short: "Manage Azure Boards.",
		Long:  "Manage Azure Boards.",
	}

	cmd.AddCommand(newWorkItemCommand())
	cmd.AddCommand(newQueryCommand())
	cmd.AddCommand(newAreaIterationCommands()...)

	return cmd
}
