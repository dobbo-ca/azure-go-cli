package pipelines

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newBuildTagListCmd implements `az pipelines build tag list` (get_build_tags,
// dev/pipelines/build.py:180).
func newBuildTagListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Get tags for a build",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildRunTagList(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("build-id", 0, "ID of the build.")
	cmd.MarkFlagRequired("build-id")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func buildRunTagList(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return buildTagList(ctx, cmd, client, dctx)
}

func buildTagList(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	buildID, _ := cmd.Flags().GetInt("build-id")

	var tags []string
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       fmt.Sprintf("build/builds/%d/tags", buildID),
		APIVersion: "5.0",
	}, &tags); err != nil {
		return fmt.Errorf("failed to get build tags: %w", err)
	}

	return runsPrintTags(cmd, tags)
}
