package blob

import (
  "context"

  "github.com/spf13/cobra"
)

func NewBlobCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "blob",
    Short: "Manage blob storage",
    Long:  "Commands to manage blobs in Azure storage containers",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List blobs in a container",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("account-name")
      containerName, _ := cmd.Flags().GetString("container-name")
      return List(context.Background(), accountName, containerName)
    },
  }
  listCmd.Flags().String("account-name", "", "Storage account name")
  listCmd.Flags().StringP("container-name", "c", "", "Container name")
  listCmd.MarkFlagRequired("account-name")
  listCmd.MarkFlagRequired("container-name")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a blob",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("account-name")
      containerName, _ := cmd.Flags().GetString("container-name")
      blobName, _ := cmd.Flags().GetString("name")
      return Show(context.Background(), cmd, accountName, containerName, blobName)
    },
  }
  showCmd.Flags().String("account-name", "", "Storage account name")
  showCmd.Flags().StringP("container-name", "c", "", "Container name")
  showCmd.Flags().StringP("name", "n", "", "Blob name")
  showCmd.MarkFlagRequired("account-name")
  showCmd.MarkFlagRequired("container-name")
  showCmd.MarkFlagRequired("name")

  uploadCmd := &cobra.Command{
    Use:   "upload",
    Short: "Upload a file to blob storage",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("account-name")
      containerName, _ := cmd.Flags().GetString("container-name")
      blobName, _ := cmd.Flags().GetString("name")
      filePath, _ := cmd.Flags().GetString("file")
      overwrite, _ := cmd.Flags().GetBool("overwrite")
      return Upload(context.Background(), accountName, containerName, blobName, filePath, overwrite)
    },
  }
  uploadCmd.Flags().String("account-name", "", "Storage account name")
  uploadCmd.Flags().StringP("container-name", "c", "", "Container name")
  uploadCmd.Flags().StringP("name", "n", "", "Blob name (defaults to filename if not specified)")
  uploadCmd.Flags().StringP("file", "f", "", "Path to file to upload")
  uploadCmd.Flags().Bool("overwrite", false, "Overwrite existing blob")
  uploadCmd.MarkFlagRequired("account-name")
  uploadCmd.MarkFlagRequired("container-name")
  uploadCmd.MarkFlagRequired("file")

  downloadCmd := &cobra.Command{
    Use:   "download",
    Short: "Download a blob from storage",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("account-name")
      containerName, _ := cmd.Flags().GetString("container-name")
      blobName, _ := cmd.Flags().GetString("name")
      filePath, _ := cmd.Flags().GetString("file")
      return Download(context.Background(), accountName, containerName, blobName, filePath)
    },
  }
  downloadCmd.Flags().String("account-name", "", "Storage account name")
  downloadCmd.Flags().StringP("container-name", "c", "", "Container name")
  downloadCmd.Flags().StringP("name", "n", "", "Blob name")
  downloadCmd.Flags().StringP("file", "f", "", "Path where to save the downloaded file")
  downloadCmd.MarkFlagRequired("account-name")
  downloadCmd.MarkFlagRequired("container-name")
  downloadCmd.MarkFlagRequired("name")
  downloadCmd.MarkFlagRequired("file")

  deleteCmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a blob",
    RunE: func(cmd *cobra.Command, args []string) error {
      accountName, _ := cmd.Flags().GetString("account-name")
      containerName, _ := cmd.Flags().GetString("container-name")
      blobName, _ := cmd.Flags().GetString("name")
      return Delete(context.Background(), accountName, containerName, blobName)
    },
  }
  deleteCmd.Flags().String("account-name", "", "Storage account name")
  deleteCmd.Flags().StringP("container-name", "c", "", "Container name")
  deleteCmd.Flags().StringP("name", "n", "", "Blob name")
  deleteCmd.MarkFlagRequired("account-name")
  deleteCmd.MarkFlagRequired("container-name")
  deleteCmd.MarkFlagRequired("name")

  cmd.AddCommand(listCmd, showCmd, uploadCmd, downloadCmd, deleteCmd)
  return cmd
}
