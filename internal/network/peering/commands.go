package peering

import (
	"context"

	"github.com/spf13/cobra"
)

func NewPeeringCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vnet-peering",
		Short: "Manage virtual network peerings",
		Long:  "Commands to manage peering connections between Azure virtual networks",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List virtual network peerings",
		RunE: func(cmd *cobra.Command, args []string) error {
			vnetName, _ := cmd.Flags().GetString("vnet-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), vnetName, resourceGroup)
		},
	}
	listCmd.Flags().String("vnet-name", "", "Virtual network name")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("vnet-name")
	listCmd.MarkFlagRequired("resource-group")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a virtual network peering",
		RunE: func(cmd *cobra.Command, args []string) error {
			vnetName, _ := cmd.Flags().GetString("vnet-name")
			peeringName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), vnetName, peeringName, resourceGroup)
		},
	}
	showCmd.Flags().String("vnet-name", "", "Virtual network name")
	showCmd.Flags().StringP("name", "n", "", "Peering name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("vnet-name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a virtual network peering",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			vnetName, _ := cmd.Flags().GetString("vnet-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			remoteVNetID, _ := cmd.Flags().GetString("remote-vnet-id")
			allowVNetAccess, _ := cmd.Flags().GetBool("allow-vnet-access")
			allowForwardedTraffic, _ := cmd.Flags().GetBool("allow-forwarded-traffic")
			allowGatewayTransit, _ := cmd.Flags().GetBool("allow-gateway-transit")
			useRemoteGateways, _ := cmd.Flags().GetBool("use-remote-gateways")
			return Create(context.Background(), cmd, name, vnetName, resourceGroup, remoteVNetID, allowVNetAccess, allowForwardedTraffic, allowGatewayTransit, useRemoteGateways)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Peering name")
	createCmd.Flags().String("vnet-name", "", "Virtual network name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("remote-vnet-id", "", "Resource ID of the remote virtual network")
	createCmd.Flags().Bool("allow-vnet-access", true, "Allow access from the local VNet to the remote VNet")
	createCmd.Flags().Bool("allow-forwarded-traffic", false, "Allow forwarded traffic from the remote VNet")
	createCmd.Flags().Bool("allow-gateway-transit", false, "Allow gateway transit")
	createCmd.Flags().Bool("use-remote-gateways", false, "Use the remote VNet's gateway")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("vnet-name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("remote-vnet-id")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a virtual network peering",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			vnetName, _ := cmd.Flags().GetString("vnet-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), name, vnetName, resourceGroup, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Peering name")
	deleteCmd.Flags().String("vnet-name", "", "Virtual network name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("vnet-name")
	deleteCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
	return cmd
}
