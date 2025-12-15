package account

import (
	"context"

	"github.com/spf13/cobra"
)

func NewAccountCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage Azure storage accounts",
		Long:  "Commands to manage Azure storage accounts",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List storage accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			accountName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), accountName, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Storage account name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			sku, _ := cmd.Flags().GetString("sku")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, sku, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Storage account name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().String("sku", "Standard_LRS", "SKU (Standard_LRS, Standard_GRS, Standard_RAGRS, Standard_ZRS, Premium_LRS, Premium_ZRS, Standard_GZRS, Standard_RAGZRS)")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("location")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Delete(context.Background(), name, resourceGroup)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Storage account name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
	return cmd
}
