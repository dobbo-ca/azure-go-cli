package networkrule

import (
	"context"

	"github.com/spf13/cobra"
)

func NewNetworkRuleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network-rule",
		Short: "Manage network rules of a storage account",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List the network rules of a storage account",
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

	cmd.AddCommand(listCmd)

	return cmd
}
