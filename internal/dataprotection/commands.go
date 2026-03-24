package dataprotection

import (
  "github.com/spf13/cobra"
)

func NewDataProtectionCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "dataprotection",
    Short: "Manage Azure Data Protection",
    Long:  "Commands to manage Azure Data Protection backup vaults, policies, instances, and restore operations",
  }

  return cmd
}
