package flexibleserver

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func serversClient() (*armpostgresqlflexibleservers.ServersClient, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL client: %w", err)
	}
	return client, nil
}

func Start(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
	client, err := serversClient()
	if err != nil {
		return err
	}

	fmt.Printf("Starting PostgreSQL flexible server '%s'...\n", name)
	poller, err := client.BeginStart(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin start: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Start of server '%s' started.", name)})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Server '%s' started.", name)})
}
