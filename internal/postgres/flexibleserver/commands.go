package flexibleserver

import (
	"context"

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

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
	return cmd
}
