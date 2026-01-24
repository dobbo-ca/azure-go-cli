package feature

import (
  "context"

  "github.com/spf13/cobra"
)

func NewFeatureCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "feat",
    Short: "Manage Azure preview features",
    Long:  "Commands to register, unregister, and view Azure preview features",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List Azure preview features",
    Long:  "List all preview features or features for a specific provider",
    RunE: func(cmd *cobra.Command, args []string) error {
      provider, _ := cmd.Flags().GetString("provider")
      return List(context.Background(), provider)
    },
  }
  listCmd.Flags().StringP("provider", "p", "", "Resource provider namespace (e.g., Microsoft.ContainerService)")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a specific feature",
    Long:  "Show details of a specific preview feature",
    RunE: func(cmd *cobra.Command, args []string) error {
      provider, _ := cmd.Flags().GetString("provider")
      name, _ := cmd.Flags().GetString("name")
      return Show(context.Background(), provider, name)
    },
  }
  showCmd.Flags().StringP("provider", "p", "", "Resource provider namespace (e.g., Microsoft.ContainerService)")
  showCmd.Flags().StringP("name", "n", "", "Feature name (e.g., EnableAPIServerVnetIntegrationPreview)")
  showCmd.MarkFlagRequired("provider")
  showCmd.MarkFlagRequired("name")

  registerCmd := &cobra.Command{
    Use:   "register",
    Short: "Register an Azure preview feature",
    Long:  "Enable a preview feature for your subscription",
    RunE: func(cmd *cobra.Command, args []string) error {
      provider, _ := cmd.Flags().GetString("provider")
      name, _ := cmd.Flags().GetString("name")
      return Register(context.Background(), provider, name)
    },
  }
  registerCmd.Flags().StringP("provider", "p", "", "Resource provider namespace (e.g., Microsoft.ContainerService)")
  registerCmd.Flags().StringP("name", "n", "", "Feature name (e.g., EnableAPIServerVnetIntegrationPreview)")
  registerCmd.MarkFlagRequired("provider")
  registerCmd.MarkFlagRequired("name")

  unregisterCmd := &cobra.Command{
    Use:   "unregister",
    Short: "Unregister an Azure preview feature",
    Long:  "Disable a preview feature for your subscription",
    RunE: func(cmd *cobra.Command, args []string) error {
      provider, _ := cmd.Flags().GetString("provider")
      name, _ := cmd.Flags().GetString("name")
      return Unregister(context.Background(), provider, name)
    },
  }
  unregisterCmd.Flags().StringP("provider", "p", "", "Resource provider namespace (e.g., Microsoft.ContainerService)")
  unregisterCmd.Flags().StringP("name", "n", "", "Feature name (e.g., EnableAPIServerVnetIntegrationPreview)")
  unregisterCmd.MarkFlagRequired("provider")
  unregisterCmd.MarkFlagRequired("name")

  cmd.AddCommand(listCmd, showCmd, registerCmd, unregisterCmd)
  return cmd
}
