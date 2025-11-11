package aks

import (
  "context"

  "github.com/cdobbyn/azure-go-cli/internal/aks/addon"
  "github.com/cdobbyn/azure-go-cli/internal/aks/machine"
  "github.com/cdobbyn/azure-go-cli/internal/aks/maintenanceconfiguration"
  "github.com/cdobbyn/azure-go-cli/internal/aks/nodepool"
  "github.com/cdobbyn/azure-go-cli/internal/aks/operation"
  "github.com/cdobbyn/azure-go-cli/internal/aks/podidentity"
  "github.com/cdobbyn/azure-go-cli/internal/aks/snapshot"
  "github.com/spf13/cobra"
)

func NewAKSCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "aks",
    Short: "Manage Azure Kubernetes Service",
    Long:  "Commands to manage Azure Kubernetes Service clusters",
  }

  getCredsCmd := &cobra.Command{
    Use:   "get-credentials",
    Short: "Get access credentials for a managed Kubernetes cluster",
    Long: `Get access credentials for a managed Kubernetes cluster.

By default, credentials are merged into ~/.kube/config. Use -f to specify a different file,
or use -f - to output to stdout.`,
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      admin, _ := cmd.Flags().GetBool("admin")
      file, _ := cmd.Flags().GetString("file")
      overwrite, _ := cmd.Flags().GetBool("overwrite-existing")
      contextName, _ := cmd.Flags().GetString("context")

      opts := GetCredentialsOptions{
        ClusterName:   clusterName,
        ResourceGroup: resourceGroup,
        Admin:         admin,
        File:          file,
        Overwrite:     overwrite,
        Context:       contextName,
      }

      return GetCredentials(context.Background(), opts)
    },
  }
  getCredsCmd.Flags().StringP("name", "n", "", "AKS cluster name")
  getCredsCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  getCredsCmd.Flags().BoolP("admin", "a", false, "Get admin credentials")
  getCredsCmd.Flags().StringP("file", "f", "", "Kubeconfig file path (use '-' for stdout, default: ~/.kube/config)")
  getCredsCmd.Flags().Bool("overwrite-existing", false, "Overwrite kubeconfig file instead of merging")
  getCredsCmd.Flags().String("context", "", "Set context name (only applicable with -f -)")
  getCredsCmd.MarkFlagRequired("name")
  getCredsCmd.MarkFlagRequired("resource-group")

  bastionCmd := &cobra.Command{
    Use:   "bastion",
    Short: "Open tunnel to AKS cluster through Azure Bastion",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      bastionResourceID, _ := cmd.Flags().GetString("bastion")
      subscription, _ := cmd.Flags().GetString("subscription")
      admin, _ := cmd.Flags().GetBool("admin")
      port, _ := cmd.Flags().GetInt("port")

      return Bastion(context.Background(), clusterName, resourceGroup, bastionResourceID, subscription, admin, port)
    },
  }
  bastionCmd.Flags().StringP("name", "n", "", "AKS cluster name")
  bastionCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  bastionCmd.Flags().String("bastion", "", "Bastion resource ID")
  bastionCmd.Flags().BoolP("admin", "a", false, "Use admin credentials")
  bastionCmd.Flags().IntP("port", "p", 8001, "Local port to use for tunnel")
  bastionCmd.MarkFlagRequired("name")
  bastionCmd.MarkFlagRequired("resource-group")
  bastionCmd.MarkFlagRequired("bastion")

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List AKS clusters",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), resourceGroup)
    },
  }
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of an AKS cluster",
    RunE: func(cmd *cobra.Command, args []string) error {
      clusterName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), clusterName, resourceGroup)
    },
  }
  showCmd.Flags().StringP("name", "n", "", "AKS cluster name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(
    getCredsCmd,
    bastionCmd,
    listCmd,
    showCmd,
    nodepool.NewNodePoolCommand(),
    addon.NewAddonCommand(),
    machine.NewMachineCommand(),
    maintenanceconfiguration.NewMaintenanceConfigurationCommand(),
    snapshot.NewSnapshotCommand(),
    operation.NewOperationCommand(),
    podidentity.NewPodIdentityCommand(),
  )
  return cmd
}
