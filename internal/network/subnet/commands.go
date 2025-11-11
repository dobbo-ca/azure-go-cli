package subnet

import (
  "context"

  "github.com/spf13/cobra"
)

func NewSubnetCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "subnet",
    Short: "Manage subnets in virtual networks",
    Long:  "Commands to manage subnets within Azure virtual networks",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List subnets in a virtual network",
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
    Short: "Show details of a subnet",
    RunE: func(cmd *cobra.Command, args []string) error {
      vnetName, _ := cmd.Flags().GetString("vnet-name")
      subnetName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), vnetName, subnetName, resourceGroup)
    },
  }
  showCmd.Flags().String("vnet-name", "", "Virtual network name")
  showCmd.Flags().StringP("name", "n", "", "Subnet name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("vnet-name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
