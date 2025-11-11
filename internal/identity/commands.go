package identity

import (
  "context"

  "github.com/spf13/cobra"
)

func NewIdentityCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "identity",
    Short: "Manage managed identities",
    Long:  "Commands to manage Azure managed identities",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List managed identities",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      subscription, _ := cmd.Flags().GetString("subscription")
      return List(context.Background(), resourceGroup, subscription)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a managed identity",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      subscription, _ := cmd.Flags().GetString("subscription")
      return Show(context.Background(), name, resourceGroup, subscription)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Managed identity name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
