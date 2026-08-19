package pipelines

import (
	"context"
	"fmt"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func releaseDefinitionNewListCmd() *cobra.Command {
	var name string
	var top int
	var artifactType string
	var artifactSourceID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List release definitions.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return releaseDefinitionRunList(context.Background(), cmd, name, top, artifactType, artifactSourceID)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", `Limit results to definitions with this name or contains this name. Example: "FabCI"`)
	cmd.Flags().IntVar(&top, "top", 0, "Maximum number of definitions to list.")
	cmd.Flags().StringVar(&artifactType, "artifact-type", "", "Release definitions with given artifactType will be returned.")
	cmd.Flags().StringVar(&artifactSourceID, "artifact-source-id", "", "Limit results to definitions associated with this artifact_source_id.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func releaseDefinitionRunList(ctx context.Context, cmd *cobra.Command, name string, top int, artifactType, artifactSourceID string) error {
	// arguments.py:57-59: choices validated + str.lower()'d at parse time,
	// before org/project resolution even runs.
	if artifactType != "" {
		artifactType = strings.ToLower(artifactType)
		if !releaseValidArtifactType(artifactType) {
			return fmt.Errorf("--artifact-type must be one of: %s", strings.Join(releaseArtifactTypes, ", "))
		}
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	defs, err := releaseListPage(ctx, client, ado.Request{
		Host:       releaseHost,
		Scope:      dctx.Project,
		Path:       "release/definitions",
		APIVersion: "5.0",
		Query:      releaseDefinitionListQuery(name, top, artifactType, artifactSourceID),
	})
	if err != nil {
		return fmt.Errorf("failed to list release definitions: %w", err)
	}

	return ado.Print(cmd, defs, releaseDefinitionColumns...)
}
