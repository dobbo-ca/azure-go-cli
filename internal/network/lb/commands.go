package lb

import (
  "context"

  "github.com/spf13/cobra"
)

func NewLoadBalancerCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "lb",
    Short: "Manage load balancers",
    Long:  "Commands to manage Azure load balancers",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List load balancers",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a load balancer",
    RunE: func(cmd *cobra.Command, args []string) error {
      lbName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), lbName, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "Load balancer name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
