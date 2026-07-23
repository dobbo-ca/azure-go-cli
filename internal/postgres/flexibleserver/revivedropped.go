package flexibleserver

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ReviveDropped(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, sourceServerID, restoreTime string, noWait bool) error {
	client, err := serversClient()
	if err != nil {
		return err
	}

	props := &armpostgresqlflexibleservers.ServerProperties{
		CreateMode:             to.Ptr(armpostgresqlflexibleservers.CreateModeReviveDropped),
		SourceServerResourceID: to.Ptr(sourceServerID),
	}
	if restoreTime != "" {
		t, err := time.Parse(time.RFC3339, restoreTime)
		if err != nil {
			return fmt.Errorf("invalid --restore-time %q, expected RFC3339 (e.g. 2026-05-08T14:30:00Z): %w", restoreTime, err)
		}
		props.PointInTimeUTC = to.Ptr(t)
	}

	params := armpostgresqlflexibleservers.Server{
		Location:   to.Ptr(location),
		Properties: props,
	}

	fmt.Printf("Reviving dropped PostgreSQL flexible server as '%s'...\n", name)
	poller, err := client.BeginCreate(ctx, resourceGroup, name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to begin revive: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Revive of server '%s' started.", name)})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to revive server: %w", err)
	}
	return output.PrintJSON(cmd, resp.Server)
}
