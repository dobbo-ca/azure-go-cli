package serverlogs

import (
	"context"

	"github.com/spf13/cobra"
)

func NewServerLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server-logs",
		Short: "Manage server logs for a PostgreSQL flexible server",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List server log files for a PostgreSQL flexible server",
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

	downloadCmd := &cobra.Command{
		Use:   "download",
		Short: "Download server log files for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			names, _ := cmd.Flags().GetStringSlice("name")
			return Download(context.Background(), cmd, rg, server, names)
		},
	}
	downloadCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	downloadCmd.Flags().String("server-name", "", "Flexible server name")
	downloadCmd.Flags().StringSlice("name", nil, "Log file name(s) to download; downloads all if omitted")
	downloadCmd.MarkFlagRequired("resource-group")
	downloadCmd.MarkFlagRequired("server-name")

	cmd.AddCommand(listCmd, downloadCmd)
	return cmd
}
