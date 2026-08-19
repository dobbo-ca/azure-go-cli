package boards

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// workitemRelationRemoveCmd is `az boards work-item relation remove`, port
// of remove_relation (relations.py:73).
func workitemRelationRemoveCmd() *cobra.Command {
	var id int
	var relationType, targetID string

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove relation(s) from work item.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return workitemRunRelationRemove(context.Background(), cmd, id, relationType, targetID)
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "The ID of the work item")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringVar(&relationType, "relation-type", "", "Relation type to remove. Example: parent, child ")
	cmd.MarkFlagRequired("relation-type")
	cmd.Flags().StringVar(&targetID, "target-id", "", "ID(s) of work-items to remove relation from. Multiple values can be passed comma separated. Example: 1,2 ")
	cmd.MarkFlagRequired("target-id")

	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func workitemRunRelationRemove(ctx context.Context, cmd *cobra.Command, id int, relationType, targetID string) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	if err := ado.Confirm(cmd, "Are you sure you want to remove this relation(s)?"); err != nil {
		return err
	}

	return workitemRelationRemove(ctx, cmd, dctx, id, relationType, targetID)
}

// workitemRelationRemove does the actual client call sequence, split out
// from workitemRunRelationRemove so tests can supply a dctx pointing at an
// httptest server without going through org validation or the confirmation
// prompt.
func workitemRelationRemove(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int, relationType, targetID string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	relationTypes, err := workitemGetRelationTypes(ctx, client)
	if err != nil {
		return err
	}
	systemName, err := workitemSystemRelationName(relationTypes, relationType)
	if err != nil {
		return err
	}

	targetIDs := strings.Split(targetID, ",")

	mainWorkItem, err := workitemGetByID(ctx, client, strconv.Itoa(id), "All")
	if err != nil {
		return fmt.Errorf("failed to get work item: %w", err)
	}

	patchDocument := []map[string]any{}
	relations := workitemExtractRelations(mainWorkItem)
	if len(relations) > 0 {
		for _, tid := range targetIDs {
			target, err := workitemGetByID(ctx, client, tid, "All")
			if err != nil {
				return fmt.Errorf("failed to get work item %s: %w", tid, err)
			}
			targetURL, _ := target["url"].(string)

			// relations.py:91-97: linear scan against the pre-fetch
			// snapshot, first match wins. All indices below are computed
			// against that single snapshot and sent together in one PATCH,
			// same as Python -- recomputing against re-fetched state
			// between removals would shift the indices.
			for i, rel := range relations {
				relType, _ := rel["rel"].(string)
				relURL, _ := rel["url"].(string)
				if relType == systemName && relURL == targetURL {
					patchDocument = append(patchDocument, map[string]any{
						"op":   "remove",
						"path": fmt.Sprintf("/relations/%d", i),
					})
					break
				}
			}
		}
	}

	if len(patchDocument) != len(targetIDs) {
		return fmt.Errorf("Id(s) supplied in --target-id is not valid")
	}

	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Path:       "wit/workitems/" + strconv.Itoa(id),
		APIVersion: "5.0",
		JSONPatch:  true,
		Body:       patchDocument,
	}, nil); err != nil {
		return fmt.Errorf("failed to remove relation: %w", err)
	}

	wi, err := workitemGetByID(ctx, client, strconv.Itoa(id), "All")
	if err != nil {
		return fmt.Errorf("failed to get work item: %w", err)
	}

	remaining := workitemFillRelationNames(relationTypes, workitemExtractRelations(wi))
	return workitemPrintRelationResult(cmd, wi, remaining)
}
