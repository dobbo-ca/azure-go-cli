package devops

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newExtensionShowCmd() *cobra.Command {
	var publisherID, extensionID string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get details about a specific extension.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionShow(context.Background(), cmd, publisherID, extensionID)
		},
	}

	extensionAddIDFlags(cmd, &publisherID, &extensionID)
	ado.AddOrgFlags(cmd)

	return cmd
}

func runExtensionShow(ctx context.Context, cmd *cobra.Command, publisherID, extensionID string) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := extensionNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var extension map[string]any
	if err := client.Do(ctx, ado.Request{
		Host:       "extmgmt",
		Path:       extensionByNamePath(publisherID, extensionID),
		APIVersion: "5.0-preview.1",
	}, &extension); err != nil {
		return fmt.Errorf("failed to get extension: %w", err)
	}

	return ado.Print(cmd, extension, extensionColumns...)
}
