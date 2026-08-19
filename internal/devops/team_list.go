package devops

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// teamNewListCmd is `az devops team list`, port of dev/team/team.py:53
// get_teams. Project-scoped GetTeams ({projectId}/teams), never the
// separate org-wide GetAllTeams — this extension never calls that route.
func teamNewListCmd() *cobra.Command {
	var top, skip int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all teams in a project",
		Long:  "List all teams in a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return teamRunList(context.Background(), cmd, top, skip)
		},
	}

	cmd.Flags().IntVar(&top, "top", 0, "Maximum number of teams to return.")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of teams to skip.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func teamRunList(ctx context.Context, cmd *cobra.Command, top, skip int) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return teamList(ctx, cmd, dctx, top, skip)
}

// teamList does the actual client call, split out from teamRunList so tests
// can supply a dctx pointing at an httptest server without going through org
// validation.
func teamList(ctx context.Context, cmd *cobra.Command, dctx ado.Context, top, skip int) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	req := ado.Request{
		Path:       "projects/" + url.PathEscape(dctx.Project) + "/teams",
		APIVersion: "5.0",
	}
	req.Query = teamTopSkipQuery(cmd, top, skip)

	var teams []map[string]any
	if err := client.List(ctx, req, &teams); err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}

	// transform_teams_table_output sorts by name.lower() (_get_team_key,
	// dev/team/_format.py:258-262), but commands.py:126 wires it only as
	// this command's table_transformer — knack applies that solely for
	// -o table with no --query; JSON/tsv keep the server's order.
	if ado.TableMode(cmd) {
		sort.Slice(teams, func(i, j int) bool {
			return strings.ToLower(devopsStr(teams[i]["name"])) < strings.ToLower(devopsStr(teams[j]["name"]))
		})
	}

	return ado.Print(cmd, teams, teamColumns...)
}

// teamTopSkipQuery builds the $top/$skip query parameters, omitted when the
// corresponding flag was not passed (get_teams(top=None, skip=None) only
// sends the ones the caller supplied, core_client.py:410-416).
func teamTopSkipQuery(cmd *cobra.Command, top, skip int) url.Values {
	q := url.Values{}
	if cmd.Flags().Changed("top") {
		q.Set("$top", fmt.Sprintf("%d", top))
	}
	if cmd.Flags().Changed("skip") {
		q.Set("$skip", fmt.Sprintf("%d", skip))
	}
	return q
}
