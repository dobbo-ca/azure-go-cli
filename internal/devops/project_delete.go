package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newProjectDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().String("id", "", "The id of the project to delete.")
	_ = cmd.MarkFlagRequired("id")
	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func runProjectDelete(ctx context.Context, cmd *cobra.Command) error {
	id, _ := cmd.Flags().GetString("id")

	if err := ado.Confirm(cmd, "Are you sure you want to delete this project?"); err != nil {
		return err
	}

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	op, err := projectDelete(ctx, client, id)
	if err != nil {
		return err
	}

	// project.py:103: unconditionally printed to stdout regardless of -o,
	// in addition to the returned operation object below.
	fmt.Printf("Deleted project %s\n", id)

	// No table_transformer registered for delete (commands.py:107) — table
	// mode falls back to JSON since we pass no columns.
	return ado.Print(cmd, op)
}

// projectDelete ports delete_project's HTTP sequence (project.py:89-103):
// queue deletion, then poll to completion. Split out from runProjectDelete
// so it is testable against an httptest server without going through
// ado.Resolve's real-org-host validation.
func projectDelete(ctx context.Context, client *ado.Client, id string) (map[string]any, error) {
	var ref map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Path:       "projects/" + url.PathEscape(id),
		APIVersion: "5.0",
	}, &ref); err != nil {
		return nil, fmt.Errorf("failed to delete project: %w", err)
	}

	opID, _ := ref["id"].(string)
	op, err := projectWaitForOperation(ctx, client, opID)
	if err != nil {
		return nil, err
	}
	if status, _ := op["status"].(string); strings.EqualFold(status, "failed") {
		return nil, fmt.Errorf("Project deletion failed.")
	} else if strings.EqualFold(status, "cancelled") {
		return nil, fmt.Errorf("Project deletion was cancelled.")
	}

	return op, nil
}
