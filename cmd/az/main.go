package main

import (
  "fmt"
  "os"

  "github.com/cdobbyn/azure-go-cli/internal/account"
  "github.com/cdobbyn/azure-go-cli/internal/aks"
  "github.com/cdobbyn/azure-go-cli/internal/auth"
  "github.com/cdobbyn/azure-go-cli/internal/group"
  "github.com/cdobbyn/azure-go-cli/internal/network"
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
    network.NewNetworkCommand(),
  )

  if err := rootCmd.Execute(); err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
  }
}
