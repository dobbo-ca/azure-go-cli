package boards

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// workitemRelationShowCmd is `az boards work-item relation show`, port of
// relations.py's show_work_item (relations.py:109) -- a name collision with
// work_item.py's own show_work_item, imported from a different module for
// this command group. Always expands All (hardcoded, unlike
// `boards work-item show`'s --expand choice list).
func workitemRelationShowCmd() *cobra.Command {
	var id int

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get work item, fill relations with friendly name",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return workitemRunRelationShow(context.Background(), cmd, id)
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "The ID of the work item")
	cmd.MarkFlagRequired("id")

	ado.AddOrgFlags(cmd)

	return cmd
}

func workitemRunRelationShow(ctx context.Context, cmd *cobra.Command, id int) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}
	return workitemRelationShow(ctx, cmd, dctx, id)
}

// workitemRelationShow does the actual client call, split out from
// workitemRunRelationShow so tests can supply a dctx pointing at an
// httptest server without going through org validation.
func workitemRelationShow(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	wi, err := workitemGetByID(ctx, client, strconv.Itoa(id), "All")
	if err != nil {
		return fmt.Errorf("failed to get work item: %w", err)
	}

	relationTypes, err := workitemGetRelationTypes(ctx, client)
	if err != nil {
		return err
	}

	relations := workitemFillRelationNames(relationTypes, workitemExtractRelations(wi))
	return workitemPrintRelationResult(cmd, wi, relations)
}
