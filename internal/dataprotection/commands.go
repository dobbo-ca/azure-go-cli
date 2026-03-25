package dataprotection

import (
  "github.com/cdobbyn/azure-go-cli/internal/dataprotection/backupinstance"
  "github.com/cdobbyn/azure-go-cli/internal/dataprotection/backuppolicy"
  "github.com/cdobbyn/azure-go-cli/internal/dataprotection/backupvault"
  "github.com/cdobbyn/azure-go-cli/internal/dataprotection/job"
  "github.com/cdobbyn/azure-go-cli/internal/dataprotection/recoverypoint"
  "github.com/spf13/cobra"
)

func NewDataProtectionCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "dataprotection",
    Short: "Manage Azure Data Protection",
    Long:  "Commands to manage Azure Data Protection backup vaults, policies, instances, and restore operations",
  }

  cmd.AddCommand(
    backupinstance.NewBackupInstanceCommand(),
    backuppolicy.NewBackupPolicyCommand(),
    backupvault.NewBackupVaultCommand(),
    job.NewJobCommand(),
    recoverypoint.NewRecoveryPointCommand(),
  )
  return cmd
}
