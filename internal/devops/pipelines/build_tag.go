package pipelines

import (
	"github.com/spf13/cobra"
)

// newBuildTagCommand wires `az pipelines build tag` (list/add/delete).
func newBuildTagCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage tags for a build",
	}
	cmd.AddCommand(newBuildTagListCmd())
	cmd.AddCommand(newBuildTagAddCmd())
	cmd.AddCommand(newBuildTagDeleteCmd())
	return cmd
}
