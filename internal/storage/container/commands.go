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

  createCmd := &cobra.Command{
    Use:   "create",
    Short: "Create a blob container",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("account-name")
      containerName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      publicAccess, _ := cmd.Flags().GetString("public-access")
      metadata, _ := cmd.Flags().GetStringToString("metadata")
      return Create(context.Background(), cmd, accountName, containerName, resourceGroup, publicAccess, metadata)
    },
  }
  createCmd.Flags().String("account-name", "", "Storage account name")
  createCmd.Flags().StringP("name", "n", "", "Container name")
  createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  createCmd.Flags().String("public-access", "None", "Public access level: None, Container, or Blob")
  createCmd.Flags().StringToString("metadata", nil, "Space-separated metadata: key1=value1 key2=value2")
  createCmd.MarkFlagRequired("account-name")
  createCmd.MarkFlagRequired("name")
  createCmd.MarkFlagRequired("resource-group")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a blob container",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("account-name")
      containerName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return Delete(context.Background(), accountName, containerName, resourceGroup)
    },
  }
  deleteCmd.Flags().String("account-name", "", "Storage account name")
  deleteCmd.Flags().StringP("name", "n", "", "Container name")
  deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  deleteCmd.MarkFlagRequired("account-name")
  deleteCmd.MarkFlagRequired("name")
  deleteCmd.MarkFlagRequired("resource-group")

  cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
  return cmd
}
