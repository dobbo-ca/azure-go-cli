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

  scaleCmd := &cobra.Command{
    Use:   "scale",
    Short: "Scale the number of nodes in a node pool",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      nodepoolName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      nodeCount, _ := cmd.Flags().GetInt32("node-count")
      return Scale(context.Background(), clusterName, nodepoolName, resourceGroup, nodeCount)
    },
  }
  scaleCmd.Flags().String("cluster-name", "", "AKS cluster name")
  scaleCmd.Flags().StringP("name", "n", "", "Node pool name")
  scaleCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  scaleCmd.Flags().Int32("node-count", 0, "Target number of nodes")
  scaleCmd.MarkFlagRequired("cluster-name")
  scaleCmd.MarkFlagRequired("name")
  scaleCmd.MarkFlagRequired("resource-group")
  scaleCmd.MarkFlagRequired("node-count")

  addCmd := &cobra.Command{
    Use:   "add",
    Short: "Add a new node pool to an AKS cluster",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      nodepoolName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      nodeCount, _ := cmd.Flags().GetInt32("node-count")
      vmSize, _ := cmd.Flags().GetString("node-vm-size")
      return Add(context.Background(), clusterName, nodepoolName, resourceGroup, nodeCount, vmSize)
    },
  }
  addCmd.Flags().String("cluster-name", "", "AKS cluster name")
  addCmd.Flags().StringP("name", "n", "", "Node pool name")
  addCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  addCmd.Flags().Int32("node-count", 3, "Number of nodes")
  addCmd.Flags().String("node-vm-size", "Standard_DS2_v2", "VM size for nodes")
  addCmd.MarkFlagRequired("cluster-name")
  addCmd.MarkFlagRequired("name")
  addCmd.MarkFlagRequired("resource-group")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a node pool from an AKS cluster",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      nodepoolName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return Delete(context.Background(), clusterName, nodepoolName, resourceGroup, noWait)
    },
  }
  deleteCmd.Flags().String("cluster-name", "", "AKS cluster name")
  deleteCmd.Flags().StringP("name", "n", "", "Node pool name")
  deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the delete operation to complete")
  deleteCmd.MarkFlagRequired("cluster-name")
  deleteCmd.MarkFlagRequired("name")
  deleteCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd, scaleCmd, addCmd, deleteCmd)
  return cmd
}
