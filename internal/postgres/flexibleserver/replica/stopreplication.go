package replica

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// StopReplication breaks the replication link, turning the replica into an
// independent read-write server by setting its replication role to None.
func StopReplication(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create servers client: %w", err)
	}

	parameters := armpostgresqlflexibleservers.ServerForUpdate{
		Properties: &armpostgresqlflexibleservers.ServerPropertiesForUpdate{
			ReplicationRole: to.Ptr(armpostgresqlflexibleservers.ReplicationRoleNone),
		},
	}

	fmt.Printf("Stopping replication on '%s'...\n", serverName)
	poller, err := client.BeginUpdate(ctx, resourceGroup, serverName, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin stop-replication: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "stop-replication started"})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("stop-replication failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.Server)
}
