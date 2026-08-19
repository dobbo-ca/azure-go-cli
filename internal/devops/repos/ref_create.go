package repos

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newRefCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a reference.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the reference to create (example: heads/my_branch or tags/my_tag).")
	cmd.Flags().String("object-id", "", "Id of the object to create the reference from.")
	refAddFlags(cmd)
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("object-id")

	return cmd
}

func runRefCreate(ctx context.Context, cmd *cobra.Command) error {
	client, dctx, err := refClient(ctx, cmd)
	if err != nil {
		return err
	}
	return refCreateExec(ctx, cmd, client, dctx)
}

// refCreateExec does the actual work given an already-resolved client and
// context, split out from runRefCreate so tests can exercise it against an
// httptest server without going through ado.ResolveProject's org validation.
func refCreateExec(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	name, _ := cmd.Flags().GetString("name")
	objectID, _ := cmd.Flags().GetString("object-id")

	body := []map[string]any{
		{
			"isLocked":    false,
			"name":        refWithPrefix(name),
			"newObjectId": objectID,
			"oldObjectId": refZeroObjectID,
		},
	}

	var resp refUpdateEnvelope
	if err := client.Do(ctx, ado.Request{
		Method:     "POST",
		Scope:      dctx.Project,
		Path:       "git/repositories/" + url.PathEscape(dctx.Repo) + "/refs",
		APIVersion: "5.0",
		Body:       body,
	}, &resp); err != nil {
		return fmt.Errorf("failed to create reference: %w", err)
	}
	if len(resp.Value) == 0 {
		return fmt.Errorf("failed to create reference: empty response")
	}
	result := resp.Value[0]

	// ref.py:55-58: the bulk-refs endpoint always answers HTTP 200, even
	// for a logical failure (e.g. ref already exists) — success is the
	// only failure signal for create.
	if success, ok := result["success"].(bool); ok && !success {
		msg, _ := result["customMessage"].(string)
		return fmt.Errorf("%s", msg)
	}

	return ado.Print(cmd, result, refColumns...)
}
