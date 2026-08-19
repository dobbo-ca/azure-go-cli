package repos

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newRefDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a reference.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the reference to delete (example: heads/my_branch).")
	cmd.Flags().String("object-id", "", "Id of the reference to delete.")
	refAddFlags(cmd)
	cmd.MarkFlagRequired("name")

	return cmd
}

func runRefDelete(ctx context.Context, cmd *cobra.Command) error {
	// commands.py:134 registers delete_ref with no confirmation=, unlike
	// e.g. delete_repo (:54) or delete_policy (:63) — no prompt here.
	client, dctx, err := refClient(ctx, cmd)
	if err != nil {
		return err
	}
	return refDeleteExec(ctx, cmd, client, dctx)
}

// refDeleteExec does the actual work given an already-resolved client and
// context, split out from runRefDelete so tests can exercise it against an
// httptest server without going through ado.ResolveProject's org validation.
func refDeleteExec(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	name, _ := cmd.Flags().GetString("name")
	objectID, _ := cmd.Flags().GetString("object-id")

	if objectID == "" {
		// ref.py:78-84: filter by the raw (unresolved) name — no
		// refs/ prefixing here, unlike the POST body below.
		var refs []map[string]any
		if err := client.List(ctx, ado.Request{
			Scope:      dctx.Project,
			Path:       "git/repositories/" + url.PathEscape(dctx.Repo) + "/refs",
			APIVersion: "5.0",
			Query:      url.Values{"filter": {name}},
		}, &refs); err != nil {
			return fmt.Errorf("failed to look up reference: %w", err)
		}
		if len(refs) != 1 {
			return fmt.Errorf("Failed to find object_id for ref %s. Please provide object_id.", name)
		}
		objectID, _ = refs[0]["objectId"].(string)
	}

	body := []map[string]any{
		{
			"name":        refWithPrefix(name),
			"newObjectId": refZeroObjectID,
			"oldObjectId": objectID,
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
		return fmt.Errorf("failed to delete reference: %w", err)
	}
	if len(resp.Value) == 0 {
		return fmt.Errorf("failed to delete reference: empty response")
	}

	// ref.py:88-89: unlike create, delete never inspects response.success
	// — a logical failure is silently rendered as-is.
	return ado.Print(cmd, resp.Value[0], refColumns...)
}
