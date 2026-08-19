package pipelines

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func releaseNewCreateCmd() *cobra.Command {
	var definitionID int
	var definitionName string
	var artifactMetadataList []string
	var description string
	var open bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Request (create) a release.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return releaseRunCreate(context.Background(), cmd, definitionID, definitionName, artifactMetadataList, description, open)
		},
	}

	cmd.Flags().IntVar(&definitionID, "definition-id", 0, "ID of the definition to create. Required if --definition-name is not supplied.")
	cmd.Flags().StringVar(&definitionName, "definition-name", "", "Name of the definition to create. Ignored if --definition-id is supplied.")
	cmd.Flags().StringSliceVar(&artifactMetadataList, "artifact-metadata-list", nil, `Space separated "alias=version_id" pairs.`)
	cmd.Flags().StringVar(&description, "description", "", "Description of the release.")
	cmd.Flags().BoolVar(&open, "open", false, "Open the release results page in your web browser.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func releaseRunCreate(ctx context.Context, cmd *cobra.Command, definitionID int, definitionName string, artifactMetadataList []string, description string, open bool) error {
	// release.py:19-38: org/project resolve first, THEN the id/name
	// presence check — order matters when both are missing, since a bad
	// --organization surfaces before the definition-id/name error.
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	if definitionID == 0 && definitionName == "" {
		return fmt.Errorf("Either the --definition-id argument or the --definition-name argument must be supplied for this command.")
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	if definitionID == 0 {
		id, err := releaseResolveDefinitionID(ctx, client, dctx.Project, definitionName)
		if err != nil {
			return err
		}
		definitionID = id
	}

	artifacts, err := releaseParseArtifactMetadata(artifactMetadataList)
	if err != nil {
		return err
	}

	release, err := releaseCreateRelease(ctx, client, dctx.Project, definitionID, artifacts, description)
	if err != nil {
		return fmt.Errorf("failed to create release: %w", err)
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
