package virtualendpoint

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Delete(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, endpointName string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewVirtualEndpointsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual endpoints client: %w", err)
	}

	poller, err := client.BeginDelete(ctx, resourceGroup, serverName, endpointName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin virtual endpoint delete: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "Virtual endpoint delete started."})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("virtual endpoint delete failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Virtual endpoint '%s' deleted.", endpointName)})
}
