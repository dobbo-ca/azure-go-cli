package migration

import (
	"context"

	"github.com/spf13/cobra"
)

func NewMigrationCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migration",
		Short: "Manage PostgreSQL flexible server migrations",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List migrations for a target PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			targetServer, _ := cmd.Flags().GetString("target-server-name")
			return List(context.Background(), cmd, rg, targetServer)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Target resource group name")
	listCmd.Flags().String("target-server-name", "", "Target flexible server name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("target-server-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			targetServer, _ := cmd.Flags().GetString("target-server-name")
			name, _ := cmd.Flags().GetString("migration-name")
			return Show(context.Background(), cmd, rg, targetServer, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Target resource group name")
	showCmd.Flags().String("target-server-name", "", "Target flexible server name")
	showCmd.Flags().StringP("migration-name", "n", "", "Migration name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("target-server-name")
	showCmd.MarkFlagRequired("migration-name")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new migration to a target PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			targetServer, _ := cmd.Flags().GetString("target-server-name")
			name, _ := cmd.Flags().GetString("migration-name")
			location, _ := cmd.Flags().GetString("location")
			sourceID, _ := cmd.Flags().GetString("source-db-server-resource-id")
			dbs, _ := cmd.Flags().GetStringSlice("dbs")
			return Create(context.Background(), cmd, rg, targetServer, name, location, sourceID, dbs)
		},
	}
	createCmd.Flags().StringP("resource-group", "g", "", "Target resource group name")
	createCmd.Flags().String("target-server-name", "", "Target flexible server name")
	createCmd.Flags().StringP("migration-name", "n", "", "Migration name")
	createCmd.Flags().StringP("location", "l", "", "Location of the target flexible server")
	createCmd.Flags().String("source-db-server-resource-id", "", "Resource ID (or ipaddress:port@username) of the source database server")
	createCmd.Flags().StringSlice("dbs", nil, "Names of the databases to migrate")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("target-server-name")
	createCmd.MarkFlagRequired("migration-name")
	createCmd.MarkFlagRequired("location")
	createCmd.MarkFlagRequired("source-db-server-resource-id")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update an existing migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			targetServer, _ := cmd.Flags().GetString("target-server-name")
			name, _ := cmd.Flags().GetString("migration-name")
			setupLogical, _ := cmd.Flags().GetBool("setup-logical-replication-on-source")
			return Update(context.Background(), cmd, rg, targetServer, name, setupLogical)
		},
	}
	updateCmd.Flags().StringP("resource-group", "g", "", "Target resource group name")
	updateCmd.Flags().String("target-server-name", "", "Target flexible server name")
	updateCmd.Flags().StringP("migration-name", "n", "", "Migration name")
	updateCmd.Flags().Bool("setup-logical-replication-on-source", false, "Set up logical replication on the source database if needed")
	updateCmd.MarkFlagRequired("resource-group")
	updateCmd.MarkFlagRequired("target-server-name")
	updateCmd.MarkFlagRequired("migration-name")

	checkNameCmd := &cobra.Command{
		Use:   "check-name-availability",
		Short: "Check the availability of a migration name in a location",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			return CheckNameAvailability(context.Background(), cmd, name, location)
		},
	}
	checkNameCmd.Flags().String("name", "", "Resource name to check for availability")
	checkNameCmd.Flags().StringP("location", "l", "", "Location in which to check name availability")
	checkNameCmd.MarkFlagRequired("name")
	checkNameCmd.MarkFlagRequired("location")

	cmd.AddCommand(listCmd, showCmd, createCmd, updateCmd, checkNameCmd)
	return cmd
}
