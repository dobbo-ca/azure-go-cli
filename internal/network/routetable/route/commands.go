package route

import (
	"context"
	"fmt"

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

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a route in a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			routeTableName, _ := cmd.Flags().GetString("route-table-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			addressPrefix, _ := cmd.Flags().GetString("address-prefix")
			nextHopType, _ := cmd.Flags().GetString("next-hop-type")
			nextHopIP, _ := cmd.Flags().GetString("next-hop-ip-address")

			if err := ValidateNextHopType(nextHopType); err != nil {
				return err
			}
			if nextHopType == "VirtualAppliance" && nextHopIP == "" {
				return fmt.Errorf("--next-hop-ip-address is required when --next-hop-type is VirtualAppliance")
			}

			return Create(context.Background(), cmd, name, routeTableName, resourceGroup, addressPrefix, nextHopType, nextHopIP)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Route name")
	createCmd.Flags().String("route-table-name", "", "Route table name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("address-prefix", "", "Destination address prefix in CIDR format (e.g., 10.0.0.0/24)")
	createCmd.Flags().String("next-hop-type", "", "Next hop type: VirtualNetworkGateway, VnetLocal, Internet, VirtualAppliance, None")
	createCmd.Flags().String("next-hop-ip-address", "", "Next hop IP address (required when --next-hop-type is VirtualAppliance)")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("route-table-name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("address-prefix")
	createCmd.MarkFlagRequired("next-hop-type")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a route from a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			routeTableName, _ := cmd.Flags().GetString("route-table-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), name, routeTableName, resourceGroup, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Route name")
	deleteCmd.Flags().String("route-table-name", "", "Route table name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("route-table-name")
	deleteCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
	return cmd
}
