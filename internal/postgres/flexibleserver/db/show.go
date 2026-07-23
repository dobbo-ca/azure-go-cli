package db

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, databaseName string) error {
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

	resp, err := client.Get(ctx, resourceGroup, serverName, databaseName, nil)
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	return output.PrintJSON(cmd, resp.Database)
}
