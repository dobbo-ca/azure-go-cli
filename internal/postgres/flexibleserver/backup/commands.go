package backup

import (
	"context"

	"github.com/spf13/cobra"
)

func NewBackupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage on-demand backups for a PostgreSQL flexible server",
		Long:  "Create, list, show, and delete on-demand backups for an Azure Database for PostgreSQL flexible server. Automated PITR backups are managed by the service and exposed via list/show.",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List backups for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			return List(context.Background(), rg, server)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("server-name", "", "Flexible server name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("server-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a backup for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), rg, server, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.Flags().StringP("name", "n", "", "Backup name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")
	showCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd)
	return cmd
}
