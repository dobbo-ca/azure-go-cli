package pipelines

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newBuildCancelCmd implements `az pipelines build cancel` (build_cancel,
// dev/pipelines/build.py:126).
func newBuildCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a build",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildRunCancel(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("build-id", 0, "ID of the build.")
	cmd.Flags().Bool("open", false, "Open the build results page in your web browser.")
	cmd.MarkFlagRequired("build-id")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func buildRunCancel(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return buildCancel(ctx, cmd, client, dctx)
}

func buildCancel(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	buildID, _ := cmd.Flags().GetInt("build-id")
	open, _ := cmd.Flags().GetBool("open")

	// build.py:137 sends a minimal partial Build{status: "Cancelling"}, not
	// a fetched-then-mutated object — the API supports sparse PATCH bodies.
	body := map[string]any{"status": "Cancelling"}

	var build map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "PATCH",
		Scope:      dctx.Project,
		Path:       fmt.Sprintf("build/Builds/%d", buildID),
		APIVersion: "5.0",
		Body:       body,
	}, &build); err != nil {
		return fmt.Errorf("failed to cancel build: %w", err)
	}

	if open {
		buildOpenInBrowser(buildBuildURL(dctx.Org, build))
	}

	return ado.Print(cmd, build, buildColumns...)
}
