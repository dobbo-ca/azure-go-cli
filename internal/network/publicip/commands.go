package publicip

import (
  "context"

  "github.com/spf13/cobra"
)

func NewPublicIPCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "public-ip",
    Short: "Manage public IP addresses",
    Long:  "Commands to manage Azure public IP addresses",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List public IP addresses",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a public IP address",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), cmd, name, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Public IP name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  createCmd := &cobra.Command{
    Use:   "create",
    Short: "Create a public IP address",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      location, _ := cmd.Flags().GetString("location")
      allocationMethod, _ := cmd.Flags().GetString("allocation-method")
      sku, _ := cmd.Flags().GetString("sku")
      tags, _ := cmd.Flags().GetStringToString("tags")
      return Create(context.Background(), cmd, name, resourceGroup, location, allocationMethod, sku, tags)
    },
  }
  createCmd.Flags().StringP("name", "n", "", "Public IP name")
  createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
  createCmd.Flags().String("allocation-method", "Static", "IP allocation method (Static or Dynamic)")
  createCmd.Flags().String("sku", "Standard", "SKU (Basic or Standard)")
  createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
  createCmd.MarkFlagRequired("name")
  createCmd.MarkFlagRequired("resource-group")
  createCmd.MarkFlagRequired("location")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a public IP address",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Delete(context.Background(), name, resourceGroup, noWait)
    },
  }
  deleteCmd.Flags().StringP("name", "n", "", "Public IP name")
  deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
  deleteCmd.MarkFlagRequired("name")
  deleteCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
  return cmd
}
