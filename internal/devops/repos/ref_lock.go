package repos

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newRefLockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock a reference.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefUpdateLock(context.Background(), cmd, true)
		},
	}

	cmd.Flags().String("name", "", "Name of the reference to update (example: heads/my_branch).")
	refAddFlags(cmd)
	cmd.MarkFlagRequired("name")

	return cmd
}

// runRefUpdateLock is shared by `repos ref lock` and `repos ref unlock`
// (ref.py:94-125, both delegate to _update_ref).
//
// Important asymmetry with create/delete, reproduced deliberately: name is
// sent as the raw `filter` query value WITHOUT the refs/-prefixing
// create/delete apply (ref.py never calls resolve_git_refs here) — a bare
// branch name that works for create/delete may match zero refs here. This
// is a genuine upstream quirk, not a bug to "fix".
func runRefUpdateLock(ctx context.Context, cmd *cobra.Command, locked bool) error {
	client, dctx, err := refClient(ctx, cmd)
	if err != nil {
		return err
	}
	return refUpdateLockExec(ctx, cmd, client, dctx, locked)
}

// refUpdateLockExec does the actual work given an already-resolved client
// and context, split out from runRefUpdateLock so tests can exercise it
// against an httptest server without going through ado.ResolveProject's org
// validation.
func refUpdateLockExec(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context, locked bool) error {
	name, _ := cmd.Flags().GetString("name")

	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "PATCH",
		Scope:      dctx.Project,
		Path:       "git/repositories/" + url.PathEscape(dctx.Repo) + "/refs",
		APIVersion: "5.0",
		Query:      url.Values{"filter": {name}},
		Body:       map[string]any{"isLocked": locked},
	}, &result); err != nil {
		return fmt.Errorf("failed to update reference: %w", err)
	}

	return ado.Print(cmd, result, refColumns...)
}
