package migration

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

func Create(ctx context.Context, cmd *cobra.Command, resourceGroup, targetServerName, migrationName, location, sourceDBServerResourceID string, dbs []string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armpostgresqlflexibleservers.NewMigrationsClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create migrations client: %w", err)
	}

	var dbsToMigrate []*string
	for _, db := range dbs {
		dbsToMigrate = append(dbsToMigrate, to.Ptr(db))
	}

	parameters := armpostgresqlflexibleservers.MigrationResource{
		Location: to.Ptr(location),
		Properties: &armpostgresqlflexibleservers.MigrationResourceProperties{
			SourceDbServerResourceID: to.Ptr(sourceDBServerResourceID),
			DbsToMigrate:             dbsToMigrate,
		},
	}

	resp, err := client.Create(ctx, subscriptionID, resourceGroup, targetServerName, migrationName, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to create migration: %w", err)
	}
	return output.PrintJSON(cmd, resp.MigrationResource)
}
