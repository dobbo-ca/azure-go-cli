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

// GeoRestore creates a new flexible server in a geo-paired region from the source server's geo-redundant backup.
func GeoRestore(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, sourceServerID, restoreTime string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	props := &armpostgresqlflexibleservers.ServerProperties{
		CreateMode:             to.Ptr(armpostgresqlflexibleservers.CreateModeGeoRestore),
		SourceServerResourceID: to.Ptr(sourceServerID),
	}

	if restoreTime != "" {
		t, err := time.Parse(time.RFC3339, restoreTime)
		if err != nil {
			return fmt.Errorf("invalid --restore-time %q: must be RFC3339: %w", restoreTime, err)
		}
		props.PointInTimeUTC = to.Ptr(t)
	}

	client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create servers client: %w", err)
	}

	parameters := armpostgresqlflexibleservers.Server{
		Location:   to.Ptr(location),
		Properties: props,
	}

	fmt.Printf("Geo-restoring '%s' in %s from %s...\n", name, location, sourceServerID)
	poller, err := client.BeginCreate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin geo-restore: %w", err)
	}

	if noWait {
		fmt.Println(`{"status": "Geo-restore started."}`)
		return nil
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("geo-restore failed: %w", err)
	}
	return output.PrintJSON(cmd, result.Server)
}
