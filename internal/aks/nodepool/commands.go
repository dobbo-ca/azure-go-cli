package nodepool

import (
  "context"

  "github.com/spf13/cobra"
)

func NewNodePoolCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "nodepool",
    Short: "Manage node pools in Azure Kubernetes Service",
    Long:  "Commands to manage node pools in managed Kubernetes clusters",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List node pools in an AKS cluster",
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
    Short: "Show details of a node pool",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      nodepoolName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), clusterName, nodepoolName, resourceGroup)
    },
  }
  showCmd.Flags().String("cluster-name", "", "AKS cluster name")
  showCmd.Flags().StringP("name", "n", "", "Node pool name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("cluster-name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
