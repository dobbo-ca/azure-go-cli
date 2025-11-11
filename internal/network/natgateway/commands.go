package natgateway

import (
  "context"

  "github.com/spf13/cobra"
)

func NewNatGatewayCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "nat",
    Short: "Manage NAT gateways",
    Long:  "Commands to manage Azure NAT gateways",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List NAT gateways",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a NAT gateway",
    RunE: func(cmd *cobra.Command, args []string) error {
      gatewayName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), gatewayName, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "NAT gateway name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
