package vm

import (
  "context"

  "github.com/spf13/cobra"
)

func NewVMCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "vm",
    Short: "Manage virtual machines",
    Long:  "Commands to manage Azure virtual machines and related resources",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List virtual machines",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a virtual machine",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), name, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "VM name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a virtual machine",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Delete(context.Background(), name, resourceGroup, noWait)
    },
  }
  deleteCmd.Flags().StringP("name", "n", "", "VM name")
  deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
  deleteCmd.MarkFlagRequired("name")
  deleteCmd.MarkFlagRequired("resource-group")

  startCmd := &cobra.Command{
    Use:   "start",
    Short: "Start a virtual machine",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Start(context.Background(), name, resourceGroup, noWait)
    },
  }
  startCmd.Flags().StringP("name", "n", "", "VM name")
  startCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  startCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
  startCmd.MarkFlagRequired("name")
  startCmd.MarkFlagRequired("resource-group")

  stopCmd := &cobra.Command{
    Use:   "stop",
    Short: "Stop and deallocate a virtual machine",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Stop(context.Background(), name, resourceGroup, noWait)
    },
  }
  stopCmd.Flags().StringP("name", "n", "", "VM name")
  stopCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  stopCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
  stopCmd.MarkFlagRequired("name")
  stopCmd.MarkFlagRequired("resource-group")

  listSkusCmd := &cobra.Command{
    Use:   "list-skus",
    Short: "List available VM SKUs in a location",
    Long:  "List all available virtual machine SKUs and their capabilities in a specific Azure location",
    RunE: func(cmd *cobra.Command, args []string) error {
      location, _ := cmd.Flags().GetString("location")
      size, _ := cmd.Flags().GetString("size")
      resourceType, _ := cmd.Flags().GetString("resource-type")
      outputFormat, _ := cmd.Flags().GetString("output")
      return ListSKUs(context.Background(), location, size, resourceType, outputFormat)
    },
  }
  listSkusCmd.Flags().StringP("location", "l", "", "Azure location (e.g., westeurope, eastus)")
  listSkusCmd.Flags().String("size", "", "Filter by size (e.g., Standard_D4s_v3)")
  listSkusCmd.Flags().String("resource-type", "virtualMachines", "Resource type to query")
  listSkusCmd.Flags().StringP("output", "o", "table", "Output format: json, table")
  listSkusCmd.MarkFlagRequired("location")

  cmd.AddCommand(listCmd, showCmd, deleteCmd, startCmd, stopCmd, listSkusCmd)
  return cmd
}
