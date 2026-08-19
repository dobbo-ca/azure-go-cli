package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// teamNewCreateCmd is `az devops team create`, port of
// dev/team/team.py:12 create_team.
func teamNewCreateCmd() *cobra.Command {
	var name, description string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a team",
		Long:  "Create a team",
		RunE: func(cmd *cobra.Command, args []string) error {
			return teamRunCreate(context.Background(), cmd, name, description)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the new team.")
	cmd.Flags().StringVar(&description, "description", "", "Description of the new team.")
	cmd.MarkFlagRequired("name")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func teamRunCreate(ctx context.Context, cmd *cobra.Command, name, description string) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return teamCreate(ctx, cmd, dctx, name, description)
}

// teamCreate does the actual client call, split out from teamRunCreate so
// tests can supply a dctx pointing at an httptest server without going
// through org validation.
func teamCreate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, name, description string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// WebApiTeam(name=name, description=description) — description is sent
	// as JSON null when --description was not passed, never omitted
	// (dev/team/team.py:24).
	body := map[string]any{
		"name":        name,
		"description": teamOptionalString(cmd, "description", description),
	}

	var team map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Path:       "projects/" + url.PathEscape(dctx.Project) + "/teams",
		APIVersion: "5.0",
		Body:       body,
	}, &team); err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	return ado.Print(cmd, team, teamColumns...)
}
