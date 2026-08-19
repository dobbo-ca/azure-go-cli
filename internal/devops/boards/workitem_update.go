package boards

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// workitemUpdateCmd is `az boards work-item update`, port of
// update_work_item (work_item.py:97). There is no --project flag at all --
// work item ids are unique per organization, so the update is org-scoped.
func workitemUpdateCmd() *cobra.Command {
	var id int
	var title, description, assignedTo, state, area, iteration, reason, discussion string
	var fields []string
	var open bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update work items.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields = append(fields, args...)
			return workitemRunUpdate(context.Background(), cmd, id, title, description, assignedTo, state, area, iteration, reason, discussion, fields, open)
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "The id of the work item to update.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringVar(&title, "title", "", "Title of the work item.")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description of the work item.")
	cmd.Flags().StringVar(&assignedTo, "assigned-to", "", "Name of the person the work item is assigned-to (e.g. fabrikam).")
	cmd.Flags().StringVar(&state, "state", "", "State of the work item (e.g. active).")
	cmd.Flags().StringVar(&area, "area", "", "Area the work item is assigned to (e.g. Demos).")
	cmd.Flags().StringVar(&iteration, "iteration", "", `Iteration path of the work item (e.g. Demos\Iteration 1).`)
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for the state of the work item.")
	cmd.Flags().StringVar(&discussion, "discussion", "", "Comment to add to a discussion in a work item.")
	cmd.Flags().StringArrayVarP(&fields, "fields", "f", nil, `Space separated "field=value" pairs for custom fields you would like to set.`)
	cmd.Flags().BoolVar(&open, "open", false, "Open the work item in the default web browser.")

	ado.AddOrgFlags(cmd)

	return cmd
}

func workitemRunUpdate(ctx context.Context, cmd *cobra.Command, id int, title, description, assignedTo, state, area, iteration, reason, discussion string, fields []string, open bool) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}
	return workitemUpdate(ctx, cmd, dctx, id, title, description, assignedTo, state, area, iteration, reason, discussion, fields, open)
}

// workitemUpdate does the actual client call, split out from
// workitemRunUpdate so tests can supply a dctx pointing at an httptest
// server without going through org validation.
func workitemUpdate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int, title, description, assignedTo, state, area, iteration, reason, discussion string, fields []string, open bool) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// Patch order matches work_item.py:126-149: Title, Description,
	// AssignedTo, State, AreaPath, IterationPath, Reason, History, then
	// --fields. Every flag is optional here, gated on Changed() so an
	// explicitly empty value ("" is not None in Python) still patches.
	patchDocument := []map[string]any{}
	if cmd.Flags().Changed("title") {
		patchDocument = append(patchDocument, workitemFieldOp("System.Title", title))
	}
	if cmd.Flags().Changed("description") {
		patchDocument = append(patchDocument, workitemFieldOp("System.Description", description))
	}
	if cmd.Flags().Changed("assigned-to") {
		resolved, err := workitemResolveAssignedTo(ctx, client, assignedTo)
		if err != nil {
			return err
		}
		patchDocument = append(patchDocument, workitemFieldOp("System.AssignedTo", resolved))
	}
	if cmd.Flags().Changed("state") {
		patchDocument = append(patchDocument, workitemFieldOp("System.State", state))
	}
	if cmd.Flags().Changed("area") {
		patchDocument = append(patchDocument, workitemFieldOp("System.AreaPath", area))
	}
	if cmd.Flags().Changed("iteration") {
		patchDocument = append(patchDocument, workitemFieldOp("System.IterationPath", iteration))
	}
	if cmd.Flags().Changed("reason") {
		patchDocument = append(patchDocument, workitemFieldOp("System.Reason", reason))
	}
	if cmd.Flags().Changed("discussion") {
		patchDocument = append(patchDocument, workitemFieldOp("System.History", discussion))
	}
	if len(fields) > 0 {
		fieldOps, err := workitemParseFieldPairs(fields)
		if err != nil {
			return err
		}
		patchDocument = append(patchDocument, fieldOps...)
	}

	var wi map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Path:       "wit/workitems/" + url.PathEscape(strconv.Itoa(id)),
		APIVersion: "5.0",
		JSONPatch:  true,
		Body:       patchDocument,
	}, &wi); err != nil {
		return fmt.Errorf("failed to update work item: %w", err)
	}

	if open {
		if err := ado.OpenBrowser(workitemOpenBrowserURL(dctx.Org, wi)); err != nil {
			logger.Warning("failed to open web browser: %v", err)
		}
	}

	return ado.Print(cmd, wi, workitemColumns...)
}
