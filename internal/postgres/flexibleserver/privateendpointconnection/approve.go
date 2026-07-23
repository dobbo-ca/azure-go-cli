package privateendpointconnection

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/spf13/cobra"
)

func Approve(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, connectionName, description string, noWait bool) error {
	return updateStatus(ctx, cmd, resourceGroup, serverName, connectionName, description, armpostgresqlflexibleservers.PrivateEndpointServiceConnectionStatusApproved, noWait)
}
