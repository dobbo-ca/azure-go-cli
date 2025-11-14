package addon

import (
  "context"

  "github.com/spf13/cobra"
)

func NewAddonCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "addon",
    Short: "Manage Azure Kubernetes Service addons",
    Long:  "Commands to manage and view addons in AKS clusters",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List status of all addons in an AKS cluster",
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

  listAvailableCmd := &cobra.Command{
    Use:   "list-available",
    Short: "List available Kubernetes addons",
    RunE: func(cmd *cobra.Command, args []string) error {
      return ListAvailable()
    },
  }

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show status and configuration for an enabled addon",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      addonName, _ := cmd.Flags().GetString("name")
      return Show(context.Background(), clusterName, resourceGroup, addonName)
    },
  }
  showCmd.Flags().String("cluster-name", "", "AKS cluster name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.Flags().StringP("name", "n", "", "Addon name")
  showCmd.MarkFlagRequired("cluster-name")
  showCmd.MarkFlagRequired("resource-group")
  showCmd.MarkFlagRequired("name")

  enableCmd := &cobra.Command{
    Use:   "enable",
    Short: "Enable an addon in an AKS cluster",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      addonName, _ := cmd.Flags().GetString("name")
      return Enable(context.Background(), clusterName, resourceGroup, addonName, nil)
    },
  }
  enableCmd.Flags().String("cluster-name", "", "AKS cluster name")
  enableCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  enableCmd.Flags().StringP("name", "n", "", "Addon name")
  enableCmd.MarkFlagRequired("cluster-name")
  enableCmd.MarkFlagRequired("resource-group")
  enableCmd.MarkFlagRequired("name")

  disableCmd := &cobra.Command{
    Use:   "disable",
    Short: "Disable an addon in an AKS cluster",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("cluster-name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      addonName, _ := cmd.Flags().GetString("name")
      return Disable(context.Background(), clusterName, resourceGroup, addonName)
    },
  }
  disableCmd.Flags().String("cluster-name", "", "AKS cluster name")
  disableCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  disableCmd.Flags().StringP("name", "n", "", "Addon name")
  disableCmd.MarkFlagRequired("cluster-name")
  disableCmd.MarkFlagRequired("resource-group")
  disableCmd.MarkFlagRequired("name")

  cmd.AddCommand(listCmd, listAvailableCmd, showCmd, enableCmd, disableCmd)
  return cmd
}
