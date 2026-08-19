package boards

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationNewIterationTeamSetBacklogCmd is `boards iteration team
// set-backlog-iteration` (set_backlog_iteration, iteration.py:246).
func areaiterationNewIterationTeamSetBacklogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-backlog-iteration",
		Short: "Set backlog iteration for a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationTeamSetBacklog(context.Background(), cmd)
		},
	}

	areaiterationAddTeamFlag(cmd)
	cmd.Flags().String("id", "", "Identifier of the iteration which needs to be set as backlog iteration.")
	_ = cmd.MarkFlagRequired("id")
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationTeamSetBacklog(ctx context.Context, cmd *cobra.Command) error {
	team, _ := cmd.Flags().GetString("team")
	id, _ := cmd.Flags().GetString("id")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Scope:      areaiterationTeamScope(dctx.Project, team),
		Path:       "work/teamsettings",
		APIVersion: "5.0",
		Body:       map[string]any{"backlogIteration": id},
	}, &result); err != nil {
		return fmt.Errorf("failed to set backlog iteration: %w", err)
	}

	return areaiterationPrintBacklogIteration(cmd, result)
}

// areaiterationNewIterationTeamShowBacklogCmd is `boards iteration team
// show-backlog-iteration` (show_backlog_iteration, iteration.py:274).
func areaiterationNewIterationTeamShowBacklogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-backlog-iteration",
		Short: "Show backlog iteration for a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationTeamShowBacklog(context.Background(), cmd)
		},
	}

	areaiterationAddTeamFlag(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationTeamShowBacklog(ctx context.Context, cmd *cobra.Command) error {
	team, _ := cmd.Flags().GetString("team")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	result, err := areaiterationGetTeamSettings(ctx, client, dctx.Project, team)
	if err != nil {
		return err
	}

	return areaiterationPrintBacklogIteration(cmd, result)
}
