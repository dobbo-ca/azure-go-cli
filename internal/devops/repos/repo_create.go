package repos

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newRepoCreateCmd implements `az repos create` (create_repo, repository.py:22).
func newRepoCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Git repository in a team project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return repoRunCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name for the new repository.")
	cmd.Flags().Bool("open", false, "Open the repository page in your web browser after creation.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.MarkFlagRequired("name")

	return cmd
}

func repoRunCreate(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	name, _ := cmd.Flags().GetString("name")
	open, _ := cmd.Flags().GetBool("open")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var repo map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "POST",
		Scope:      dctx.Project,
		Path:       "git/repositories",
		APIVersion: "5.0",
		Body:       map[string]any{"name": name},
	}, &repo); err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	if open {
		repoOpenInBrowser(dctx.Org, repo)
	}

	return ado.Print(cmd, repo, repoColumns...)
}
