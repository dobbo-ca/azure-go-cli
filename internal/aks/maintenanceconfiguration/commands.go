package maintenanceconfiguration

import (
  "context"

  "github.com/spf13/cobra"
)

func NewMaintenanceConfigurationCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "maintenanceconfiguration",
    Short: "Manage maintenance configurations in AKS clusters",
    Long:  "Commands to manage maintenance configurations in managed Kubernetes clusters",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List maintenance configurations in an AKS cluster",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), clusterName, resourceGroup)
    },
  }
  listCmd.Flags().String("cluster-name", "", "AKS cluster name")
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  listCmd.MarkFlagRequired("cluster-name")
  listCmd.MarkFlagRequired("resource-group")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a maintenance configuration",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      configName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), clusterName, configName, resourceGroup)
    },
  }
  showCmd.Flags().String("cluster-name", "", "AKS cluster name")
  showCmd.Flags().StringP("name", "n", "", "Maintenance configuration name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("cluster-name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
