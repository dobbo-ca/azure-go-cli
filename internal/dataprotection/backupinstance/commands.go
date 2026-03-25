package backupinstance

import (
  "github.com/spf13/cobra"
)

func NewBackupInstanceCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "backup-instance",
    Short: "Manage backup instances",
    Long:  "Commands to manage backup instances within a backup vault",
  }

  restoreCmd := newRestoreCommand()

  cmd.AddCommand(
    restoreCmd,
    newValidateForRestoreCommand(),
    newAdhocBackupCommand(),
    newCreateCommand(),
    newShowCommand(),
    newListCommand(),
    newDeleteCommand(),
    newValidateForBackupCommand(),
    newStopProtectionCommand(),
    newSuspendBackupCommand(),
    newResumeProtectionCommand(),
  )
  return cmd
}

func newRestoreCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "restore",
    Short: "Restore backed up instances",
    Long:  "Commands to restore backed up instances from recovery points",
  }

  cmd.AddCommand(newRestoreTriggerCommand())
  return cmd
}
