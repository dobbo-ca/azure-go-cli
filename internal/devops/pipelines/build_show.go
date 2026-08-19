package pipelines

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newBuildShowCmd implements `az pipelines build show` (build_show,
// dev/pipelines/build.py:69).
func newBuildShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details of a build",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildRunShow(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the build.")
	cmd.Flags().Bool("open", false, "Open the build results page in your web browser.")
	cmd.MarkFlagRequired("id")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func buildRunShow(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return buildShow(ctx, cmd, client, dctx)
}

func buildShow(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	id, _ := cmd.Flags().GetInt("id")
	open, _ := cmd.Flags().GetBool("open")

	var build map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       fmt.Sprintf("build/Builds/%d", id),
		APIVersion: "5.0",
	}, &build); err != nil {
		return fmt.Errorf("failed to get build: %w", err)
	}

	if open {
		buildOpenInBrowser(buildBuildURL(dctx.Org, build))
	}

	return ado.Print(cmd, build, buildColumns...)
}
