package machine

import (
  "context"

  "github.com/spf13/cobra"
)

func NewMachineCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "machine",
    Short: "Get information about machines in a node pool",
    Long:  "Commands to get details about virtual machines in AKS node pools",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List machines in a node pool",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      nodepoolName, _ := cmd.Flags().GetString("nodepool-name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), clusterName, nodepoolName, resourceGroup)
    },
  }
  listCmd.Flags().String("cluster-name", "", "AKS cluster name")
  listCmd.Flags().String("nodepool-name", "", "Node pool name")
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  listCmd.MarkFlagRequired("cluster-name")
  listCmd.MarkFlagRequired("nodepool-name")
  listCmd.MarkFlagRequired("resource-group")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a specific machine",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      nodepoolName, _ := cmd.Flags().GetString("nodepool-name")
      machineName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), clusterName, nodepoolName, machineName, resourceGroup)
    },
  }
  showCmd.Flags().String("cluster-name", "", "AKS cluster name")
  showCmd.Flags().String("nodepool-name", "", "Node pool name")
  showCmd.Flags().StringP("name", "n", "", "Machine name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("cluster-name")
  showCmd.MarkFlagRequired("nodepool-name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
