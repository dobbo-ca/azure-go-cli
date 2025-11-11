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

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
