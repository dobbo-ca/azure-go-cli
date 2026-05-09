package flexibleserver

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/backup"
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
			return List(context.Background(), resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), serverName, resourceGroup)
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
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, adminUser, adminPassword, version, tier, sku, storageSizeGB, tags)
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
			return ListSKUs(context.Background(), location)
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

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, listSkusCmd, restoreCmd, geoRestoreCmd, backup.NewBackupCommand())
	return cmd
}
