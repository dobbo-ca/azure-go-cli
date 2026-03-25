package backuppolicy

import "github.com/spf13/cobra"

func NewBackupPolicyCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "backup-policy",
    Short: "Manage backup policies",
    Long:  "Commands to manage backup policies within a backup vault",
  }
  cmd.AddCommand(
    newCreateCommand(),
    newShowCommand(),
    newListCommand(),
    newDeleteCommand(),
    newGetDefaultPolicyTemplateCommand(),
  )
  return cmd
}
