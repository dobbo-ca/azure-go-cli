package flexibleserver

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
	client, err := serversClient()
	if err != nil {
		return err
	}

	flags := cmd.Flags()
	props := &armpostgresqlflexibleservers.ServerPropertiesForUpdate{}
	update := armpostgresqlflexibleservers.ServerForUpdate{Properties: props}

	if flags.Changed("sku-name") || flags.Changed("tier") {
		sku := &armpostgresqlflexibleservers.SKU{}
		if flags.Changed("sku-name") {
			v, _ := flags.GetString("sku-name")
			sku.Name = to.Ptr(v)
		}
		if flags.Changed("tier") {
			v, _ := flags.GetString("tier")
			sku.Tier = to.Ptr(armpostgresqlflexibleservers.SKUTier(v))
		}
		update.SKU = sku
	}
	if flags.Changed("storage-size") {
		v, _ := flags.GetInt32("storage-size")
		props.Storage = &armpostgresqlflexibleservers.Storage{StorageSizeGB: to.Ptr(v)}
	}
	if flags.Changed("backup-retention") {
		v, _ := flags.GetInt32("backup-retention")
		props.Backup = &armpostgresqlflexibleservers.Backup{BackupRetentionDays: to.Ptr(v)}
	}
	if flags.Changed("admin-password") {
		v, _ := flags.GetString("admin-password")
		props.AdministratorLoginPassword = to.Ptr(v)
	}
	if flags.Changed("tags") {
		tags, _ := flags.GetStringToString("tags")
		azureTags := make(map[string]*string)
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		update.Tags = azureTags
	}

	fmt.Printf("Updating PostgreSQL flexible server '%s'...\n", name)
	poller, err := client.BeginUpdate(ctx, resourceGroup, name, update, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Update of server '%s' started.", name)})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}
	return output.PrintJSON(cmd, resp.Server)
}
