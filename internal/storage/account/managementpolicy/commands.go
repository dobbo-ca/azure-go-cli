package managementpolicy

import (
	"context"

	"github.com/spf13/cobra"
)

func NewManagementPolicyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "management-policy",
		Short: "Manage data management policies for a storage account",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create or update the management policy for a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			policyFile, _ := cmd.Flags().GetString("policy")
			return Create(context.Background(), cmd, account, resourceGroup, policyFile)
		},
	}
	createCmd.Flags().String("account-name", "", "Storage account name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("policy", "", "Path to a JSON file with the management policy rules, e.g. {\"rules\":[...]}")
	createCmd.MarkFlagRequired("account-name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("policy")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show the management policy for a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, account, resourceGroup)
		},
	}
	showCmd.Flags().String("account-name", "", "Storage account name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("account-name")
	showCmd.MarkFlagRequired("resource-group")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the management policy for a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Delete(context.Background(), cmd, account, resourceGroup)
		},
	}
	deleteCmd.Flags().String("account-name", "", "Storage account name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.MarkFlagRequired("account-name")
	deleteCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(createCmd, showCmd, deleteCmd)

	return cmd
}
