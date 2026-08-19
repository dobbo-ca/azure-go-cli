package repos

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newRepoShowCmd implements `az repos show` (show_repo, repository.py:98).
func newRepoShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details of a Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return repoRunShow(context.Background(), cmd)
		},
	}

	cmd.Flags().Bool("open", false, "Open the repository page in your web browser.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddRepoFlag(cmd)
	cmd.MarkFlagRequired("repository")

	return cmd
}

func repoRunShow(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveRepo(cmd)
	if err != nil {
		return err
	}

	open, _ := cmd.Flags().GetBool("open")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var repo map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "git/repositories/" + url.PathEscape(dctx.Repo),
		APIVersion: "5.0",
	}, &repo); err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	if open {
		repoOpenInBrowser(dctx.Org, repo)
	}

	return ado.Print(cmd, repo, repoColumns...)
}
