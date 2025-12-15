package keyvault

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/keyvault/secret"
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

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Key vault name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("location")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Delete(context.Background(), name, resourceGroup)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Key vault name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, secret.NewSecretCommand())
	return cmd
}
