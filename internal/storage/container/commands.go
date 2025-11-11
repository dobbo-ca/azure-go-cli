package container

import (
  "context"

  "github.com/spf13/cobra"
)

func NewContainerCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "container",
    Short: "Manage blob containers",
    Long:  "Commands to manage blob containers in storage accounts",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List blob containers in a storage account",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("account-name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return List(context.Background(), accountName, resourceGroup)
    },
  }
  listCmd.Flags().String("account-name", "", "Storage account name")
  listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  listCmd.MarkFlagRequired("account-name")
  listCmd.MarkFlagRequired("resource-group")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a blob container",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("account-name")
      containerName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Show(context.Background(), accountName, containerName, resourceGroup)
    },
  }
  showCmd.Flags().String("account-name", "", "Storage account name")
  showCmd.Flags().StringP("name", "n", "", "Container name")
  showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  showCmd.MarkFlagRequired("account-name")
  showCmd.MarkFlagRequired("name")
  showCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd)
  return cmd
}
