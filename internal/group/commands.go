package group

import (
  "context"

  "github.com/spf13/cobra"
)

func NewGroupCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "group",
    Short: "Manage resource groups",
    Long:  "Commands to manage Azure resource groups",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List resource groups",
    RunE: func(cmd *cobra.Command, args []string) error {
      return List(context.Background())
    },
  }

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a resource group",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      return Show(context.Background(), name)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Resource group name")
  showCmd.MarkFlagRequired("name")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
