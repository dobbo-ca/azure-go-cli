package pipelines

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newBuildTagDeleteCmd implements `az pipelines build tag delete`
// (delete_build_tag, dev/pipelines/build.py:164). commands.py:125 registers
// no confirmation for this command — no --yes flag here, unlike most other
// delete commands in the surface.
func newBuildTagDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a build tag",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildRunTagDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("build-id", 0, "ID of the build.")
	cmd.MarkFlagRequired("build-id")
	cmd.Flags().String("tag", "", "Tag to be deleted from the build.")
	cmd.MarkFlagRequired("tag")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func buildRunTagDelete(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return buildTagDelete(ctx, cmd, client, dctx)
}

func buildTagDelete(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	buildID, _ := cmd.Flags().GetInt("build-id")
	tag, _ := cmd.Flags().GetString("tag")

	var result []string
	if err := client.List(ctx, ado.Request{
		Method:     "DELETE",
		Scope:      dctx.Project,
		Path:       fmt.Sprintf("build/builds/%d/tags/%s", buildID, url.PathEscape(tag)),
		APIVersion: "5.0",
	}, &result); err != nil {
		return fmt.Errorf("failed to delete build tag: %w", err)
	}

	return runsPrintTags(cmd, result)
}
