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
    Long:  "Show details of a managed identity by name and resource group, or by one or more resource IDs",
    RunE: func(cmd *cobra.Command, args []string) error {
      ids, _ := cmd.Flags().GetStringSlice("ids")
      subscription, _ := cmd.Flags().GetString("subscription")

      // If --ids is provided, use ShowByIDs
      if len(ids) > 0 {
        return ShowByIDs(context.Background(), cmd, ids, subscription)
      }

      // Otherwise use name and resource-group
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")

      if name == "" || resourceGroup == "" {
        return cmd.Usage()
      }

      return Show(context.Background(), cmd, name, resourceGroup, subscription)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Managed identity name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.Flags().StringSlice("ids", []string{}, "One or more resource IDs (space-separated)")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
