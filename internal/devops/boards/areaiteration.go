// Package boards implements `az boards` (work-item, query, area, iteration).
// This file wires the `area` and `iteration` command groups: `boards area
// project`, `boards area team`, `boards iteration project`, `boards
// iteration team` — ports of azext_devops/dev/boards/{area,iteration}.py.
package boards

import (
	"github.com/spf13/cobra"
)

// newAreaIterationCommands returns the `boards area` and `boards iteration`
// top-level command groups as siblings — they are two distinct command
// groups under `boards` (like `boards work-item` and `boards query`), not
// one command with two subcommands.
func newAreaIterationCommands() []*cobra.Command {
	area := &cobra.Command{
		Use:   "area",
		Short: "Manage area paths.",
	}
	area.AddCommand(areaiterationNewAreaProjectCmd())
	area.AddCommand(areaiterationNewAreaTeamCmd())

	iteration := &cobra.Command{
		Use:   "iteration",
		Short: "Manage iterations.",
	}
	iteration.AddCommand(areaiterationNewIterationProjectCmd())
	iteration.AddCommand(areaiterationNewIterationTeamCmd())

	return []*cobra.Command{area, iteration}
}

func areaiterationNewAreaProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage areas for a project.",
	}
	cmd.AddCommand(areaiterationNewProjectListCmd(areaiterationStructureGroupArea))
	cmd.AddCommand(areaiterationNewProjectShowCmd(areaiterationStructureGroupArea))
	cmd.AddCommand(areaiterationNewAreaProjectCreateCmd())
	cmd.AddCommand(areaiterationNewAreaProjectUpdateCmd())
	cmd.AddCommand(areaiterationNewProjectDeleteCmd(areaiterationStructureGroupArea))
	return cmd
}

func areaiterationNewAreaTeamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage areas for a team.",
	}
	cmd.AddCommand(areaiterationNewAreaTeamListCmd())
	cmd.AddCommand(areaiterationNewAreaTeamAddCmd())
	cmd.AddCommand(areaiterationNewAreaTeamRemoveCmd())
	cmd.AddCommand(areaiterationNewAreaTeamUpdateCmd())
	return cmd
}

func areaiterationNewIterationProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage iterations for a project.",
	}
	cmd.AddCommand(areaiterationNewProjectListCmd(areaiterationStructureGroupIteration))
	cmd.AddCommand(areaiterationNewIterationProjectUpdateCmd())
	cmd.AddCommand(areaiterationNewProjectDeleteCmd(areaiterationStructureGroupIteration))
	cmd.AddCommand(areaiterationNewProjectShowCmd(areaiterationStructureGroupIteration))
	cmd.AddCommand(areaiterationNewIterationProjectCreateCmd())
	return cmd
}

func areaiterationNewIterationTeamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage iterations for a team.",
	}
	cmd.AddCommand(areaiterationNewIterationTeamListCmd())
	cmd.AddCommand(areaiterationNewIterationTeamListWorkItemsCmd())
	cmd.AddCommand(areaiterationNewIterationTeamSetDefaultCmd())
	cmd.AddCommand(areaiterationNewIterationTeamSetBacklogCmd())
	cmd.AddCommand(areaiterationNewIterationTeamShowDefaultCmd())
	cmd.AddCommand(areaiterationNewIterationTeamShowBacklogCmd())
	cmd.AddCommand(areaiterationNewIterationTeamRemoveCmd())
	cmd.AddCommand(areaiterationNewIterationTeamAddCmd())
	return cmd
}
