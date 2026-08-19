package devops

import "github.com/spf13/cobra"

// NewDevOpsCommand returns the top-level `az devops` command with its
// subcommands wired. Short/Long mirror azext_devops's own group help
// (dev/team/_help.py:10-19, helps['devops']).
func NewDevOpsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devops",
		Short: "Manage Azure DevOps organization level operations.",
		Long: "Manage Azure DevOps organization level operations.\n\n" +
			"Related Groups\n" +
			"az pipelines: Manage Azure Pipelines\n" +
			"az boards: Manage Azure Boards\n" +
			"az repos: Manage Azure Repos\n" +
			"az artifacts: Manage Azure Artifacts",
	}

	cmd.AddCommand(newCoreCommands()...)
	cmd.AddCommand(newProjectCommand())
	cmd.AddCommand(newTeamCommand())
	cmd.AddCommand(newUserCommand())
	cmd.AddCommand(newServiceEndpointCommand())
	cmd.AddCommand(newExtensionCommand())
	cmd.AddCommand(newSecurityCommand())
	cmd.AddCommand(newWikiCommand())
	cmd.AddCommand(newAdminCommand())

	return cmd
}
