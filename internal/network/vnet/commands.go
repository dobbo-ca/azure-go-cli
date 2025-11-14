package vnet

import (
  "context"

  "github.com/spf13/cobra"
)

func NewVNetCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "vnet",
    Short: "Manage Azure virtual networks",
    Long:  "Commands to manage Azure virtual networks (VNets)",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List virtual networks",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a virtual network",
    RunE: func(cmd *cobra.Command, args []string) error {
      vnetName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), vnetName, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Virtual network name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a virtual network",
    RunE: func(cmd *cobra.Command, args []string) error {
      vnetName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Delete(context.Background(), vnetName, resourceGroup, noWait)
    },
  }
  deleteCmd.Flags().StringP("name", "n", "", "Virtual network name")
  deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  deleteCmd.MarkFlagRequired("name")
  deleteCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd, deleteCmd)
  return cmd
}
