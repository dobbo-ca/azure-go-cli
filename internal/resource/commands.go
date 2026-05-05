package resource

import (
  "fmt"

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

// stub helper used by the per-subcommand files until each is implemented.
func notImplemented(name string) func(cmd *cobra.Command, args []string) error {
  return func(cmd *cobra.Command, args []string) error {
    return fmt.Errorf("az resource %s: not yet implemented", name)
  }
}
