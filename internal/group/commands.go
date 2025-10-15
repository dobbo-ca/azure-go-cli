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

  cmd.AddCommand(listCmd)
  return cmd
}
