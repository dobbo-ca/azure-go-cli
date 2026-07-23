package privateendpointconnection

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

func updateStatus(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, connectionName, description string, status armpostgresqlflexibleservers.PrivateEndpointServiceConnectionStatus, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewPrivateEndpointConnectionClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create private endpoint connection client: %w", err)
	}

	state := &armpostgresqlflexibleservers.PrivateLinkServiceConnectionState{
		Status: to.Ptr(status),
	}
	if description != "" {
		state.Description = to.Ptr(description)
	}

	parameters := armpostgresqlflexibleservers.PrivateEndpointConnection{
		Properties: &armpostgresqlflexibleservers.PrivateEndpointConnectionProperties{
			PrivateLinkServiceConnectionState: state,
		},
	}

	fmt.Printf("Setting private endpoint connection '%s' to %s...\n", connectionName, *state.Status)

	poller, err := client.BeginUpdate(ctx, resourceGroup, serverName, connectionName, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin private endpoint connection update: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Private endpoint connection update to %s started.", *state.Status)})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("private endpoint connection update failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.PrivateEndpointConnection)
}
