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

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
