package storage

import (
  "github.com/cdobbyn/azure-go-cli/internal/storage/account"
  "github.com/cdobbyn/azure-go-cli/internal/storage/container"
  "github.com/spf13/cobra"
)

func NewStorageCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "storage",
    Short: "Manage Azure storage resources",
    Long:  "Commands to manage Azure storage accounts and containers",
  }

  cmd.AddCommand(
    account.NewAccountCommand(),
    container.NewContainerCommand(),
  )
  return cmd
}
