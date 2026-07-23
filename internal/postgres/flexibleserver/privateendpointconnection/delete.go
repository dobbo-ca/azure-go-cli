package privateendpointconnection

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Delete(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, connectionName string, noWait bool) error {
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

	fmt.Printf("Deleting private endpoint connection '%s'...\n", connectionName)

	poller, err := client.BeginDelete(ctx, resourceGroup, serverName, connectionName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin private endpoint connection delete: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "Private endpoint connection delete started."})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("private endpoint connection delete failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Private endpoint connection '%s' deleted.", connectionName)})
}
