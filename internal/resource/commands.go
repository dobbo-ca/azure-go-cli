package resource

import (
  "github.com/spf13/cobra"
)

// NewResourceCommand returns the root `az resource` cobra command.
func NewResourceCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "resource",
    Short: "Manage Azure resources generically",
    Long:  "Generic ARM resource access (list, show, delete, tag, move, wait, create, update, invoke-action)",
  }

  cmd.AddCommand(
    newListCmd(),
    newShowCmd(),
    newDeleteCmd(),
    newTagCmd(),
    newMoveCmd(),
    newWaitCmd(),
    newCreateCmd(),
    newUpdateCmd(),
    newInvokeActionCmd(),
  )
  return cmd
}
