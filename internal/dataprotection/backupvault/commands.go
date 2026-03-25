package backupvault

import "github.com/spf13/cobra"

func NewBackupVaultCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "backup-vault",
    Short: "Manage backup vaults",
    Long:  "Commands to manage Azure Data Protection backup vaults",
  }
  cmd.AddCommand(
    newCreateCommand(),
    newShowCommand(),
    newListCommand(),
    newUpdateCommand(),
    newDeleteCommand(),
  )
  return cmd
}
