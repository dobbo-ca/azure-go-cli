package keyvault

import (
  "context"

  "github.com/spf13/cobra"
)

func NewKeyVaultCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "keyvault",
    Short: "Manage Azure Key Vault",
    Long:  "Commands to manage Azure Key Vault instances",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List key vaults",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a key vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      vaultName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), vaultName, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Key vault name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
