package containerrm

import (
	"context"

	"github.com/spf13/cobra"
)

func NewContainerRMCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "container-rm",
		Short: "Manage Azure storage containers using the Microsoft.Storage resource provider",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a blob container",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			account, _ := cmd.Flags().GetString("account-name")
			name, _ := cmd.Flags().GetString("name")
			publicAccess, _ := cmd.Flags().GetString("public-access")
			metadata, _ := cmd.Flags().GetStringToString("metadata")
			return Create(context.Background(), cmd, resourceGroup, account, name, publicAccess, metadata)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Container name")
	createCmd.Flags().String("account-name", "", "Storage account name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("public-access", "", "Container public access: blob, container, or off (default)")
	createCmd.Flags().StringToString("metadata", nil, "Metadata: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("account-name")
	createCmd.MarkFlagRequired("resource-group")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a blob container",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			account, _ := cmd.Flags().GetString("account-name")
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), cmd, resourceGroup, account, name)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Container name")
	deleteCmd.Flags().String("account-name", "", "Storage account name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("account-name")
	deleteCmd.MarkFlagRequired("resource-group")

	existsCmd := &cobra.Command{
		Use:   "exists",
		Short: "Check whether a blob container exists",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			account, _ := cmd.Flags().GetString("account-name")
			name, _ := cmd.Flags().GetString("name")
			return Exists(context.Background(), cmd, resourceGroup, account, name)
		},
	}
	existsCmd.Flags().StringP("name", "n", "", "Container name")
	existsCmd.Flags().String("account-name", "", "Storage account name")
	existsCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	existsCmd.MarkFlagRequired("name")
	existsCmd.MarkFlagRequired("account-name")
	existsCmd.MarkFlagRequired("resource-group")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List blob containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			account, _ := cmd.Flags().GetString("account-name")
			return List(context.Background(), cmd, resourceGroup, account)
		},
	}
	listCmd.Flags().String("account-name", "", "Storage account name")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("account-name")
	listCmd.MarkFlagRequired("resource-group")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a blob container",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			account, _ := cmd.Flags().GetString("account-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, resourceGroup, account, name)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Container name")
	showCmd.Flags().String("account-name", "", "Storage account name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("account-name")
	showCmd.MarkFlagRequired("resource-group")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a blob container",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			account, _ := cmd.Flags().GetString("account-name")
			name, _ := cmd.Flags().GetString("name")
			publicAccess, _ := cmd.Flags().GetString("public-access")
			metadata, _ := cmd.Flags().GetStringToString("metadata")
			return Update(context.Background(), cmd, resourceGroup, account, name, publicAccess, metadata)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "Container name")
	updateCmd.Flags().String("account-name", "", "Storage account name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().String("public-access", "", "Container public access: blob, container, or off (default)")
	updateCmd.Flags().StringToString("metadata", nil, "Metadata: key1=value1 key2=value2")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("account-name")
	updateCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(createCmd, deleteCmd, existsCmd, listCmd, showCmd, updateCmd)

	return cmd
}
