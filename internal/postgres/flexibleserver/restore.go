package flexibleserver

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// Restore creates a new flexible server from a point-in-time restore of an existing source server.
// restoreTime must be RFC3339 (e.g. 2026-05-08T14:30:00Z).
func Restore(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, sourceServerID, restoreTime string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	t, err := time.Parse(time.RFC3339, restoreTime)
	if err != nil {
		return fmt.Errorf("invalid --restore-time %q: must be RFC3339 (e.g. 2026-05-08T14:30:00Z): %w", restoreTime, err)
	}

	client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create servers client: %w", err)
	}

	parameters := armpostgresqlflexibleservers.Server{
		Location: to.Ptr(location),
		Properties: &armpostgresqlflexibleservers.ServerProperties{
			CreateMode:             to.Ptr(armpostgresqlflexibleservers.CreateModePointInTimeRestore),
			SourceServerResourceID: to.Ptr(sourceServerID),
			PointInTimeUTC:         to.Ptr(t),
		},
	}

	fmt.Printf("Restoring '%s' to point-in-time %s from %s...\n", name, t.Format(time.RFC3339), sourceServerID)
	poller, err := client.BeginCreate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin PITR restore: %w", err)
	}

	if noWait {
		fmt.Println(`{"status": "PITR restore started."}`)
		return nil
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("PITR restore failed: %w", err)
	}
	return output.PrintJSON(cmd, result.Server)
}
