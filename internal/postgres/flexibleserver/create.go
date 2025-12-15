package flexibleserver

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

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, adminUser, adminPassword, version, tier, sku string, storageSizeGB int32, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL client: %w", err)
	}

	// Convert tags to Azure format
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	// Parse tier
	var skuTier armpostgresqlflexibleservers.SKUTier
	switch tier {
	case "Burstable":
		skuTier = armpostgresqlflexibleservers.SKUTierBurstable
	case "GeneralPurpose":
		skuTier = armpostgresqlflexibleservers.SKUTierGeneralPurpose
	case "MemoryOptimized":
		skuTier = armpostgresqlflexibleservers.SKUTierMemoryOptimized
	default:
		skuTier = armpostgresqlflexibleservers.SKUTierBurstable
	}

	// Parse version
	var serverVersion armpostgresqlflexibleservers.ServerVersion
	switch version {
	case "11":
		serverVersion = armpostgresqlflexibleservers.ServerVersionEleven
	case "12":
		serverVersion = armpostgresqlflexibleservers.ServerVersionTwelve
	case "13":
		serverVersion = armpostgresqlflexibleservers.ServerVersionThirteen
	case "14":
		serverVersion = armpostgresqlflexibleservers.ServerVersionFourteen
	case "15":
		serverVersion = armpostgresqlflexibleservers.ServerVersionFifteen
	case "16":
		serverVersion = armpostgresqlflexibleservers.ServerVersionSixteen
	default:
		serverVersion = armpostgresqlflexibleservers.ServerVersionSixteen
	}

	parameters := armpostgresqlflexibleservers.Server{
		Location: to.Ptr(location),
		Tags:     azureTags,
		SKU: &armpostgresqlflexibleservers.SKU{
			Name: to.Ptr(sku),
			Tier: to.Ptr(skuTier),
		},
		Properties: &armpostgresqlflexibleservers.ServerProperties{
			AdministratorLogin:         to.Ptr(adminUser),
			AdministratorLoginPassword: to.Ptr(adminPassword),
			Version:                    to.Ptr(serverVersion),
			Storage: &armpostgresqlflexibleservers.Storage{
				StorageSizeGB: to.Ptr(storageSizeGB),
			},
			Backup: &armpostgresqlflexibleservers.Backup{
				BackupRetentionDays: to.Ptr[int32](7),
				GeoRedundantBackup:  to.Ptr(armpostgresqlflexibleservers.GeoRedundantBackupEnumDisabled),
			},
			HighAvailability: &armpostgresqlflexibleservers.HighAvailability{
				Mode: to.Ptr(armpostgresqlflexibleservers.HighAvailabilityModeDisabled),
			},
		},
	}

	fmt.Printf("Creating PostgreSQL flexible server '%s'...\n", name)
	poller, err := client.BeginCreate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create PostgreSQL server: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL server: %w", err)
	}

	return output.PrintJSON(cmd, result.Server)
}
