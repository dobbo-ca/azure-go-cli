package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// teamNewShowCmd is `az devops team show`, port of
// dev/team/team.py:40 get_team.
func teamNewShowCmd() *cobra.Command {
	var team string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show team details",
		Long:  "Show team details",
		RunE: func(cmd *cobra.Command, args []string) error {
			return teamRunShow(context.Background(), cmd, team)
		},
	}

	cmd.Flags().StringVar(&team, "team", "", "The name or id of the team to show.")
	cmd.MarkFlagRequired("team")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func teamRunShow(ctx context.Context, cmd *cobra.Command, team string) error {
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
		Path:       "projects/" + url.PathEscape(dctx.Project) + "/teams/" + url.PathEscape(team),
		APIVersion: "5.0",
	}, &result); err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}

	return ado.Print(cmd, result, teamColumns...)
}
