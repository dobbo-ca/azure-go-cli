package devops

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// teamNewUpdateCmd is `az devops team update`, port of
// dev/team/team.py:85 update_team.
func teamNewUpdateCmd() *cobra.Command {
	var team, name, description string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a team's name and/or description",
		Long:  "Update a team's name and/or description",
		RunE: func(cmd *cobra.Command, args []string) error {
			return teamRunUpdate(context.Background(), cmd, team, name, description)
		},
	}

	cmd.Flags().StringVar(&team, "team", "", "The name or id of the team to be updated.")
	cmd.MarkFlagRequired("team")
	cmd.Flags().StringVar(&name, "name", "", "New name of the team.")
	cmd.Flags().StringVar(&description, "description", "", "New description of the team.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func teamRunUpdate(ctx context.Context, cmd *cobra.Command, team, name, description string) error {
	// team.py:95-96: raised before any org/project resolution happens.
	if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("description") {
		return errors.New("Either name or description argument must be provided.")
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return teamUpdate(ctx, cmd, dctx, team, name, description)
}

// teamUpdate does the actual client call, split out from teamRunUpdate so
// tests can supply a dctx pointing at an httptest server without going
// through org validation.
func teamUpdate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, team, name, description string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// WebApiTeam(name=name, description=description) always constructs both
	// fields — whichever flag was not passed is sent as JSON null, not
	// omitted (team.py:102).
	body := map[string]any{
		"name":        teamOptionalString(cmd, "name", name),
		"description": teamOptionalString(cmd, "description", description),
	}

	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Path:       "projects/" + url.PathEscape(dctx.Project) + "/teams/" + url.PathEscape(team),
		APIVersion: "5.0",
		Body:       body,
	}, &result); err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}

	return ado.Print(cmd, result, teamColumns...)
}
