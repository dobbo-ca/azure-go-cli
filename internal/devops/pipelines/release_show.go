package pipelines

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func releaseNewShowCmd() *cobra.Command {
	var id int
	var open bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details of a release.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return releaseRunShow(context.Background(), cmd, id, open)
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "ID of the release.")
	cmd.Flags().BoolVar(&open, "open", false, "Open the release results page in your web browser.")
	cmd.MarkFlagRequired("id")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func releaseRunShow(ctx context.Context, cmd *cobra.Command, id int, open bool) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var release map[string]any
	if err := client.Do(ctx, ado.Request{
		Host:       releaseHost,
		Scope:      dctx.Project,
		Path:       fmt.Sprintf("release/releases/%d", id),
		APIVersion: "5.0",
	}, &release); err != nil {
		return fmt.Errorf("failed to get release: %w", err)
	}

	if open {
		if u := releaseWebURL(release); u != "" {
			if err := ado.OpenBrowser(u); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to open browser: %v\n", err)
			}
		}
	}

	return ado.Print(cmd, release, releaseColumns...)
}
