package account

import (
  "github.com/spf13/cobra"
)

func NewAccountCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "account",
    Short: "Manage Azure subscription information",
    Long:  "Commands to manage Azure subscriptions",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List all subscriptions for the logged in account",
    RunE: func(cmd *cobra.Command, args []string) error {
      return List()
    },
  }

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of the current/default subscription",
    RunE: func(cmd *cobra.Command, args []string) error {
      return Show()
    },
  }

  setCmd := &cobra.Command{
    Use:   "set",
    Short: "Set a subscription to be the current active subscription",
    RunE: func(cmd *cobra.Command, args []string) error {
      subscriptionID, _ := cmd.Flags().GetString("subscription")
      return Set(subscriptionID)
    },
  }
  setCmd.Flags().StringP("subscription", "s", "", "Subscription ID or name")
  setCmd.MarkFlagRequired("subscription")

  clearCmd := &cobra.Command{
    Use:   "clear",
    Short: "Clear all subscriptions from the CLI's local cache",
    RunE: func(cmd *cobra.Command, args []string) error {
      return Clear()
    },
  }

  cmd.AddCommand(listCmd, showCmd, setCmd, clearCmd)
  return cmd
}
