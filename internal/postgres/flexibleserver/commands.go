package flexibleserver

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/advancedthreatprotection"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/backup"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/db"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/entraadmin"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/firewallrule"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/identity"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/longtermretention"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/migration"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/parameter"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/privateendpointconnection"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/privatelinkresource"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/replica"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/serverlogs"
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/virtualendpoint"
	"github.com/spf13/cobra"
)

func NewFlexibleServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flexible-server",
		Short: "Manage Azure Database for PostgreSQL flexible servers",
		Long:  "Commands to manage Azure Database for PostgreSQL flexible servers",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List PostgreSQL flexible servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, serverName, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Server name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			adminUser, _ := cmd.Flags().GetString("admin-user")
			adminPassword, _ := cmd.Flags().GetString("admin-password")
			version, _ := cmd.Flags().GetString("version")
			tier, _ := cmd.Flags().GetString("tier")
			sku, _ := cmd.Flags().GetString("sku-name")
			storageSizeGB, _ := cmd.Flags().GetInt32("storage-size")
			backupRetention, _ := cmd.Flags().GetInt32("backup-retention")
			geoRedundant, _ := cmd.Flags().GetBool("geo-redundant-backup")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, adminUser, adminPassword, version, tier, sku, storageSizeGB, backupRetention, geoRedundant, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Server name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().String("admin-user", "", "Administrator username")
	createCmd.Flags().String("admin-password", "", "Administrator password")
	createCmd.Flags().String("version", "16", "PostgreSQL version (11, 12, 13, 14, 15, 16)")
	createCmd.Flags().String("tier", "Burstable", "Pricing tier (Burstable, GeneralPurpose, MemoryOptimized)")
	createCmd.Flags().String("sku-name", "Standard_B1ms", "SKU name (e.g., Standard_B1ms, Standard_D2s_v3)")
	createCmd.Flags().Int32("storage-size", 32, "Storage size in GB")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.Flags().Int32("backup-retention", 7, "Backup retention in days (7-35)")
	createCmd.Flags().Bool("geo-redundant-backup", false, "Enable geo-redundant backup (required for geo-restore)")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("location")
	createCmd.MarkFlagRequired("admin-user")
	createCmd.MarkFlagRequired("admin-password")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), name, resourceGroup, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Server name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")

	listSkusCmd := &cobra.Command{
		Use:   "list-skus",
		Short: "List available SKUs for PostgreSQL flexible servers in a location",
		RunE: func(cmd *cobra.Command, args []string) error {
			location, _ := cmd.Flags().GetString("location")
			return ListSKUs(context.Background(), cmd, location)
		},
	}
	listSkusCmd.Flags().StringP("location", "l", "", "Azure location (e.g., eastus, westus2)")
	listSkusCmd.MarkFlagRequired("location")

	restoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Point-in-time restore a PostgreSQL flexible server to a new server",
		Long:  "Creates a new PostgreSQL flexible server by performing a point-in-time restore from an existing source server. The source server must be running and within the configured backup retention window.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			sourceID, _ := cmd.Flags().GetString("source-server")
			restoreTime, _ := cmd.Flags().GetString("restore-time")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Restore(context.Background(), cmd, name, rg, location, sourceID, restoreTime, noWait)
		},
	}
	restoreCmd.Flags().StringP("name", "n", "", "Name of the new restored server")
	restoreCmd.Flags().StringP("resource-group", "g", "", "Resource group for the new server")
	restoreCmd.Flags().StringP("location", "l", "", "Location of the new server (must match source for PITR)")
	restoreCmd.Flags().String("source-server", "", "Full Azure resource ID of the source flexible server")
	restoreCmd.Flags().String("restore-time", "", "Point-in-time UTC, RFC3339 (e.g. 2026-05-08T14:30:00Z)")
	restoreCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	restoreCmd.MarkFlagRequired("name")
	restoreCmd.MarkFlagRequired("resource-group")
	restoreCmd.MarkFlagRequired("location")
	restoreCmd.MarkFlagRequired("source-server")
	restoreCmd.MarkFlagRequired("restore-time")

	geoRestoreCmd := &cobra.Command{
		Use:   "geo-restore",
		Short: "Geo-restore a PostgreSQL flexible server to a paired region",
		Long:  "Creates a new PostgreSQL flexible server in a paired region from the source server's geo-redundant backup. Source server must have geo-redundant backup enabled.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			sourceID, _ := cmd.Flags().GetString("source-server")
			restoreTime, _ := cmd.Flags().GetString("restore-time")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return GeoRestore(context.Background(), cmd, name, rg, location, sourceID, restoreTime, noWait)
		},
	}
	geoRestoreCmd.Flags().StringP("name", "n", "", "Name of the new restored server")
	geoRestoreCmd.Flags().StringP("resource-group", "g", "", "Resource group for the new server")
	geoRestoreCmd.Flags().StringP("location", "l", "", "Target location (paired region)")
	geoRestoreCmd.Flags().String("source-server", "", "Full Azure resource ID of the source flexible server")
	geoRestoreCmd.Flags().String("restore-time", "", "Optional point-in-time UTC RFC3339; defaults to latest available geo backup")
	geoRestoreCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	geoRestoreCmd.MarkFlagRequired("name")
	geoRestoreCmd.MarkFlagRequired("resource-group")
	geoRestoreCmd.MarkFlagRequired("location")
	geoRestoreCmd.MarkFlagRequired("source-server")

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a stopped PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Start(context.Background(), cmd, name, rg, noWait)
		},
	}
	startCmd.Flags().StringP("name", "n", "", "Server name")
	startCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	startCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	startCmd.MarkFlagRequired("name")
	startCmd.MarkFlagRequired("resource-group")

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a running PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Stop(context.Background(), cmd, name, rg, noWait)
		},
	}
	stopCmd.Flags().StringP("name", "n", "", "Server name")
	stopCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	stopCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	stopCmd.MarkFlagRequired("name")
	stopCmd.MarkFlagRequired("resource-group")

	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Restart(context.Background(), cmd, name, rg, noWait)
		},
	}
	restartCmd.Flags().StringP("name", "n", "", "Server name")
	restartCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	restartCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	restartCmd.MarkFlagRequired("name")
	restartCmd.MarkFlagRequired("resource-group")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a PostgreSQL flexible server",
		Long:  "Update tier, SKU, storage, backup retention, administrator password, or tags of a PostgreSQL flexible server. Only the fields you pass are changed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Update(context.Background(), cmd, name, rg, noWait)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "Server name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().String("tier", "", "Pricing tier (Burstable, GeneralPurpose, MemoryOptimized)")
	updateCmd.Flags().String("sku-name", "", "SKU name (e.g., Standard_B1ms, Standard_D2s_v3)")
	updateCmd.Flags().Int32("storage-size", 0, "Storage size in GB")
	updateCmd.Flags().Int32("backup-retention", 0, "Backup retention in days (7-35)")
	updateCmd.Flags().String("admin-password", "", "New administrator password")
	updateCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	updateCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("resource-group")

	upgradeCmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the major PostgreSQL version of a flexible server",
		Long:  "Perform a major version upgrade of a PostgreSQL flexible server. This is an in-place, irreversible operation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			version, _ := cmd.Flags().GetString("version")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Upgrade(context.Background(), cmd, name, rg, version, noWait)
		},
	}
	upgradeCmd.Flags().StringP("name", "n", "", "Server name")
	upgradeCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	upgradeCmd.Flags().String("version", "", "Target major PostgreSQL version (e.g., 14, 15, 16)")
	upgradeCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	upgradeCmd.MarkFlagRequired("name")
	upgradeCmd.MarkFlagRequired("resource-group")
	upgradeCmd.MarkFlagRequired("version")

	reviveCmd := &cobra.Command{
		Use:   "revive-dropped",
		Short: "Revive a dropped PostgreSQL flexible server",
		Long:  "Create a new PostgreSQL flexible server by reviving a recently dropped server from its backups.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			sourceID, _ := cmd.Flags().GetString("source-server")
			restoreTime, _ := cmd.Flags().GetString("restore-time")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return ReviveDropped(context.Background(), cmd, name, rg, location, sourceID, restoreTime, noWait)
		},
	}
	reviveCmd.Flags().StringP("name", "n", "", "Name of the revived server")
	reviveCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	reviveCmd.Flags().StringP("location", "l", "", "Location of the revived server")
	reviveCmd.Flags().String("source-server", "", "Full Azure resource ID of the dropped source server")
	reviveCmd.Flags().String("restore-time", "", "Optional point-in-time UTC, RFC3339 (e.g. 2026-05-08T14:30:00Z)")
	reviveCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	reviveCmd.MarkFlagRequired("name")
	reviveCmd.MarkFlagRequired("resource-group")
	reviveCmd.MarkFlagRequired("location")
	reviveCmd.MarkFlagRequired("source-server")

	waitCmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait until a PostgreSQL flexible server reaches a desired state",
		Long:  "Poll a PostgreSQL flexible server until it is ready (default/--created), exists, or is deleted.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			deleted, _ := cmd.Flags().GetBool("deleted")
			exists, _ := cmd.Flags().GetBool("exists")
			interval, _ := cmd.Flags().GetInt("interval")
			timeout, _ := cmd.Flags().GetInt("timeout")
			return Wait(context.Background(), cmd, name, rg, deleted, exists, interval, timeout)
		},
	}
	waitCmd.Flags().StringP("name", "n", "", "Server name")
	waitCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	waitCmd.Flags().Bool("created", false, "Wait until ready (default behavior)")
	waitCmd.Flags().Bool("deleted", false, "Wait until deleted")
	waitCmd.Flags().Bool("exists", false, "Wait until the server exists")
	waitCmd.Flags().Int("interval", 30, "Polling interval in seconds")
	waitCmd.Flags().Int("timeout", 3600, "Maximum wait time in seconds")
	waitCmd.MarkFlagRequired("name")
	waitCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(
		listCmd, showCmd, createCmd, deleteCmd, listSkusCmd, restoreCmd, geoRestoreCmd,
		startCmd, stopCmd, restartCmd, updateCmd, upgradeCmd, reviveCmd, waitCmd,
		newShowConnectionStringCmd(),
		backup.NewBackupCommand(),
		advancedthreatprotection.NewAdvancedThreatProtectionSettingCommand(),
		db.NewDBCommand(),
		entraadmin.NewMicrosoftEntraAdminCommand(),
		firewallrule.NewFirewallRuleCommand(),
		identity.NewIdentityCommand(),
		longtermretention.NewLongTermRetentionCommand(),
		migration.NewMigrationCommand(),
		parameter.NewParameterCommand(),
		privateendpointconnection.NewPrivateEndpointConnectionCommand(),
		privatelinkresource.NewPrivateLinkResourceCommand(),
		replica.NewReplicaCommand(),
		serverlogs.NewServerLogsCommand(),
		virtualendpoint.NewVirtualEndpointCommand(),
	)
	return cmd
}
