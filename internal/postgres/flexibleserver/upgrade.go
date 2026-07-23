package flexibleserver

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Upgrade(ctx context.Context, cmd *cobra.Command, name, resourceGroup, version string, noWait bool) error {
	client, err := serversClient()
	if err != nil {
		return err
	}

	update := armpostgresqlflexibleservers.ServerForUpdate{
		Properties: &armpostgresqlflexibleservers.ServerPropertiesForUpdate{
			Version: to.Ptr(armpostgresqlflexibleservers.ServerVersion(version)),
		},
	}

	fmt.Printf("Upgrading PostgreSQL flexible server '%s' to major version %s...\n", name, version)
	poller, err := client.BeginUpdate(ctx, resourceGroup, name, update, nil)
	if err != nil {
		return fmt.Errorf("failed to begin upgrade: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Upgrade of server '%s' started.", name)})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade server: %w", err)
	}
	return output.PrintJSON(cmd, resp.Server)
}
