package repos

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newRefListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the references.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefList(context.Background(), cmd)
		},
	}

	cmd.Flags().String("filter", "", "A filter to apply to the refs (starts with). Example: head or heads/ for the branches.")
	refAddFlags(cmd)

	return cmd
}

func runRefList(ctx context.Context, cmd *cobra.Command) error {
	client, dctx, err := refClient(ctx, cmd)
	if err != nil {
		return err
	}
	return refListExec(ctx, cmd, client, dctx)
}

// refListExec does the actual work given an already-resolved client and
// context, split out from runRefList so tests can exercise it against an
// httptest server without going through ado.ResolveProject's org validation.
func refListExec(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	filter, _ := cmd.Flags().GetString("filter")

	req := ado.Request{
		Scope:      dctx.Project,
		Path:       "git/repositories/" + url.PathEscape(dctx.Repo) + "/refs",
		APIVersion: "5.0",
	}
	if filter != "" {
		req.Query = url.Values{"filter": {filter}}
	}

	var refs []map[string]any
	if err := client.List(ctx, req, &refs); err != nil {
		return fmt.Errorf("failed to list references: %w", err)
	}

	// _format.py:262-266: sorted by name, but only for table rendering —
	// -o json preserves server order, and so does -o table with --query
	// (ado.Print bypasses the table path whenever --query is set, same
	// guard as repo_list.go's repoList).
	rows := refs
	if ado.TableMode(cmd) {
		rows = append([]map[string]any(nil), refs...)
		sort.Slice(rows, func(i, j int) bool {
			ni, _ := rows[i]["name"].(string)
			nj, _ := rows[j]["name"].(string)
			return ni < nj
		})
	}

	return ado.Print(cmd, rows, refColumns...)
}
