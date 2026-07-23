package longtermretention

import (
	"context"

	"github.com/spf13/cobra"
)

func NewLongTermRetentionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "long-term-retention",
		Short: "Manage long-term retention (LTR) backups for a PostgreSQL flexible server",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List long-term retention backup operations for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			return List(context.Background(), cmd, rg, server)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("server-name", "", "Flexible server name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("server-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a long-term retention backup operation for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("backup-name")
			return Show(context.Background(), cmd, rg, server, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.Flags().StringP("backup-name", "n", "", "Long-term retention backup name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")
	showCmd.MarkFlagRequired("backup-name")

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a long-term retention backup for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("backup-name")
			sasURL, _ := cmd.Flags().GetString("sas-url")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Start(context.Background(), cmd, rg, server, name, sasURL, noWait)
		},
	}
	startCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	startCmd.Flags().String("server-name", "", "Flexible server name")
	startCmd.Flags().StringP("backup-name", "n", "", "Long-term retention backup name")
	startCmd.Flags().String("sas-url", "", "SAS URL of the storage container where the backup is streamed")
	startCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	startCmd.MarkFlagRequired("resource-group")
	startCmd.MarkFlagRequired("server-name")
	startCmd.MarkFlagRequired("backup-name")
	startCmd.MarkFlagRequired("sas-url")

	cmd.AddCommand(listCmd, showCmd, startCmd)
	return cmd
}
