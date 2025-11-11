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

  cmd.AddCommand(listSkusCmd)
  return cmd
}
