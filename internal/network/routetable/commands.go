package routetable

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRouteTableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route-table",
		Short: "Manage route tables",
		Long:  "Commands to manage Azure route tables and their routes",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List route tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, name, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Route table name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd)
	return cmd
}
