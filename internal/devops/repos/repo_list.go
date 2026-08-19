package repos

import (
	"context"
	"fmt"
	"sort"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newRepoListCmd implements `az repos list` (list_repos, repository.py:57).
func newRepoListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Git repositories of a team project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return repoRunList(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func repoRunList(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return repoList(ctx, cmd, client, dctx)
}

// repoList does the list + client-side sort, split out from repoRunList so
// tests can drive it against an httptest server with a hand-built
// ado.Context.
func repoList(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	var repositories []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "git/repositories",
		APIVersion: "5.0",
	}, &repositories); err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	// Table view sorts client-side by name (transform_repos_table_output,
	// _format.py:295-299, _get_repo_key); JSON/other formats keep server
	// order. This mirrors exactly the condition ado.Print uses to pick the
	// table path, so the two never disagree.
	if ado.TableMode(cmd) {
		sort.Slice(repositories, func(i, j int) bool {
			return fmt.Sprint(repositories[i]["name"]) < fmt.Sprint(repositories[j]["name"])
		})
	}

	return ado.Print(cmd, repositories, repoColumns...)
}
