package devops

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newExtensionUninstallCmd() *cobra.Command {
	var publisherID, extensionID string

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall an extension.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionUninstall(context.Background(), cmd, publisherID, extensionID)
		},
	}

	extensionAddIDFlags(cmd, &publisherID, &extensionID)
	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func runExtensionUninstall(ctx context.Context, cmd *cobra.Command, publisherID, extensionID string) error {
	if err := ado.Confirm(cmd, "Are you sure you want to uninstall this extension?"); err != nil {
		return err
	}

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := extensionNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Host:       "extmgmt",
		Path:       extensionByNamePath(publisherID, extensionID),
		APIVersion: "5.0-preview.1",
	}, nil); err != nil {
		return fmt.Errorf("failed to uninstall extension: %w", err)
	}

	// No table_transformer is registered for uninstall (dev/team/commands.py:142)
	// and the SDK method returns nothing (extension_management_client.py:111-133)
	// — print nil, which Print falls through to JSON ("null") for, same as
	// azure-cli's default rendering of an empty result.
	return ado.Print(cmd, nil)
}
