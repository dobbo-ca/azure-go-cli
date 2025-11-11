package podidentity

import (
  "context"

  "github.com/spf13/cobra"
)

func NewPodIdentityCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "pod-identity",
    Short: "Manage pod identities in AKS clusters",
    Long:  "Commands to manage pod identities in managed Kubernetes clusters",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List pod identities in an AKS cluster",
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

  cmd.AddCommand(listCmd)
  return cmd
}
