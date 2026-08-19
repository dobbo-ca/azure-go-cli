package boards

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// workitemCreateCmd is `az boards work-item create`, port of
// create_work_item (work_item.py:25).
func workitemCreateCmd() *cobra.Command {
	var workItemType, title, description, assignedTo, area, iteration, reason, discussion string
	var fields []string
	var open bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a work item.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields = append(fields, args...)
			return workitemRunCreate(context.Background(), cmd, workItemType, title, description, assignedTo, area, iteration, reason, discussion, fields, open)
		},
	}

	cmd.Flags().StringVar(&workItemType, "type", "", "Name of the work item type (e.g. Bug).")
	cmd.MarkFlagRequired("type")
	cmd.Flags().StringVar(&title, "title", "", "Title of the work item.")
	cmd.MarkFlagRequired("title")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description of the work item.")
	cmd.Flags().StringVar(&assignedTo, "assigned-to", "", "Name of the person the work item is assigned-to (e.g. fabrikam).")
	cmd.Flags().StringVar(&area, "area", "", "Area the work item is assigned to (e.g. Demos)")
	cmd.Flags().StringVar(&iteration, "iteration", "", `Iteration path of the work item (e.g. Demos\Iteration 1).`)
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for the state of the work item.")
	cmd.Flags().StringVar(&discussion, "discussion", "", "Comment to add to a discussion in a work item.")
	cmd.Flags().StringArrayVarP(&fields, "fields", "f", nil, `Space separated "field=value" pairs for custom fields you would like to set. In case of multiple fields : "field1=value1" "field2=value2".`)
	cmd.Flags().BoolVar(&open, "open", false, "Open the work item in the default web browser.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func workitemRunCreate(ctx context.Context, cmd *cobra.Command, workItemType, title, description, assignedTo, area, iteration, reason, discussion string, fields []string, open bool) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	return workitemCreate(ctx, cmd, dctx, workItemType, title, description, assignedTo, area, iteration, reason, discussion, fields, open)
}

// workitemCreate does the actual client call sequence, split out from
// workitemRunCreate so tests can supply a dctx pointing at an httptest
// server without going through org validation.
func workitemCreate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, workItemType, title, description, assignedTo, area, iteration, reason, discussion string, fields []string, open bool) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// Patch order matches work_item.py:56-87 exactly: Title, Description,
	// AssignedTo, AreaPath, IterationPath, Reason, History, then --fields.
	// Each optional flag is gated on Changed(), not a non-empty check, since
	// Python's "is not None" is also true for an explicitly empty string.
	patchDocument := []map[string]any{workitemFieldOp("System.Title", title)}
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

	// validateOnly/bypassRules/suppressNotifications have no corresponding
	// CLI flags, so they're never populated and stay off the request
	// (work_item.py:86 never passes them to create_work_item).
	var wi map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      dctx.Project,
		Path:       "wit/workitems/$" + url.PathEscape(workItemType),
		APIVersion: "5.0",
		JSONPatch:  true,
		Body:       patchDocument,
	}, &wi); err != nil {
		return fmt.Errorf("failed to create work item: %w", err)
	}

	if open {
		if err := ado.OpenBrowser(workitemOpenBrowserURL(dctx.Org, wi)); err != nil {
			logger.Warning("failed to open web browser: %v", err)
		}
	}

	return ado.Print(cmd, wi, workitemColumns...)
}
