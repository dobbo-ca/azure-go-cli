package pipelines

import "github.com/spf13/cobra"

// releaseDefinitionNewCommand builds the `az pipelines release definition`
// subgroup nested under `release`.
func releaseDefinitionNewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "definition",
		Short: "Manage release definitions",
		Long:  "Manage Azure Pipelines release definitions",
	}

	cmd.AddCommand(releaseDefinitionNewListCmd())
	cmd.AddCommand(releaseDefinitionNewShowCmd())

	return cmd
}
