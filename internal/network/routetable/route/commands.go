package route

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRouteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Manage routes within a route table",
		Long:  "Commands to manage individual routes within an Azure route table",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List routes in a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			routeTableName, _ := cmd.Flags().GetString("route-table-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, routeTableName, resourceGroup)
		},
	}
	listCmd.Flags().String("route-table-name", "", "Route table name")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("route-table-name")
	listCmd.MarkFlagRequired("resource-group")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a route",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			routeTableName, _ := cmd.Flags().GetString("route-table-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, name, routeTableName, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Route name")
	showCmd.Flags().String("route-table-name", "", "Route table name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("route-table-name")
	showCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd)
	return cmd
}
