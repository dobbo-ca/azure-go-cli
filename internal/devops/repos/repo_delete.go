package repos

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newRepoDeleteCmd implements `az repos delete` (delete_repo, repository.py:43).
func newRepoDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return repoRunDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().String("id", "", "ID of the repository.")
	ado.AddYesFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.MarkFlagRequired("id")

	return cmd
}

func repoRunDelete(ctx context.Context, cmd *cobra.Command) error {
	if err := ado.Confirm(cmd, "Are you sure you want to delete this repository?"); err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return repoDelete(ctx, cmd, client, dctx, id)
}

// repoDelete does the delete HTTP call, split out from repoRunDelete so
// tests can drive it against an httptest server with a hand-built
// ado.Context.
func repoDelete(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context, id string) error {
	if err := client.Do(ctx, ado.Request{
		Method:     "DELETE",
		Scope:      dctx.Project,
		Path:       "git/repositories/" + url.PathEscape(id),
		APIVersion: "5.0",
	}, nil); err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	// Python prints this directly to stdout as a side effect (repository.py:52),
	// independent of -o/--output. delete_repository returns None
	// (git_client_base.py:2821-2835), and knack skips all output whenever
	// cmd_result.result is None (cli.py:237) — so nothing else is printed,
	// not even for --query or -o json/table.
	fmt.Printf("Deleted repository %s\n", id)

	return nil
}
