package sharerm

import (
	"context"

	"github.com/spf13/cobra"
)

func NewShareRMCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share-rm",
		Short: "Manage Azure file shares using the Microsoft.Storage resource provider",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a file share",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			quota, _ := cmd.Flags().GetInt32("quota")
			accessTier, _ := cmd.Flags().GetString("access-tier")
			metadata, _ := cmd.Flags().GetStringToString("metadata")
			return Create(context.Background(), cmd, resourceGroup, account, name, quota, accessTier, metadata)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "File share name")
	createCmd.Flags().String("account-name", "", "Storage account name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().Int32("quota", 0, "Share quota in GiB")
	createCmd.Flags().String("access-tier", "", "TransactionOptimized, Hot, Cool, Premium")
	createCmd.Flags().StringToString("metadata", nil, "Metadata: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("account-name")
	createCmd.MarkFlagRequired("resource-group")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a file share",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Delete(context.Background(), cmd, resourceGroup, account, name)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "File share name")
	deleteCmd.Flags().String("account-name", "", "Storage account name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("account-name")
	deleteCmd.MarkFlagRequired("resource-group")

	existsCmd := &cobra.Command{
		Use:   "exists",
		Short: "Check whether a file share exists",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Exists(context.Background(), cmd, resourceGroup, account, name)
		},
	}
	existsCmd.Flags().StringP("name", "n", "", "File share name")
	existsCmd.Flags().String("account-name", "", "Storage account name")
	existsCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	existsCmd.MarkFlagRequired("name")
	existsCmd.MarkFlagRequired("account-name")
	existsCmd.MarkFlagRequired("resource-group")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List file shares",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, resourceGroup, account)
		},
	}
	listCmd.Flags().String("account-name", "", "Storage account name")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("account-name")
	listCmd.MarkFlagRequired("resource-group")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a file share",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, resourceGroup, account, name)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "File share name")
	showCmd.Flags().String("account-name", "", "Storage account name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("account-name")
	showCmd.MarkFlagRequired("resource-group")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a file share",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			accessTier, _ := cmd.Flags().GetString("access-tier")
			metadata, _ := cmd.Flags().GetStringToString("metadata")
			var quota *int32
			if cmd.Flags().Changed("quota") {
				v, _ := cmd.Flags().GetInt32("quota")
				quota = &v
			}
			return Update(context.Background(), cmd, resourceGroup, account, name, quota, accessTier, metadata)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "File share name")
	updateCmd.Flags().String("account-name", "", "Storage account name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().Int32("quota", 0, "Share quota in GiB")
	updateCmd.Flags().String("access-tier", "", "TransactionOptimized, Hot, Cool, Premium")
	updateCmd.Flags().StringToString("metadata", nil, "Metadata: key1=value1 key2=value2")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("account-name")
	updateCmd.MarkFlagRequired("resource-group")

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show usage statistics for a file share",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Stats(context.Background(), cmd, resourceGroup, account, name)
		},
	}
	statsCmd.Flags().StringP("name", "n", "", "File share name")
	statsCmd.Flags().String("account-name", "", "Storage account name")
	statsCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	statsCmd.MarkFlagRequired("name")
	statsCmd.MarkFlagRequired("account-name")
	statsCmd.MarkFlagRequired("resource-group")

	restoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a soft-deleted file share",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			deletedVersion, _ := cmd.Flags().GetString("deleted-version")
			return Restore(context.Background(), cmd, resourceGroup, account, name, deletedVersion)
		},
	}
	restoreCmd.Flags().StringP("name", "n", "", "File share name")
	restoreCmd.Flags().String("account-name", "", "Storage account name")
	restoreCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	restoreCmd.Flags().String("deleted-version", "", "Version of the deleted share to restore")
	restoreCmd.MarkFlagRequired("name")
	restoreCmd.MarkFlagRequired("account-name")
	restoreCmd.MarkFlagRequired("resource-group")
	restoreCmd.MarkFlagRequired("deleted-version")

	cmd.AddCommand(createCmd, deleteCmd, existsCmd, listCmd, showCmd, updateCmd, statsCmd, restoreCmd)

	return cmd
}
