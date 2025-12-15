package nsg

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/network/nsg/rule"
	"github.com/spf13/cobra"
)

func NewNsgCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nsg",
		Short: "Manage network security groups",
		Long:  "Commands to manage Azure network security groups",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List network security groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a network security group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, name, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "NSG name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a network security group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "NSG name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("location")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a network security group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), name, resourceGroup, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "NSG name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, rule.NewRuleCommand())
	return cmd
}
