package devops

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newExtensionInstallCmd() *cobra.Command {
	var publisherID, extensionID string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install an extension.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionInstall(context.Background(), cmd, publisherID, extensionID)
		},
	}

	extensionAddIDFlags(cmd, &publisherID, &extensionID)
	ado.AddOrgFlags(cmd)

	return cmd
}

func runExtensionInstall(ctx context.Context, cmd *cobra.Command, publisherID, extensionID string) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := extensionNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// install_extension_by_name never passes a version (extension.py:125-131),
	// so the SDK's optional {version} route segment is omitted here too.
	var extension map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Host:       "extmgmt",
		Path:       extensionByNamePath(publisherID, extensionID),
		APIVersion: "5.0-preview.1",
	}, &extension); err != nil {
		return fmt.Errorf("failed to install extension: %w", err)
	}

	return ado.Print(cmd, extension, extensionColumns...)
}
