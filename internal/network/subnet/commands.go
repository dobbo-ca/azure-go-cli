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

  createCmd := &cobra.Command{
    Use:   "create",
    Short: "Create a subnet in a virtual network",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vnetName, _ := cmd.Flags().GetString("vnet-name")
      addressPrefix, _ := cmd.Flags().GetString("address-prefix")
      return Create(context.Background(), cmd, name, resourceGroup, vnetName, addressPrefix)
    },
  }
  createCmd.Flags().StringP("name", "n", "", "Subnet name")
  createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  createCmd.Flags().String("vnet-name", "", "Virtual network name")
  createCmd.Flags().String("address-prefix", "", "Address prefix in CIDR format (e.g., 10.0.1.0/24)")
  createCmd.MarkFlagRequired("name")
  createCmd.MarkFlagRequired("resource-group")
  createCmd.MarkFlagRequired("vnet-name")
  createCmd.MarkFlagRequired("address-prefix")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a subnet",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vnetName, _ := cmd.Flags().GetString("vnet-name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Delete(context.Background(), name, resourceGroup, vnetName, noWait)
    },
  }
  deleteCmd.Flags().StringP("name", "n", "", "Subnet name")
  deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  deleteCmd.Flags().String("vnet-name", "", "Virtual network name")
  deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  deleteCmd.MarkFlagRequired("name")
  deleteCmd.MarkFlagRequired("resource-group")
  deleteCmd.MarkFlagRequired("vnet-name")

  cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
  return cmd
}
