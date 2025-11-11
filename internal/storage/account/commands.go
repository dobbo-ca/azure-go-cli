package account

import (
  "context"

  "github.com/spf13/cobra"
)

func NewAccountCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "account",
    Short: "Manage Azure storage accounts",
    Long:  "Commands to manage Azure storage accounts",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List storage accounts",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a storage account",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), accountName, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Storage account name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
