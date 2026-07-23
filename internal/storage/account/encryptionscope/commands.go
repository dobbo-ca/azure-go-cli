package encryptionscope

import (
	"context"

	"github.com/spf13/cobra"
)

func NewEncryptionScopeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encryption-scope",
		Short: "Manage encryption scopes for a storage account",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an encryption scope",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			keySource, _ := cmd.Flags().GetString("key-source")
			keyURI, _ := cmd.Flags().GetString("key-uri")
			return Create(context.Background(), cmd, account, resourceGroup, name, keySource, keyURI)
		},
	}
	createCmd.Flags().String("account-name", "", "Storage account name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("name", "n", "", "Encryption scope name")
	createCmd.Flags().String("key-source", "Microsoft.Storage", "Microsoft.Storage or Microsoft.KeyVault")
	createCmd.Flags().String("key-uri", "", "Key URI, required when --key-source=Microsoft.KeyVault")
	createCmd.MarkFlagRequired("account-name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List encryption scopes for a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, account, resourceGroup)
		},
	}
	listCmd.Flags().String("account-name", "", "Storage account name")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("account-name")
	listCmd.MarkFlagRequired("resource-group")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show an encryption scope",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, account, resourceGroup, name)
		},
	}
	showCmd.Flags().String("account-name", "", "Storage account name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().StringP("name", "n", "", "Encryption scope name")
	showCmd.MarkFlagRequired("account-name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("name")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update an encryption scope",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			state, _ := cmd.Flags().GetString("state")
			return Update(context.Background(), cmd, account, resourceGroup, name, state)
		},
	}
	updateCmd.Flags().String("account-name", "", "Storage account name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().StringP("name", "n", "", "Encryption scope name")
	updateCmd.Flags().String("state", "", "Enabled or Disabled")
	updateCmd.MarkFlagRequired("account-name")
	updateCmd.MarkFlagRequired("resource-group")
	updateCmd.MarkFlagRequired("name")

	cmd.AddCommand(createCmd, listCmd, showCmd, updateCmd)

	return cmd
}
