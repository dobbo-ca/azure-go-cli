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

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
