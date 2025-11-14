package disk

import (
  "context"

  "github.com/cdobbyn/azure-go-cli/internal/disk/encryptionset"
  "github.com/spf13/cobra"
)

func NewDiskCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "disk",
    Short: "Manage managed disks",
    Long:  "Commands to manage Azure managed disks",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List managed disks",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a managed disk",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), cmd, name, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Disk name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  createCmd := &cobra.Command{
    Use:   "create",
    Short: "Create a managed disk",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      location, _ := cmd.Flags().GetString("location")
      sizeGB, _ := cmd.Flags().GetInt32("size-gb")
      sku, _ := cmd.Flags().GetString("sku")
      tags, _ := cmd.Flags().GetStringToString("tags")
      return Create(context.Background(), cmd, name, resourceGroup, location, sizeGB, sku, tags)
    },
  }
  createCmd.Flags().StringP("name", "n", "", "Disk name")
  createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
  createCmd.Flags().Int32("size-gb", 128, "Disk size in GB")
  createCmd.Flags().String("sku", "Premium_LRS", "SKU (Standard_LRS, Premium_LRS, StandardSSD_LRS, UltraSSD_LRS, Premium_ZRS, StandardSSD_ZRS)")
  createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
  createCmd.MarkFlagRequired("name")
  createCmd.MarkFlagRequired("resource-group")
  createCmd.MarkFlagRequired("location")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a managed disk",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Delete(context.Background(), name, resourceGroup, noWait)
    },
  }
  deleteCmd.Flags().StringP("name", "n", "", "Disk name")
  deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
  deleteCmd.MarkFlagRequired("name")
  deleteCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)

  // Add encryption-set as a subcommand to support "az disk encryption-set" syntax
  // This allows both "az disk-encryption-set" and "az disk encryption-set" to work
  encryptionSetCmd := encryptionset.NewEncryptionSetCommand()
  encryptionSetCmd.Use = "encryption-set"  // Override to remove "disk-" prefix
  cmd.AddCommand(encryptionSetCmd)

  return cmd
}
