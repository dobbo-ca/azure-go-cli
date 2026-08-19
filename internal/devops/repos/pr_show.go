package repos

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPRShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details of a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunShow(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().Bool("open", false, "Open the pull request in your web browser.")

	// show_pull_request declares only organization/detect — no --project,
	// no --repository (both are discovered from the by-id lookup below).
	ado.AddOrgFlags(cmd)

	return cmd
}

func prRunShow(ctx context.Context, cmd *cobra.Command) error {
	client, dctx, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prShowExec(ctx, cmd, client, dctx)
}

// prShowExec does the actual work given an already-resolved client, split
// out from prRunShow so tests can exercise it against an httptest server
// without going through ado.Resolve's org validation.
func prShowExec(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)

	byID, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	repoID, projectID := prRepoProjectID(byID)

	var pr map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      projectID,
		Path:       "git/repositories/" + url.PathEscape(repoID) + "/pullRequests/" + idStr,
		APIVersion: "5.0",
		Query:      url.Values{"includeCommits": {"false"}, "includeWorkItemRefs": {"true"}},
	}, &pr); err != nil {
		return fmt.Errorf("failed to get pull request: %w", err)
	}

	if open, _ := cmd.Flags().GetBool("open"); open {
		prOpenInBrowser(dctx.Org, pr)
	}

	return ado.Print(cmd, pr, prColumns...)
}
