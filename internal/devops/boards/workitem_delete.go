package boards

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// workitemDeleteCmd is `az boards work-item delete`, port of
// delete_work_item (work_item.py:166).
func workitemDeleteCmd() *cobra.Command {
	var id int
	var destroy bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a work item.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return workitemRunDelete(context.Background(), cmd, id, destroy)
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "Unique id of the work item.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().BoolVar(&destroy, "destroy", false, "Permanently delete this work item.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func workitemRunDelete(ctx context.Context, cmd *cobra.Command, id int, destroy bool) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	if err := ado.Confirm(cmd, "Are you sure you want to delete this work item?"); err != nil {
		return err
	}

	return workitemDelete(ctx, cmd, dctx, id, destroy)
}

// workitemDelete does the actual client call, split out from
// workitemRunDelete so tests can supply a dctx pointing at an httptest
// server without going through org validation or the confirmation prompt.
func workitemDelete(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int, destroy bool) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{}
	if destroy {
		q.Set("destroy", "true")
	} else {
		q.Set("destroy", "false")
	}

	var deleted map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Scope:      dctx.Project,
		Path:       "wit/workitems/" + url.PathEscape(strconv.Itoa(id)),
		APIVersion: "5.0",
		Query:      q,
	}, &deleted); err != nil {
		return fmt.Errorf("failed to delete work item: %w", err)
	}

	// work_item.py:179: a literal, unformatted stdout line independent of
	// -o/--query, in addition to the normal result print below.
	fmt.Printf("Deleted work item %d\n", id)

	// No table_transformer is registered for delete_work_item (commands.py:50-51),
	// so ado.Print falls back to JSON in table mode too (no cols passed).
	return ado.Print(cmd, deleted)
}
