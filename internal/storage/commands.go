package storage

import (
	"github.com/cdobbyn/azure-go-cli/internal/storage/account"
	"github.com/cdobbyn/azure-go-cli/internal/storage/blob"
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
		blob.NewBlobCommand(),
		container.NewContainerCommand(),
	)
	return cmd
}
