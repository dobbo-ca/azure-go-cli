package recoverypoint

import "github.com/spf13/cobra"

func NewRecoveryPointCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "recovery-point",
    Short: "Manage recovery points",
    Long:  "Commands to manage recovery points for backup instances",
  }
  cmd.AddCommand(newListCommand(), newShowCommand())
  return cmd
}
