package vpngateway

import (
  "context"

  "github.com/spf13/cobra"
)

func NewVpnGatewayCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "vnet-gateway",
    Short: "Manage virtual network gateways",
    Long:  "Commands to manage Azure virtual network gateways (VPN/ExpressRoute)",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List virtual network gateways",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  listCmd.MarkFlagRequired("resource-group")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a virtual network gateway",
    RunE: func(cmd *cobra.Command, args []string) error {
      gatewayName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), gatewayName, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Gateway name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  createCmd := &cobra.Command{
    Use:   "create",
    Short: "Create a virtual network gateway",
    Long:  "Create a virtual network gateway (VPN or ExpressRoute). Note: This operation typically takes 30-45 minutes to complete.",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      location, _ := cmd.Flags().GetString("location")
      publicIPID, _ := cmd.Flags().GetString("public-ip-id")
      subnetID, _ := cmd.Flags().GetString("subnet-id")
      gatewayType, _ := cmd.Flags().GetString("gateway-type")
      vpnType, _ := cmd.Flags().GetString("vpn-type")
      skuName, _ := cmd.Flags().GetString("sku")
      tags, _ := cmd.Flags().GetStringToString("tags")
      return Create(context.Background(), cmd, name, resourceGroup, location, publicIPID, subnetID, gatewayType, vpnType, skuName, tags)
    },
  }
  createCmd.Flags().StringP("name", "n", "", "Gateway name")
  createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
  createCmd.Flags().String("public-ip-id", "", "Resource ID of the public IP address")
  createCmd.Flags().String("subnet-id", "", "Resource ID of the gateway subnet")
  createCmd.Flags().String("gateway-type", "Vpn", "Gateway type: Vpn or ExpressRoute")
  createCmd.Flags().String("vpn-type", "RouteBased", "VPN type: RouteBased or PolicyBased")
  createCmd.Flags().String("sku", "VpnGw1", "SKU: Basic, VpnGw1, VpnGw2, VpnGw3, VpnGw1AZ, VpnGw2AZ, VpnGw3AZ")
  createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
  createCmd.MarkFlagRequired("name")
  createCmd.MarkFlagRequired("resource-group")
  createCmd.MarkFlagRequired("location")
  createCmd.MarkFlagRequired("public-ip-id")
  createCmd.MarkFlagRequired("subnet-id")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a virtual network gateway",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Delete(context.Background(), name, resourceGroup, noWait)
    },
  }
  deleteCmd.Flags().StringP("name", "n", "", "Gateway name")
  deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  deleteCmd.MarkFlagRequired("name")
  deleteCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
  return cmd
}
