package pipelines

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func releaseNewListCmd() *cobra.Command {
	var definitionID int
	var minCreatedTime, maxCreatedTime, sourceBranch, status string
	var top int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List release results.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return releaseRunList(context.Background(), cmd, definitionID, minCreatedTime, maxCreatedTime, sourceBranch, status, top)
		},
	}

	cmd.Flags().IntVar(&definitionID, "definition-id", 0, "ID of definition to list releases for.")
	cmd.Flags().StringVar(&minCreatedTime, "min-created-time", "", "Releases that were created after this time.")
	cmd.Flags().StringVar(&maxCreatedTime, "max-created-time", "", "Releases that were created before this time.")
	cmd.Flags().StringVar(&sourceBranch, "source-branch", "", "Filter releases for this branch.")
	cmd.Flags().StringVar(&status, "status", "", "Limit to releases with this status.")
	cmd.Flags().IntVar(&top, "top", 0, "Maximum number of releases to list. Default is 50.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func releaseRunList(ctx context.Context, cmd *cobra.Command, definitionID int, minCreatedTime, maxCreatedTime, sourceBranch, status string, top int) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	releases, err := releaseListPage(ctx, client, ado.Request{
		Host:       releaseHost,
		Scope:      dctx.Project,
		Path:       "release/releases",
		APIVersion: "5.0",
		Query:      releaseListQuery(definitionID, minCreatedTime, maxCreatedTime, sourceBranch, status, top),
	})
	if err != nil {
		return fmt.Errorf("failed to list releases: %w", err)
	}

	return ado.Print(cmd, releases, releaseColumns...)
}
