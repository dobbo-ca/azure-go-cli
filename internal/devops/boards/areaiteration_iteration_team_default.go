package boards

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationNewIterationTeamSetDefaultCmd is `boards iteration team
// set-default-iteration` (set_default_iteration, iteration.py:223).
func areaiterationNewIterationTeamSetDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-default-iteration",
		Short: "Set default iteration for a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationTeamSetDefault(context.Background(), cmd)
		},
	}

	areaiterationAddTeamFlag(cmd)
	cmd.Flags().String("id", "", "Identifier of the iteration which needs to be set as default.")
	cmd.Flags().String("default-iteration-macro", "", "Default iteration macro. Example: @CurrentIteration.")
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationTeamSetDefault(ctx context.Context, cmd *cobra.Command) error {
	team, _ := cmd.Flags().GetString("team")
	id, _ := cmd.Flags().GetString("id")
	macro, _ := cmd.Flags().GetString("default-iteration-macro")

	// iteration.py:232-233.
	if id == "" && macro == "" {
		return errors.New("Either --id or --default-iteration-macro is required.")
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	body := map[string]any{}
	if id != "" {
		body["defaultIteration"] = id
	}
	if macro != "" {
		body["defaultIterationMacro"] = macro
	}

	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Scope:      areaiterationTeamScope(dctx.Project, team),
		Path:       "work/teamsettings",
		APIVersion: "5.0",
		Body:       body,
	}, &result); err != nil {
		return fmt.Errorf("failed to set default iteration: %w", err)
	}

	return areaiterationPrintDefaultIteration(cmd, result)
}

// areaiterationNewIterationTeamShowDefaultCmd is `boards iteration team
// show-default-iteration` (show_default_iteration, iteration.py:262) — the
// exact same GET as show-backlog-iteration, differing only in which table
// transformer is applied.
func areaiterationNewIterationTeamShowDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-default-iteration",
		Short: "Show default iteration for a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationTeamShowDefault(context.Background(), cmd)
		},
	}

	areaiterationAddTeamFlag(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationTeamShowDefault(ctx context.Context, cmd *cobra.Command) error {
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

	return areaiterationPrintDefaultIteration(cmd, result)
}

// areaiterationGetTeamSettings is get_team_settings
// (work_client.py:1282-1310), shared by show-default-iteration and
// show-backlog-iteration.
func areaiterationGetTeamSettings(ctx context.Context, client *ado.Client, project, team string) (map[string]any, error) {
	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      areaiterationTeamScope(project, team),
		Path:       "work/teamsettings",
		APIVersion: "5.0",
	}, &result); err != nil {
		return nil, fmt.Errorf("failed to get team settings: %w", err)
	}
	return result, nil
}
