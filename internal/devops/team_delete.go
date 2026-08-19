package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// teamNewDeleteCmd is `az devops team delete`, port of
// dev/team/team.py:28 delete_team.
func teamNewDeleteCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a team",
		Long:  "Delete a team",
		RunE: func(cmd *cobra.Command, args []string) error {
			return teamRunDelete(context.Background(), cmd, id)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "The id of the team to delete.")
	cmd.MarkFlagRequired("id")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func teamRunDelete(ctx context.Context, cmd *cobra.Command, id string) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return teamDelete(ctx, cmd, dctx, id)
}

// teamDelete does the actual client call, split out from teamRunDelete so
// tests can supply a dctx pointing at an httptest server without going
// through org validation.
func teamDelete(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id string) error {
	if err := ado.Confirm(cmd, "Are you sure you want to delete this team?"); err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Path:       "projects/" + url.PathEscape(dctx.Project) + "/teams/" + url.PathEscape(id),
		APIVersion: "5.0",
	}, nil); err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}

	return nil
}
