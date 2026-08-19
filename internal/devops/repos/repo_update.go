package repos

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newRepoUpdateCmd implements `az repos update` (update_repo, repository.py:68).
func newRepoUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return repoRunUpdate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("default-branch", "", "Default branch to be set for the repository. Example: 'refs/heads/live' or 'live'.")
	cmd.Flags().String("name", "", "New name for the repository.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddRepoFlag(cmd)
	cmd.MarkFlagRequired("repository")
	cmd.MarkFlagsOneRequired("default-branch", "name")

	return cmd
}

func repoRunUpdate(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveRepo(cmd)
	if err != nil {
		return err
	}

	defaultBranch, _ := cmd.Flags().GetString("default-branch")
	newName, _ := cmd.Flags().GetString("name")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return repoUpdate(ctx, cmd, client, dctx, defaultBranch, newName)
}

// repoUpdate does the fetch-mutate-PATCH HTTP work, split out from
// repoRunUpdate so tests can drive it against an httptest server with a
// hand-built ado.Context — dctx.Org there need not satisfy
// ado.ResolveRepo's dev.azure.com/visualstudio.com URL check.
func repoUpdate(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context, defaultBranch, newName string) error {
	repoPath := "git/repositories/" + url.PathEscape(dctx.Repo)

	// Fetch the current repository, mutate the requested fields locally, and
	// PATCH the whole object back — Python does not build a partial/delta
	// patch (update_repo, repository.py:68-90). A minimal-diff PATCH body
	// would not match observable behaviour.
	var repo map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       repoPath,
		APIVersion: "5.0",
	}, &repo); err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	if defaultBranch != "" {
		repo["defaultBranch"] = policyResolveRefHeads(defaultBranch)
	}
	if newName != "" {
		repo["name"] = newName
	}

	var updated map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "PATCH",
		Scope:      dctx.Project,
		Path:       repoPath,
		APIVersion: "5.0",
		Body:       repo,
	}, &updated); err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}

	return ado.Print(cmd, updated, repoColumns...)
}
