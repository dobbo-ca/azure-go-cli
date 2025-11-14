package privateendpoint

import (
  "context"

  "github.com/spf13/cobra"
)

func NewPrivateEndpointCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "private-endpoint",
    Short: "Manage private endpoints",
    Long:  "Commands to manage Azure private endpoints",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List private endpoints",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a private endpoint",
    RunE: func(cmd *cobra.Command, args []string) error {
      endpointName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), endpointName, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Private endpoint name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  createCmd := &cobra.Command{
    Use:   "create",
    Short: "Create a private endpoint",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      location, _ := cmd.Flags().GetString("location")
      subnetID, _ := cmd.Flags().GetString("subnet-id")
      privateLinkResourceID, _ := cmd.Flags().GetString("private-link-resource-id")
      groupID, _ := cmd.Flags().GetString("group-id")
      tags, _ := cmd.Flags().GetStringToString("tags")
      return Create(context.Background(), cmd, name, resourceGroup, location, subnetID, privateLinkResourceID, groupID, tags)
    },
  }
  createCmd.Flags().StringP("name", "n", "", "Private endpoint name")
  createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
  createCmd.Flags().String("subnet-id", "", "Resource ID of the subnet")
  createCmd.Flags().String("private-link-resource-id", "", "Resource ID of the private link resource")
  createCmd.Flags().String("group-id", "", "Group ID of the private link service (e.g., sqlServer, blob)")
  createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
  createCmd.MarkFlagRequired("name")
  createCmd.MarkFlagRequired("resource-group")
  createCmd.MarkFlagRequired("location")
  createCmd.MarkFlagRequired("subnet-id")
  createCmd.MarkFlagRequired("private-link-resource-id")
  createCmd.MarkFlagRequired("group-id")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a private endpoint",
    RunE: func(cmd *cobra.Command, args []string) error {
      name, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Delete(context.Background(), name, resourceGroup, noWait)
    },
  }
  deleteCmd.Flags().StringP("name", "n", "", "Private endpoint name")
  deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  deleteCmd.MarkFlagRequired("name")
  deleteCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
  return cmd
}
