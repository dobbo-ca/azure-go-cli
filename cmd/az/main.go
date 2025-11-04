package main

import (
  "fmt"
  "os"

  "github.com/cdobbyn/azure-go-cli/internal/account"
  "github.com/cdobbyn/azure-go-cli/internal/aks"
  "github.com/cdobbyn/azure-go-cli/internal/auth"
  "github.com/cdobbyn/azure-go-cli/internal/group"
  "github.com/cdobbyn/azure-go-cli/internal/identity"
  "github.com/cdobbyn/azure-go-cli/internal/keyvault"
  "github.com/cdobbyn/azure-go-cli/internal/network"
  "github.com/cdobbyn/azure-go-cli/internal/postgres"
  "github.com/cdobbyn/azure-go-cli/internal/quota"
  "github.com/cdobbyn/azure-go-cli/internal/storage"
  "github.com/cdobbyn/azure-go-cli/internal/vm"
  "github.com/spf13/cobra"
)

func main() {
  rootCmd := &cobra.Command{
    Use:   "az",
    Short: "Azure CLI implemented in Go",
    Long:  "A lightweight Azure CLI implementation in Go with core authentication and management commands",
  }

  // Add all domain commands
  rootCmd.AddCommand(
    auth.NewLoginCommand(),
    auth.NewLogoutCommand(),
    account.NewAccountCommand(),
    aks.NewAKSCommand(),
    group.NewGroupCommand(),
    identity.NewIdentityCommand(),
    network.NewNetworkCommand(),
    storage.NewStorageCommand(),
    postgres.NewPostgresCommand(),
    keyvault.NewKeyVaultCommand(),
    quota.NewQuotaCommand(),
    vm.NewVMCommand(),
  )

  if err := rootCmd.Execute(); err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
  }
}
