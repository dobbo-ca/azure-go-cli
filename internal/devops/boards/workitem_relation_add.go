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

// workitemRelationAddCmd is `az boards work-item relation add`, port of
// add_relation (relations.py:24).
func workitemRelationAddCmd() *cobra.Command {
	var id int
	var relationType, targetID, targetURL string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add relation(s) to work item.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetID == "" && targetURL == "" {
				return fmt.Errorf("--target-id or --target-url must be provided")
			}
			return workitemRunRelationAdd(context.Background(), cmd, id, relationType, targetID, targetURL)
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "The ID of the work item")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringVar(&relationType, "relation-type", "", "Relation type to create. Example: parent, child ")
	cmd.MarkFlagRequired("relation-type")
	cmd.Flags().StringVar(&targetID, "target-id", "", "ID(s) of work-items to create relation with. Multiple values can be passed comma separated. Example: 1,2 ")
	cmd.Flags().StringVar(&targetURL, "target-url", "", "URL(s) of work-items to create relation with. Multiple values can be passed comma separated.")

	ado.AddOrgFlags(cmd)

	return cmd
}

func workitemRunRelationAdd(ctx context.Context, cmd *cobra.Command, id int, relationType, targetID, targetURL string) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}
	return workitemRelationAdd(ctx, cmd, dctx, id, relationType, targetID, targetURL)
}

// workitemRelationAdd does the actual client call sequence, split out from
// workitemRunRelationAdd so tests can supply a dctx pointing at an httptest
// server without going through org validation.
func workitemRelationAdd(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int, relationType, targetID, targetURL string) error {
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

	patchDocument := []map[string]any{}

	if targetID != "" {
		targetIDs := strings.Split(targetID, ",")
		clauses := make([]string, len(targetIDs))
		for i, tid := range targetIDs {
			clauses[i] = fmt.Sprintf("[System.Id] = %s", tid)
		}
		wiql := fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE (%s)", strings.Join(clauses, " OR "))

		var queryResult struct {
			WorkItems []map[string]any `json:"workItems"`
		}
		if err := client.Do(ctx, ado.Request{
			Method:     http.MethodPost,
			Path:       "wit/wiql",
			APIVersion: "5.0",
			Body:       map[string]any{"query": wiql},
		}, &queryResult); err != nil {
			return fmt.Errorf("failed to query target work items: %w", err)
		}

		if len(queryResult.WorkItems) != len(targetIDs) {
			return fmt.Errorf("Id(s) supplied in --target-id is not valid")
		}

		for _, target := range queryResult.WorkItems {
			targetWIURL, _ := target["url"].(string)
			patchDocument = append(patchDocument, map[string]any{
				"op":   "add",
				"path": "/relations/-",
				"value": map[string]any{
					"rel": systemName,
					"url": targetWIURL,
				},
			})
		}
	}

	if targetURL != "" {
		for _, u := range strings.Split(targetURL, ",") {
			patchDocument = append(patchDocument, map[string]any{
				"op":   "add",
				"path": "/relations/-",
				"value": map[string]any{
					"rel": systemName,
					"url": u,
				},
			})
		}
	}

	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Path:       "wit/workitems/" + strconv.Itoa(id),
		APIVersion: "5.0",
		JSONPatch:  true,
		Body:       patchDocument,
	}, nil); err != nil {
		return fmt.Errorf("failed to add relation: %w", err)
	}

	wi, err := workitemGetByID(ctx, client, strconv.Itoa(id), "All")
	if err != nil {
		return fmt.Errorf("failed to get work item: %w", err)
	}

	relations := workitemFillRelationNames(relationTypes, workitemExtractRelations(wi))
	return workitemPrintRelationResult(cmd, wi, relations)
}
