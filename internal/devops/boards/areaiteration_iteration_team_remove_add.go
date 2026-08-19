package boards

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

const areaiterationEmptyUUID = "00000000-0000-0000-0000-000000000000"

// areaiterationNewIterationTeamRemoveCmd is `boards iteration team remove`
// (delete_team_iteration, iteration.py:170). commands.py:79-80 registers no
// confirmation= kwarg — no --yes/-y prompt for this one, unlike the
// classification-node deletes.
func areaiterationNewIterationTeamRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove iteration from a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationTeamRemove(context.Background(), cmd)
		},
	}

	cmd.Flags().String("id", "", "Identifier of the iteration.")
	_ = cmd.MarkFlagRequired("id")
	areaiterationAddTeamFlag(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationTeamRemove(ctx context.Context, cmd *cobra.Command) error {
	id, _ := cmd.Flags().GetString("id")
	team, _ := cmd.Flags().GetString("team")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Scope:      areaiterationTeamScope(dctx.Project, team),
		Path:       "work/teamsettings/iterations/" + url.PathEscape(id),
		APIVersion: "5.0",
	}, nil); err != nil {
		return areaiterationHandleBoardsError(err)
	}

	// commands.py:79-80 registers transform_work_item_team_iteration_table_output
	// against this command even though the DELETE response has no body
	// (work_client.py:882-883 has no _deserialize) — rendering it as a
	// TeamSettingsIteration row would crash on the missing data. Per the
	// crash-fix policy, fall back to the same "no data to render" shape
	// used by the classification-node deletes instead.
	return ado.Print(cmd, nil)
}

// areaiterationNewIterationTeamAddCmd is `boards iteration team add`
// (post_team_iteration, iteration.py:187).
func areaiterationNewIterationTeamAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add iteration to a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationTeamAdd(context.Background(), cmd)
		},
	}

	cmd.Flags().String("id", "", "Identifier of the iteration.")
	_ = cmd.MarkFlagRequired("id")
	areaiterationAddTeamFlag(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationTeamAdd(ctx context.Context, cmd *cobra.Command) error {
	id, _ := cmd.Flags().GetString("id")
	team, _ := cmd.Flags().GetString("team")

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
		Method:     http.MethodPost,
		Scope:      areaiterationTeamScope(dctx.Project, team),
		Path:       "work/teamsettings/iterations",
		APIVersion: "5.0",
		Body:       map[string]any{"id": id},
	}, &result); err != nil {
		return areaiterationHandleEmptyBacklogIterationID(ctx, client, dctx.Project, team, err)
	}

	return ado.Print(cmd, result, areaiterationVisibleColumns([]map[string]any{result}, areaiterationTeamIterationColumns)...)
}

// areaiterationHandleEmptyBacklogIterationID ports
// _handle_empty_backlog_iteration_id (iteration.py:296-309): on a service
// error whose message contains "TF400497", check whether the team's
// backlog iteration is still the empty-UUID placeholder and, if so, raise a
// more actionable error instead of the raw server message. Any other case
// falls through to the shared troubleshooting-link wrapper.
func areaiterationHandleEmptyBacklogIterationID(ctx context.Context, client *ado.Client, project, team string, err error) error {
	var apiErr *ado.APIError
	if errors.As(err, &apiErr) && strings.Contains(apiErr.Message, "TF400497") {
		if settings, getErr := areaiterationGetTeamSettings(ctx, client, project, team); getErr == nil {
			if bi, ok := settings["backlogIteration"].(map[string]any); ok {
				if bid, _ := bi["id"].(string); bid == areaiterationEmptyUUID {
					return errors.New("No backlog iteration has been selected for your team. " +
						"Before you can select iterations for your team to participate in, " +
						"you must first specify a backlog iteration.\nYou can set backlog iteration by " +
						"running following command: az boards iteration team set-backlog-iteration " +
						"--team <TeamID> --id <BacklogIterationID>")
				}
			}
		}
	}
	return areaiterationHandleBoardsError(err)
}
