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

// Create provisions a new read replica of the given source flexible server.
func Create(ctx context.Context, cmd *cobra.Command, resourceGroup, replicaName, sourceServerID, location string, noWait bool) error {
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

	parameters := armpostgresqlflexibleservers.Server{
		Location: to.Ptr(location),
		Properties: &armpostgresqlflexibleservers.ServerProperties{
			CreateMode:             to.Ptr(armpostgresqlflexibleservers.CreateModeReplica),
			SourceServerResourceID: to.Ptr(sourceServerID),
		},
	}

	fmt.Printf("Creating read replica '%s' from %s...\n", replicaName, sourceServerID)
	poller, err := client.BeginCreate(ctx, resourceGroup, replicaName, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin replica create: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "replica create started"})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("replica create failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.Server)
}
