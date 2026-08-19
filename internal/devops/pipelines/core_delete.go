package pipelines

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func coreNewDeleteCmd() *cobra.Command {
	var id int

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a pipeline",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return coreRunDelete(context.Background(), cmd, id)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddYesFlag(cmd)
	cmd.Flags().IntVar(&id, "id", 0, "ID of the pipeline.")
	cmd.MarkFlagRequired("id")

	return cmd
}

// coreRunDelete ports pipeline.py:184-195. No table transformer is
// registered for delete in Python (commands.py:111), and the v5.0 endpoint
// returns no content, so nothing is rendered through ado.Print — only the
// same success line Python prints itself.
func coreRunDelete(ctx context.Context, cmd *cobra.Command, id int) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	if err := ado.Confirm(cmd, "Are you sure you want to delete this pipeline?"); err != nil {
		return err
	}

	return coreDelete(ctx, cmd, dctx, id)
}

// coreDelete does the actual client call, split out from coreRunDelete for
// testability (see coreList's doc comment).
func coreDelete(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	if err := client.Do(ctx, ado.Request{Method: http.MethodDelete, Scope: dctx.Project, Path: fmt.Sprintf("build/Definitions/%d", id), APIVersion: "5.0"}, nil); err != nil {
		return fmt.Errorf("failed to delete pipeline: %w", err)
	}

	fmt.Printf("Pipeline %d was deleted successfully.\n", id)
	return nil
}
