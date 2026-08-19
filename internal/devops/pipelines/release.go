package pipelines

import "github.com/spf13/cobra"

// newReleaseCommand builds `az pipelines release` (list/create/show) and its
// nested `az pipelines release definition` (list/show) subgroup.
func newReleaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Manage releases",
		Long:  "Manage Azure Pipelines releases",
	}

	cmd.AddCommand(releaseNewListCmd())
	cmd.AddCommand(releaseNewCreateCmd())
	cmd.AddCommand(releaseNewShowCmd())
	cmd.AddCommand(releaseDefinitionNewCommand())

	return cmd
}
