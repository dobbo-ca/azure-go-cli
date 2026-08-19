package devops

import (
	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newTeamCommand wires the `az devops team` command group: create, delete,
// show, list, list-member, update. Mirrors azext_devops/dev/team/team.py and
// its registration in dev/team/commands.py:123-128.
func newTeamCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage Azure DevOps teams",
		Long:  "Manage Azure DevOps teams within a project",
	}

	cmd.AddCommand(teamNewCreateCmd())
	cmd.AddCommand(teamNewDeleteCmd())
	cmd.AddCommand(teamNewShowCmd())
	cmd.AddCommand(teamNewListCmd())
	cmd.AddCommand(teamNewListMemberCmd())
	cmd.AddCommand(teamNewUpdateCmd())

	return cmd
}

// teamColumns is the table shape shared by create/show/list/update
// (transform_team_table_output / transform_teams_table_output,
// dev/team/_format.py:267-269).
var teamColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Description", Field: "description"},
}

// teamMemberColumns is transform_team_members_table_output's row shape
// (dev/team/_format.py:315-327) — note isTeamAdmin is fetched but never
// shown in the table, only visible in -o json.
var teamMemberColumns = []ado.Column{
	{Header: "ID", Field: "identity.id"},
	{Header: "Name", Field: "identity.displayName"},
	{Header: "Email", Field: "identity.uniqueName"},
}

// teamOptionalString returns value when flagName was explicitly set on cmd,
// else nil. Used to reproduce WebApiTeam(name=..., description=...) always
// constructing both fields — an unset flag must serialize as JSON null, not
// be omitted (team.py:102, dev/team/team.py:95-96).
func teamOptionalString(cmd *cobra.Command, flagName, value string) any {
	if cmd.Flags().Changed(flagName) {
		return value
	}
	return nil
}
