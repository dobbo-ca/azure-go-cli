package db

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

func Create(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, databaseName, charset, collation string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewDatabasesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create databases client: %w", err)
	}

	parameters := armpostgresqlflexibleservers.Database{
		Properties: &armpostgresqlflexibleservers.DatabaseProperties{
			Charset:   to.Ptr(charset),
			Collation: to.Ptr(collation),
		},
	}

	fmt.Printf("Creating database '%s' on server '%s'...\n", databaseName, serverName)
	poller, err := client.BeginCreate(ctx, resourceGroup, serverName, databaseName, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "create started"})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("create operation failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.Database)
}
