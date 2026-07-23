package keys

import (
	"context"

	"github.com/spf13/cobra"
)

func NewKeysCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage storage account keys",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List the access keys for a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, account, resourceGroup)
		},
	}
	listCmd.Flags().StringP("account-name", "", "", "Storage account name")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("account-name")
	listCmd.MarkFlagRequired("resource-group")

	renewCmd := &cobra.Command{
		Use:   "renew",
		Short: "Regenerate an access key for a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			keyName, _ := cmd.Flags().GetString("key")
			return Renew(context.Background(), cmd, account, resourceGroup, keyName)
		},
	}
	renewCmd.Flags().StringP("account-name", "", "", "Storage account name")
	renewCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	renewCmd.Flags().String("key", "", "Key to renew: key1, key2, kerb1, kerb2")
	renewCmd.MarkFlagRequired("account-name")
	renewCmd.MarkFlagRequired("resource-group")
	renewCmd.MarkFlagRequired("key")

	cmd.AddCommand(listCmd, renewCmd)

	return cmd
}
