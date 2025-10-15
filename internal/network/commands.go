package network

import (
  "github.com/cdobbyn/azure-go-cli/internal/network/bastion"
  "github.com/spf13/cobra"
)

func NewNetworkCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "network",
    Short: "Manage Azure network resources",
    Long:  "Commands to manage Azure network resources",
  }

  cmd.AddCommand(bastion.NewBastionCommand())
  return cmd
}
