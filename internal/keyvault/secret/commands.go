package secret

import (
  "context"

  "github.com/spf13/cobra"
)

func NewSecretCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "secret",
    Short: "Manage Key Vault secrets",
    Long:  "Commands to manage secrets in Azure Key Vault",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List secrets in a key vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      vaultName, _ := cmd.Flags().GetString("vault-name")
      return List(context.Background(), vaultName)
    },
  }
  listCmd.Flags().String("vault-name", "", "Key vault name")
  listCmd.MarkFlagRequired("vault-name")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show a secret from a key vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      showValue, _ := cmd.Flags().GetBool("show-value")
      return Show(context.Background(), cmd, vaultName, name, showValue)
    },
  }
  showCmd.Flags().String("vault-name", "", "Key vault name")
  showCmd.Flags().StringP("name", "n", "", "Secret name")
  showCmd.Flags().Bool("show-value", false, "Show the secret value (WARNING: displays sensitive data)")
  showCmd.MarkFlagRequired("vault-name")
  showCmd.MarkFlagRequired("name")

  setCmd := &cobra.Command{
    Use:   "set",
    Short: "Set a secret in a key vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      value, _ := cmd.Flags().GetString("value")
      tags, _ := cmd.Flags().GetStringToString("tags")
      return Set(context.Background(), cmd, vaultName, name, value, tags)
    },
  }
  setCmd.Flags().String("vault-name", "", "Key vault name")
  setCmd.Flags().StringP("name", "n", "", "Secret name")
  setCmd.Flags().String("value", "", "Secret value")
  setCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
  setCmd.MarkFlagRequired("vault-name")
  setCmd.MarkFlagRequired("name")
  setCmd.MarkFlagRequired("value")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a secret from a key vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      return Delete(context.Background(), vaultName, name)
    },
  }
  deleteCmd.Flags().String("vault-name", "", "Key vault name")
  deleteCmd.Flags().StringP("name", "n", "", "Secret name")
  deleteCmd.MarkFlagRequired("vault-name")
  deleteCmd.MarkFlagRequired("name")

  cmd.AddCommand(listCmd, showCmd, setCmd, deleteCmd)
  return cmd
}
